package client

import (
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/defaults"
)

func TestEachDeliveryModeInProfile(t *testing.T) {
	modes := []string{"mse", "hls", "dash", "webrtc", "stream"}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			c := Client{
				Name:      "test-" + mode,
				IsEnabled: true,
				Profile:   Profile{Delivery: mode},
			}
			if c.Profile.Delivery != mode {
				t.Fatalf("expected delivery %q, got %q", mode, c.Profile.Delivery)
			}
		})
	}
}

func TestUserChoiceDeliveryMode(t *testing.T) {
	c := Client{
		Name:      "user-choice-client",
		IsEnabled: true,
		Profile:   Profile{Delivery: "user"},
	}
	if c.Profile.Delivery != "user" {
		t.Fatalf("expected delivery %q, got %q", "user", c.Profile.Delivery)
	}
}

func TestDefaultClientProfilesBrowserHasMSE(t *testing.T) {
	defs, err := defaults.LoadClients("")
	if err != nil {
		t.Fatalf("loading default clients: %v", err)
	}

	var found bool
	for _, def := range defs {
		if def.Name == "Browser" {
			found = true
			if def.Profile.Delivery != "mse" {
				t.Fatalf("Browser default delivery: expected %q, got %q", "mse", def.Profile.Delivery)
			}
			break
		}
	}
	if !found {
		t.Fatal("Browser client not found in defaults")
	}
}

func TestProfileDeliveryOverridesGlobal(t *testing.T) {
	globalDefault := "stream"
	clientDelivery := "hls"

	c := Client{
		Name:      "override-test",
		IsEnabled: true,
		Profile:   Profile{Delivery: clientDelivery},
	}

	resolved := c.Profile.Delivery
	if resolved == "" {
		resolved = globalDefault
	}

	if resolved != clientDelivery {
		t.Fatalf("client delivery %q should override global %q, got %q", clientDelivery, globalDefault, resolved)
	}
}

func TestDetectorReturnsCorrectProfile(t *testing.T) {
	clients := []Client{
		{
			Name:      "VLC",
			IsEnabled: true,
			Priority:  70,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "VLC/"},
			},
			Profile: Profile{Delivery: "stream", Container: "matroska"},
		},
		{
			Name:      "Browser",
			IsEnabled: true,
			Priority:  100,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Mozilla/"},
			},
			Profile: Profile{Delivery: "mse", Container: "mp4"},
		},
	}

	d := NewDetector(clients)

	headers := map[string]string{"User-Agent": "VLC/3.0.20 LibVLC/3.0.20"}
	got := d.Detect(0, headers)
	if got == nil {
		t.Fatal("expected VLC client, got nil")
	}
	if got.Profile.Delivery != "stream" {
		t.Fatalf("expected delivery %q, got %q", "stream", got.Profile.Delivery)
	}

	headers = map[string]string{"User-Agent": "Mozilla/5.0 Chrome/131"}
	got = d.Detect(0, headers)
	if got == nil {
		t.Fatal("expected Browser client, got nil")
	}
	if got.Profile.Delivery != "mse" {
		t.Fatalf("expected delivery %q, got %q", "mse", got.Profile.Delivery)
	}
}

func TestPortBasedDetection(t *testing.T) {
	clients := []Client{
		{
			Name:       "Jellyfin",
			IsEnabled:  true,
			Priority:   90,
			ListenPort: 8096,
			Profile:    Profile{Delivery: "hls"},
		},
		{
			Name:       "Dashboard",
			IsEnabled:  true,
			Priority:   80,
			ListenPort: 9090,
			MatchRules: []MatchRule{
				{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Mozilla/"},
			},
			Profile: Profile{Delivery: "mse"},
		},
	}

	d := NewDetector(clients)

	got := d.Detect(8096, map[string]string{})
	if got == nil {
		t.Fatal("expected Jellyfin client on port 8096, got nil")
	}
	if got.Profile.Delivery != "hls" {
		t.Fatalf("expected delivery %q for port 8096, got %q", "hls", got.Profile.Delivery)
	}

	got = d.Detect(9090, map[string]string{"User-Agent": "Mozilla/5.0"})
	if got == nil {
		t.Fatal("expected Dashboard client on port 9090, got nil")
	}
	if got.Profile.Delivery != "mse" {
		t.Fatalf("expected delivery %q for port 9090, got %q", "mse", got.Profile.Delivery)
	}
}

func TestEmptyDeliveryDefaultsToMSE(t *testing.T) {
	c := Client{
		Name:      "no-delivery",
		IsEnabled: true,
		Profile:   Profile{Delivery: ""},
	}

	delivery := c.Profile.Delivery
	if delivery == "" {
		delivery = "mse"
	}

	if delivery != "mse" {
		t.Fatalf("empty delivery should default to %q, got %q", "mse", delivery)
	}
}

func TestDetectorEmptyDeliveryDefaultsToMSE(t *testing.T) {
	clients := []Client{
		{
			Name:      "EmptyDelivery",
			IsEnabled: true,
			Priority:  100,
			Profile:   Profile{Delivery: ""},
		},
	}

	d := NewDetector(clients)
	got := d.Detect(0, map[string]string{})
	if got == nil {
		t.Fatal("expected client, got nil")
	}

	delivery := got.Profile.Delivery
	if delivery == "" {
		delivery = "mse"
	}
	if delivery != "mse" {
		t.Fatalf("empty delivery should resolve to %q, got %q", "mse", delivery)
	}
}
