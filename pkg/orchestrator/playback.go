package orchestrator

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/codec"
	"github.com/mcnairstudios/mediahub/pkg/hwcaps"
	"github.com/mcnairstudios/mediahub/pkg/connectivity"
	"github.com/mcnairstudios/mediahub/pkg/connectivity/wg"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	webrtcplugin "github.com/mcnairstudios/mediahub/pkg/output/webrtc"
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
	DeliveryOverride   string
	VideoCodecOverride string
	AudioCodecOverride string
}

type PlaybackResult struct {
	Session             *session.Session
	Plugin              output.OutputPlugin
	Servable            output.ServablePlugin
	Decision            strategy.Decision
	IsNew               bool
	Delivery            string
	DeliverySwitchable  bool
	ProbeInfo           *media.ProbeResult
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
		log.Printf("detected client: %s video=%s audio=%s delivery=%s hwaccel=%s",
			detectedClient.Name, detectedClient.Profile.VideoCodec,
			detectedClient.Profile.AudioCodec, detectedClient.Profile.Delivery,
			detectedClient.Profile.HWAccel)
	}

	// Determine delivery mode early for constraints
	earlyDelivery := output.DeliveryMode(deps.DeliveryOverride)
	if earlyDelivery == "" && detectedClient != nil && detectedClient.Profile.Delivery != "" && detectedClient.Profile.Delivery != "user" {
		earlyDelivery = output.DeliveryMode(detectedClient.Profile.Delivery)
	}

	// Build codec.Input from all sources and resolve ONCE
	codecInput := codec.Input{
		Defaults:            codec.Preference{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4"},
		Settings:            gatherSettingsPreference(ctx, deps),
		ClientProfile:       gatherClientPreference(detectedClient),
		ClientOverride:      gatherClientOverride(deps),
		DeliveryConstraints: codec.DeliveryConstraints(string(earlyDelivery)),
	}
	resolved := codec.Resolve(codecInput)

	audioLanguage := resolveAudioLanguage(ctx, deps)

	out := strategy.Output{
		VideoCodec:   resolved.VideoCodec,
		AudioCodec:   resolved.AudioCodec,
		Container:    resolved.Container,
		HWAccel:      resolved.HWAccel,
		OutputHeight: resolved.OutputHeight,
		MaxBitDepth:  resolved.MaxBitDepth,
	}

	decision := deps.Strategy(in, out)
	log.Printf("strategy: stream=%s in_video=%s in_audio=%s out_video=%s out_audio=%s transcode=%v audio_transcode=%v hwaccel=%s",
		stream.Name, in.VideoCodec, in.AudioCodec, decision.VideoCodec, decision.AudioCodec,
		decision.NeedsTranscode, decision.NeedsAudioTranscode, decision.HWAccel)

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
		if deps.DeliveryOverride != "" && deps.DeliveryOverride != sess.Delivery {
			deps.SessionMgr.Stop(stream.ID)
			sess, isNew, err = deps.SessionMgr.GetOrCreate(ctx, stream.ID, stream.URL, stream.Name)
			if err != nil {
				return nil, fmt.Errorf("recreate session: %w", err)
			}
			result.Session = sess
			result.IsNew = isNew
		} else {
			result.Delivery = sess.Delivery
			return result, nil
		}
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
	decoderName := resolveDecoderName(ctx, deps, stream.VideoCodec)

	audioCodec := string(decision.AudioCodec)
	isWebRTC := earlyDelivery == output.DeliveryWebRTC

	// Apply delivery constraints: force transcode and disable HW decode
	if resolved.ForceTranscode {
		decision.NeedsTranscode = true
		decision.NeedsAudioTranscode = true
	}
	if resolved.DisableDecodeHW {
		decodeHWAccel = ""
		decoderName = ""
	}

	buildPipeCfg := func(videoCodec string) session.PipelineConfig {
		encoderName := resolveEncoderName(ctx, deps, videoCodec)
		cfg := session.PipelineConfig{
			StreamURL:           pipelineURL,
			StreamID:            stream.ID,
			UserAgent:           deps.UserAgent,
			AudioLanguage:       audioLanguage,
			NeedsTranscode:      decision.NeedsTranscode,
			NeedsAudioTranscode: decision.NeedsAudioTranscode,
			OutputCodec:         videoCodec,
			OutputAudioCodec:    audioCodec,
			HWAccel:             decision.HWAccel,
			DecodeHWAccel:       decodeHWAccel,
			Deinterlace:         decision.Deinterlace,
			OutputHeight:        out.OutputHeight,
			MaxBitDepth:         out.MaxBitDepth,
			EncoderName:         encoderName,
			DecoderName:         decoderName,
			Bitrate:             resolved.Bitrate,
			IsLive:              true,
			CachedStreamInfo:    cachedProbe,
		}
		applySourceProfile(ctx, deps, stream, &cfg)
		return cfg
	}

	runner := deps.PipelineRunner
	if runner == nil {
		runner = deps.SessionMgr.RunPipeline
	}

	var pipelineResult *session.PipelineResult

	if isWebRTC {
		// Defer pipeline start until WHEP offer arrives with browser SDP.
		// Register callback that parses SDP, picks best codec, starts pipeline.
		sess.SetOnSDPOffer(func(sdp string) (string, error) {
			browserCodecs := hwcaps.ParseSDPVideoCodecs(sdp)
			log.Printf("webrtc: SDP negotiation browser=%v", browserCodecs)

			// Re-resolve with browser's allowed video codecs
			sdpInput := codecInput
			sdpInput.DeliveryConstraints.AllowedVideoCodecs = browserCodecs
			sdpResolved := codec.Resolve(sdpInput)
			negotiated := sdpResolved.VideoCodec
			log.Printf("webrtc: negotiated codec=%s", negotiated)

			decision.VideoCodec = media.VideoCodec(negotiated)
			decision.NeedsTranscode = true

			pipeCfg := buildPipeCfg(negotiated)
			pr, err := runner(sess, pipeCfg)
			if err != nil {
				deps.SessionMgr.Stop(stream.ID)
				return "", fmt.Errorf("pipeline failed: %w", err)
			}
			if pr.Info != nil {
				if deps.ProbeCache != nil && cachedProbe == nil {
					_ = deps.ProbeCache.Set(stream.URL, pr.Info)
				}
				if stream.VideoCodec == "" {
					updateStreamFromProbe(ctx, deps.StreamStore, stream, pr.Info)
				}
			}
			return negotiated, nil
		})
	} else {
		// Non-WebRTC: start pipeline immediately
		pipeCfg := buildPipeCfg(string(decision.VideoCodec))
		var err error
		pipelineResult, err = runner(sess, pipeCfg)
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
		result.ProbeInfo = pipelineResult.Info

		if pipelineResult.Info != nil && stream.VideoCodec == "" {
			updateStreamFromProbe(ctx, deps.StreamStore, stream, pipelineResult.Info)
		}
	}

	delivery := resolveDelivery(ctx, deps)
	deliverySwitchable := false
	if detectedClient != nil && detectedClient.Profile.Delivery != "" {
		if detectedClient.Profile.Delivery == "user" {
			deliverySwitchable = true
		} else {
			delivery = output.DeliveryMode(detectedClient.Profile.Delivery)
		}
	}
	if deps.DeliveryOverride != "" {
		delivery = output.DeliveryMode(deps.DeliveryOverride)
	}
	sess.Delivery = string(delivery)
	result.DeliverySwitchable = deliverySwitchable

	// Determine if source is VOD (has known duration) or live
	isLive := true
	if !isWebRTC && pipelineResult != nil && pipelineResult.Info != nil && pipelineResult.Info.DurationMs > 0 {
		isLive = false
	}

	pluginCfg := output.PluginConfig{
		OutputDir:      sess.OutputDir,
		OutputFilePath: filepath.Join(sess.OutputDir, "source.ts"),
		OutputFormat:   "mpegts",
		IsLive:         isLive,
	}

	info := result.ProbeInfo
	if isWebRTC {
		// WebRTC: pipeline deferred — use placeholder probe info for plugin creation.
		// Actual codec will be set after SDP negotiation.
		pluginCfg.Video = &media.VideoInfo{Codec: resolved.VideoCodec, Width: 1920, Height: 1080}
		pluginCfg.Audio = &media.AudioTrack{Codec: resolved.AudioCodec, Channels: 2, SampleRate: 48000}
	} else {
		// Non-WebRTC: pipeline already ran — populate from pipeline result
		if !decision.NeedsTranscode {
			pluginCfg.VideoCodecParams = pipelineResult.VideoCodecParams
		}
		if !decision.NeedsAudioTranscode {
			pluginCfg.AudioCodecParams = pipelineResult.AudioCodecParams
		}
		if len(pipelineResult.VideoExtradata) > 0 {
			pluginCfg.VideoExtradata = pipelineResult.VideoExtradata
		} else if !decision.NeedsTranscode && info != nil && info.Video != nil && len(info.Video.Extradata) > 0 {
			pluginCfg.VideoExtradata = info.Video.Extradata
		}
		if len(pipelineResult.AudioExtradata) > 0 {
			pluginCfg.AudioExtradata = pipelineResult.AudioExtradata
		}
		// Pass encoder's ToCodecParameters functions for correct muxer setup
		pluginCfg.CopyVideoParams = pipelineResult.CopyVideoParams
		pluginCfg.CopyAudioParams = pipelineResult.CopyAudioParams
		if info != nil && info.Video != nil {
			v := *info.Video
			if decision.NeedsTranscode && string(decision.VideoCodec) != "" {
				v.Codec = string(decision.VideoCodec)
				v.Extradata = nil
			}
			if decision.Deinterlace && v.Interlaced {
				v.Interlaced = false
			}
			pluginCfg.Video = &v
		}
		if info != nil && len(info.AudioTracks) > 0 {
			a := info.AudioTracks[0]
			if decision.NeedsAudioTranscode && string(decision.AudioCodec) != "" {
				a.Codec = string(decision.AudioCodec)
				a.Channels = 2
				a.SampleRate = 48000
			}
			pluginCfg.Audio = &a
		}
	}

	plugin, err := deps.OutputReg.Create(delivery, pluginCfg)
	if err != nil {
		deps.SessionMgr.Stop(stream.ID)
		return nil, fmt.Errorf("create output plugin: %w", err)
	}

	sess.FanOut.Add(plugin)
	result.Plugin = plugin
	result.Delivery = string(delivery)

	// Wire SDP negotiation callback to WebRTC plugin
	if isWebRTC {
		if wp, ok := plugin.(*webrtcplugin.Plugin); ok {
			wp.SetSDPNegotiator(sess.OnSDPOffer)
		}
	}

	if sp, ok := plugin.(output.ServablePlugin); ok {
		result.Servable = sp
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("record plugin panic: %v", r)
			}
		}()
		recCfg := pluginCfg
		recExt := media.ContainerFileExtension(decision.Container)
		recFmt := media.ContainerOutputFormat(decision.Container)
		recCfg.OutputFilePath = filepath.Join(sess.OutputDir, "source"+recExt)
		recCfg.OutputFormat = recFmt
		if decision.NeedsTranscode {
			recCfg.VideoCodecParams = pipelineResult.VideoCodecParams
		}
		if decision.NeedsAudioTranscode {
			recCfg.AudioCodecParams = pipelineResult.AudioCodecParams
		}
		recPlugin, recErr := deps.OutputReg.Create(output.DeliveryRecord, recCfg)
		if recErr != nil {
			log.Printf("record plugin: %v", recErr)
		} else if recPlugin != nil {
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

	var detectedClient *client.Client
	if deps.Detector != nil {
		detectedClient = deps.Detector.Detect(port, headers)
	}

	// Use codec.Resolve for recording playback too
	codecInput := codec.Input{
		Defaults:       codec.Preference{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4"},
		Settings:       gatherSettingsPreference(ctx, deps),
		ClientProfile:  gatherClientPreference(detectedClient),
		ClientOverride: gatherClientOverride(deps),
	}
	resolved := codec.Resolve(codecInput)
	audioLanguage := resolveAudioLanguage(ctx, deps)

	out := strategy.Output{
		VideoCodec:   resolved.VideoCodec,
		AudioCodec:   resolved.AudioCodec,
		Container:    resolved.Container,
		HWAccel:      resolved.HWAccel,
		OutputHeight: resolved.OutputHeight,
		MaxBitDepth:  resolved.MaxBitDepth,
	}

	in := strategy.Input{
		VideoCodec: resolved.VideoCodec,
		AudioCodec: resolved.AudioCodec,
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
		OutputHeight:        resolved.OutputHeight,
		MaxBitDepth:         resolved.MaxBitDepth,
		EncoderName:         encoderName,
		DecoderName:         decoderName,
		Bitrate:             resolved.Bitrate,
	}

	runner := deps.PipelineRunner
	if runner == nil {
		runner = deps.SessionMgr.RunPipeline
	}
	pipelineResult, err := runner(sess, pipeCfg)
	if err != nil {
		deps.SessionMgr.Stop(sessionKey)
		return nil, fmt.Errorf("pipeline failed for recording %q (%s): %w", title, filePath, err)
	}
	info := pipelineResult.Info
	result.ProbeInfo = info

	delivery := resolveDelivery(ctx, deps)
	deliverySwitchable := false
	if detectedClient != nil && detectedClient.Profile.Delivery != "" {
		if detectedClient.Profile.Delivery == "user" {
			deliverySwitchable = true
		} else {
			delivery = output.DeliveryMode(detectedClient.Profile.Delivery)
		}
	}
	if deps.DeliveryOverride != "" {
		delivery = output.DeliveryMode(deps.DeliveryOverride)
	}
	sess.Delivery = string(delivery)
	result.DeliverySwitchable = deliverySwitchable

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
		v := *info.Video
		if decision.NeedsTranscode && string(decision.VideoCodec) != "" {
			v.Codec = string(decision.VideoCodec)
			if len(pipelineResult.VideoExtradata) > 0 {
				v.Extradata = pipelineResult.VideoExtradata
			} else {
				v.Extradata = nil
			}
		}
		pluginCfg.Video = &v
	}
	if len(info.AudioTracks) > 0 {
		a := info.AudioTracks[0]
		if decision.NeedsAudioTranscode && string(decision.AudioCodec) != "" {
			a.Codec = string(decision.AudioCodec)
			a.Channels = 2
			a.SampleRate = 48000
		}
		pluginCfg.Audio = &a
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

// gatherSettingsPreference extracts codec preferences from the settings store.
func gatherSettingsPreference(ctx context.Context, deps PlaybackDeps) codec.Preference {
	p := codec.Preference{}
	if deps.SettingsStore == nil {
		return p
	}
	if hw, err := deps.SettingsStore.Get(ctx, "default_hwaccel"); err == nil && hw != "" {
		p.HWAccel = hw
	}
	if mbd, err := deps.SettingsStore.Get(ctx, "default_max_bit_depth"); err == nil && mbd != "" {
		if v, err := strconv.Atoi(mbd); err == nil && v > 0 {
			p.MaxBitDepth = v
		}
	}
	// default_video_codec: used as the transcode target when the strategy
	// determines transcoding is needed but the profile doesn't specify a codec.
	// Kept as-is (including "auto") — resolution of "auto" happens after all
	// layers are merged in codec.Resolve.
	if dvc, err := deps.SettingsStore.Get(ctx, "default_video_codec"); err == nil && dvc != "" && dvc != "copy" {
		p.VideoCodec = dvc
	}
	return p
}

// gatherClientPreference extracts codec preferences from a detected client profile.
func gatherClientPreference(c *client.Client) codec.Preference {
	p := codec.Preference{}
	if c == nil {
		return p
	}
	prof := c.Profile
	if prof.VideoCodec != "" {
		p.VideoCodec = prof.VideoCodec
	}
	if prof.AudioCodec != "" {
		p.AudioCodec = prof.AudioCodec
	}
	if prof.Container != "" {
		p.Container = prof.Container
	}
	if prof.HWAccel != "" {
		p.HWAccel = prof.HWAccel
	}
	if prof.OutputHeight > 0 {
		p.OutputHeight = prof.OutputHeight
	}
	if prof.Bitrate > 0 {
		p.Bitrate = prof.Bitrate
	}
	return p
}

// gatherClientOverride builds a Preference from explicit API-level codec overrides.
func gatherClientOverride(deps PlaybackDeps) codec.Preference {
	p := codec.Preference{}
	if deps.VideoCodecOverride != "" {
		p.VideoCodec = deps.VideoCodecOverride
	}
	if deps.AudioCodecOverride != "" {
		p.AudioCodec = deps.AudioCodecOverride
	}
	return p
}

// resolveAudioLanguage reads the audio language setting.
func resolveAudioLanguage(ctx context.Context, deps PlaybackDeps) string {
	if deps.SettingsStore == nil {
		return ""
	}
	if al, err := deps.SettingsStore.Get(ctx, "audio_language"); err == nil && al != "" {
		return al
	}
	return ""
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

func updateStreamFromProbe(ctx context.Context, ss store.StreamStore, stream *media.Stream, info *media.ProbeResult) {
	if ss == nil {
		return
	}
	changed := false
	if info.Video != nil {
		if info.Video.Codec != "" && stream.VideoCodec == "" {
			stream.VideoCodec = info.Video.Codec
			changed = true
		}
		if info.Video.Width > 0 && stream.Width == 0 {
			stream.Width = info.Video.Width
			changed = true
		}
		if info.Video.Height > 0 && stream.Height == 0 {
			stream.Height = info.Video.Height
			changed = true
		}
		if info.Video.Interlaced && !stream.Interlaced {
			stream.Interlaced = true
			changed = true
		}
		if info.Video.BitDepth > 0 && stream.BitDepth == 0 {
			stream.BitDepth = info.Video.BitDepth
			changed = true
		}
	}
	if len(info.AudioTracks) > 0 && stream.AudioCodec == "" {
		stream.AudioCodec = info.AudioTracks[0].Codec
		changed = true
	}
	if changed {
		_ = ss.BulkUpsert(ctx, []media.Stream{*stream})
	}
}

