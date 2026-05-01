package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/connectivity"
	"github.com/mcnairstudios/mediahub/pkg/connectivity/wg"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/output/hls"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/sourceprofile"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
)

type PipelineRunner func(sess *session.Session, cfg session.PipelineConfig) (*session.PipelineResult, error)

type PlaybackDeps struct {
	StreamStore        store.StreamStore
	SettingsStore      store.SettingsStore
	SourceConfigStore  sourceconfig.Store
	SourceProfileStore sourceprofile.Store
	ConnRegistry       *connectivity.Registry
	WGService          *wg.Service
	SessionMgr         *session.Manager
	Detector           *client.Detector
	ClientStore        client.Store
	OutputReg          *output.Registry
	Strategy           func(strategy.Input, strategy.Output) strategy.Decision
	ProbeCache         store.ProbeCache
	UserAgent          string
	PipelineRunner     PipelineRunner
	ClientOverrideID   string
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
		AudioCodec: "aac",
		Container:  "mp4",
	}

	var detectedClient *client.Client
	if deps.ClientOverrideID != "" && deps.ClientStore != nil {
		overrideClient, lookupErr := deps.ClientStore.Get(ctx, deps.ClientOverrideID)
		if lookupErr == nil && overrideClient != nil {
			detectedClient = overrideClient
		}
	}
	if detectedClient == nil && deps.Detector != nil {
		detectedClient = deps.Detector.Detect(port, headers)
	}
	if detectedClient != nil {
		p := detectedClient.Profile
		if p.VideoCodec != "" {
			out.VideoCodec = p.VideoCodec
		}
		if p.AudioCodec != "" {
			out.AudioCodec = p.AudioCodec
		}
		if p.Container != "" {
			out.Container = p.Container
		}
		if p.HWAccel != "" {
			out.HWAccel = p.HWAccel
		}
		if p.OutputHeight > 0 {
			out.OutputHeight = p.OutputHeight
		}
	}

	var audioLanguage string
	if deps.SettingsStore != nil {
		if hw, err := deps.SettingsStore.Get(ctx, "default_hwaccel"); err == nil && hw != "" && out.HWAccel == "" {
			out.HWAccel = hw
		}
		if mbd, err := deps.SettingsStore.Get(ctx, "default_max_bit_depth"); err == nil && mbd != "" {
			if v, err := strconv.Atoi(mbd); err == nil && v > 0 {
				out.MaxBitDepth = v
			}
		}
		if al, err := deps.SettingsStore.Get(ctx, "audio_language"); err == nil && al != "" {
			audioLanguage = al
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

	if !isNew {
		result.Delivery = sess.Delivery
		return result, nil
	}

	pipelineURL := stream.URL

	if deps.SourceConfigStore != nil && stream.SourceID != "" {
		sc, _ := deps.SourceConfigStore.Get(ctx, stream.SourceID)
		if sc != nil && sc.Config["use_wireguard"] == "true" {
			wgProfileID := sc.Config["wg_profile_id"]
			if wgProfileID != "" && deps.WGService != nil {
				_ = deps.WGService.EnsureProfileProxy(wgProfileID)
				pipelineURL = deps.WGService.ProxyURLForProfile(wgProfileID, stream.URL)
			} else if deps.WGService != nil {
				pipelineURL = deps.WGService.BestProxyURL(stream.URL)
			} else if deps.ConnRegistry != nil {
				if active := deps.ConnRegistry.Active(); active != nil {
					pipelineURL = active.ProxyURL(stream.URL)
				}
			}
		}
	}

	var cachedProbe *media.ProbeResult
	if deps.ProbeCache != nil && !strings.HasPrefix(stream.URL, "rtsp://") {
		cachedProbe, _ = deps.ProbeCache.Get(stream.URL)
	}

	decodeHWAccel := resolveDecodeHWAccel(ctx, deps)
	encoderName := resolveEncoderName(ctx, deps, string(decision.VideoCodec))
	decoderName := resolveDecoderName(ctx, deps, stream.VideoCodec)

	pipeCfg := session.PipelineConfig{
		StreamURL:           pipelineURL,
		StreamID:            stream.ID,
		UserAgent:           deps.UserAgent,
		AudioLanguage:       audioLanguage,
		NeedsTranscode:      decision.NeedsTranscode,
		NeedsAudioTranscode: decision.NeedsAudioTranscode,
		OutputCodec:         string(decision.VideoCodec),
		OutputAudioCodec:    string(decision.AudioCodec),
		HWAccel:             decision.HWAccel,
		DecodeHWAccel:       decodeHWAccel,
		Deinterlace:         decision.Deinterlace,
		OutputHeight:        out.OutputHeight,
		MaxBitDepth:         out.MaxBitDepth,
		EncoderName:         encoderName,
		DecoderName:         decoderName,
		IsLive:              true,
		CachedStreamInfo:    cachedProbe,
	}

	applySourceProfile(ctx, deps, stream, &pipeCfg)

	if deps.SettingsStore != nil {
		if v, err := deps.SettingsStore.Get(ctx, "subprocess_transcode"); err == nil && v == "true" {
			pipeCfg.UseSubprocess = true
		}
	}

	if !pipeCfg.UseSubprocess && pipeCfg.NeedsTranscode && needsSubprocessFallback(pipeCfg.OutputCodec) {
		pipeCfg.UseSubprocess = true
	}

	runner := deps.PipelineRunner
	if runner == nil {
		if pipeCfg.UseSubprocess && pipeCfg.NeedsTranscode {
			runner = deps.SessionMgr.RunSubprocessPipelineMethod
		} else {
			runner = deps.SessionMgr.RunPipeline
		}
	}
	pipelineResult, err := runner(sess, pipeCfg)
	if err != nil && !pipeCfg.UseSubprocess && pipeCfg.NeedsTranscode && session.IsEncoderInitError(err) {
		deps.SessionMgr.Stop(stream.ID)
		sess, _, err = deps.SessionMgr.GetOrCreate(ctx, stream.ID, stream.URL, stream.Name)
		if err != nil {
			return nil, fmt.Errorf("get or create session for subprocess fallback: %w", err)
		}
		pipeCfg.UseSubprocess = true
		fallbackRunner := deps.PipelineRunner
		if fallbackRunner == nil {
			fallbackRunner = deps.SessionMgr.RunSubprocessPipelineMethod
		}
		pipelineResult, err = fallbackRunner(sess, pipeCfg)
	}
	if err != nil {
		if deps.ProbeCache != nil && cachedProbe != nil {
			_ = deps.ProbeCache.Delete(stream.URL)
		}
		deps.SessionMgr.Stop(stream.ID)
		return nil, fmt.Errorf("pipeline failed for stream %q (%s): %w", stream.Name, stream.URL, err)
	}

	if deps.ProbeCache != nil && cachedProbe == nil && pipelineResult.Info != nil {
		_ = deps.ProbeCache.Set(stream.URL, pipelineResult.Info)
	}
	info := pipelineResult.Info
	result.ProbeInfo = info

	delivery := resolveDelivery(ctx, deps)
	if detectedClient != nil && detectedClient.Profile.Delivery != "" {
		delivery = output.DeliveryMode(detectedClient.Profile.Delivery)
	}
	if pipeCfg.UseSubprocess && pipeCfg.NeedsTranscode {
		delivery = output.DeliveryHLS
	}
	sess.Delivery = string(delivery)

	if pipeCfg.UseSubprocess && pipeCfg.NeedsTranscode {
		plugin, plugErr := createSubprocessHLSPlugin(sess.OutputDir)
		if plugErr != nil {
			deps.SessionMgr.Stop(stream.ID)
			return nil, fmt.Errorf("create subprocess HLS plugin: %w", plugErr)
		}
		sess.FanOut.Add(plugin)
		result.Plugin = plugin
		result.Delivery = string(delivery)
		if sp, ok := plugin.(output.ServablePlugin); ok {
			result.Servable = sp
		}
		return result, nil
	}

	pluginCfg := output.PluginConfig{
		OutputDir: sess.OutputDir,
		IsLive:    true,
	}
	if !decision.NeedsTranscode {
		pluginCfg.VideoCodecParams = pipelineResult.VideoCodecParams
	}
	if !decision.NeedsAudioTranscode {
		pluginCfg.AudioCodecParams = pipelineResult.AudioCodecParams
	}
	if len(pipelineResult.VideoExtradata) > 0 {
		pluginCfg.VideoExtradata = pipelineResult.VideoExtradata
	} else if info != nil && info.Video != nil && len(info.Video.Extradata) > 0 {
		pluginCfg.VideoExtradata = info.Video.Extradata
	}
	if len(pipelineResult.AudioExtradata) > 0 {
		pluginCfg.AudioExtradata = pipelineResult.AudioExtradata
	}
	if info != nil && info.Video != nil {
		v := *info.Video
		if decision.NeedsTranscode && string(decision.VideoCodec) != "" {
			v.Codec = string(decision.VideoCodec)
		}
		pluginCfg.Video = &v
	}
	if info != nil && len(info.AudioTracks) > 0 {
		a := info.AudioTracks[0]
		if decision.NeedsAudioTranscode && string(decision.AudioCodec) != "" {
			a.Codec = string(decision.AudioCodec)
		}
		pluginCfg.Audio = &a
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

	func() {
		defer func() {
			recover() //nolint:errcheck
		}()
		return
		recCfg := pluginCfg
		recCfg.OutputFilePath = filepath.Join(sess.OutputDir, "source.ts")
		recCfg.OutputFormat = "mpegts"
		recPlugin, recErr := deps.OutputReg.Create(output.DeliveryRecord, recCfg)
		if recErr != nil {
			return
		}
		if recPlugin != nil {
			sess.FanOut.Add(recPlugin)
		}
	}()

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

func PlayRecording(ctx context.Context, deps PlaybackDeps, recordingID, filePath, title string, port int, headers map[string]string) (*PlaybackResult, error) {
	sessionKey := "rec:" + recordingID

	sess, isNew, err := deps.SessionMgr.GetOrCreate(ctx, sessionKey, filePath, title)
	if err != nil {
		return nil, fmt.Errorf("get or create session: %w", err)
	}

	result := &PlaybackResult{
		Session: sess,
		IsNew:   isNew,
	}

	if !isNew {
		result.Delivery = sess.Delivery
		return result, nil
	}

	out := strategy.Output{
		VideoCodec: "copy",
		AudioCodec: "aac",
		Container:  "mp4",
	}

	var detectedClient *client.Client
	if deps.Detector != nil {
		detectedClient = deps.Detector.Detect(port, headers)
		if detectedClient != nil {
			p := detectedClient.Profile
			if p.VideoCodec != "" {
				out.VideoCodec = p.VideoCodec
			}
			if p.AudioCodec != "" {
				out.AudioCodec = p.AudioCodec
			}
			if p.Container != "" {
				out.Container = p.Container
			}
			if p.HWAccel != "" {
				out.HWAccel = p.HWAccel
			}
			if p.OutputHeight > 0 {
				out.OutputHeight = p.OutputHeight
			}
		}
	}

	var audioLanguage string
	if deps.SettingsStore != nil {
		if hw, err := deps.SettingsStore.Get(ctx, "default_hwaccel"); err == nil && hw != "" && out.HWAccel == "" {
			out.HWAccel = hw
		}
		if mbd, err := deps.SettingsStore.Get(ctx, "default_max_bit_depth"); err == nil && mbd != "" {
			if v, err := strconv.Atoi(mbd); err == nil && v > 0 {
				out.MaxBitDepth = v
			}
		}
		if al, err := deps.SettingsStore.Get(ctx, "audio_language"); err == nil && al != "" {
			audioLanguage = al
		}
	}

	in := strategy.Input{
		VideoCodec: "h264",
		AudioCodec: "aac",
	}
	decision := deps.Strategy(in, out)
	result.Decision = decision

	decodeHWAccel := resolveDecodeHWAccel(ctx, deps)
	encoderName := resolveEncoderName(ctx, deps, string(decision.VideoCodec))
	decoderName := resolveDecoderName(ctx, deps, in.VideoCodec)

	pipeCfg := session.PipelineConfig{
		StreamURL:           filePath,
		StreamID:            sessionKey,
		UserAgent:           deps.UserAgent,
		AudioLanguage:       audioLanguage,
		NeedsTranscode:      decision.NeedsTranscode,
		NeedsAudioTranscode: decision.NeedsAudioTranscode,
		OutputCodec:         string(decision.VideoCodec),
		OutputAudioCodec:    string(decision.AudioCodec),
		HWAccel:             decision.HWAccel,
		DecodeHWAccel:       decodeHWAccel,
		Deinterlace:         decision.Deinterlace,
		OutputHeight:        out.OutputHeight,
		MaxBitDepth:         out.MaxBitDepth,
		EncoderName:         encoderName,
		DecoderName:         decoderName,
	}

	if deps.SettingsStore != nil {
		if v, err := deps.SettingsStore.Get(ctx, "subprocess_transcode"); err == nil && v == "true" {
			pipeCfg.UseSubprocess = true
		}
	}

	if !pipeCfg.UseSubprocess && pipeCfg.NeedsTranscode && needsSubprocessFallback(pipeCfg.OutputCodec) {
		pipeCfg.UseSubprocess = true
	}

	runner := deps.PipelineRunner
	if runner == nil {
		if pipeCfg.UseSubprocess && pipeCfg.NeedsTranscode {
			runner = deps.SessionMgr.RunSubprocessPipelineMethod
		} else {
			runner = deps.SessionMgr.RunPipeline
		}
	}
	pipelineResult, err := runner(sess, pipeCfg)
	if err != nil && !pipeCfg.UseSubprocess && pipeCfg.NeedsTranscode && session.IsEncoderInitError(err) {
		deps.SessionMgr.Stop(sessionKey)
		sess, _, err = deps.SessionMgr.GetOrCreate(ctx, sessionKey, filePath, title)
		if err != nil {
			return nil, fmt.Errorf("get or create session for subprocess fallback: %w", err)
		}
		pipeCfg.UseSubprocess = true
		fallbackRunner := deps.PipelineRunner
		if fallbackRunner == nil {
			fallbackRunner = deps.SessionMgr.RunSubprocessPipelineMethod
		}
		pipelineResult, err = fallbackRunner(sess, pipeCfg)
	}
	if err != nil {
		deps.SessionMgr.Stop(sessionKey)
		return nil, fmt.Errorf("pipeline failed for recording %q (%s): %w", title, filePath, err)
	}
	info := pipelineResult.Info
	result.ProbeInfo = info

	delivery := resolveDelivery(ctx, deps)
	if detectedClient != nil && detectedClient.Profile.Delivery != "" {
		delivery = output.DeliveryMode(detectedClient.Profile.Delivery)
	}
	if pipeCfg.UseSubprocess && pipeCfg.NeedsTranscode {
		delivery = output.DeliveryHLS
	}
	sess.Delivery = string(delivery)

	if pipeCfg.UseSubprocess && pipeCfg.NeedsTranscode {
		plugin, plugErr := createSubprocessHLSPlugin(sess.OutputDir)
		if plugErr != nil {
			deps.SessionMgr.Stop(sessionKey)
			return nil, fmt.Errorf("create subprocess HLS plugin: %w", plugErr)
		}
		sess.FanOut.Add(plugin)
		result.Plugin = plugin
		result.Delivery = string(delivery)
		if sp, ok := plugin.(output.ServablePlugin); ok {
			result.Servable = sp
		}
		return result, nil
	}

	pluginCfg := output.PluginConfig{
		OutputDir:        sess.OutputDir,
		IsLive:           false,
		VideoCodecParams: pipelineResult.VideoCodecParams,
		AudioCodecParams: pipelineResult.AudioCodecParams,
	}
	if len(pipelineResult.VideoExtradata) > 0 {
		pluginCfg.VideoExtradata = pipelineResult.VideoExtradata
	}
	if len(pipelineResult.AudioExtradata) > 0 {
		pluginCfg.AudioExtradata = pipelineResult.AudioExtradata
	}
	if info.Video != nil {
		pluginCfg.Video = info.Video
	}
	if len(info.AudioTracks) > 0 {
		pluginCfg.Audio = &info.AudioTracks[0]
	}

	plugin, err := deps.OutputReg.Create(delivery, pluginCfg)
	if err != nil {
		deps.SessionMgr.Stop(sessionKey)
		return nil, fmt.Errorf("create output plugin: %w", err)
	}

	sess.FanOut.Add(plugin)
	result.Plugin = plugin
	result.Delivery = string(delivery)

	if sp, ok := plugin.(output.ServablePlugin); ok {
		result.Servable = sp
	}

	return result, nil
}

func StopRecordingPlayback(deps PlaybackDeps, recordingID string) {
	deps.SessionMgr.Stop("rec:" + recordingID)
}

func resolveDecodeHWAccel(ctx context.Context, deps PlaybackDeps) string {
	if deps.SettingsStore == nil {
		return ""
	}
	if val, err := deps.SettingsStore.Get(ctx, "default_decode_hwaccel"); err == nil && val != "" {
		return val
	}
	if val, err := deps.SettingsStore.Get(ctx, "default_hwaccel"); err == nil && val != "" {
		return val
	}
	return ""
}

func resolveEncoderName(ctx context.Context, deps PlaybackDeps, codec string) string {
	if deps.SettingsStore == nil || codec == "" {
		return ""
	}
	codec = strings.ToLower(codec)
	switch codec {
	case "h264", "h265", "av1":
		if val, err := deps.SettingsStore.Get(ctx, "encoder_"+codec); err == nil && val != "" {
			return val
		}
	}
	return ""
}

func resolveDecoderName(ctx context.Context, deps PlaybackDeps, codec string) string {
	if deps.SettingsStore == nil || codec == "" {
		return ""
	}
	codec = strings.ToLower(codec)
	switch codec {
	case "h264", "h265", "av1":
		if val, err := deps.SettingsStore.Get(ctx, "decoder_"+codec); err == nil && val != "" {
			return val
		}
	case "mpeg2", "mpeg2video":
		if val, err := deps.SettingsStore.Get(ctx, "decoder_mpeg2"); err == nil && val != "" {
			return val
		}
	}
	return ""
}

func applySourceProfile(ctx context.Context, deps PlaybackDeps, stream *media.Stream, cfg *session.PipelineConfig) {
	if deps.SourceProfileStore == nil || deps.SourceConfigStore == nil || stream.SourceID == "" {
		return
	}
	sc, err := deps.SourceConfigStore.Get(ctx, stream.SourceID)
	if err != nil || sc == nil {
		return
	}
	profileID := sc.Config["source_profile_id"]
	if profileID == "" {
		return
	}
	profile, err := deps.SourceProfileStore.Get(ctx, profileID)
	if err != nil || profile == nil {
		return
	}

	if profile.Deinterlace {
		cfg.Deinterlace = true
	}
	if profile.HTTPTimeoutSec > 0 && cfg.TimeoutSec == 0 {
		cfg.TimeoutSec = profile.HTTPTimeoutSec
	}
	if profile.HTTPUserAgent != "" && cfg.UserAgent == "" {
		cfg.UserAgent = profile.HTTPUserAgent
	}
	if cfg.FormatHint == "" && profile.FormatHint != "" &&
		!strings.HasPrefix(stream.URL, "rtsp://") &&
		!strings.HasPrefix(stream.URL, "http://") &&
		!strings.HasPrefix(stream.URL, "https://") {
		cfg.FormatHint = profile.FormatHint
	}
	if profile.ProbeDurationSec > 0 && cfg.ProbeDurationSec == 0 {
		cfg.ProbeDurationSec = profile.ProbeDurationSec
	}
}

func Seek(deps PlaybackDeps, streamID string, positionMs int64) error {
	sess := deps.SessionMgr.Get(streamID)
	if sess == nil {
		return fmt.Errorf("session %s not found", streamID)
	}
	sess.SeekTo(positionMs)
	return nil
}

func createSubprocessHLSPlugin(outputDir string) (output.OutputPlugin, error) {
	return hls.NewSubprocessPlugin(outputDir)
}

func needsSubprocessFallback(outputCodec string) bool {
	if runtime.GOARCH != "arm64" {
		return false
	}
	codec := strings.ToLower(outputCodec)
	return codec == "h265" || codec == "hevc"
}
