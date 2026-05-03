package client

import (
	"encoding/json"
	"testing"
)

func TestProfileJSONRoundTrip(t *testing.T) {
	original := Profile{
		Delivery:     "hls",
		VideoCodec:   "h264",
		AudioCodec:   "aac",
		Container:    "mpegts",
		HWAccel:      "vaapi",
		OutputHeight: 720,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Profile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Delivery != original.Delivery {
		t.Errorf("Delivery: got %q, want %q", decoded.Delivery, original.Delivery)
	}
	if decoded.VideoCodec != original.VideoCodec {
		t.Errorf("VideoCodec: got %q, want %q", decoded.VideoCodec, original.VideoCodec)
	}
	if decoded.AudioCodec != original.AudioCodec {
		t.Errorf("AudioCodec: got %q, want %q", decoded.AudioCodec, original.AudioCodec)
	}
	if decoded.Container != original.Container {
		t.Errorf("Container: got %q, want %q", decoded.Container, original.Container)
	}
	if decoded.HWAccel != original.HWAccel {
		t.Errorf("HWAccel: got %q, want %q", decoded.HWAccel, original.HWAccel)
	}
	if decoded.OutputHeight != original.OutputHeight {
		t.Errorf("OutputHeight: got %d, want %d", decoded.OutputHeight, original.OutputHeight)
	}
}

func TestProfileJSONRoundTripZeroHeight(t *testing.T) {
	original := Profile{
		Delivery:   "mse",
		VideoCodec: "copy",
		AudioCodec: "aac",
		Container:  "mp4",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, exists := raw["output_height"]; exists {
		t.Error("output_height should be omitted when zero")
	}

	var decoded Profile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.OutputHeight != 0 {
		t.Errorf("OutputHeight: got %d, want 0", decoded.OutputHeight)
	}
}

func TestAllDeliveryModeStringsValid(t *testing.T) {
	validModes := map[string]bool{
		"mse":    true,
		"hls":    true,
		"dash":   true,
		"webrtc": true,
		"stream": true,
		"user":   true,
	}

	for mode := range validModes {
		t.Run(mode, func(t *testing.T) {
			p := Profile{Delivery: mode}
			data, err := json.Marshal(p)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var decoded Profile
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if decoded.Delivery != mode {
				t.Fatalf("expected %q, got %q", mode, decoded.Delivery)
			}
		})
	}
}

func TestClientJSONRoundTrip(t *testing.T) {
	original := Client{
		ID:         "test-123",
		Name:       "TestClient",
		Priority:   50,
		ListenPort: 8096,
		IsEnabled:  true,
		IsSystem:   false,
		MatchRules: []MatchRule{
			{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Test/"},
		},
		Profile: Profile{
			Delivery:     "hls",
			VideoCodec:   "h265",
			AudioCodec:   "opus",
			Container:    "mpegts",
			HWAccel:      "nvenc",
			OutputHeight: 1080,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Client
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Priority != original.Priority {
		t.Errorf("Priority: got %d, want %d", decoded.Priority, original.Priority)
	}
	if decoded.ListenPort != original.ListenPort {
		t.Errorf("ListenPort: got %d, want %d", decoded.ListenPort, original.ListenPort)
	}
	if decoded.IsEnabled != original.IsEnabled {
		t.Errorf("IsEnabled: got %v, want %v", decoded.IsEnabled, original.IsEnabled)
	}
	if decoded.IsSystem != original.IsSystem {
		t.Errorf("IsSystem: got %v, want %v", decoded.IsSystem, original.IsSystem)
	}
	if len(decoded.MatchRules) != len(original.MatchRules) {
		t.Fatalf("MatchRules length: got %d, want %d", len(decoded.MatchRules), len(original.MatchRules))
	}
	if decoded.MatchRules[0].MatchValue != original.MatchRules[0].MatchValue {
		t.Errorf("MatchRule value: got %q, want %q", decoded.MatchRules[0].MatchValue, original.MatchRules[0].MatchValue)
	}
	if decoded.Profile.Delivery != original.Profile.Delivery {
		t.Errorf("Profile.Delivery: got %q, want %q", decoded.Profile.Delivery, original.Profile.Delivery)
	}
	if decoded.Profile.VideoCodec != original.Profile.VideoCodec {
		t.Errorf("Profile.VideoCodec: got %q, want %q", decoded.Profile.VideoCodec, original.Profile.VideoCodec)
	}
	if decoded.Profile.AudioCodec != original.Profile.AudioCodec {
		t.Errorf("Profile.AudioCodec: got %q, want %q", decoded.Profile.AudioCodec, original.Profile.AudioCodec)
	}
}

func TestProfileCodecFieldsPopulated(t *testing.T) {
	tests := []struct {
		name       string
		videoCodec string
		audioCodec string
		container  string
	}{
		{"browser-mse", "copy", "aac", "mp4"},
		{"jellyfin-hls", "h264", "aac", "mpegts"},
		{"plex-stream", "copy", "copy", "mpegts"},
		{"vlc-stream", "copy", "copy", "matroska"},
		{"transcode", "h265", "opus", "mp4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Profile{
				VideoCodec: tt.videoCodec,
				AudioCodec: tt.audioCodec,
				Container:  tt.container,
			}
			if p.VideoCodec != tt.videoCodec {
				t.Errorf("VideoCodec: got %q, want %q", p.VideoCodec, tt.videoCodec)
			}
			if p.AudioCodec != tt.audioCodec {
				t.Errorf("AudioCodec: got %q, want %q", p.AudioCodec, tt.audioCodec)
			}
			if p.Container != tt.container {
				t.Errorf("Container: got %q, want %q", p.Container, tt.container)
			}
		})
	}
}
