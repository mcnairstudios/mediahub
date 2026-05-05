package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/demux"
	"github.com/mcnairstudios/mediahub/pkg/av/demuxloop"
	"github.com/mcnairstudios/mediahub/pkg/av/selector"
	"github.com/mcnairstudios/mediahub/pkg/av/subtitle"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output/bridge"
	ytresolve "github.com/mcnairstudios/mediahub/pkg/youtube"
	"github.com/rs/zerolog"
)

var ErrEncoderInit = errors.New("encoder initialization failed")

func IsEncoderInitError(err error) bool {
	if errors.Is(err, ErrEncoderInit) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "video encoder") || strings.Contains(msg, "transcode bridge")
}

const (
	maxLiveRetries    = 10
	liveRetryBaseWait = 2 * time.Second
	liveRetryMaxWait  = 30 * time.Second
)

type PipelineConfig struct {
	StreamURL          string
	StreamID           string
	UserAgent          string
	AudioLanguage      string
	NeedsTranscode     bool
	NeedsAudioTranscode bool
	OutputCodec        string
	OutputAudioCodec   string
	HWAccel          string
	DecodeHWAccel    string
	Bitrate          int
	OutputHeight     int
	MaxBitDepth      int
	Deinterlace      bool
	EncoderName      string
	DecoderName      string
	Framerate        int
	FormatHint        string
	ProbeDurationSec  int
	TimeoutSec        int
	IsLive            bool
	CachedStreamInfo  *media.ProbeResult
}

type demuxCloser struct {
	d *demux.Demuxer
}

func (dc *demuxCloser) Close() error {
	dc.d.Close()
	return nil
}

type PipelineResult struct {
	Info             *media.ProbeResult
	VideoCodecParams any // *astiav.CodecParameters
	AudioCodecParams any // *astiav.CodecParameters
	VideoExtradata   []byte
	AudioExtradata   []byte
}

func (m *Manager) RunPipeline(sess *Session, cfg PipelineConfig) (*PipelineResult, error) {
	log := zerolog.New(os.Stderr).With().
		Str("session", sess.StreamID).
		Str("stream", sess.StreamName).
		Logger()

	if cfg.StreamURL == "" {
		return nil, fmt.Errorf("pipeline: stream URL is empty")
	}

	log.Info().Str("url", cfg.StreamURL).Msg("pipeline: opening stream")

	if ytresolve.IsYouTubeURL(cfg.StreamURL) {
		resolved, ytErr := ytresolve.ResolveStreamURL(sess.Context(), cfg.StreamURL)
		if ytErr != nil {
			return nil, fmt.Errorf("pipeline: resolve YouTube URL %q: %w", cfg.StreamURL, ytErr)
		}
		log.Info().Str("original", cfg.StreamURL).Msg("pipeline: resolved YouTube URL")
		cfg.StreamURL = resolved
	}

	if err := os.MkdirAll(sess.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("pipeline: create output dir: %w", err)
	}

	opts := demux.DefaultDemuxOpts()
	opts.UserAgent = cfg.UserAgent
	opts.IsLive = cfg.IsLive
	timeoutSec := cfg.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	opts.TimeoutSec = timeoutSec
	if cfg.AudioLanguage != "" {
		opts.AudioLanguage = cfg.AudioLanguage
	}
	if cfg.FormatHint != "" {
		opts.FormatHint = cfg.FormatHint
	}
	if cfg.ProbeDurationSec > 0 {
		opts.AnalyzeDuration = cfg.ProbeDurationSec * 1_000_000
	}
	if cfg.CachedStreamInfo != nil {
		opts.CachedStreamInfo = cfg.CachedStreamInfo
	}

	d, err := demux.NewDemuxer(cfg.StreamURL, opts)
	if err != nil {
		return nil, fmt.Errorf("pipeline: open stream %q: %w", cfg.StreamURL, err)
	}

	info := d.StreamInfo()
	if info == nil {
		d.Close()
		return nil, fmt.Errorf("pipeline: probe returned no stream info for %q — the stream may be offline, encrypted, or not a valid media source", cfg.StreamURL)
	}
	if info.Video == nil && len(info.AudioTracks) == 0 {
		d.Close()
		return nil, fmt.Errorf("pipeline: no video or audio streams detected in %q — the stream may be audio-only, encrypted (DRM), or contain unsupported codecs", cfg.StreamURL)
	}

	if info.Video != nil {
		if vcp := d.VideoCodecParameters(); vcp != nil {
			if ed := vcp.ExtraData(); len(ed) > 0 && len(info.Video.Extradata) == 0 {
				info.Video.Extradata = make([]byte, len(ed))
				copy(info.Video.Extradata, ed)
			}
		}
	}

	if info.Video != nil {
		fps := 0.0
		if info.Video.FramerateD > 0 {
			fps = float64(info.Video.FramerateN) / float64(info.Video.FramerateD)
		}
		log.Info().
			Str("video_codec", info.Video.Codec).
			Int("width", info.Video.Width).
			Int("height", info.Video.Height).
			Int("bit_depth", info.Video.BitDepth).
			Bool("interlaced", info.Video.Interlaced).
			Float64("fps", fps).
			Msg("probed video")
	} else {
		log.Warn().Msg("no video stream detected (audio-only)")
	}

	if len(info.AudioTracks) > 0 {
		for _, at := range info.AudioTracks {
			log.Info().
				Int("index", at.Index).
				Str("codec", at.Codec).
				Str("language", at.Language).
				Int("channels", at.Channels).
				Int("sample_rate", at.SampleRate).
				Bool("ad", at.IsAD).
				Msg("probed audio track")
		}
	} else {
		log.Warn().Msg("no audio tracks detected (video-only)")
	}

	audioIdx := selector.SelectAudio(info.AudioTracks, selector.AudioPrefs{
		Language: cfg.AudioLanguage,
	})
	if audioIdx >= 0 {
		log.Info().Int("selected_audio_index", audioIdx).Msg("selected audio track")
	}

	if len(info.SubTracks) > 0 {
		st := info.SubTracks[0]
		collector := subtitle.NewCollector(subtitle.TrackInfo{
			Index:    st.Index,
			Codec:    st.Codec,
			Language: st.Language,
		})
		sess.Subtitles = collector
		sess.FanOut.SetSubtitleCollector(collector)
		log.Info().Str("codec", st.Codec).Str("language", st.Language).Int("index", st.Index).Msg("subtitle track detected")
	}

	sess.AddCloser(&demuxCloser{d: d})

	sess.SetSeekFunc(func(posMs int64) {
		d.RequestSeek(posMs)
	})

	if !cfg.NeedsAudioTranscode && cfg.OutputAudioCodec != "" && cfg.OutputAudioCodec != "copy" && audioIdx >= 0 && audioIdx < len(info.AudioTracks) {
		probed := info.AudioTracks[audioIdx].Codec
		if probed != "" && probed != cfg.OutputAudioCodec && probed != "aac" {
			cfg.NeedsAudioTranscode = true
			log.Info().Str("probed_audio", probed).Str("output_audio", cfg.OutputAudioCodec).Msg("forcing audio transcode after probe")
		}
	}

	var sink av.PacketSink = sess.FanOut
	var videoExtradata, audioExtradata []byte

	if info.Video == nil && cfg.NeedsTranscode {
		cfg.NeedsTranscode = false
		log.Info().Msg("audio-only stream: disabling video transcode")
	}

	if cfg.NeedsTranscode || cfg.NeedsAudioTranscode {
		audioOnly := !cfg.NeedsTranscode && cfg.NeedsAudioTranscode
		if info.Video == nil {
			audioOnly = true
		}
		b, err := bridge.New(bridge.Config{
			Downstream:       sess.FanOut,
			Info:             info,
			AudioIndex:       audioIdx,
			VideoCodecParams: d.VideoCodecParameters(),
			AudioCodecParams: d.AudioCodecParameters(),
			HWAccel:          cfg.HWAccel,
			DecodeHWAccel:    cfg.DecodeHWAccel,
			OutputCodec:      cfg.OutputCodec,
			OutputAudioCodec: cfg.OutputAudioCodec,
			Bitrate:          cfg.Bitrate,
			OutputHeight:     cfg.OutputHeight,
			MaxBitDepth:      cfg.MaxBitDepth,
			Deinterlace:      cfg.Deinterlace,
			EncoderName:      cfg.EncoderName,
			DecoderName:      cfg.DecoderName,
			Framerate:        cfg.Framerate,
			AudioOnly:        audioOnly,
			Log:              log,
		})
		if err != nil {
			d.Close()
			return nil, fmt.Errorf("pipeline: create transcode bridge for %q (codec=%s, hwaccel=%s) — check that the encoder is installed and the hardware accelerator is available: %w: %w", cfg.StreamURL, cfg.OutputCodec, cfg.HWAccel, err, ErrEncoderInit)
		}
		sess.AddCloser(bridgeCloser{b: b})
		sink = b
		videoExtradata = b.VideoEncoderExtradata()
		audioExtradata = b.AudioEncoderExtradata()
	}

	log.Info().Str("stream_id", cfg.StreamID).Msg("pipeline: demuxloop starting")

	go func() {
		defer sess.MarkDone()
		defer func() {
			if r := recover(); r != nil {
				pErr := fmt.Errorf("pipeline panic: %v", r)
				sess.SetError(pErr)
				log.Error().Interface("panic", r).Str("stack", string(debug.Stack())).Msg("pipeline: PANIC in demuxloop")
			}
		}()

		loopErr := demuxloop.Run(sess.Context(), demuxloop.Config{
			Reader: d,
			Sink:   sink,
		})

		if !cfg.IsLive {
			if loopErr == nil {
				log.Info().Str("stream_id", cfg.StreamID).Msg("pipeline: demuxloop ended cleanly")
				return
			}
			sess.SetError(loopErr)
			log.Error().Err(loopErr).Str("stream_id", cfg.StreamID).Msg("pipeline: demuxloop ended with error")
			return
		}

		if loopErr == nil {
			loopErr = fmt.Errorf("live stream ended unexpectedly (EOF)")
			log.Warn().Str("stream_id", cfg.StreamID).Msg("pipeline: live demuxloop ended (EOF), will retry")
		}

		for attempt := 1; attempt <= maxLiveRetries; attempt++ {
			select {
			case <-sess.Context().Done():
				return
			default:
			}

			wait := liveRetryBaseWait * time.Duration(1<<(attempt-1))
			if wait > liveRetryMaxWait {
				wait = liveRetryMaxWait
			}
			log.Warn().Err(loopErr).Int("attempt", attempt).Int("max", maxLiveRetries).Dur("backoff", wait).
				Str("stream_id", cfg.StreamID).Msg("pipeline: live stream failed, retrying")

			select {
			case <-sess.Context().Done():
				return
			case <-time.After(wait):
			}

			for _, c := range sess.DrainClosers() {
				c.Close()
			}

			segDir := filepath.Join(sess.OutputDir, "segments")
			if entries, err := os.ReadDir(segDir); err == nil {
				for _, e := range entries {
					if !e.IsDir() {
						os.Remove(filepath.Join(segDir, e.Name()))
					}
				}
			}

			sess.FanOut.ResetForSeek()
			sess.ClearFinished()

			newD, newSink, retryErr := m.openDemuxAndSink(sess, cfg, log, audioIdx, info)
			if retryErr != nil {
				log.Error().Err(retryErr).Int("attempt", attempt).Str("stream_id", cfg.StreamID).
					Msg("pipeline: retry failed to reopen stream")
				loopErr = retryErr
				continue
			}

			loopErr = demuxloop.Run(sess.Context(), demuxloop.Config{
				Reader: newD,
				Sink:   newSink,
			})
			if loopErr == nil {
				log.Info().Str("stream_id", cfg.StreamID).Msg("pipeline: demuxloop ended cleanly after retry")
				return
			}
		}

		sess.SetError(loopErr)
		log.Error().Err(loopErr).Int("retries_exhausted", maxLiveRetries).
			Str("stream_id", cfg.StreamID).Msg("pipeline: all retries failed")
	}()

	var videoCP any = d.VideoCodecParameters()
	var audioCP any = d.AudioCodecParameters()
	if cfg.NeedsTranscode || cfg.NeedsAudioTranscode {
		if sink != sess.FanOut {
			if bc, ok := sink.(interface{ VideoCodecParameters() any }); ok {
				if vcp := bc.VideoCodecParameters(); vcp != nil {
					videoCP = vcp
				}
			}
			if bc, ok := sink.(interface{ AudioCodecParameters() any }); ok {
				if acp := bc.AudioCodecParameters(); acp != nil {
					audioCP = acp
				}
			}
		}
	}

	return &PipelineResult{
		Info:             info,
		VideoCodecParams: videoCP,
		AudioCodecParams: audioCP,
		VideoExtradata:   videoExtradata,
		AudioExtradata:   audioExtradata,
	}, nil
}

func (m *Manager) openDemuxAndSink(sess *Session, cfg PipelineConfig, log zerolog.Logger, audioIdx int, info *media.ProbeResult) (*demux.Demuxer, av.PacketSink, error) {
	if ytresolve.IsYouTubeURL(cfg.StreamURL) {
		resolved, ytErr := ytresolve.ResolveStreamURL(sess.Context(), cfg.StreamURL)
		if ytErr != nil {
			return nil, nil, fmt.Errorf("resolve YouTube URL %q: %w", cfg.StreamURL, ytErr)
		}
		log.Info().Str("original", cfg.StreamURL).Msg("pipeline: re-resolved YouTube URL for retry")
		cfg.StreamURL = resolved
	}

	opts := demux.DefaultDemuxOpts()
	opts.UserAgent = cfg.UserAgent
	opts.IsLive = cfg.IsLive
	timeoutSec := cfg.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	opts.TimeoutSec = timeoutSec
	if cfg.AudioLanguage != "" {
		opts.AudioLanguage = cfg.AudioLanguage
	}
	if cfg.FormatHint != "" {
		opts.FormatHint = cfg.FormatHint
	}
	if cfg.ProbeDurationSec > 0 {
		opts.AnalyzeDuration = cfg.ProbeDurationSec * 1_000_000
	}

	d, err := demux.NewDemuxer(cfg.StreamURL, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("open stream %q: %w", cfg.StreamURL, err)
	}

	sess.AddCloser(&demuxCloser{d: d})

	sess.SetSeekFunc(func(posMs int64) {
		d.RequestSeek(posMs)
	})

	var sink av.PacketSink = sess.FanOut

	if cfg.NeedsTranscode || cfg.NeedsAudioTranscode {
		audioOnly := !cfg.NeedsTranscode && cfg.NeedsAudioTranscode
		b, bErr := bridge.New(bridge.Config{
			Downstream:       sess.FanOut,
			Info:             info,
			AudioIndex:       audioIdx,
			HWAccel:          cfg.HWAccel,
			DecodeHWAccel:    cfg.DecodeHWAccel,
			OutputCodec:      cfg.OutputCodec,
			OutputAudioCodec: cfg.OutputAudioCodec,
			Bitrate:          cfg.Bitrate,
			OutputHeight:     cfg.OutputHeight,
			MaxBitDepth:      cfg.MaxBitDepth,
			Deinterlace:      cfg.Deinterlace,
			EncoderName:      cfg.EncoderName,
			DecoderName:      cfg.DecoderName,
			Framerate:        cfg.Framerate,
			AudioOnly:        audioOnly,
			Log:              log,
		})
		if bErr != nil {
			d.Close()
			return nil, nil, fmt.Errorf("create transcode bridge: %w", bErr)
		}
		sess.AddCloser(bridgeCloser{b: b})
		sink = b
	}

	return d, sink, nil
}

func (m *Manager) RunSubprocessPipelineMethod(sess *Session, cfg PipelineConfig) (*PipelineResult, error) {
	log := zerolog.New(os.Stderr).With().
		Str("session", sess.StreamID).
		Str("stream", sess.StreamName).
		Logger()

	if cfg.StreamURL == "" {
		return nil, fmt.Errorf("subprocess pipeline: stream URL is empty")
	}

	if ytresolve.IsYouTubeURL(cfg.StreamURL) {
		resolved, ytErr := ytresolve.ResolveStreamURL(sess.Context(), cfg.StreamURL)
		if ytErr != nil {
			return nil, fmt.Errorf("subprocess pipeline: resolve YouTube URL %q: %w", cfg.StreamURL, ytErr)
		}
		log.Info().Str("original", cfg.StreamURL).Msg("subprocess pipeline: resolved YouTube URL")
		cfg.StreamURL = resolved
	}

	if err := os.MkdirAll(sess.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("subprocess pipeline: create output dir: %w", err)
	}

	subCfg := SubprocessConfig{
		InputURL:     cfg.StreamURL,
		OutputDir:    sess.OutputDir,
		VideoCodec:   cfg.OutputCodec,
		AudioCodec:   cfg.OutputAudioCodec,
		VideoBitrate: cfg.Bitrate,
		HWAccel:      cfg.HWAccel,
		OutputHeight: cfg.OutputHeight,
		Deinterlace:  cfg.Deinterlace,
		IsLive:       cfg.IsLive,
	}

	go func() {
		defer sess.MarkDone()
		defer func() {
			if r := recover(); r != nil {
				pErr := fmt.Errorf("subprocess pipeline panic: %v", r)
				sess.SetError(pErr)
				log.Error().Interface("panic", r).Msg("subprocess pipeline: PANIC")
			}
		}()

		err := RunSubprocessPipeline(sess.Context(), subCfg, log)
		if err == nil {
			log.Info().Str("stream_id", cfg.StreamID).Msg("subprocess pipeline: ended cleanly")
			return
		}

		if sess.Context().Err() != nil {
			return
		}

		if !cfg.IsLive {
			sess.SetError(err)
			log.Error().Err(err).Str("stream_id", cfg.StreamID).Msg("subprocess pipeline: ended with error")
			return
		}

		for attempt := 1; attempt <= maxLiveRetries; attempt++ {
			select {
			case <-sess.Context().Done():
				return
			default:
			}

			wait := liveRetryBaseWait * time.Duration(1<<(attempt-1))
			if wait > liveRetryMaxWait {
				wait = liveRetryMaxWait
			}
			log.Warn().Err(err).Int("attempt", attempt).Int("max", maxLiveRetries).Dur("backoff", wait).
				Str("stream_id", cfg.StreamID).Msg("subprocess pipeline: live stream failed, retrying")

			select {
			case <-sess.Context().Done():
				return
			case <-time.After(wait):
			}

			sess.FanOut.ResetForSeek()
			sess.ClearFinished()

			err = RunSubprocessPipeline(sess.Context(), subCfg, log)
			if err == nil {
				log.Info().Str("stream_id", cfg.StreamID).Msg("subprocess pipeline: ended cleanly after retry")
				return
			}
		}

		sess.SetError(err)
		log.Error().Err(err).Int("retries_exhausted", maxLiveRetries).
			Str("stream_id", cfg.StreamID).Msg("subprocess pipeline: all retries failed")
	}()

	info := &media.ProbeResult{}
	if cfg.OutputCodec != "" && cfg.OutputCodec != "copy" {
		info.Video = &media.VideoInfo{
			Codec: cfg.OutputCodec,
		}
	}
	if cfg.OutputAudioCodec != "" && cfg.OutputAudioCodec != "copy" {
		info.AudioTracks = []media.AudioTrack{{
			Codec:      cfg.OutputAudioCodec,
			Channels:   2,
			SampleRate: 48000,
		}}
	}

	return &PipelineResult{
		Info: info,
	}, nil
}

type bridgeCloser struct {
	b *bridge.Bridge
}

func (bc bridgeCloser) Close() error {
	bc.b.Stop()
	return nil
}
