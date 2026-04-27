package api

import (
	"net/http"
	"testing"
)

func TestCapabilitiesEndpoint(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/capabilities", nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var caps platformCapabilities
	decodeBody(resp, &caps)

	if caps.VideoEncoders == nil && caps.VideoDecoders == nil && caps.AudioEncoders == nil {
		t.Fatal("expected at least one category of codecs to be populated")
	}
}

func TestCapabilitiesRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/capabilities", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCapabilitiesStructure(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/capabilities", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var caps platformCapabilities
	decodeBody(resp, &caps)

	for _, enc := range caps.VideoEncoders {
		if enc.Name == "" {
			t.Error("video encoder has empty name")
		}
		if enc.Codec == "" {
			t.Error("video encoder has empty codec")
		}
		if enc.Type != "video" {
			t.Errorf("video encoder %s has type %q, want %q", enc.Name, enc.Type, "video")
		}
		if enc.HW && enc.Platform == "" {
			t.Errorf("hw encoder %s has no platform", enc.Name)
		}
	}

	for _, dec := range caps.VideoDecoders {
		if dec.Name == "" {
			t.Error("video decoder has empty name")
		}
		if dec.Type != "video" {
			t.Errorf("video decoder %s has type %q, want %q", dec.Name, dec.Type, "video")
		}
	}

	for _, enc := range caps.AudioEncoders {
		if enc.Name == "" {
			t.Error("audio encoder has empty name")
		}
		if enc.Type != "audio" {
			t.Errorf("audio encoder %s has type %q, want %q", enc.Name, enc.Type, "audio")
		}
	}
}

func TestCapabilitiesCached(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp1 := env.request("GET", "/api/capabilities", nil, env.adminToken)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", resp1.StatusCode)
	}
	var caps1 platformCapabilities
	decodeBody(resp1, &caps1)

	resp2 := env.request("GET", "/api/capabilities", nil, env.adminToken)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", resp2.StatusCode)
	}
	var caps2 platformCapabilities
	decodeBody(resp2, &caps2)

	if len(caps1.VideoEncoders) != len(caps2.VideoEncoders) {
		t.Error("cached response differs from first response")
	}
}
