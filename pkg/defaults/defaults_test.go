package defaults

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClients_Embedded(t *testing.T) {
	clients, err := LoadClients("")
	if err != nil {
		t.Fatalf("LoadClients: %v", err)
	}
	if len(clients) == 0 {
		t.Fatal("expected at least one embedded client")
	}

	found := false
	for _, c := range clients {
		if c.Name == "Browser" {
			found = true
			if c.Profile.Delivery != "mse" {
				t.Errorf("Browser delivery = %q, want mse", c.Profile.Delivery)
			}
			if !c.IsSystem {
				t.Error("Browser should be a system client")
			}
			if !c.IsEnabled {
				t.Error("Browser should be enabled")
			}
			break
		}
	}
	if !found {
		t.Error("expected to find Browser client in defaults")
	}
}

func TestLoadClients_ExternalOverride(t *testing.T) {
	dir := t.TempDir()
	overrideData := clientsFile{
		Clients: []ClientDef{
			{Name: "Custom", Priority: 50, IsEnabled: true, Profile: ProfileDef{Delivery: "stream"}},
		},
	}
	data, _ := json.Marshal(overrideData)
	os.WriteFile(filepath.Join(dir, "clients.json"), data, 0644)

	clients, err := LoadClients(dir)
	if err != nil {
		t.Fatalf("LoadClients: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("expected 1 client from override, got %d", len(clients))
	}
	if clients[0].Name != "Custom" {
		t.Errorf("Name = %q, want Custom", clients[0].Name)
	}
}

func TestLoadClients_ExternalInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "clients.json"), []byte("{invalid"), 0644)

	_, err := LoadClients(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadClients_ExternalNotFoundFallsBackToEmbedded(t *testing.T) {
	clients, err := LoadClients("/nonexistent/path")
	if err != nil {
		t.Fatalf("LoadClients: %v", err)
	}
	if len(clients) == 0 {
		t.Fatal("expected embedded clients when external dir doesn't exist")
	}
}

func TestLoadSourceProfiles_Embedded(t *testing.T) {
	profiles, err := LoadSourceProfiles("")
	if err != nil {
		t.Fatalf("LoadSourceProfiles: %v", err)
	}
	if len(profiles) == 0 {
		t.Fatal("expected at least one embedded source profile")
	}

	found := false
	for _, p := range profiles {
		if p.Name != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one named source profile")
	}
}

func TestLoadSourceProfiles_ExternalOverride(t *testing.T) {
	dir := t.TempDir()
	overrideData := sourceProfilesFile{
		SourceProfiles: []SourceProfileDef{
			{Name: "Custom Profile", Deinterlace: true},
		},
	}
	data, _ := json.Marshal(overrideData)
	os.WriteFile(filepath.Join(dir, "source_profiles.json"), data, 0644)

	profiles, err := LoadSourceProfiles(dir)
	if err != nil {
		t.Fatalf("LoadSourceProfiles: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile from override, got %d", len(profiles))
	}
	if profiles[0].Name != "Custom Profile" {
		t.Errorf("Name = %q, want Custom Profile", profiles[0].Name)
	}
	if !profiles[0].Deinterlace {
		t.Error("expected Deinterlace to be true")
	}
}

func TestLoadSourceProfiles_ExternalInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "source_profiles.json"), []byte("{bad"), 0644)

	_, err := LoadSourceProfiles(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadSettings_Embedded(t *testing.T) {
	settings, err := LoadSettings("")
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if len(settings) == 0 {
		t.Fatal("expected at least one embedded setting")
	}

	if val, ok := settings["default_hwaccel"]; !ok {
		t.Error("expected default_hwaccel in settings")
	} else if val != "none" {
		t.Errorf("default_hwaccel = %q, want none", val)
	}

	if _, ok := settings["dlna_enabled"]; !ok {
		t.Error("expected dlna_enabled in settings")
	}
}

func TestLoadSettings_ExternalOverride(t *testing.T) {
	dir := t.TempDir()
	override := map[string]string{
		"custom_key":      "custom_value",
		"default_hwaccel": "vaapi",
	}
	data, _ := json.Marshal(override)
	os.WriteFile(filepath.Join(dir, "settings.json"), data, 0644)

	settings, err := LoadSettings(dir)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if settings["custom_key"] != "custom_value" {
		t.Errorf("custom_key = %q, want custom_value", settings["custom_key"])
	}
	if settings["default_hwaccel"] != "vaapi" {
		t.Errorf("default_hwaccel = %q, want vaapi", settings["default_hwaccel"])
	}
}

func TestLoadSettings_ExternalInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{bad"), 0644)

	_, err := LoadSettings(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadSettings_ExternalNotFoundFallsBackToEmbedded(t *testing.T) {
	settings, err := LoadSettings("/nonexistent/path")
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if len(settings) == 0 {
		t.Fatal("expected embedded settings when external dir doesn't exist")
	}
}

func TestClientDefFields(t *testing.T) {
	c := ClientDef{
		Name:       "Test",
		Priority:   10,
		ListenPort: 8096,
		IsEnabled:  true,
		IsSystem:   false,
		MatchRules: []MatchRule{
			{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "test"},
		},
		Profile: ProfileDef{
			Delivery:     "hls",
			VideoCodec:   "h264",
			AudioCodec:   "aac",
			Container:    "mpegts",
			HWAccel:      "vaapi",
			OutputHeight: 720,
		},
	}

	if c.Name != "Test" {
		t.Errorf("Name = %q, want Test", c.Name)
	}
	if c.Profile.OutputHeight != 720 {
		t.Errorf("OutputHeight = %d, want 720", c.Profile.OutputHeight)
	}
	if len(c.MatchRules) != 1 {
		t.Errorf("MatchRules len = %d, want 1", len(c.MatchRules))
	}
}
