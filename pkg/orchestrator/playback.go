package orchestrator

import (
	"context"
	"fmt"

	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
)

type PlaybackDeps struct {
	StreamStore store.StreamStore
	SessionMgr  *session.Manager
	Detector    *client.Detector
	OutputReg   *output.Registry
	Strategy    func(strategy.Input, strategy.Output) strategy.Decision
}

type PlaybackResult struct {
	Session  *session.Session
	Plugin   output.OutputPlugin
	Servable output.ServablePlugin
	Decision strategy.Decision
	IsNew    bool
}

func StartPlayback(ctx context.Context, deps PlaybackDeps, streamID string, port int, headers map[string]string) (*PlaybackResult, error) {
	stream, err := deps.StreamStore.Get(ctx, streamID)
	if err != nil {
		return nil, fmt.Errorf("get stream: %w", err)
	}
	if stream == nil {
		return nil, fmt.Errorf("stream %s not found", streamID)
	}

	detected := deps.Detector.Detect(port, headers)
	if detected == nil {
		return nil, fmt.Errorf("no client matched for port %d", port)
	}

	in := strategy.Input{
		VideoCodec: stream.VideoCodec,
		AudioCodec: stream.AudioCodec,
		Width:      stream.Width,
		Height:     stream.Height,
		Interlaced: stream.Interlaced,
		BitDepth:   stream.BitDepth,
	}

	out := strategy.Output{
		VideoCodec: detected.Name,
		AudioCodec: detected.Name,
		Container:  "mpegts",
	}

	decision := deps.Strategy(in, out)

	sess, isNew, err := deps.SessionMgr.GetOrCreate(ctx, stream.ID, stream.URL, stream.Name)
	if err != nil {
		return nil, fmt.Errorf("get or create session: %w", err)
	}

	mode := output.DeliveryMSE
	plugin, err := deps.OutputReg.Create(mode, output.PluginConfig{
		OutputDir: sess.OutputDir,
		IsLive:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("create output plugin: %w", err)
	}

	sess.FanOut.Add(plugin)

	result := &PlaybackResult{
		Session:  sess,
		Plugin:   plugin,
		Decision: decision,
		IsNew:    isNew,
	}

	if sp, ok := plugin.(output.ServablePlugin); ok {
		result.Servable = sp
	}

	return result, nil
}

func StopPlayback(deps PlaybackDeps, streamID string) {
	deps.SessionMgr.Stop(streamID)
}

func Seek(deps PlaybackDeps, streamID string, positionMs int64) error {
	sess := deps.SessionMgr.Get(streamID)
	if sess == nil {
		return fmt.Errorf("session %s not found", streamID)
	}
	sess.Seek(positionMs)
	return nil
}
