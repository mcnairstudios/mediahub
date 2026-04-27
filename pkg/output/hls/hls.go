package hls

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/conv"
	"github.com/mcnairstudios/mediahub/pkg/av/mux"
	"github.com/mcnairstudios/mediahub/pkg/output"
)

type Plugin struct {
	muxer      *mux.HLSMuxer
	segDir     string
	videoTB    astiav.Rational
	audioTB    astiav.Rational
	hasAudio   bool
	generation atomic.Int64
	stopped    bool
	mu         sync.Mutex
	lastErr    error
}

func New(cfg output.PluginConfig) (*Plugin, error) {
	if cfg.OutputDir == "" {
		return nil, errors.New("hls: OutputDir is required")
	}
	if cfg.Video == nil {
		return nil, errors.New("hls: Video info is required")
	}

	segDir := filepath.Join(cfg.OutputDir, "segments")
	if err := os.MkdirAll(segDir, 0755); err != nil {
		return nil, fmt.Errorf("hls: create segments dir: %w", err)
	}
	resolvedSegDir, err := filepath.EvalSymlinks(segDir)
	if err != nil {
		return nil, fmt.Errorf("hls: resolve segments dir: %w", err)
	}
	segDir = resolvedSegDir

	videoCodecID, err := conv.CodecIDFromString(cfg.Video.Codec)
	if err != nil {
		return nil, fmt.Errorf("hls: video codec: %w", err)
	}

	videoFPS := 25
	if cfg.Video.FramerateN > 0 && cfg.Video.FramerateD > 0 {
		videoFPS = cfg.Video.FramerateN / cfg.Video.FramerateD
		if videoFPS <= 0 {
			videoFPS = 25
		}
	}

	segDur := cfg.SegmentDurationSec
	if segDur <= 0 {
		segDur = 6
	}

	hlsOpts := mux.HLSMuxOpts{
		OutputDir:          segDir,
		SegmentDurationSec: segDur,
		VideoCodecID:       videoCodecID,
		VideoExtradata:     cfg.Video.Extradata,
		VideoWidth:         cfg.Video.Width,
		VideoHeight:        cfg.Video.Height,
		VideoTimeBase:      astiav.NewRational(1, 90000),
		VideoFrameRate:     videoFPS,
	}

	if cfg.Audio != nil {
		audioCodecID, err := conv.CodecIDFromString(cfg.Audio.Codec)
		if err != nil {
			return nil, fmt.Errorf("hls: audio codec: %w", err)
		}

		sampleRate := cfg.Audio.SampleRate
		if sampleRate <= 0 {
			sampleRate = 48000
		}

		hlsOpts.AudioCodecID = audioCodecID
		hlsOpts.AudioChannels = cfg.Audio.Channels
		hlsOpts.AudioSampleRate = sampleRate
		hlsOpts.AudioTimeBase = astiav.NewRational(1, sampleRate)
		hlsOpts.AudioFrameSize = 1024
	}

	muxer, err := mux.NewHLSMuxer(hlsOpts)
	if err != nil {
		return nil, fmt.Errorf("hls: create muxer: %w", err)
	}

	videoTB := astiav.NewRational(1, 90000)
	audioTB := astiav.NewRational(1, 48000)
	if cfg.Audio != nil && cfg.Audio.SampleRate > 0 {
		audioTB = astiav.NewRational(1, cfg.Audio.SampleRate)
	}

	p := &Plugin{
		muxer:    muxer,
		segDir:   segDir,
		videoTB:  videoTB,
		audioTB:  audioTB,
		hasAudio: cfg.Audio != nil,
	}
	p.generation.Store(1)

	return p, nil
}

func (p *Plugin) Mode() output.DeliveryMode {
	return output.DeliveryHLS
}

func (p *Plugin) PushVideo(data []byte, pts, dts int64, keyframe bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped || p.muxer == nil {
		return nil
	}

	pkt := &av.Packet{Type: av.Video, Data: data, PTS: pts, DTS: dts, Keyframe: keyframe}
	avPkt, err := conv.ToAVPacket(pkt, p.videoTB)
	if err != nil {
		p.lastErr = err
		return err
	}
	err = p.muxer.WriteVideoPacket(avPkt)
	avPkt.Free()
	if err != nil {
		p.lastErr = err
	}
	return err
}

func (p *Plugin) PushAudio(data []byte, pts, dts int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped || p.muxer == nil || !p.hasAudio {
		return nil
	}

	pkt := &av.Packet{Type: av.Audio, Data: data, PTS: pts, DTS: dts}
	avPkt, err := conv.ToAVPacket(pkt, p.audioTB)
	if err != nil {
		p.lastErr = err
		return err
	}
	err = p.muxer.WriteAudioPacket(avPkt)
	avPkt.Free()
	if err != nil {
		p.lastErr = err
	}
	return err
}

func (p *Plugin) PushSubtitle(_ []byte, _ int64, _ int64) error {
	return nil
}

func (p *Plugin) EndOfStream() {
	p.Stop()
}

func (p *Plugin) ResetForSeek() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}

	p.generation.Add(1)
	if p.muxer != nil {
		p.muxer.Reset() //nolint:errcheck
	}
}

func (p *Plugin) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}
	p.stopped = true

	if p.muxer != nil {
		p.muxer.Close() //nolint:errcheck
	}
}

func (p *Plugin) Status() output.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	segCount := 0
	if p.muxer != nil {
		segCount = p.muxer.SegmentCount()
	}

	errStr := ""
	if p.lastErr != nil {
		errStr = p.lastErr.Error()
	}

	return output.PluginStatus{
		Mode:         output.DeliveryHLS,
		SegmentCount: segCount,
		Healthy:      !p.stopped,
		Error:        errStr,
	}
}

func (p *Plugin) Generation() int64 {
	return p.generation.Load()
}

func (p *Plugin) WaitReady(ctx context.Context) error {
	playlistPath := filepath.Join(p.segDir, "playlist.m3u8")

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := os.Stat(playlistPath); err == nil {
				return nil
			}
		}
	}
}

func (p *Plugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := r.URL.Path

	if path == "/playlist.m3u8" || path == "playlist.m3u8" {
		p.servePlaylist(w, r)
		return
	}

	if strings.HasSuffix(path, ".ts") {
		p.serveSegment(w, r, path)
		return
	}

	http.NotFound(w, r)
}

func (p *Plugin) servePlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := p.WaitReady(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	playlistPath := filepath.Join(p.segDir, "playlist.m3u8")
	data, err := os.ReadFile(playlistPath)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Write(data) //nolint:errcheck
}

func (p *Plugin) serveSegment(w http.ResponseWriter, _ *http.Request, path string) {
	name := filepath.Base(path)

	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, string(os.PathSeparator)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	cleaned := filepath.Clean(name)
	if cleaned != name || strings.Contains(cleaned, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	segPath := filepath.Join(p.segDir, name)

	resolved, err := filepath.EvalSymlinks(segPath)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	if !strings.HasPrefix(resolved, p.segDir) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	data, err := os.ReadFile(segPath)
	if err != nil {
		http.NotFound(w, nil)
		return
	}

	w.Header().Set("Content-Type", "video/mp2t")
	w.Write(data) //nolint:errcheck
}
