package dash

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/bsf"
	"github.com/mcnairstudios/mediahub/pkg/av/conv"
	"github.com/mcnairstudios/mediahub/pkg/av/decode"
	"github.com/mcnairstudios/mediahub/pkg/av/encode"
	"github.com/mcnairstudios/mediahub/pkg/av/extradata"
	"github.com/mcnairstudios/mediahub/pkg/av/mux"
	"github.com/mcnairstudios/mediahub/pkg/av/resample"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/rs/zerolog"
)

const (
	defaultSegmentDuration = 6
	videoTimescale         = 90000
	audioTimescale         = 48000
)

type Plugin struct {
	cfg    output.PluginConfig
	segDir string
	log    zerolog.Logger

	muxer   *mux.FragmentedMuxer
	watcher *watcher

	generation atomic.Int64
	stopped    atomic.Bool
	eos        atomic.Bool

	videoTB            astiav.Rational
	audioTB            astiav.Rational
	needsAnnexBConvert bool
	hasAudio           bool
	deferredMuxOpts    *mux.MuxOpts

	audioDec      *decode.Decoder
	audioResample *resample.Resampler
	audioEnc      *encode.Encoder
	audioFifo     *encode.AudioFIFO
	audioLatched  bool

	startTime time.Time

	videoWidth      int
	videoHeight     int
	videoBandwidth  int
	audioSampleRate int
	audioChannels   int
	audioBandwidth  int
	videoCodecStr   string
	audioCodecStr   string

	mu           sync.Mutex
	bytesWritten int64
}

func New(cfg output.PluginConfig) (*Plugin, error) {
	segDir := filepath.Join(cfg.OutputDir, "segments")
	os.RemoveAll(segDir)
	if err := os.MkdirAll(segDir, 0755); err != nil {
		return nil, fmt.Errorf("dash: create segments dir: %w", err)
	}

	log := zerolog.New(os.Stderr).With().Str("plugin", "dash").Logger()

	p := &Plugin{
		cfg:       cfg,
		segDir:    segDir,
		log:       log,
		startTime: time.Now().UTC(),
	}
	p.generation.Store(1)
	p.videoTB = astiav.NewRational(1, videoTimescale)
	p.audioTB = astiav.NewRational(1, audioTimescale)

	muxOpts := mux.MuxOpts{
		OutputDir:      segDir,
		VideoTimeBase:  p.videoTB,
		VideoFrameRate: 25, // Default
		AudioFrameSize: 1024, // Default for AAC
	}

	if cfg.Video != nil && cfg.Video.FramerateN > 0 && cfg.Video.FramerateD > 0 {
		muxOpts.VideoFrameRate = cfg.Video.FramerateN / cfg.Video.FramerateD
	}

	if len(cfg.VideoExtradata) > 0 {
		muxOpts.VideoExtradata = cfg.VideoExtradata
	}
	if cfg.VideoCodecParams != nil {
		vcp, _ := cfg.VideoCodecParams.(*astiav.CodecParameters)
		if vcp != nil {
			muxOpts.VideoCodecID = vcp.CodecID()
			if len(muxOpts.VideoExtradata) == 0 {
				muxOpts.VideoExtradata = vcp.ExtraData()
			}
			if len(muxOpts.VideoExtradata) == 0 && cfg.Video != nil {
				muxOpts.VideoExtradata = cfg.Video.Extradata
			}
			muxOpts.VideoWidth = vcp.Width()
			muxOpts.VideoHeight = vcp.Height()
		}
	}
	if muxOpts.VideoCodecID == 0 && cfg.Video != nil {
		codecID, err := conv.CodecIDFromString(cfg.Video.Codec)
		if err != nil {
			return nil, fmt.Errorf("dash: video codec: %w", err)
		}
		muxOpts.VideoCodecID = codecID
		if len(muxOpts.VideoExtradata) == 0 {
			muxOpts.VideoExtradata = cfg.Video.Extradata
		}
		muxOpts.VideoWidth = cfg.Video.Width
		muxOpts.VideoHeight = cfg.Video.Height
	}

	p.videoWidth = muxOpts.VideoWidth
	p.videoHeight = muxOpts.VideoHeight
	p.videoBandwidth = 2000000
	if cfg.Video != nil {
		p.videoCodecStr = cfg.Video.Codec
	}

	if len(cfg.AudioExtradata) > 0 {
		acp, _ := cfg.AudioCodecParams.(*astiav.CodecParameters)
		if acp != nil {
			muxOpts.AudioCodecID = acp.CodecID()
			muxOpts.AudioExtradata = cfg.AudioExtradata
			muxOpts.AudioChannels = acp.ChannelLayout().Channels()
			muxOpts.AudioSampleRate = acp.SampleRate()
			p.hasAudio = true
			p.audioSampleRate = acp.SampleRate()
			p.audioChannels = acp.ChannelLayout().Channels()
		} else if cfg.Audio != nil {
			audioCodecID, err := conv.CodecIDFromString(cfg.Audio.Codec)
			if err != nil {
				log.Warn().Err(err).Msg("unknown audio codec, video-only")
			} else {
				muxOpts.AudioCodecID = audioCodecID
				muxOpts.AudioExtradata = cfg.AudioExtradata
				muxOpts.AudioChannels = cfg.Audio.Channels
				muxOpts.AudioSampleRate = cfg.Audio.SampleRate
				p.hasAudio = true
				p.audioSampleRate = cfg.Audio.SampleRate
				p.audioChannels = cfg.Audio.Channels
			}
		}
	} else if cfg.AudioCodecParams != nil || cfg.Audio != nil {
		var decErr error
		if cfg.AudioCodecParams != nil {
			acp := cfg.AudioCodecParams.(*astiav.CodecParameters)
			p.audioDec, decErr = decode.NewAudioDecoderFromParams(acp)
			if decErr != nil {
				log.Warn().Err(decErr).Msg("audio decoder init failed, video-only")
				p.audioDec = nil
			}
		} else if cfg.Audio != nil {
			audioCodecID, cerr := conv.CodecIDFromString(cfg.Audio.Codec)
			if cerr != nil {
				log.Warn().Err(cerr).Msg("unknown audio codec, video-only")
			} else {
				p.audioDec, decErr = decode.NewAudioDecoder(audioCodecID, nil)
				if decErr != nil {
					log.Warn().Err(decErr).Msg("audio decoder init failed, video-only")
					p.audioDec = nil
				}
			}
		}

		if p.audioDec != nil {
			srcChannels := 2
			srcRate := 48000
			if cfg.Audio != nil {
				if cfg.Audio.Channels > 0 {
					srcChannels = cfg.Audio.Channels
				}
				if cfg.Audio.SampleRate > 0 {
					srcRate = cfg.Audio.SampleRate
				}
			} else if cfg.AudioCodecParams != nil {
				acp := cfg.AudioCodecParams.(*astiav.CodecParameters)
				if acp.ChannelLayout().Channels() > 0 {
					srcChannels = acp.ChannelLayout().Channels()
				}
				if acp.SampleRate() > 0 {
					srcRate = acp.SampleRate()
				}
			}
			p.audioResample, decErr = resample.NewResampler(
				srcChannels, srcRate, astiav.SampleFormatFltp,
				2, 48000, astiav.SampleFormatFltp,
			)
			if decErr != nil {
				p.audioDec.Close()
				p.audioDec = nil
				log.Warn().Err(decErr).Msg("resampler init failed, video-only")
			}
		}

		if p.audioDec != nil {
			encName := encode.ResolveAudioEncoderName("aac")
			p.audioEnc, decErr = encode.NewAudioEncoder(encode.AudioEncodeOpts{
				Codec: encName, Channels: 2, SampleRate: 48000,
			})
			if decErr != nil {
				if p.audioResample != nil {
					p.audioResample.Close()
				}
				p.audioDec.Close()
				p.audioDec = nil
				log.Warn().Err(decErr).Msg("audio encoder init failed, video-only")
			} else {
				p.audioFifo = encode.NewAudioFIFOFromEncoder(p.audioEnc, 2, astiav.ChannelLayoutStereo, 48000)
			}
		}

		if p.audioEnc != nil {
			muxOpts.AudioCodecID = astiav.CodecIDAac
			muxOpts.AudioChannels = 2
			muxOpts.AudioSampleRate = 48000
			muxOpts.AudioExtradata = p.audioEnc.Extradata()
			p.hasAudio = true
			p.audioSampleRate = 48000
			p.audioChannels = 2
		}
	}

	p.audioBandwidth = 128000
	if cfg.Audio != nil && cfg.Audio.BitRate > 0 {
		p.audioBandwidth = cfg.Audio.BitRate
	}
	p.audioCodecStr = "mp4a.40.2"

	needsDeferred := len(muxOpts.VideoExtradata) == 0 &&
		muxOpts.VideoCodecID != astiav.CodecIDNone &&
		(muxOpts.VideoCodecID == astiav.CodecIDHevc || muxOpts.VideoCodecID == astiav.CodecIDH264)

	if needsDeferred {
		p.needsAnnexBConvert = true
		saved := muxOpts
		p.deferredMuxOpts = &saved
		log.Info().Str("codec_id", muxOpts.VideoCodecID.String()).Msg("dash: deferring muxer creation until first keyframe provides extradata")
	} else if muxOpts.VideoCodecID != astiav.CodecIDNone || muxOpts.AudioCodecID != astiav.CodecIDNone {
		m, err := mux.NewFragmentedMuxer(muxOpts)
		if err != nil {
			return nil, fmt.Errorf("dash: create muxer: %w", err)
		}
		p.muxer = m
	}

	p.watcher = newWatcher(segDir)

	return p, nil
}

func (p *Plugin) initDeferredMuxer(keyframeData []byte) error {
	opts := p.deferredMuxOpts
	p.deferredMuxOpts = nil

	codecName := "hevc"
	if opts.VideoCodecID == astiav.CodecIDH264 {
		codecName = "h264"
	}

	var converted []byte
	ext, bsfErr := bsf.NewExtraDataExtractor(opts.VideoCodecID, p.videoTB)
	if bsfErr == nil {
		defer ext.Close()
		pkt := astiav.AllocPacket()
		if pkt != nil {
			defer pkt.Free()
			if err := pkt.FromData(keyframeData); err == nil {
				pkt.SetPts(0)
				pkt.SetDts(0)
				pkt.SetFlags(astiav.NewPacketFlags(astiav.PacketFlagKey))
				annexB, err := ext.ProcessPacket(pkt)
				if err == nil && len(annexB) > 0 {
					converted, _ = extradata.ToCodecData(codecName, annexB)
				}
			}
		}
	}

	if len(converted) == 0 {
		var err error
		converted, err = extradata.ToCodecData(codecName, keyframeData)
		if err != nil {
			return fmt.Errorf("extract extradata from keyframe: %w", err)
		}
	}

	if len(converted) == 0 {
		return fmt.Errorf("no extradata found in first %s keyframe", codecName)
	}

	opts.VideoExtradata = converted
	p.log.Info().Str("codec", codecName).Int("extradata_bytes", len(converted)).Msg("dash: extracted extradata from first keyframe, creating muxer")

	m, err := mux.NewFragmentedMuxer(*opts)
	if err != nil {
		return fmt.Errorf("create deferred muxer: %w", err)
	}
	p.muxer = m
	return nil
}

func (p *Plugin) Mode() output.DeliveryMode {
	return output.DeliveryDASH
}

func (p *Plugin) PushVideo(data []byte, pts, dts, duration int64, keyframe bool) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: dash PushVideo: %v\n%s", r, debug.Stack())
			retErr = fmt.Errorf("dash: PushVideo panic: %v", r)
		}
	}()

	if p.stopped.Load() {
		return nil
	}

	if p.deferredMuxOpts != nil && keyframe {
		if err := p.initDeferredMuxer(data); err != nil {
			p.log.Error().Err(err).Msg("dash: failed to init deferred muxer")
			return nil
		}
	}

	if p.muxer == nil {
		return nil
	}

	pkt := &av.Packet{Type: av.Video, Data: data, PTS: pts, DTS: dts, Duration: duration, Keyframe: keyframe}
	avPkt, err := conv.ToAVPacket(pkt, p.videoTB)
	if err != nil {
		return err
	}
	defer avPkt.Free()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.bytesWritten += int64(len(data))
	if err := p.muxer.WriteVideoPacket(avPkt); err != nil {
		log.Printf("dash: skip corrupt video packet (pts=%d dts=%d): %v", pts, dts, err)
		return nil
	}
	return nil
}

func (p *Plugin) PushAudio(data []byte, pts, dts, duration int64) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: dash PushAudio: %v\n%s", r, debug.Stack())
			retErr = fmt.Errorf("dash: PushAudio panic: %v", r)
		}
	}()

	if p.stopped.Load() || !p.hasAudio || p.muxer == nil || p.audioLatched {
		return nil
	}

	if p.audioDec != nil {
		return p.pushAudioDecode(data, pts, dts, duration)
	}

	pkt := &av.Packet{Type: av.Audio, Data: data, PTS: pts, DTS: dts, Duration: duration}
	avPkt, err := conv.ToAVPacket(pkt, p.audioTB)
	if err != nil {
		return err
	}
	defer avPkt.Free()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.bytesWritten += int64(len(data))
	if err := p.muxer.WriteAudioPacket(avPkt); err != nil {
		log.Printf("dash: skip corrupt audio packet (pts=%d dts=%d): %v", pts, dts, err)
		return nil
	}
	return nil
}

func (p *Plugin) pushAudioDecode(data []byte, pts, dts, duration int64) error {
	pkt := &av.Packet{Type: av.Audio, Data: data, PTS: pts, DTS: dts, Duration: duration}
	avPkt, err := conv.ToAVPacket(pkt, p.audioTB)
	if err != nil {
		p.audioLatched = true
		return nil
	}
	frames, err := p.audioDec.Decode(avPkt)
	avPkt.Free()
	if err != nil {
		for _, f := range frames {
			f.Free()
		}
		p.audioLatched = true
		p.log.Warn().Err(err).Msg("dash audio decode error latched")
		return nil
	}
	for _, frame := range frames {
		outFrame := frame
		if p.audioResample != nil {
			outFrame, err = p.audioResample.Convert(frame)
			frame.Free()
			if err != nil {
				p.audioLatched = true
				return nil
			}
			if outFrame == nil {
				continue
			}
		}
		encPkts, err := p.audioFifo.Write(outFrame)
		outFrame.Free()
		if err != nil {
			p.audioLatched = true
			p.log.Warn().Err(err).Msg("dash audio encode error latched")
			return nil
		}
		p.mu.Lock()
		for _, ep := range encPkts {
			if writeErr := p.muxer.WriteAudioPacket(ep); writeErr != nil {
				ep.Free()
				p.mu.Unlock()
				p.audioLatched = true
				return nil
			}
			p.bytesWritten += int64(ep.Size())
			ep.Free()
		}
		p.mu.Unlock()
	}
	return nil
}

func (p *Plugin) PushSubtitle(_ []byte, _ int64, _ int64) error {
	return nil
}

func (p *Plugin) EndOfStream() {
	p.eos.Store(true)
	p.closeAudioChain()
	if p.muxer != nil {
		p.muxer.Close()
	}
}

func (p *Plugin) ResetForSeek() {
	p.generation.Add(1)
	p.startTime = time.Now().UTC()
	if p.watcher != nil {
		p.watcher.Reset()
	}
	if p.muxer != nil {
		p.muxer.Reset()
	}
}

func (p *Plugin) closeAudioChain() {
	if p.audioFifo != nil {
		p.audioFifo.Close()
		p.audioFifo = nil
	}
	if p.audioEnc != nil {
		p.audioEnc.Close()
		p.audioEnc = nil
	}
	if p.audioResample != nil {
		p.audioResample.Close()
		p.audioResample = nil
	}
	if p.audioDec != nil {
		p.audioDec.Close()
		p.audioDec = nil
	}
}

func (p *Plugin) Stop() {
	if p.stopped.Swap(true) {
		return
	}
	p.closeAudioChain()
	if p.watcher != nil {
		p.watcher.Close()
	}
	if p.muxer != nil {
		p.muxer.Close()
	}
}

func (p *Plugin) Status() output.PluginStatus {
	videoCount := 0
	audioCount := 0
	if p.watcher != nil {
		videoCount = p.watcher.videoSegs.Count()
		audioCount = p.watcher.audioSegs.Count()
	}

	p.mu.Lock()
	bw := p.bytesWritten
	p.mu.Unlock()

	return output.PluginStatus{
		Mode:         output.DeliveryDASH,
		SegmentCount: videoCount + audioCount,
		BytesWritten: bw,
		Healthy:      !p.eos.Load() && !p.stopped.Load(),
	}
}

func (p *Plugin) Generation() int64 {
	return p.generation.Load()
}

func (p *Plugin) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if p.watcher.VideoInit() != nil {
				return nil
			}
		}
	}
}

func (p *Plugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := r.URL.Path

	switch {
	case path == "/manifest.mpd":
		p.serveManifest(w)
	case path == "/init-video.mp4":
		p.serveInit(w, p.watcher.VideoInit, "video/mp4")
	case path == "/init-audio.mp4":
		p.serveInit(w, p.watcher.AudioInit, "video/mp4")
	case strings.HasPrefix(path, "/video/") && strings.HasSuffix(path, ".m4s"):
		p.serveMediaSegment(w, r, path, p.watcher.VideoSegment)
	case strings.HasPrefix(path, "/audio/") && strings.HasSuffix(path, ".m4s"):
		p.serveMediaSegment(w, r, path, p.watcher.AudioSegment)
	case path == "/debug":
		p.serveDebug(w)
	default:
		http.NotFound(w, r)
	}
}

func (p *Plugin) serveManifest(w http.ResponseWriter) {
	segDur := defaultSegmentDuration
	videoDuration := segDur * videoTimescale
	audioDuration := segDur * audioTimescale

	videoCount := p.watcher.videoSegs.Count()

	var videoCodec string
	if p.muxer != nil {
		videoCodec = p.muxer.VideoCodecString()
	}
	if videoCodec == "" {
		videoCodec = "avc1.640028"
	}

	isLive := p.cfg.IsLive || !p.eos.Load()
	mpdType := "dynamic"
	if !isLive {
		mpdType = "static"
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")

	if isLive {
		b.WriteString(fmt.Sprintf(
			`<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="%s" minimumUpdatePeriod="PT2S" availabilityStartTime="%s" minBufferTime="PT%dS" profiles="urn:mpeg:dash:profile:isoff-live:2011">`,
			mpdType, p.startTime.Format(time.RFC3339), segDur,
		))
	} else {
		totalDur := time.Duration(videoCount*segDur) * time.Second
		b.WriteString(fmt.Sprintf(
			`<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="%s" mediaPresentationDuration="%s" minBufferTime="PT%dS" profiles="urn:mpeg:dash:profile:isoff-on-demand:2011">`,
			mpdType, formatDuration(totalDur), segDur,
		))
	}
	b.WriteString("\n<Period>\n")

	b.WriteString(fmt.Sprintf(
		`<AdaptationSet mimeType="video/mp4" contentType="video" segmentAlignment="true" startWithSAP="1">`+"\n"+
			`<Representation id="video" bandwidth="%d" width="%d" height="%d" codecs="%s">`+"\n"+
			`<SegmentTemplate media="video/$Number$.m4s" initialization="init-video.mp4" startNumber="1" timescale="%d" duration="%d"/>`+"\n"+
			`</Representation>`+"\n"+
			`</AdaptationSet>`+"\n",
		p.videoBandwidth, p.videoWidth, p.videoHeight, videoCodec,
		videoTimescale, videoDuration,
	))

	if p.hasAudio {
		b.WriteString(fmt.Sprintf(
			`<AdaptationSet mimeType="audio/mp4" contentType="audio" segmentAlignment="true" startWithSAP="1">`+"\n"+
				`<Representation id="audio" bandwidth="%d" audioSamplingRate="%d" codecs="%s">`+"\n"+
				`<AudioChannelConfiguration schemeIdUri="urn:mpeg:dash:23003:3:audio_channel_configuration:2011" value="%d"/>`+"\n"+
				`<SegmentTemplate media="audio/$Number$.m4s" initialization="init-audio.mp4" startNumber="1" timescale="%d" duration="%d"/>`+"\n"+
				`</Representation>`+"\n"+
				`</AdaptationSet>`+"\n",
			p.audioBandwidth, p.audioSampleRate, p.audioCodecStr,
			p.audioChannels, audioTimescale, audioDuration,
		))
	}

	b.WriteString("</Period>\n</MPD>\n")

	w.Header().Set("Content-Type", "application/dash+xml")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(b.String()))
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("PT%dH%dM%dS", h, m, s)
}

func (p *Plugin) serveInit(w http.ResponseWriter, getter func() []byte, contentType string) {
	data := getter()
	if data == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}

func (p *Plugin) serveMediaSegment(w http.ResponseWriter, _ *http.Request, path string, getter func(int) ([]byte, bool)) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	filename := parts[len(parts)-1]
	seqStr := strings.TrimSuffix(filename, ".m4s")
	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	deadline := time.Now().Add(5 * time.Second)
	var data []byte
	var ok bool
	for time.Now().Before(deadline) {
		data, ok = getter(seq)
		if ok {
			break
		}
		if p.stopped.Load() || p.eos.Load() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !ok {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}

func (p *Plugin) serveDebug(w http.ResponseWriter) {
	info := map[string]interface{}{
		"generation":     p.generation.Load(),
		"video_segments": p.watcher.videoSegs.Count(),
		"audio_segments": p.watcher.audioSegs.Count(),
		"has_video_init": p.watcher.VideoInit() != nil,
		"has_audio_init": p.watcher.AudioInit() != nil,
		"stopped":        p.stopped.Load(),
		"eos":            p.eos.Load(),
	}
	if p.muxer != nil {
		info["codec_string"] = p.muxer.VideoCodecString()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
