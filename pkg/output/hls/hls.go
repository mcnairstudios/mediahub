package hls

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/bsf"
	"github.com/mcnairstudios/mediahub/pkg/av/conv"
	"github.com/mcnairstudios/mediahub/pkg/av/extradata"
	"github.com/mcnairstudios/mediahub/pkg/av/mux"
	"github.com/mcnairstudios/mediahub/pkg/output"
)

type Plugin struct {
	muxer      *mux.HLSMuxer
	segDir     string
	videoTB    astiav.Rational
	audioTB    astiav.Rational
	hasVideo   bool
	hasAudio   bool
	generation atomic.Int64
	stopped    bool
	mu         sync.Mutex
	lastErr    error

	// Deferred muxer creation: when encoder extradata isn't available at
	// init time (e.g. VT H.265), defer muxer creation until the first
	// keyframe provides extradata via BSF extraction.
	deferredOpts *mux.HLSMuxOpts
}

func New(cfg output.PluginConfig) (*Plugin, error) {
	if cfg.OutputDir == "" {
		return nil, errors.New("hls: OutputDir is required")
	}
	hasVideo := cfg.Video != nil || cfg.VideoCodecParams != nil
	hasAudio := cfg.Audio != nil || cfg.AudioCodecParams != nil
	if !hasVideo && !hasAudio {
		return nil, errors.New("hls: at least one of Video or Audio must be configured")
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

	segDur := cfg.SegmentDurationSec
	if segDur <= 0 {
		segDur = 6
	}

	segType := "mpegts"
	if cfg.Video != nil {
		vc := strings.ToLower(cfg.Video.Codec)
		if vc == "h265" || vc == "hevc" {
			segType = "fmp4"
		}
	}
	if cfg.OutputFormat == "fmp4" {
		segType = "fmp4"
	}
	if v, ok := cfg.Options["segment_type"]; ok {
		if st, ok := v.(string); ok && (st == "fmp4" || st == "mpegts") {
			segType = st
		}
	}

	hlsOpts := mux.HLSMuxOpts{
		OutputDir:          segDir,
		SegmentDurationSec: segDur,
		SegmentType:        segType,
		IsLive:             cfg.IsLive,
		CopyVideoParams:    cfg.CopyVideoParams,
		CopyAudioParams:    cfg.CopyAudioParams,
	}

	if hasVideo {
		hlsOpts.VideoTimeBase = astiav.NewRational(1, 90000)
	}

	if cfg.VideoCodecParams != nil {
		vcp := cfg.VideoCodecParams.(*astiav.CodecParameters)
		hlsOpts.VideoCodecID = vcp.CodecID()
		hlsOpts.VideoExtradata = vcp.ExtraData()
		hlsOpts.VideoWidth = vcp.Width()
		hlsOpts.VideoHeight = vcp.Height()
		hlsOpts.VideoFrameRate = 25
	} else if cfg.Video != nil {
		videoCodecID, err := conv.CodecIDFromString(cfg.Video.Codec)
		if err != nil {
			return nil, fmt.Errorf("hls: video codec: %w", err)
		}
		hlsOpts.VideoCodecID = videoCodecID
		hlsOpts.VideoExtradata = cfg.Video.Extradata
		hlsOpts.VideoWidth = cfg.Video.Width
		hlsOpts.VideoHeight = cfg.Video.Height
		hlsOpts.VideoFrameRate = 25
	}

	// Fallback: use encoder extradata from pluginCfg when Video.Extradata is empty
	// (transcoding path: orchestrator sets VideoExtradata from bridge encoder)
	if len(hlsOpts.VideoExtradata) == 0 && len(cfg.VideoExtradata) > 0 {
		hlsOpts.VideoExtradata = cfg.VideoExtradata
	}

	if cfg.Video != nil && cfg.Video.FramerateN > 0 && cfg.Video.FramerateD > 0 {
		fps := cfg.Video.FramerateN / cfg.Video.FramerateD
		if fps > 0 {
			hlsOpts.VideoFrameRate = fps
		}
	}

	if cfg.AudioCodecParams != nil {
		acp := cfg.AudioCodecParams.(*astiav.CodecParameters)
		hlsOpts.AudioCodecID = acp.CodecID()
		hlsOpts.AudioExtradata = acp.ExtraData()
		sampleRate := acp.SampleRate()
		if sampleRate <= 0 {
			sampleRate = 48000
		}
		hlsOpts.AudioChannels = acp.ChannelLayout().Channels()
		hlsOpts.AudioSampleRate = sampleRate
		hlsOpts.AudioTimeBase = astiav.NewRational(1, sampleRate)
		hlsOpts.AudioFrameSize = 1024
	} else if cfg.Audio != nil {
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

	videoTB := astiav.NewRational(1, 90000)
	audioTB := astiav.NewRational(1, 48000)
	if cfg.Audio != nil && cfg.Audio.SampleRate > 0 {
		audioTB = astiav.NewRational(1, cfg.Audio.SampleRate)
	}

	p := &Plugin{
		segDir:   segDir,
		videoTB:  videoTB,
		audioTB:  audioTB,
		hasVideo: hasVideo,
		hasAudio: hasAudio,
	}
	p.generation.Store(1)

	// Defer muxer creation if video extradata is missing (VT H.265 encoder
	// doesn't provide extradata at init time — needs first keyframe).
	needsDeferred := len(hlsOpts.VideoExtradata) == 0 &&
		hlsOpts.VideoCodecID != astiav.CodecIDNone &&
		(hlsOpts.VideoCodecID == astiav.CodecIDHevc || hlsOpts.VideoCodecID == astiav.CodecIDH264)

	if needsDeferred {
		saved := hlsOpts
		p.deferredOpts = &saved
		log.Printf("hls: deferring muxer creation until first keyframe provides extradata (codec=%s)", hlsOpts.VideoCodecID)
	} else {
		muxer, err := mux.NewHLSMuxer(hlsOpts)
		if err != nil {
			return nil, fmt.Errorf("hls: create muxer: %w", err)
		}
		p.muxer = muxer
	}

	return p, nil
}

func (p *Plugin) initDeferredMuxer(keyframeData []byte) error {
	opts := p.deferredOpts
	p.deferredOpts = nil

	codecName := "hevc"
	if opts.VideoCodecID == astiav.CodecIDH264 {
		codecName = "h264"
	}

	// Try BSF extraction first (more reliable for Annex-B → hvcC/avcC conversion)
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

	// Fallback: try direct extraction from keyframe data
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
	log.Printf("hls: extracted extradata from first keyframe (%s, %d bytes), creating muxer", codecName, len(converted))

	muxer, err := mux.NewHLSMuxer(*opts)
	if err != nil {
		return fmt.Errorf("hls: create deferred muxer: %w", err)
	}
	p.muxer = muxer
	return nil
}

func (p *Plugin) Mode() output.DeliveryMode {
	return output.DeliveryHLS
}

func (p *Plugin) PushVideo(data []byte, pts, dts, duration int64, keyframe bool) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: hls PushVideo: %v\n%s", r, debug.Stack())
			retErr = fmt.Errorf("hls: PushVideo panic: %v", r)
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped || !p.hasVideo {
		return nil
	}

	// Deferred muxer creation: extract extradata from first keyframe
	if p.deferredOpts != nil && keyframe {
		if err := p.initDeferredMuxer(data); err != nil {
			log.Printf("hls: deferred muxer init failed: %v", err)
			return nil
		}
	}

	if p.muxer == nil {
		return nil
	}

	pkt := &av.Packet{Type: av.Video, Data: data, PTS: pts, DTS: dts, Duration: duration, Keyframe: keyframe}
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

func (p *Plugin) PushAudio(data []byte, pts, dts, duration int64) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: hls PushAudio: %v\n%s", r, debug.Stack())
			retErr = fmt.Errorf("hls: PushAudio panic: %v", r)
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped || p.muxer == nil || !p.hasAudio {
		return nil
	}

	pkt := &av.Packet{Type: av.Audio, Data: data, PTS: pts, DTS: dts, Duration: duration}
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
	// HLS muxer cannot be reset mid-stream; segments continue.
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

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			data, err := os.ReadFile(playlistPath)
			if err != nil {
				continue
			}
			if strings.Contains(string(data), "#EXTINF:") {
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

	if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".m4s") || strings.HasSuffix(path, ".mp4") {
		p.serveSegment(w, r, path)
		return
	}

	http.NotFound(w, r)
}

func (p *Plugin) servePlaylist(w http.ResponseWriter, r *http.Request) {
	playlistPath := filepath.Join(p.segDir, "playlist.m3u8")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var data []byte
	for {
		select {
		case <-ctx.Done():
			log.Printf("hls: playlist not ready: %v", ctx.Err())
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		case <-ticker.C:
			d, err := os.ReadFile(playlistPath)
			if err != nil {
				continue
			}
			if strings.Contains(string(d), "#EXTINF:") {
				data = d
				goto ready
			}
		}
	}

ready:
	log.Printf("hls: serve playlist (%d bytes)", len(data))

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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

	// Patch init.mp4 in memory before serving — fix hvcC compat flags
	// and AAC esds so browsers get valid codec strings
	if name == "init.mp4" {
		data = mux.PatchInitSegment(data)
	}

	contentType := "video/mp2t"
	if strings.HasSuffix(name, ".m4s") || strings.HasSuffix(name, ".mp4") {
		contentType = "video/mp4"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(data) //nolint:errcheck
}
