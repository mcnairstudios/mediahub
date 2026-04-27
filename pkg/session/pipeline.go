package session

import (
	"fmt"
	"os"

	"github.com/mcnairstudios/mediahub/pkg/av/demux"
	"github.com/mcnairstudios/mediahub/pkg/av/demuxloop"
	"github.com/mcnairstudios/mediahub/pkg/media"
)

type PipelineConfig struct {
	StreamURL string
	StreamID  string
	UserAgent string
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

	go func() {
		err := demuxloop.Run(sess.Context(), demuxloop.Config{
			Reader: d,
			Sink:   sess.FanOut,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "pipeline %s: demuxloop error: %v\n", sess.StreamID, err)
		}
	}()

	return info, nil
}
