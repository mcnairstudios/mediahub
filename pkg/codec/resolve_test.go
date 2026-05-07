package codec

import (
	"testing"
)

func TestResolve_DefaultsOnly(t *testing.T) {
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "copy",
			AudioCodec: "aac",
			Container:  "mp4",
		},
	})
	if r.VideoCodec != "copy" {
		t.Errorf("expected copy, got %s", r.VideoCodec)
	}
	if r.AudioCodec != "aac" {
		t.Errorf("expected aac, got %s", r.AudioCodec)
	}
	if r.Container != "mp4" {
		t.Errorf("expected mp4, got %s", r.Container)
	}
}

func TestResolve_SettingsOverride(t *testing.T) {
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "copy",
			AudioCodec: "aac",
			Container:  "mp4",
		},
		Settings: Preference{
			VideoCodec: "h264",
			HWAccel:    "vaapi",
		},
	})
	if r.VideoCodec != "h264" {
		t.Errorf("expected h264, got %s", r.VideoCodec)
	}
	if r.HWAccel != "vaapi" {
		t.Errorf("expected vaapi, got %s", r.HWAccel)
	}
	// Audio should still come from defaults
	if r.AudioCodec != "aac" {
		t.Errorf("expected aac, got %s", r.AudioCodec)
	}
}

func TestResolve_ClientProfileOverridesSettings(t *testing.T) {
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "copy",
			AudioCodec: "aac",
			Container:  "mp4",
		},
		Settings: Preference{
			VideoCodec: "h264",
		},
		ClientProfile: Preference{
			VideoCodec: "h265",
			Container:  "mkv",
		},
	})
	if r.VideoCodec != "h265" {
		t.Errorf("expected h265, got %s", r.VideoCodec)
	}
	if r.Container != "mkv" {
		t.Errorf("expected mkv, got %s", r.Container)
	}
}

func TestResolve_ClientOverrideWins(t *testing.T) {
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "copy",
			AudioCodec: "aac",
			Container:  "mp4",
		},
		Settings: Preference{
			VideoCodec: "h264",
		},
		ClientProfile: Preference{
			VideoCodec: "h265",
		},
		ClientOverride: Preference{
			VideoCodec: "av1",
			AudioCodec: "opus",
		},
	})
	if r.VideoCodec != "av1" {
		t.Errorf("expected av1, got %s", r.VideoCodec)
	}
	if r.AudioCodec != "opus" {
		t.Errorf("expected opus, got %s", r.AudioCodec)
	}
}

func TestResolve_DeliveryConstraintsForcesAudio(t *testing.T) {
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "h264",
			AudioCodec: "aac",
			Container:  "mp4",
		},
		DeliveryConstraints: DeliveryConstraints("webrtc"),
	})
	if r.AudioCodec != "opus" {
		t.Errorf("expected opus for WebRTC, got %s", r.AudioCodec)
	}
	if !r.ForceTranscode {
		t.Error("expected ForceTranscode for WebRTC")
	}
	if !r.DisableDecodeHW {
		t.Error("expected DisableDecodeHW for WebRTC")
	}
}

func TestResolve_DeliveryConstraintsFilterVideo(t *testing.T) {
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "av1",
			AudioCodec: "aac",
		},
		DeliveryConstraints: Constraints{
			AllowedVideoCodecs: map[string]bool{"h264": true, "h265": true},
		},
	})
	// av1 is not in allowed set, should fall back
	if r.VideoCodec == "av1" {
		t.Errorf("expected av1 to be filtered out, got %s", r.VideoCodec)
	}
	// Should pick one of the allowed codecs
	if r.VideoCodec != "h264" && r.VideoCodec != "h265" {
		t.Errorf("expected h264 or h265, got %s", r.VideoCodec)
	}
}

func TestResolve_AllowedVideoCodecsPassesThrough(t *testing.T) {
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "h264",
			AudioCodec: "aac",
		},
		DeliveryConstraints: Constraints{
			AllowedVideoCodecs: map[string]bool{"h264": true, "h265": true},
		},
	})
	// h264 is in allowed set, should pass through
	if r.VideoCodec != "h264" {
		t.Errorf("expected h264, got %s", r.VideoCodec)
	}
}

func TestResolve_CopyProfileNotOverriddenBySettings(t *testing.T) {
	// If the client profile says "copy", the settings "auto" should not override it
	// because ClientProfile comes AFTER Settings in the layering.
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "copy",
			AudioCodec: "aac",
			Container:  "mp4",
		},
		Settings: Preference{
			VideoCodec: "h264",
		},
		ClientProfile: Preference{
			VideoCodec: "copy",
		},
	})
	if r.VideoCodec != "copy" {
		t.Errorf("expected copy from profile to win over settings, got %s", r.VideoCodec)
	}
}

func TestResolve_EmptyProfileInheritsSettings(t *testing.T) {
	// If the client profile has no video codec opinion, settings value flows through
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "copy",
			AudioCodec: "aac",
			Container:  "mp4",
		},
		Settings: Preference{
			VideoCodec: "h265",
		},
		ClientProfile: Preference{
			// No video codec opinion
			AudioCodec: "opus",
		},
	})
	if r.VideoCodec != "h265" {
		t.Errorf("expected h265 from settings, got %s", r.VideoCodec)
	}
	if r.AudioCodec != "opus" {
		t.Errorf("expected opus from profile, got %s", r.AudioCodec)
	}
}

func TestResolve_OutputHeightLayering(t *testing.T) {
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "copy",
		},
		ClientProfile: Preference{
			OutputHeight: 720,
		},
	})
	if r.OutputHeight != 720 {
		t.Errorf("expected 720, got %d", r.OutputHeight)
	}
}

func TestResolve_BitrateLayering(t *testing.T) {
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "copy",
		},
		ClientProfile: Preference{
			Bitrate: 5000,
		},
		ClientOverride: Preference{
			Bitrate: 8000,
		},
	})
	if r.Bitrate != 8000 {
		t.Errorf("expected 8000 from override, got %d", r.Bitrate)
	}
}

func TestResolve_NonWebRTCNoConstraints(t *testing.T) {
	c := DeliveryConstraints("mse")
	if c.RequiredAudioCodec != "" {
		t.Errorf("expected no audio constraint for MSE, got %s", c.RequiredAudioCodec)
	}
	if c.ForceTranscode {
		t.Error("expected no force transcode for MSE")
	}
	if c.DisableDecodeHW {
		t.Error("expected no disable decode HW for MSE")
	}
}

func TestDeliveryConstraints_WebRTC(t *testing.T) {
	c := DeliveryConstraints("webrtc")
	if c.RequiredAudioCodec != "opus" {
		t.Errorf("expected opus, got %s", c.RequiredAudioCodec)
	}
	if !c.ForceTranscode {
		t.Error("expected ForceTranscode")
	}
	if !c.DisableDecodeHW {
		t.Error("expected DisableDecodeHW")
	}
	if c.AllowedVideoCodecs != nil {
		t.Error("expected no AllowedVideoCodecs in static constraints")
	}
}

func TestResolve_DefaultAudioResolvesToAAC(t *testing.T) {
	r := Resolve(Input{
		Defaults: Preference{
			VideoCodec: "copy",
			AudioCodec: "default",
		},
	})
	if r.AudioCodec != "aac" {
		t.Errorf("expected default audio to resolve to aac, got %s", r.AudioCodec)
	}
}

func TestResolve_MaxBitDepthLayering(t *testing.T) {
	r := Resolve(Input{
		Settings: Preference{
			MaxBitDepth: 8,
		},
		ClientProfile: Preference{
			MaxBitDepth: 10,
		},
	})
	if r.MaxBitDepth != 10 {
		t.Errorf("expected 10, got %d", r.MaxBitDepth)
	}
}
