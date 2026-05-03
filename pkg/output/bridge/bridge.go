package bridge

import (
	"errors"
	"fmt"
	"sync"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/bsf"
	"github.com/mcnairstudios/mediahub/pkg/av/conv"
	"github.com/mcnairstudios/mediahub/pkg/av/decode"
	"github.com/mcnairstudios/mediahub/pkg/av/encode"
	"github.com/mcnairstudios/mediahub/pkg/av/extradata"
	"github.com/mcnairstudios/mediahub/pkg/av/filter"
	"github.com/mcnairstudios/mediahub/pkg/av/resample"
	"github.com/mcnairstudios/mediahub/pkg/av/scale"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/rs/zerolog"
)

var (
	ErrNoDownstream  = errors.New("bridge: downstream PacketSink is required")
	ErrNoProbeResult = errors.New("bridge: ProbeResult is required")
	ErrNoVideoInfo   = errors.New("bridge: ProbeResult.Video is required")
)

type Config struct {
	Downstream       av.PacketSink
	Info             *media.ProbeResult
	AudioIndex       int
	HWAccel          string
	DecodeHWAccel    string
	OutputCodec      string
	OutputAudioCodec string
	Bitrate          int
	OutputHeight     int
	MaxBitDepth      int
	Deinterlace      bool
	EncoderName      string
	DecoderName      string
	Framerate        int
	VideoCodecParams any
	AudioCodecParams any
	AudioOnly        bool
	Preset           string
	Log              zerolog.Logger
}

type Bridge struct {
	downstream           av.PacketSink
	videoDec             *decode.Decoder
	videoEnc             *encode.Encoder
	deint                *filter.Deinterlacer
	scaler               *scale.Scaler
	audioDec             *decode.Decoder
	audioResample        *resample.Resampler
	audioEnc             *encode.Encoder
	audioFifo            *encode.AudioFIFO
	videoTB              astiav.Rational
	audioTB              astiav.Rational
	audioLatched         bool
	audioErrorCount      int
	audioPacketCount     int
	stopped              bool
	videoEncoderExtradata []byte
	extractedExtradata   bool
	bsfExtractor         *bsf.ExtraDataExtractor
	hwVideoPath          bool
	hwDeintPending       bool
	mu                   sync.Mutex
	log                  zerolog.Logger
}

func New(cfg Config) (*Bridge, error) {
	if cfg.Downstream == nil {
		return nil, ErrNoDownstream
	}
	if cfg.Info == nil {
		return nil, ErrNoProbeResult
	}
	if cfg.Info.Video == nil && !cfg.AudioOnly {
		return nil, ErrNoVideoInfo
	}

	info := cfg.Info
	b := &Bridge{
		downstream: cfg.Downstream,
		videoTB:    astiav.NewRational(1, 90000),
		audioTB:    astiav.NewRational(1, 48000),
		log:        cfg.Log,
	}

	var err error

	if !cfg.AudioOnly && info.Video != nil {
		decHW := cfg.DecodeHWAccel
		if decHW == "" {
			decHW = cfg.HWAccel
		}

		useHWVideoPath := (cfg.HWAccel == "videotoolbox") && decHW != "" && decHW != "none"
		canPassInterlaced := cfg.HWAccel == "videotoolbox" || cfg.HWAccel == "vaapi"
		needsDeinterlace := (cfg.Deinterlace || info.Video.Interlaced) && !canPassInterlaced

		if cp, ok := cfg.VideoCodecParams.(*astiav.CodecParameters); ok && cp != nil {
			b.videoDec, err = decode.NewVideoDecoderFromParams(cp, decode.DecodeOpts{
				HWAccel:      decHW,
				MaxBitDepth:  cfg.MaxBitDepth,
				DecoderName:  cfg.DecoderName,
				KeepHWFrames: useHWVideoPath,
			})
		} else {
			videoCodecID, cerr := conv.CodecIDFromString(info.Video.Codec)
			if cerr != nil {
				return nil, fmt.Errorf("bridge: video codec ID: %w", cerr)
			}
			b.videoDec, err = decode.NewVideoDecoder(videoCodecID, info.Video.Extradata, decode.DecodeOpts{
				HWAccel:      decHW,
				MaxBitDepth:  cfg.MaxBitDepth,
				DecoderName:  cfg.DecoderName,
				KeepHWFrames: useHWVideoPath,
			})
		}
		if err != nil {
			return nil, fmt.Errorf("bridge: video decoder: %w", err)
		}

		if useHWVideoPath && b.videoDec.HWDeviceContext() != nil {
			b.hwVideoPath = true
			if needsDeinterlace {
				b.hwDeintPending = true
			}
			cfg.Log.Info().Msg("bridge: GPU video path enabled (zero CPU copies)")
		}

		if !b.hwVideoPath {
			srcPixFmt := astiav.PixelFormatYuv420P
			needsBitDepthConversion := cfg.MaxBitDepth > 0 && info.Video.BitDepth > cfg.MaxBitDepth
			if needsBitDepthConversion {
				srcPixFmt = astiav.PixelFormatYuv420P10Le
			}

			if needsDeinterlace {
				b.deint, err = filter.NewDeinterlacer(
					info.Video.Width, info.Video.Height,
					srcPixFmt,
					astiav.NewRational(1, 90000),
				)
				if err != nil {
					b.closeAll()
					return nil, fmt.Errorf("bridge: deinterlacer: %w", err)
				}
			}

			outW, outH, needsResolutionScale := resolveOutputDimensions(info.Video.Width, info.Video.Height, cfg.OutputHeight)
			needsPixFmtConversion := cfg.HWAccel == "videotoolbox" || cfg.HWAccel == "vaapi"
			outPixFmt := astiav.PixelFormatYuv420P
			if needsPixFmtConversion {
				outPixFmt = astiav.PixelFormatNv12
			}
			if needsResolutionScale || needsBitDepthConversion || needsPixFmtConversion {
				b.scaler, err = scale.NewScaler(
					info.Video.Width, info.Video.Height, srcPixFmt,
					outW, outH, outPixFmt,
				)
				if err != nil {
					b.closeAll()
					return nil, fmt.Errorf("bridge: scaler: %w", err)
				}
			}
		}

		outW, outH, _ := resolveOutputDimensions(info.Video.Width, info.Video.Height, cfg.OutputHeight)

		outCodec := cfg.OutputCodec
		if outCodec == "" {
			outCodec = "h264"
		}

		videoFPS := resolveFramerate(info.Video.FramerateN, info.Video.FramerateD, info.Video.Interlaced, needsDeinterlace, cfg.Framerate)

		preset := cfg.Preset
		if preset == "" {
			preset = "ultrafast"
		}
		passInterlaced := info.Video.Interlaced && !needsDeinterlace
		b.videoEnc, err = encode.NewVideoEncoder(encode.EncodeOpts{
			Codec:         outCodec,
			HWAccel:       cfg.HWAccel,
			Bitrate:       cfg.Bitrate,
			Width:         outW,
			Height:        outH,
			EncoderName:   cfg.EncoderName,
			Framerate:     videoFPS,
			Preset:        preset,
			Interlaced:    passInterlaced,
			InputTimeBase: b.videoTB,
		})
		if err != nil {
			b.closeAll()
			return nil, fmt.Errorf("bridge: video encoder: %w", err)
		}

		cfg.Log.Info().Int("width", outW).Int("height", outH).Int("fps", videoFPS).Str("codec", outCodec).Str("hwaccel", cfg.HWAccel).Msg("bridge: video encoder created")
		if ed := b.videoEnc.Extradata(); len(ed) > 0 {
			b.videoEncoderExtradata = make([]byte, len(ed))
			copy(b.videoEncoderExtradata, ed)
			b.extractedExtradata = true
			cfg.Log.Info().Int("extradata_len", len(ed)).Msg("bridge: encoder extradata (Annex B, ffmpeg muxer will convert)")
		}

		if !b.extractedExtradata {
			encCodecID := b.videoEnc.CodecID()
			if encCodecID == astiav.CodecIDH264 || encCodecID == astiav.CodecIDHevc {
				b.bsfExtractor, err = bsf.NewExtraDataExtractor(encCodecID, b.videoTB)
				if err != nil {
					cfg.Log.Warn().Err(err).Msg("bridge: BSF extractor init failed, will fall back to manual extraction")
				}
			}
		}
	}

	var audioTrack *media.AudioTrack
	for i := range info.AudioTracks {
		if info.AudioTracks[i].Index == cfg.AudioIndex {
			audioTrack = &info.AudioTracks[i]
			break
		}
	}

	if audioTrack != nil {
		if cp, ok := cfg.AudioCodecParams.(*astiav.CodecParameters); ok && cp != nil {
			b.audioDec, err = decode.NewAudioDecoderFromParams(cp)
		} else {
			audioCodecID, cerr := conv.CodecIDFromString(audioTrack.Codec)
			if cerr != nil {
				b.closeAll()
				return nil, fmt.Errorf("bridge: audio codec ID: %w", cerr)
			}
			b.audioDec, err = decode.NewAudioDecoder(audioCodecID, nil)
		}
		if err != nil {
			b.closeAll()
			return nil, fmt.Errorf("bridge: audio decoder: %w", err)
		}

		b.audioResample, err = resample.NewResampler(
			audioTrack.Channels, audioTrack.SampleRate, astiav.SampleFormatFltp,
			2, 48000, astiav.SampleFormatFltp,
		)
		if err != nil {
			b.closeAll()
			return nil, fmt.Errorf("bridge: audio resampler: %w", err)
		}

		audioCodecName := cfg.OutputAudioCodec
		if audioCodecName == "" {
			audioCodecName = "aac"
		}
		encName := encode.ResolveAudioEncoderName(audioCodecName)
		b.audioEnc, err = encode.NewAudioEncoder(encode.AudioEncodeOpts{
			Codec: encName, Channels: 2, SampleRate: 48000,
		})
		if err != nil {
			b.closeAll()
			return nil, fmt.Errorf("bridge: audio encoder (%s): %w", audioCodecName, err)
		}

		b.audioFifo = encode.NewAudioFIFOFromEncoder(b.audioEnc, 2, astiav.ChannelLayoutStereo, 48000)
	}

	return b, nil
}

func (b *Bridge) VideoEncoderExtradata() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.videoEncoderExtradata) > 0 {
		return b.videoEncoderExtradata
	}
	if b.videoEnc == nil {
		return nil
	}
	return b.videoEnc.Extradata()
}

func (b *Bridge) VideoEncoderCodecID() astiav.CodecID {
	if b.videoEnc == nil {
		return astiav.CodecIDNone
	}
	return b.videoEnc.CodecID()
}

func (b *Bridge) AudioEncoderExtradata() []byte {
	if b.audioEnc == nil {
		return nil
	}
	return b.audioEnc.Extradata()
}

func (b *Bridge) AudioEncoderCodecID() astiav.CodecID {
	if b.audioEnc == nil {
		return astiav.CodecIDNone
	}
	return b.audioEnc.CodecID()
}

func (b *Bridge) PushVideo(data []byte, pts, dts int64, keyframe bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return nil
	}
	if b.videoDec == nil {
		return b.downstream.PushVideo(data, pts, dts, keyframe)
	}

	pkt := &av.Packet{Type: av.Video, Data: data, PTS: pts, DTS: dts, Keyframe: keyframe}
	avPkt, err := conv.ToAVPacket(pkt, b.videoTB)
	if err != nil {
		return err
	}

	frames, err := b.videoDec.Decode(avPkt)
	avPkt.Free()
	if err != nil {
		for _, f := range frames {
			f.Free()
		}
		return fmt.Errorf("bridge: video decode: %w", err)
	}

	for i, frame := range frames {
		decFrame := frame

		if b.hwVideoPath && b.hwDeintPending && b.deint == nil {
			hwFramesCtx := frame.HardwareFramesContext()
			hwDevCtx := b.videoDec.HWDeviceContext()
			if hwFramesCtx != nil && hwDevCtx != nil {
				b.deint, err = filter.NewDeinterlacerWithOpts(filter.DeinterlaceOpts{
					Width:       frame.Width(),
					Height:      frame.Height(),
					PixFmt:      frame.PixelFormat(),
					TimeBase:    astiav.NewRational(1, 90000),
					HWAccel:     "videotoolbox",
					HWDeviceCtx: hwDevCtx,
					HWFramesCtx: hwFramesCtx,
				})
				if err != nil {
					b.log.Warn().Err(err).Msg("bridge: HW deinterlacer init failed, disabling deinterlace")
					b.hwDeintPending = false
				} else {
					b.hwDeintPending = false
				}
			}
		}

		if b.deint != nil {
			frame, err = b.deint.Process(frame)
			decFrame.Free()
			if err != nil {
				for _, f := range frames[i+1:] {
					f.Free()
				}
				return fmt.Errorf("bridge: deinterlace: %w", err)
			}
			if frame == nil {
				continue
			}
			decFrame = frame
		}
		if b.scaler != nil {
			frame, err = b.scaler.Scale(frame)
			decFrame.Free()
			if err != nil {
				for _, f := range frames[i+1:] {
					f.Free()
				}
				return fmt.Errorf("bridge: scale: %w", err)
			}
		}
		if !b.extractedExtradata {
			b.log.Info().Int64("frame_pts", frame.Pts()).Str("pixfmt", frame.PixelFormat().Name()).Bool("hw_path", b.hwVideoPath).Msg("bridge: first frames before encode")
		}
		encPkts, err := b.videoEnc.Encode(frame)
		frame.Free()
		if err != nil {
			for _, f := range frames[i+1:] {
				f.Free()
			}
			return fmt.Errorf("bridge: video encode: %w", err)
		}
		for _, encPkt := range encPkts {
			encData := make([]byte, encPkt.Size())
			copy(encData, encPkt.Data())
			encPTS := b.avTSToNanos(encPkt.Pts(), b.videoTB)
			encDTS := b.avTSToNanos(encPkt.Dts(), b.videoTB)
			if !b.extractedExtradata {
				b.log.Info().Int64("raw_pts", encPkt.Pts()).Int64("raw_dts", encPkt.Dts()).Int64("raw_dur", encPkt.Duration()).Int64("nanos_pts", encPTS).Int64("nanos_dts", encDTS).Str("enc_tb", b.videoEnc.TimeBase().String()).Msg("bridge: encoded pkt PTS")
			}
			isKey := encPkt.Flags().Has(astiav.PacketFlagKey)

			if !b.extractedExtradata && b.videoEnc != nil {
				b.extractVideoExtradata(encPkt, encData)
			}
			encPkt.Free()

			if err := b.downstream.PushVideo(encData, encPTS, encDTS, isKey); err != nil {
				for _, f := range frames[i+1:] {
					f.Free()
				}
				return fmt.Errorf("bridge: push video downstream: %w", err)
			}
		}
	}
	return nil
}

func (b *Bridge) PushAudio(data []byte, pts, dts int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped || b.audioLatched {
		return nil
	}

	if b.audioDec == nil {
		return nil
	}

	b.audioPacketCount++

	pkt := &av.Packet{Type: av.Audio, Data: data, PTS: pts, DTS: dts}
	avPkt, err := conv.ToAVPacket(pkt, b.audioTB)
	if err != nil {
		if b.audioPacketCount <= 5 {
			b.log.Debug().Int("pkt", b.audioPacketCount).Msg("bridge: skipping initial audio packet error")
			return nil
		}
		b.latchAudioError(err)
		return nil
	}

	frames, err := b.audioDec.Decode(avPkt)
	avPkt.Free()
	if err != nil {
		if b.audioPacketCount <= 5 {
			b.log.Debug().Int("pkt", b.audioPacketCount).Msg("bridge: skipping initial audio decode error")
			return nil
		}
		b.latchAudioError(err)
		return nil
	}

	for _, frame := range frames {
		outFrame := frame
		if b.audioResample != nil {
			outFrame, err = b.audioResample.Convert(frame)
			frame.Free()
			if err != nil {
				b.latchAudioError(err)
				return nil
			}
			if outFrame == nil {
				continue
			}
		}
		encPkts, err := b.audioFifo.Write(outFrame)
		outFrame.Free()
		if err != nil {
			b.latchAudioError(err)
			return nil
		}
		for _, encPkt := range encPkts {
			encData := make([]byte, encPkt.Size())
			copy(encData, encPkt.Data())
			encPTS := b.avTSToNanos(encPkt.Pts(), b.audioTB)
			encDTS := b.avTSToNanos(encPkt.Dts(), b.audioTB)
			encPkt.Free()
			if err := b.downstream.PushAudio(encData, encPTS, encDTS); err != nil {
				b.latchAudioError(err)
				return nil
			}
		}
	}
	if b.audioErrorCount > 0 {
		b.audioErrorCount = 0
	}
	return nil
}

func (b *Bridge) PushSubtitle(data []byte, pts int64, duration int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return nil
	}
	return b.downstream.PushSubtitle(data, pts, duration)
}

func (b *Bridge) EndOfStream() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return
	}

	if b.videoEnc != nil {
		if pkts, err := b.videoEnc.Flush(); err == nil {
			for _, encPkt := range pkts {
				encData := make([]byte, encPkt.Size())
				copy(encData, encPkt.Data())
				encPTS := b.avTSToNanos(encPkt.Pts(), b.videoTB)
				encDTS := b.avTSToNanos(encPkt.Dts(), b.videoTB)
				isKey := encPkt.Flags().Has(astiav.PacketFlagKey)
				encPkt.Free()
				b.downstream.PushVideo(encData, encPTS, encDTS, isKey) //nolint:errcheck
			}
		}
	}

	if b.audioEnc != nil && !b.audioLatched {
		if b.audioResample != nil {
			flushed, err := b.audioResample.Flush()
			if err == nil {
				for _, frame := range flushed {
					if b.audioFifo != nil {
						pkts, fifoErr := b.audioFifo.Write(frame)
						frame.Free()
						if fifoErr != nil {
							break
						}
						b.pushAudioPackets(pkts)
					} else {
						frame.Free()
					}
				}
			}
		}

		if b.audioFifo != nil {
			if pkts, err := b.audioFifo.Flush(); err == nil {
				b.pushAudioPackets(pkts)
			}
		}

		if pkts, err := b.audioEnc.Flush(); err == nil {
			b.pushAudioPackets(pkts)
		}
	}

	b.downstream.EndOfStream()
}

func (b *Bridge) pushAudioPackets(pkts []*astiav.Packet) {
	for _, encPkt := range pkts {
		encData := make([]byte, encPkt.Size())
		copy(encData, encPkt.Data())
		encPTS := b.avTSToNanos(encPkt.Pts(), b.audioTB)
		encDTS := b.avTSToNanos(encPkt.Dts(), b.audioTB)
		encPkt.Free()
		b.downstream.PushAudio(encData, encPTS, encDTS) //nolint:errcheck
	}
}

type seekResetter interface {
	ResetForSeek()
}

func (b *Bridge) ResetForSeek() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.videoDec != nil {
		b.videoDec.FlushBuffers()
	}
	if b.audioDec != nil {
		b.audioDec.FlushBuffers()
	}
	if b.audioResample != nil {
		b.audioResample.Reset()
	}
	if b.audioFifo != nil {
		b.audioFifo.Reset()
	}
	b.audioLatched = false
	b.audioPacketCount = 0
	b.audioErrorCount = 0
	if sr, ok := b.downstream.(seekResetter); ok {
		sr.ResetForSeek()
	}
}

func (b *Bridge) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return
	}
	b.stopped = true
	b.closeAll()
}

func (b *Bridge) closeAll() {
	b.closeBSFExtractor()
	if b.audioFifo != nil {
		b.audioFifo.Close()
	}
	if b.videoEnc != nil {
		b.videoEnc.Close()
	}
	if b.scaler != nil {
		b.scaler.Close()
	}
	if b.deint != nil {
		b.deint.Close()
	}
	if b.videoDec != nil {
		b.videoDec.Close()
	}
	if b.audioEnc != nil {
		b.audioEnc.Close()
	}
	if b.audioResample != nil {
		b.audioResample.Close()
	}
	if b.audioDec != nil {
		b.audioDec.Close()
	}
}

func (b *Bridge) extractVideoExtradata(encPkt *astiav.Packet, packetData []byte) {
	if ed := b.videoEnc.Extradata(); len(ed) > 0 {
		b.extractedExtradata = true
		b.videoEncoderExtradata = make([]byte, len(ed))
		copy(b.videoEncoderExtradata, ed)
		b.closeBSFExtractor()
		return
	}

	codecID := b.videoEnc.CodecID()
	if codecID != astiav.CodecIDHevc && codecID != astiav.CodecIDH264 {
		b.extractedExtradata = true
		b.closeBSFExtractor()
		return
	}

	codecName := "h264"
	if codecID == astiav.CodecIDHevc {
		codecName = "hevc"
	}

	if b.bsfExtractor != nil {
		annexB, err := b.bsfExtractor.ProcessPacket(encPkt)
		if err != nil {
			b.log.Warn().Err(err).Str("codec", codecName).Msg("bridge: BSF extraction failed, trying manual fallback")
		} else if len(annexB) > 0 {
			converted, cerr := extradata.ToCodecData(codecName, annexB)
			if cerr == nil && len(converted) > 0 {
				b.extractedExtradata = true
				b.videoEncoderExtradata = converted
				b.log.Info().Str("codec", codecName).Int("bytes", len(converted)).Msg("bridge: extracted encoder extradata via BSF")
				b.closeBSFExtractor()
				return
			}
			if cerr != nil {
				b.log.Warn().Err(cerr).Str("codec", codecName).Msg("bridge: BSF extradata conversion failed, trying manual fallback")
			}
		}
	}

	converted, err := extradata.ToCodecData(codecName, packetData)
	if err != nil {
		b.log.Warn().Err(err).Str("codec", codecName).Msg("bridge: failed to extract extradata from first keyframe")
		return
	}
	if len(converted) > 0 {
		b.extractedExtradata = true
		b.videoEncoderExtradata = converted
		b.log.Info().Str("codec", codecName).Int("bytes", len(converted)).Msg("bridge: extracted encoder extradata from first keyframe")
		b.closeBSFExtractor()
	}
}

func (b *Bridge) closeBSFExtractor() {
	if b.bsfExtractor != nil {
		b.bsfExtractor.Close()
		b.bsfExtractor = nil
	}
}

func (b *Bridge) latchAudioError(err error) {
	b.audioErrorCount++
	if b.audioErrorCount <= 100 {
		if b.audioErrorCount <= 5 || b.audioErrorCount%10 == 0 {
			b.log.Warn().Err(err).Int("errors", b.audioErrorCount).Msg("bridge: audio decode error (will keep trying)")
		}
		return
	}
	if !b.audioLatched {
		b.audioLatched = true
		b.log.Error().Err(err).Int("errors", b.audioErrorCount).Msg("bridge: audio error latched — video continues")
	}
}

func (b *Bridge) avTSToNanos(ts int64, tb astiav.Rational) int64 {
	return ts * 1_000_000_000 * int64(tb.Num()) / int64(tb.Den())
}

func resolveFramerate(framerateN, framerateD int, interlaced, hasDeinterlacer bool, explicit int) int {
	if explicit > 0 {
		return explicit
	}
	fps := 25
	if framerateN > 0 && framerateD > 0 {
		fps = framerateN / framerateD
		if fps <= 0 {
			fps = 25
		}
	}
	if interlaced && fps > 25 {
		fps = fps / 2
	}
	return fps
}

func resolveOutputDimensions(srcW, srcH, outputHeight int) (int, int, bool) {
	if outputHeight <= 0 || outputHeight >= srcH {
		return srcW, srcH, false
	}
	outH := outputHeight
	outW := srcW * outputHeight / srcH
	outW = outW &^ 1
	return outW, outH, true
}
