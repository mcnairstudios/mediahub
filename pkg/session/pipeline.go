package session

import (
	"fmt"
	"os"

	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/demux"
	"github.com/mcnairstudios/mediahub/pkg/av/demuxloop"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output/bridge"
	"github.com/rs/zerolog"
)

type PipelineConfig struct {
	StreamURL        string
	StreamID         string
	UserAgent        string
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
}

type demuxCloser struct {
	d *demux.Demuxer
}

func (dc *demuxCloser) Close() error {
	dc.d.Close()
	return nil
}

func (m *Manager) RunPipeline(sess *Session, cfg PipelineConfig) (*media.ProbeResult, error) {
	if err := os.MkdirAll(sess.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("pipeline: create output dir: %w", err)
	}

	opts := demux.DefaultDemuxOpts()
	opts.UserAgent = cfg.UserAgent
	opts.TimeoutSec = 10

	d, err := demux.NewDemuxer(cfg.StreamURL, opts)
	if err != nil {
		return nil, fmt.Errorf("pipeline: open demuxer: %w", err)
	}

	info := d.StreamInfo()
	if info == nil {
		d.Close()
		return nil, fmt.Errorf("pipeline: probe returned nil")
	}

	sess.AddCloser(&demuxCloser{d: d})

	sess.SetSeekFunc(func(posMs int64) {
		d.RequestSeek(posMs)
	})

	var sink av.PacketSink = sess.FanOut

	if cfg.NeedsTranscode {
		log := zerolog.New(os.Stderr).With().Str("session", sess.StreamID).Logger()
		b, err := bridge.New(bridge.Config{
			Downstream:       sess.FanOut,
			Info:             info,
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
			return nil, fmt.Errorf("pipeline: create bridge: %w", err)
		}
		sess.AddCloser(bridgeCloser{b: b})
		sink = b
	}

	go func() {
		err := demuxloop.Run(sess.Context(), demuxloop.Config{
			Reader: d,
			Sink:   sink,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "pipeline %s: demuxloop error: %v\n", sess.StreamID, err)
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
