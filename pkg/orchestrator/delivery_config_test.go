package orchestrator

import (
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
)

func buildPluginConfig(decision strategy.Decision, info *media.ProbeResult, pipelineResult *session.PipelineResult, outputDir string) output.PluginConfig {
	pluginCfg := output.PluginConfig{
		OutputDir: outputDir,
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
	} else if !decision.NeedsTranscode && info != nil && info.Video != nil && len(info.Video.Extradata) > 0 {
		pluginCfg.VideoExtradata = info.Video.Extradata
	}
	if len(pipelineResult.AudioExtradata) > 0 {
		pluginCfg.AudioExtradata = pipelineResult.AudioExtradata
	}
	if info != nil && info.Video != nil {
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
	if info != nil && len(info.AudioTracks) > 0 {
		a := info.AudioTracks[0]
		if decision.NeedsAudioTranscode && string(decision.AudioCodec) != "" {
			a.Codec = string(decision.AudioCodec)
			a.Channels = 2
			a.SampleRate = 48000
		}
		pluginCfg.Audio = &a
	}
	return pluginCfg
}

func TestAudioOverrideWhenTranscoding(t *testing.T) {
	tests := []struct {
		name                string
		needsAudioTranscode bool
		audioCodec          media.AudioCodec
		sourceChannels      int
		sourceSampleRate    int
		sourceCodec         string
		wantChannels        int
		wantSampleRate      int
		wantCodec           string
	}{
		{
			name:                "transcode to AAC forces stereo 48kHz",
			needsAudioTranscode: true,
			audioCodec:          media.AudioAAC,
			sourceChannels:      6,
			sourceSampleRate:    44100,
			sourceCodec:         "ac3",
			wantChannels:        2,
			wantSampleRate:      48000,
			wantCodec:           "aac",
		},
		{
			name:                "transcode to opus forces stereo 48kHz",
			needsAudioTranscode: true,
			audioCodec:          media.AudioOpus,
			sourceChannels:      8,
			sourceSampleRate:    96000,
			sourceCodec:         "flac",
			wantChannels:        2,
			wantSampleRate:      48000,
			wantCodec:           "opus",
		},
		{
			name:                "no audio transcode keeps original params",
			needsAudioTranscode: false,
			audioCodec:          media.AudioCopy,
			sourceChannels:      6,
			sourceSampleRate:    44100,
			sourceCodec:         "ac3",
			wantChannels:        6,
			wantSampleRate:      44100,
			wantCodec:           "ac3",
		},
		{
			name:                "no audio transcode keeps stereo AAC as-is",
			needsAudioTranscode: false,
			audioCodec:          media.AudioCopy,
			sourceChannels:      2,
			sourceSampleRate:    48000,
			sourceCodec:         "aac",
			wantChannels:        2,
			wantSampleRate:      48000,
			wantCodec:           "aac",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := strategy.Decision{
				VideoCodec:          media.VideoCopy,
				AudioCodec:          tt.audioCodec,
				NeedsAudioTranscode: tt.needsAudioTranscode,
			}
			info := &media.ProbeResult{
				Video: &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
				AudioTracks: []media.AudioTrack{{
					Codec:      tt.sourceCodec,
					Channels:   tt.sourceChannels,
					SampleRate: tt.sourceSampleRate,
				}},
			}
			pr := &session.PipelineResult{
				Info: info,
			}

			cfg := buildPluginConfig(decision, info, pr, "/tmp/test")

			if cfg.Audio == nil {
				t.Fatal("expected Audio to be set")
			}
			if cfg.Audio.Channels != tt.wantChannels {
				t.Errorf("channels: got %d, want %d", cfg.Audio.Channels, tt.wantChannels)
			}
			if cfg.Audio.SampleRate != tt.wantSampleRate {
				t.Errorf("sample_rate: got %d, want %d", cfg.Audio.SampleRate, tt.wantSampleRate)
			}
			if cfg.Audio.Codec != tt.wantCodec {
				t.Errorf("codec: got %q, want %q", cfg.Audio.Codec, tt.wantCodec)
			}
		})
	}
}

func TestVideoExtradataWhenTranscoding(t *testing.T) {
	probeExtradata := []byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42}
	encoderExtradata := []byte{0x01, 0x64, 0x00, 0x1f, 0xff}

	tests := []struct {
		name             string
		needsTranscode   bool
		pipelineExtra    []byte
		probeExtra       []byte
		wantExtra        []byte
		wantVideoCodec   string
		wantVideoNilExtra bool
	}{
		{
			name:           "transcode with encoder extradata uses encoder extradata",
			needsTranscode: true,
			pipelineExtra:  encoderExtradata,
			probeExtra:     probeExtradata,
			wantExtra:      encoderExtradata,
			wantVideoCodec: "h265",
		},
		{
			name:              "transcode without encoder extradata clears video extradata",
			needsTranscode:    true,
			pipelineExtra:     nil,
			probeExtra:        probeExtradata,
			wantExtra:         nil,
			wantVideoNilExtra: true,
			wantVideoCodec:    "h265",
		},
		{
			name:           "copy mode with probe extradata uses probe extradata",
			needsTranscode: false,
			pipelineExtra:  nil,
			probeExtra:     probeExtradata,
			wantExtra:      probeExtradata,
			wantVideoCodec: "h264",
		},
		{
			name:           "copy mode with encoder extradata prefers encoder extradata",
			needsTranscode: false,
			pipelineExtra:  encoderExtradata,
			probeExtra:     probeExtradata,
			wantExtra:      encoderExtradata,
			wantVideoCodec: "h264",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			videoCodec := media.VideoCopy
			if tt.needsTranscode {
				videoCodec = media.VideoH265
			}
			decision := strategy.Decision{
				VideoCodec:     videoCodec,
				AudioCodec:     media.AudioAAC,
				NeedsTranscode: tt.needsTranscode,
			}
			info := &media.ProbeResult{
				Video: &media.VideoInfo{
					Codec:     "h264",
					Width:     1920,
					Height:    1080,
					Extradata: tt.probeExtra,
				},
				AudioTracks: []media.AudioTrack{{Codec: "aac", Channels: 2, SampleRate: 48000}},
			}
			pr := &session.PipelineResult{
				Info:           info,
				VideoExtradata: tt.pipelineExtra,
			}

			cfg := buildPluginConfig(decision, info, pr, "/tmp/test")

			if tt.wantExtra != nil {
				if len(cfg.VideoExtradata) == 0 {
					t.Fatal("expected VideoExtradata to be set on PluginConfig")
				}
				if string(cfg.VideoExtradata) != string(tt.wantExtra) {
					t.Errorf("PluginConfig.VideoExtradata mismatch")
				}
			} else if len(cfg.VideoExtradata) > 0 {
				t.Errorf("expected empty PluginConfig.VideoExtradata, got %d bytes", len(cfg.VideoExtradata))
			}

			if cfg.Video == nil {
				t.Fatal("expected Video to be set")
			}
			if cfg.Video.Codec != tt.wantVideoCodec {
				t.Errorf("video codec: got %q, want %q", cfg.Video.Codec, tt.wantVideoCodec)
			}
			if tt.wantVideoNilExtra {
				if cfg.Video.Extradata != nil {
					t.Errorf("expected Video.Extradata to be nil, got %d bytes", len(cfg.Video.Extradata))
				}
			} else if tt.wantExtra != nil && tt.needsTranscode {
				if string(cfg.Video.Extradata) != string(tt.wantExtra) {
					t.Errorf("Video.Extradata mismatch")
				}
			}
		})
	}
}

func TestDeliveryResolution(t *testing.T) {
	tests := []struct {
		name              string
		deliveryOverride  string
		clientDelivery    string
		settingsDelivery  string
		wantDelivery      output.DeliveryMode
		wantSwitchable    bool
	}{
		{
			name:             "override takes precedence over client",
			deliveryOverride: "hls",
			clientDelivery:   "mse",
			settingsDelivery: "mse",
			wantDelivery:     output.DeliveryHLS,
		},
		{
			name:             "override takes precedence over settings default",
			deliveryOverride: "stream",
			clientDelivery:   "",
			settingsDelivery: "mse",
			wantDelivery:     output.DeliveryStream,
		},
		{
			name:             "client profile delivery used when no override",
			deliveryOverride: "",
			clientDelivery:   "hls",
			settingsDelivery: "mse",
			wantDelivery:     output.DeliveryHLS,
		},
		{
			name:             "settings default when no override and no client",
			deliveryOverride: "",
			clientDelivery:   "",
			settingsDelivery: "dash",
			wantDelivery:     output.DeliveryDASH,
		},
		{
			name:             "MSE default when no override, no client, no settings",
			deliveryOverride: "",
			clientDelivery:   "",
			settingsDelivery: "",
			wantDelivery:     output.DeliveryMSE,
		},
		{
			name:             "client 'user' delivery is switchable, falls through to settings",
			deliveryOverride: "",
			clientDelivery:   "user",
			settingsDelivery: "hls",
			wantDelivery:     output.DeliveryHLS,
			wantSwitchable:   true,
		},
		{
			name:             "override still wins even with user-switchable client",
			deliveryOverride: "stream",
			clientDelivery:   "user",
			settingsDelivery: "hls",
			wantDelivery:     output.DeliveryStream,
			wantSwitchable:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delivery := output.DeliveryMSE
			if tt.settingsDelivery != "" {
				delivery = output.DeliveryMode(tt.settingsDelivery)
			}

			deliverySwitchable := false
			if tt.clientDelivery != "" {
				if tt.clientDelivery == "user" {
					deliverySwitchable = true
				} else {
					delivery = output.DeliveryMode(tt.clientDelivery)
				}
			}
			if tt.deliveryOverride != "" {
				delivery = output.DeliveryMode(tt.deliveryOverride)
			}

			if delivery != tt.wantDelivery {
				t.Errorf("delivery: got %q, want %q", delivery, tt.wantDelivery)
			}
			if deliverySwitchable != tt.wantSwitchable {
				t.Errorf("switchable: got %v, want %v", deliverySwitchable, tt.wantSwitchable)
			}
		})
	}
}

func TestPluginConfigCodecParamsCopyMode(t *testing.T) {
	videoCP := "mock-video-codec-params"
	audioCP := "mock-audio-codec-params"

	decision := strategy.Decision{
		VideoCodec:          media.VideoCopy,
		AudioCodec:          media.AudioCopy,
		NeedsTranscode:      false,
		NeedsAudioTranscode: false,
	}
	info := &media.ProbeResult{
		Video:       &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
		AudioTracks: []media.AudioTrack{{Codec: "ac3", Channels: 6, SampleRate: 48000}},
	}
	pr := &session.PipelineResult{
		Info:             info,
		VideoCodecParams: videoCP,
		AudioCodecParams: audioCP,
	}

	cfg := buildPluginConfig(decision, info, pr, "/tmp/test")

	if cfg.VideoCodecParams != videoCP {
		t.Errorf("expected VideoCodecParams to be passed through in copy mode")
	}
	if cfg.AudioCodecParams != audioCP {
		t.Errorf("expected AudioCodecParams to be passed through in copy mode")
	}
}

func TestPluginConfigCodecParamsTranscodeMode(t *testing.T) {
	videoCP := "mock-video-codec-params"
	audioCP := "mock-audio-codec-params"

	decision := strategy.Decision{
		VideoCodec:          media.VideoH265,
		AudioCodec:          media.AudioAAC,
		NeedsTranscode:      true,
		NeedsAudioTranscode: true,
	}
	info := &media.ProbeResult{
		Video:       &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
		AudioTracks: []media.AudioTrack{{Codec: "ac3", Channels: 6, SampleRate: 48000}},
	}
	pr := &session.PipelineResult{
		Info:             info,
		VideoCodecParams: videoCP,
		AudioCodecParams: audioCP,
		VideoExtradata:   []byte{0x01, 0x02},
		AudioExtradata:   []byte{0x03, 0x04},
	}

	cfg := buildPluginConfig(decision, info, pr, "/tmp/test")

	if cfg.VideoCodecParams != nil {
		t.Errorf("expected VideoCodecParams to be nil in transcode mode, got %v", cfg.VideoCodecParams)
	}
	if cfg.AudioCodecParams != nil {
		t.Errorf("expected AudioCodecParams to be nil in transcode mode, got %v", cfg.AudioCodecParams)
	}

	if cfg.Video == nil {
		t.Fatal("expected Video to be set")
	}
	if cfg.Video.Codec != "h265" {
		t.Errorf("video codec: got %q, want %q", cfg.Video.Codec, "h265")
	}

	if cfg.Audio == nil {
		t.Fatal("expected Audio to be set")
	}
	if cfg.Audio.Codec != "aac" {
		t.Errorf("audio codec: got %q, want %q", cfg.Audio.Codec, "aac")
	}
	if cfg.Audio.Channels != 2 {
		t.Errorf("audio channels: got %d, want 2", cfg.Audio.Channels)
	}
	if cfg.Audio.SampleRate != 48000 {
		t.Errorf("audio sample_rate: got %d, want 48000", cfg.Audio.SampleRate)
	}
}

func TestPluginConfigMSECompleteness(t *testing.T) {
	decision := strategy.Decision{
		VideoCodec:          media.VideoH264,
		AudioCodec:          media.AudioAAC,
		NeedsTranscode:      true,
		NeedsAudioTranscode: true,
	}
	info := &media.ProbeResult{
		Video:       &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080, Extradata: []byte{0x01}},
		AudioTracks: []media.AudioTrack{{Codec: "ac3", Channels: 6, SampleRate: 48000}},
	}
	pr := &session.PipelineResult{
		Info:           info,
		VideoExtradata: []byte{0x01, 0x64, 0x00},
		AudioExtradata: []byte{0x12, 0x10},
	}

	cfg := buildPluginConfig(decision, info, pr, "/tmp/mse-output")

	if cfg.OutputDir == "" {
		t.Error("MSE: OutputDir must be set")
	}
	if cfg.Video == nil {
		t.Error("MSE: Video must be set")
	}
	if cfg.Audio == nil {
		t.Error("MSE: Audio must be set")
	}
	if len(cfg.VideoExtradata) == 0 {
		t.Error("MSE: VideoExtradata must be set when transcoding")
	}
}

func TestPluginConfigHLSCompleteness(t *testing.T) {
	decision := strategy.Decision{
		VideoCodec:          media.VideoCopy,
		AudioCodec:          media.AudioAAC,
		NeedsTranscode:      false,
		NeedsAudioTranscode: true,
	}
	probeExtra := []byte{0x00, 0x00, 0x00, 0x01, 0x67}
	info := &media.ProbeResult{
		Video:       &media.VideoInfo{Codec: "h264", Width: 1280, Height: 720, Extradata: probeExtra},
		AudioTracks: []media.AudioTrack{{Codec: "mp2", Channels: 2, SampleRate: 48000}},
	}
	pr := &session.PipelineResult{
		Info:             info,
		VideoCodecParams: "hls-video-cp",
		AudioCodecParams: "hls-audio-cp",
	}

	cfg := buildPluginConfig(decision, info, pr, "/tmp/hls-output")

	if cfg.OutputDir == "" {
		t.Error("HLS: OutputDir must be set")
	}
	if cfg.Video == nil {
		t.Error("HLS: Video must be set")
	}
	if cfg.Audio == nil {
		t.Error("HLS: Audio must be set")
	}

	if cfg.VideoCodecParams != "hls-video-cp" {
		t.Error("HLS copy mode: VideoCodecParams must be passed through")
	}

	if cfg.Video.Codec != "h264" {
		t.Errorf("HLS copy mode: video codec should remain %q, got %q", "h264", cfg.Video.Codec)
	}
	if string(cfg.Video.Extradata) != string(probeExtra) {
		t.Error("HLS copy mode: Video.Extradata should come from probe")
	}
	if string(cfg.VideoExtradata) != string(probeExtra) {
		t.Error("HLS copy mode: PluginConfig.VideoExtradata should come from probe when no encoder extradata")
	}

	if cfg.Audio.Codec != "aac" {
		t.Errorf("HLS audio transcode: codec should be %q, got %q", "aac", cfg.Audio.Codec)
	}
	if cfg.Audio.Channels != 2 {
		t.Errorf("HLS audio transcode: channels should be 2, got %d", cfg.Audio.Channels)
	}
}

func TestPluginConfigStreamCompleteness(t *testing.T) {
	decision := strategy.Decision{
		VideoCodec:          media.VideoCopy,
		AudioCodec:          media.AudioCopy,
		NeedsTranscode:      false,
		NeedsAudioTranscode: false,
	}
	info := &media.ProbeResult{
		Video:       &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
		AudioTracks: []media.AudioTrack{{Codec: "aac", Channels: 2, SampleRate: 48000}},
	}
	pr := &session.PipelineResult{
		Info:             info,
		VideoCodecParams: "stream-vcp",
		AudioCodecParams: "stream-acp",
	}

	cfg := buildPluginConfig(decision, info, pr, "/tmp/stream-output")

	if cfg.Video == nil {
		t.Error("Stream: Video must be set")
	}
	if cfg.Audio == nil {
		t.Error("Stream: Audio must be set")
	}
	if cfg.VideoCodecParams == nil {
		t.Error("Stream copy mode: VideoCodecParams must be set")
	}
	if cfg.AudioCodecParams == nil {
		t.Error("Stream copy mode: AudioCodecParams must be set")
	}
}

func TestPluginConfigNoAudioTracks(t *testing.T) {
	decision := strategy.Decision{
		VideoCodec:     media.VideoCopy,
		NeedsTranscode: false,
	}
	info := &media.ProbeResult{
		Video: &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}
	pr := &session.PipelineResult{Info: info}

	cfg := buildPluginConfig(decision, info, pr, "/tmp/test")

	if cfg.Audio != nil {
		t.Error("expected Audio to be nil when no audio tracks in probe")
	}
	if cfg.Video == nil {
		t.Error("expected Video to be set")
	}
}

func TestPluginConfigNoVideo(t *testing.T) {
	decision := strategy.Decision{
		AudioCodec:          media.AudioAAC,
		NeedsAudioTranscode: true,
	}
	info := &media.ProbeResult{
		AudioTracks: []media.AudioTrack{{Codec: "mp2", Channels: 2, SampleRate: 44100}},
	}
	pr := &session.PipelineResult{Info: info}

	cfg := buildPluginConfig(decision, info, pr, "/tmp/test")

	if cfg.Video != nil {
		t.Error("expected Video to be nil when no video in probe")
	}
	if cfg.Audio == nil {
		t.Fatal("expected Audio to be set")
	}
	if cfg.Audio.Codec != "aac" {
		t.Errorf("audio codec: got %q, want %q", cfg.Audio.Codec, "aac")
	}
}

func TestPluginConfigVideoOnlyTranscode(t *testing.T) {
	decision := strategy.Decision{
		VideoCodec:          media.VideoH265,
		AudioCodec:          media.AudioCopy,
		NeedsTranscode:      true,
		NeedsAudioTranscode: false,
	}
	info := &media.ProbeResult{
		Video:       &media.VideoInfo{Codec: "h264", Width: 3840, Height: 2160, Extradata: []byte{0xAA}},
		AudioTracks: []media.AudioTrack{{Codec: "aac", Channels: 2, SampleRate: 48000}},
	}
	encoderExtra := []byte{0x01, 0x21, 0x00}
	pr := &session.PipelineResult{
		Info:             info,
		VideoExtradata:   encoderExtra,
		AudioCodecParams: "passthrough-audio-cp",
	}

	cfg := buildPluginConfig(decision, info, pr, "/tmp/test")

	if cfg.Video == nil {
		t.Fatal("expected Video to be set")
	}
	if cfg.Video.Codec != "h265" {
		t.Errorf("video codec: got %q, want %q", cfg.Video.Codec, "h265")
	}
	if string(cfg.Video.Extradata) != string(encoderExtra) {
		t.Error("video extradata should be encoder extradata when transcoding")
	}

	if cfg.Audio == nil {
		t.Fatal("expected Audio to be set")
	}
	if cfg.Audio.Codec != "aac" {
		t.Errorf("audio codec should remain %q in copy mode, got %q", "aac", cfg.Audio.Codec)
	}
	if cfg.Audio.Channels != 2 {
		t.Errorf("audio channels should remain 2 in copy mode, got %d", cfg.Audio.Channels)
	}
	if cfg.Audio.SampleRate != 48000 {
		t.Errorf("audio sample_rate should remain 48000 in copy mode, got %d", cfg.Audio.SampleRate)
	}

	if cfg.VideoCodecParams != nil {
		t.Error("VideoCodecParams should be nil when video is transcoded")
	}
	if cfg.AudioCodecParams != "passthrough-audio-cp" {
		t.Error("AudioCodecParams should be passed through when audio is not transcoded")
	}
}

func TestPluginConfigAudioOnlyTranscode(t *testing.T) {
	decision := strategy.Decision{
		VideoCodec:          media.VideoCopy,
		AudioCodec:          media.AudioAAC,
		NeedsTranscode:      false,
		NeedsAudioTranscode: true,
	}
	info := &media.ProbeResult{
		Video:       &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080, Extradata: []byte{0xBB}},
		AudioTracks: []media.AudioTrack{{Codec: "ac3", Channels: 6, SampleRate: 48000}},
	}
	pr := &session.PipelineResult{
		Info:             info,
		VideoCodecParams: "passthrough-video-cp",
	}

	cfg := buildPluginConfig(decision, info, pr, "/tmp/test")

	if cfg.VideoCodecParams != "passthrough-video-cp" {
		t.Error("VideoCodecParams should be passed through when video is not transcoded")
	}
	if cfg.AudioCodecParams != nil {
		t.Error("AudioCodecParams should be nil when audio is transcoded")
	}

	if cfg.Video == nil {
		t.Fatal("expected Video to be set")
	}
	if cfg.Video.Codec != "h264" {
		t.Errorf("video codec should remain %q in copy mode, got %q", "h264", cfg.Video.Codec)
	}
	if string(cfg.Video.Extradata) != string([]byte{0xBB}) {
		t.Error("video extradata should come from probe in copy mode")
	}

	if cfg.Audio == nil {
		t.Fatal("expected Audio to be set")
	}
	if cfg.Audio.Codec != "aac" {
		t.Errorf("audio codec: got %q, want %q", cfg.Audio.Codec, "aac")
	}
	if cfg.Audio.Channels != 2 {
		t.Errorf("audio channels: got %d, want 2", cfg.Audio.Channels)
	}
	if cfg.Audio.SampleRate != 48000 {
		t.Errorf("audio sample_rate: got %d, want 48000", cfg.Audio.SampleRate)
	}
}

func TestPluginConfigOriginalInfoNotMutated(t *testing.T) {
	decision := strategy.Decision{
		VideoCodec:          media.VideoH265,
		AudioCodec:          media.AudioAAC,
		NeedsTranscode:      true,
		NeedsAudioTranscode: true,
	}
	info := &media.ProbeResult{
		Video:       &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080, Extradata: []byte{0xAA}},
		AudioTracks: []media.AudioTrack{{Codec: "ac3", Channels: 6, SampleRate: 44100}},
	}
	pr := &session.PipelineResult{
		Info:           info,
		VideoExtradata: []byte{0x01, 0x02},
	}

	_ = buildPluginConfig(decision, info, pr, "/tmp/test")

	if info.Video.Codec != "h264" {
		t.Errorf("original video codec mutated: got %q, want %q", info.Video.Codec, "h264")
	}
	if string(info.Video.Extradata) != string([]byte{0xAA}) {
		t.Error("original video extradata mutated")
	}
	if info.AudioTracks[0].Codec != "ac3" {
		t.Errorf("original audio codec mutated: got %q, want %q", info.AudioTracks[0].Codec, "ac3")
	}
	if info.AudioTracks[0].Channels != 6 {
		t.Errorf("original audio channels mutated: got %d, want 6", info.AudioTracks[0].Channels)
	}
	if info.AudioTracks[0].SampleRate != 44100 {
		t.Errorf("original audio sample_rate mutated: got %d, want 44100", info.AudioTracks[0].SampleRate)
	}
}
