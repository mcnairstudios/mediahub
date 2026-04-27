package session

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/demux"
	"github.com/mcnairstudios/mediahub/pkg/av/demuxloop"
	"github.com/mcnairstudios/mediahub/pkg/av/selector"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output/bridge"
	"github.com/rs/zerolog"
)

type PipelineConfig struct {
	StreamURL        string
	StreamID         string
	UserAgent        string
	AudioLanguage    string
	NeedsTranscode   bool
	OutputCodec      string
	OutputAudioCodec string
	HWAccel          string
	DecodeHWAccel    string
	Bitrate          int
	OutputHeight     int
	MaxBitDepth      int
	Deinterlace      bool
	EncoderName      string
	DecoderName      string
	Framerate        int
	FormatHint       string
	TimeoutSec       int
}

type demuxCloser struct {
	d *demux.Demuxer
}

func (dc *demuxCloser) Close() error {
	dc.d.Close()
	return nil
}

func (m *Manager) RunPipeline(sess *Session, cfg PipelineConfig) (*media.ProbeResult, error) {
	log := zerolog.New(os.Stderr).With().
		Str("session", sess.StreamID).
		Str("stream", sess.StreamName).
		Logger()

	if cfg.StreamURL == "" {
		return nil, fmt.Errorf("pipeline: stream URL is empty")
	}

	log.Info().Str("url", cfg.StreamURL).Msg("pipeline: opening stream")

	if err := os.MkdirAll(sess.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("pipeline: create output dir: %w", err)
	}

	opts := demux.DefaultDemuxOpts()
	opts.UserAgent = cfg.UserAgent
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
	if strings.HasPrefix(cfg.StreamURL, "rtsp://") && opts.FormatHint == "" {
		opts.FormatHint = "rtsp"
	}

	d, err := demux.NewDemuxer(cfg.StreamURL, opts)
	if err != nil {
		return nil, fmt.Errorf("pipeline: open stream %q: %w", cfg.StreamURL, err)
	}

	info := d.StreamInfo()
	if info == nil {
		d.Close()
		return nil, fmt.Errorf("pipeline: probe returned no stream info for %q", cfg.StreamURL)
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

	sess.AddCloser(&demuxCloser{d: d})

	sess.SetSeekFunc(func(posMs int64) {
		d.RequestSeek(posMs)
	})

	var sink av.PacketSink = sess.FanOut

	if cfg.NeedsTranscode {
		if info.Video == nil {
			d.Close()
			return nil, fmt.Errorf("pipeline: transcode requested but no video stream in %q", cfg.StreamURL)
		}
		b, err := bridge.New(bridge.Config{
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
			Log:              log,
		})
		if err != nil {
			d.Close()
			return nil, fmt.Errorf("pipeline: create transcode bridge: %w", err)
		}
		sess.AddCloser(bridgeCloser{b: b})
		sink = b
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

		err := demuxloop.Run(sess.Context(), demuxloop.Config{
			Reader: d,
			Sink:   sink,
		})
		if err != nil {
			sess.SetError(err)
			log.Error().Err(err).Str("stream_id", cfg.StreamID).Msg("pipeline: demuxloop ended with error")
		} else {
			log.Info().Str("stream_id", cfg.StreamID).Msg("pipeline: demuxloop ended cleanly")
		}
	}()

	return info, nil
}

type bridgeCloser struct {
	b *bridge.Bridge
}

func (bc bridgeCloser) Close() error {
	bc.b.Stop()
	return nil
}
