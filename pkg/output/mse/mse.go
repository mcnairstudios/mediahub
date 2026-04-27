package mse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/conv"
	"github.com/mcnairstudios/mediahub/pkg/av/mux"
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

	videoTB astiav.Rational
	audioTB astiav.Rational

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

	if cfg.Video != nil {
		codecID, err := conv.CodecIDFromString(cfg.Video.Codec)
		if err != nil {
			return nil, fmt.Errorf("mse: video codec: %w", err)
		}
		muxOpts.VideoCodecID = codecID
		muxOpts.VideoExtradata = cfg.Video.Extradata
		muxOpts.VideoWidth = cfg.Video.Width
		muxOpts.VideoHeight = cfg.Video.Height
	}

	if cfg.Audio != nil {
		audioCodecID, err := conv.CodecIDFromString(cfg.Audio.Codec)
		if err != nil {
			log.Warn().Err(err).Msg("unknown audio codec, video-only")
		} else {
			muxOpts.AudioCodecID = audioCodecID
			muxOpts.AudioChannels = cfg.Audio.Channels
			muxOpts.AudioSampleRate = cfg.Audio.SampleRate
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

func (p *Plugin) PushVideo(data []byte, pts, dts int64, keyframe bool) error {
	if p.stopped.Load() {
		return nil
	}
	if p.muxer == nil {
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

func (p *Plugin) PushAudio(data []byte, pts, dts int64) error {
	if p.stopped.Load() {
		return nil
	}
	if p.muxer == nil {
		return nil
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

func (p *Plugin) PushSubtitle(_ []byte, _ int64, _ int64) error {
	return nil
}

func (p *Plugin) EndOfStream() {
	p.eos.Store(true)
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

func (p *Plugin) Stop() {
	if p.stopped.Swap(true) {
		return
	}
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
		if err == nil && gen < p.generation.Load() {
			w.WriteHeader(http.StatusGone)
			return
		}
	}

	data, ok := getter(seq)
	if !ok {
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
