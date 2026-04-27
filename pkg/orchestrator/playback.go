package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/connectivity"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
)

type PipelineRunner func(sess *session.Session, cfg session.PipelineConfig) (*media.ProbeResult, error)

type PlaybackDeps struct {
	StreamStore       store.StreamStore
	SettingsStore     store.SettingsStore
	SourceConfigStore sourceconfig.Store
	ConnRegistry      *connectivity.Registry
	SessionMgr        *session.Manager
	Detector          *client.Detector
	OutputReg         *output.Registry
	Strategy          func(strategy.Input, strategy.Output) strategy.Decision
	UserAgent         string
	PipelineRunner    PipelineRunner
}

type PlaybackResult struct {
	Session   *session.Session
	Plugin    output.OutputPlugin
	Servable  output.ServablePlugin
	Decision  strategy.Decision
	IsNew     bool
	Delivery  string
	ProbeInfo *media.ProbeResult
}

func StartPlayback(ctx context.Context, deps PlaybackDeps, streamID string, port int, headers map[string]string) (*PlaybackResult, error) {
	stream, err := deps.StreamStore.Get(ctx, streamID)
	if err != nil {
		return nil, fmt.Errorf("get stream: %w", err)
	}
	if stream == nil {
		return nil, fmt.Errorf("stream %s not found", streamID)
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
		VideoCodec: "copy",
		AudioCodec: "copy",
		Container:  "mp4",
	}

	if deps.Detector != nil {
		detected := deps.Detector.Detect(port, headers)
		if detected != nil {
			out.VideoCodec = detected.Name
			out.AudioCodec = detected.Name
		}
	}

	decision := deps.Strategy(in, out)

	sess, isNew, err := deps.SessionMgr.GetOrCreate(ctx, stream.ID, stream.URL, stream.Name)
	if err != nil {
		return nil, fmt.Errorf("get or create session: %w", err)
	}

	result := &PlaybackResult{
		Session:  sess,
		Decision: decision,
		IsNew:    isNew,
	}

	if isNew {
		pipelineURL := stream.URL

		if deps.SourceConfigStore != nil && deps.ConnRegistry != nil && stream.SourceID != "" {
			sc, _ := deps.SourceConfigStore.Get(ctx, stream.SourceID)
			if sc != nil && sc.Config["use_wireguard"] == "true" {
				if active := deps.ConnRegistry.Active(); active != nil {
					pipelineURL = active.ProxyURL(stream.URL)
				}
			}
		}

		runner := deps.PipelineRunner
		if runner == nil {
			runner = deps.SessionMgr.RunPipeline
		}
		info, err := runner(sess, session.PipelineConfig{
			StreamURL:        pipelineURL,
			StreamID:         stream.ID,
			UserAgent:        deps.UserAgent,
			NeedsTranscode:   decision.NeedsTranscode,
			OutputCodec:      string(decision.VideoCodec),
			OutputAudioCodec: string(decision.AudioCodec),
			HWAccel:          decision.HWAccel,
			Deinterlace:      decision.Deinterlace,
		})
		if err != nil {
			deps.SessionMgr.Stop(stream.ID)
			return nil, fmt.Errorf("run pipeline: %w", err)
		}
		result.ProbeInfo = info

		delivery := resolveDelivery(ctx, deps)
		sess.Delivery = string(delivery)

		pluginCfg := output.PluginConfig{
			OutputDir: sess.OutputDir,
			IsLive:    true,
		}
		if info.Video != nil {
			pluginCfg.Video = info.Video
		}
		if len(info.AudioTracks) > 0 {
			pluginCfg.Audio = &info.AudioTracks[0]
		}

		plugin, err := deps.OutputReg.Create(delivery, pluginCfg)
		if err != nil {
			deps.SessionMgr.Stop(stream.ID)
			return nil, fmt.Errorf("create output plugin: %w", err)
		}

		sess.FanOut.Add(plugin)
		result.Plugin = plugin
		result.Delivery = string(delivery)

		if sp, ok := plugin.(output.ServablePlugin); ok {
			result.Servable = sp
		}

		recCfg := pluginCfg
		recCfg.OutputFilePath = filepath.Join(sess.OutputDir, "source.mp4")
		recPlugin, recErr := deps.OutputReg.Create(output.DeliveryRecord, recCfg)
		if recErr == nil && recPlugin != nil {
			sess.FanOut.Add(recPlugin)
		}
	} else {
		result.Delivery = sess.Delivery
	}

	return result, nil
}

func StopPlayback(deps PlaybackDeps, streamID string) {
	deps.SessionMgr.Stop(streamID)
}

func resolveDelivery(ctx context.Context, deps PlaybackDeps) output.DeliveryMode {
	delivery := output.DeliveryMSE
	if deps.SettingsStore != nil {
		if d, err := deps.SettingsStore.Get(ctx, "delivery"); err == nil && d != "" {
			delivery = output.DeliveryMode(d)
		}
	}
	return delivery
}

func Seek(deps PlaybackDeps, streamID string, positionMs int64) error {
	sess := deps.SessionMgr.Get(streamID)
	if sess == nil {
		return fmt.Errorf("session %s not found", streamID)
	}
	sess.Seek(positionMs)
	return nil
}
