package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	c := Load()

	if c.BaseURL != "" {
		t.Errorf("BaseURL: got %q, want empty", c.BaseURL)
	}
	if c.ListenAddr != ":8080" {
		t.Errorf("ListenAddr: got %q, want %q", c.ListenAddr, ":8080")
	}
	if c.DataDir != "/config" {
		t.Errorf("DataDir: got %q, want %q", c.DataDir, "/config")
	}
	if c.RecordDir != "/record" {
		t.Errorf("RecordDir: got %q, want %q", c.RecordDir, "/record")
	}
	if c.VODOutputDir != "/record" {
		t.Errorf("VODOutputDir: got %q, want %q (should default to RecordDir)", c.VODOutputDir, "/record")
	}
	if c.UserAgent != "MediaHub" {
		t.Errorf("UserAgent: got %q, want %q", c.UserAgent, "MediaHub")
	}
	if c.JellyfinPort != 8096 {
		t.Errorf("JellyfinPort: got %d, want %d", c.JellyfinPort, 8096)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("MEDIAHUB_BASE_URL", "http://example.com")
	t.Setenv("MEDIAHUB_LISTEN_ADDR", ":9090")
	t.Setenv("MEDIAHUB_DATA_DIR", "/data")
	t.Setenv("MEDIAHUB_RECORD_DIR", "/recordings")
	t.Setenv("MEDIAHUB_VOD_OUTPUT_DIR", "/vod")
	t.Setenv("MEDIAHUB_USER_AGENT", "TestAgent/1.0")
	t.Setenv("MEDIAHUB_JELLYFIN_PORT", "9096")

	c := Load()

	if c.BaseURL != "http://example.com" {
		t.Errorf("BaseURL: got %q, want %q", c.BaseURL, "http://example.com")
	}
	if c.ListenAddr != ":9090" {
		t.Errorf("ListenAddr: got %q, want %q", c.ListenAddr, ":9090")
	}
	if c.DataDir != "/data" {
		t.Errorf("DataDir: got %q, want %q", c.DataDir, "/data")
	}
	if c.RecordDir != "/recordings" {
		t.Errorf("RecordDir: got %q, want %q", c.RecordDir, "/recordings")
	}
	if c.VODOutputDir != "/vod" {
		t.Errorf("VODOutputDir: got %q, want %q", c.VODOutputDir, "/vod")
	}
	if c.UserAgent != "TestAgent/1.0" {
		t.Errorf("UserAgent: got %q, want %q", c.UserAgent, "TestAgent/1.0")
	}
	if c.JellyfinPort != 9096 {
		t.Errorf("JellyfinPort: got %d, want %d", c.JellyfinPort, 9096)
	}
}

func TestVODOutputDirDefaultsToRecordDir(t *testing.T) {
	t.Setenv("MEDIAHUB_RECORD_DIR", "/my-recordings")

	c := Load()

	if c.VODOutputDir != "/my-recordings" {
		t.Errorf("VODOutputDir: got %q, want %q (should default to RecordDir)", c.VODOutputDir, "/my-recordings")
	}
}

func TestVODOutputDirOverridesRecordDir(t *testing.T) {
	t.Setenv("MEDIAHUB_RECORD_DIR", "/my-recordings")
	t.Setenv("MEDIAHUB_VOD_OUTPUT_DIR", "/my-vod")

	c := Load()

	if c.VODOutputDir != "/my-vod" {
		t.Errorf("VODOutputDir: got %q, want %q", c.VODOutputDir, "/my-vod")
	}
}

func TestJellyfinPortInvalidFallsBackToDefault(t *testing.T) {
	t.Setenv("MEDIAHUB_JELLYFIN_PORT", "notanumber")

	c := Load()

	if c.JellyfinPort != 8096 {
		t.Errorf("JellyfinPort: got %d, want %d (should keep default on invalid input)", c.JellyfinPort, 8096)
	}
}
