package config

import (
	"os"
	"strconv"
)

type Config struct {
	BaseURL      string
	ListenAddr   string
	DataDir      string
	RecordDir    string
	VODOutputDir string
	UserAgent    string
	JellyfinPort int
}

func Load() *Config {
	c := &Config{
		ListenAddr:   ":8080",
		DataDir:      "/config",
		RecordDir:    "/record",
		UserAgent:    "MediaHub",
		JellyfinPort: 8096,
	}

	if v := os.Getenv("MEDIAHUB_BASE_URL"); v != "" {
		c.BaseURL = v
	}
	if v := os.Getenv("MEDIAHUB_LISTEN_ADDR"); v != "" {
		c.ListenAddr = v
	}
	if v := os.Getenv("MEDIAHUB_DATA_DIR"); v != "" {
		c.DataDir = v
	}
	if v := os.Getenv("MEDIAHUB_RECORD_DIR"); v != "" {
		c.RecordDir = v
	}
	if v := os.Getenv("MEDIAHUB_VOD_OUTPUT_DIR"); v != "" {
		c.VODOutputDir = v
	} else {
		c.VODOutputDir = c.RecordDir
	}
	if v := os.Getenv("MEDIAHUB_USER_AGENT"); v != "" {
		c.UserAgent = v
	}
	if v := os.Getenv("MEDIAHUB_JELLYFIN_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.JellyfinPort = port
		}
	}

	return c
}
