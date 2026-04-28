package mse

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
	"sync"
	"sync/atomic"
	"time"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/conv"
	"github.com/mcnairstudios/mediahub/pkg/av/decode"
	"github.com/mcnairstudios/mediahub/pkg/av/encode"
	"github.com/mcnairstudios/mediahub/pkg/av/mux"
	"github.com/mcnairstudios/mediahub/pkg/av/resample"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/rs/zerolog"
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

	videoTB  astiav.Rational
	audioTB  astiav.Rational
	hasAudio bool

	audioDec      *decode.Decoder
	audioResample *resample.Resampler
	audioEnc      *encode.Encoder
	audioFifo     *encode.AudioFIFO
	audioLatched  bool

	mu           sync.Mutex
	bytesWritten int64
}

func New(cfg output.PluginConfig) (*Plugin, error) {
	segDir := filepath.Join(cfg.OutputDir, "segments")
	if err := os.MkdirAll(segDir, 0755); err != nil {
		return nil, fmt.Errorf("mse: create segments dir: %w", err)
	}

	log := zerolog.New(os.Stderr).With().Str("plugin", "mse").Logger()

	p := &Plugin{
		cfg:    cfg,
		segDir: segDir,
		log:    log,
	}
	p.generation.Store(1)
	p.videoTB = astiav.NewRational(1, 90000)
	p.audioTB = astiav.NewRational(1, 48000)

	muxOpts := mux.MuxOpts{
		OutputDir:     segDir,
		VideoTimeBase: p.videoTB,
	}

	if len(cfg.VideoExtradata) > 0 {
		muxOpts.VideoExtradata = cfg.VideoExtradata
	}
	if cfg.VideoCodecParams != nil {
		vcp := cfg.VideoCodecParams.(*astiav.CodecParameters)
		muxOpts.VideoCodecID = vcp.CodecID()
		if len(muxOpts.VideoExtradata) == 0 {
			muxOpts.VideoExtradata = vcp.ExtraData()
		}
		if len(muxOpts.VideoExtradata) == 0 && cfg.Video != nil {
			muxOpts.VideoExtradata = cfg.Video.Extradata
		}
		muxOpts.VideoWidth = vcp.Width()
		muxOpts.VideoHeight = vcp.Height()
	} else if cfg.Video != nil {
		codecID, err := conv.CodecIDFromString(cfg.Video.Codec)
		if err != nil {
			return nil, fmt.Errorf("mse: video codec: %w", err)
		}
		muxOpts.VideoCodecID = codecID
		if len(muxOpts.VideoExtradata) == 0 {
			muxOpts.VideoExtradata = cfg.Video.Extradata
		}
		muxOpts.VideoWidth = cfg.Video.Width
		muxOpts.VideoHeight = cfg.Video.Height
	}

	if len(cfg.AudioExtradata) > 0 {
		if cfg.AudioCodecParams != nil {
			acp := cfg.AudioCodecParams.(*astiav.CodecParameters)
			muxOpts.AudioCodecID = acp.CodecID()
			muxOpts.AudioExtradata = cfg.AudioExtradata
			muxOpts.AudioChannels = acp.ChannelLayout().Channels()
			muxOpts.AudioSampleRate = acp.SampleRate()
			p.hasAudio = true
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
		}
	}

	if muxOpts.VideoCodecID != astiav.CodecIDNone || muxOpts.AudioCodecID != astiav.CodecIDNone {
		m, err := mux.NewFragmentedMuxer(muxOpts)
		if err != nil {
			return nil, fmt.Errorf("mse: create muxer: %w", err)
		}
		p.muxer = m
	}

	p.watcher = newWatcher(segDir)

	return p, nil
}

func (p *Plugin) Mode() output.DeliveryMode {
	return output.DeliveryMSE
}

func (p *Plugin) PushVideo(data []byte, pts, dts int64, keyframe bool) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: mse PushVideo: %v\n%s", r, debug.Stack())
			retErr = fmt.Errorf("mse: PushVideo panic: %v", r)
		}
	}()

	if p.stopped.Load() || p.muxer == nil {
		return nil
	}

	pkt := &av.Packet{Type: av.Video, Data: data, PTS: pts, DTS: dts, Keyframe: keyframe}
	avPkt, err := conv.ToAVPacket(pkt, p.videoTB)
	if err != nil {
		return err
	}
	defer avPkt.Free()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.bytesWritten += int64(len(data))
	return p.muxer.WriteVideoPacket(avPkt)
}

func (p *Plugin) PushAudio(data []byte, pts, dts int64) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: mse PushAudio: %v\n%s", r, debug.Stack())
			retErr = fmt.Errorf("mse: PushAudio panic: %v", r)
		}
	}()

	if p.stopped.Load() || !p.hasAudio || p.muxer == nil || p.audioLatched {
		return nil
	}

	if p.audioDec != nil {
		return p.pushAudioDecode(data, pts, dts)
	}

	pkt := &av.Packet{Type: av.Audio, Data: data, PTS: pts, DTS: dts}
	avPkt, err := conv.ToAVPacket(pkt, p.audioTB)
	if err != nil {
		return err
	}
	defer avPkt.Free()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.bytesWritten += int64(len(data))
	return p.muxer.WriteAudioPacket(avPkt)
}

func (p *Plugin) pushAudioDecode(data []byte, pts, dts int64) error {
	pkt := &av.Packet{Type: av.Audio, Data: data, PTS: pts, DTS: dts}
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
		p.log.Warn().Err(err).Msg("mse audio decode error latched")
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
		}
		encPkts, err := p.audioFifo.Write(outFrame)
		outFrame.Free()
		if err != nil {
			p.audioLatched = true
			p.log.Warn().Err(err).Msg("mse audio encode error latched")
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
		Mode:         output.DeliveryMSE,
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
	path := r.URL.Path

	switch path {
	case "/video/init":
		p.serveInit(w, p.watcher.VideoInit, "video/mp4")
	case "/audio/init":
		p.serveInit(w, p.watcher.AudioInit, "video/mp4")
	case "/video/segment":
		p.serveSegment(w, r, p.watcher.VideoSegment, "video/mp4")
	case "/audio/segment":
		p.serveSegment(w, r, p.watcher.AudioSegment, "video/mp4")
	case "/debug":
		p.serveDebug(w)
	default:
		http.NotFound(w, r)
	}
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

func (p *Plugin) serveSegment(w http.ResponseWriter, r *http.Request, getter func(int) ([]byte, bool), contentType string) {
	seqStr := r.URL.Query().Get("seq")
	genStr := r.URL.Query().Get("gen")

	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if genStr != "" {
		gen, err := strconv.ParseInt(genStr, 10, 64)
		if err == nil && gen != p.generation.Load() {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusGone)
			return
		}
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

	w.Header().Set("Content-Type", contentType)
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
