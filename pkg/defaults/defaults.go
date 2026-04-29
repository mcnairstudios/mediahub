package defaults

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type MatchRule struct {
	HeaderName string `json:"header_name"`
	MatchType  string `json:"match_type"`
	MatchValue string `json:"match_value"`
}

type ProfileDef struct {
	Delivery     string `json:"delivery"`
	VideoCodec   string `json:"video_codec"`
	AudioCodec   string `json:"audio_codec"`
	Container    string `json:"container"`
	HWAccel      string `json:"hwaccel,omitempty"`
	OutputHeight int    `json:"output_height,omitempty"`
}

type ClientDef struct {
	Name       string      `json:"name"`
	Priority   int         `json:"priority"`
	ListenPort int         `json:"listen_port,omitempty"`
	IsEnabled  bool        `json:"is_enabled"`
	IsSystem   bool        `json:"is_system"`
	MatchRules []MatchRule `json:"match_rules"`
	Profile    ProfileDef  `json:"profile"`
}

type clientsFile struct {
	Clients []ClientDef `json:"clients"`
}

type SourceProfileDef struct {
	Name              string `json:"name"`
	Deinterlace       bool   `json:"deinterlace,omitempty"`
	DeinterlaceMethod string `json:"deinterlace_method,omitempty"`
	RTSPProtocols     string `json:"rtsp_protocols,omitempty"`
	RTSPLatency       int    `json:"rtsp_latency,omitempty"`
	HTTPTimeoutSec    int    `json:"http_timeout_sec,omitempty"`
	HTTPUserAgent     string `json:"http_user_agent,omitempty"`
}

type sourceProfilesFile struct {
	SourceProfiles []SourceProfileDef `json:"source_profiles"`
}

func LoadClients(externalDir string) ([]ClientDef, error) {
	if externalDir != "" {
		if data, err := os.ReadFile(filepath.Join(externalDir, "clients.json")); err == nil {
			var f clientsFile
			if err := json.Unmarshal(data, &f); err != nil {
				return nil, fmt.Errorf("parsing external clients.json: %w", err)
			}
			return f.Clients, nil
		}
	}

	data, err := Assets.ReadFile("clients.json")
	if err != nil {
		return nil, fmt.Errorf("reading embedded clients.json: %w", err)
	}
	var f clientsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing embedded clients.json: %w", err)
	}
	return f.Clients, nil
}

func LoadSourceProfiles(externalDir string) ([]SourceProfileDef, error) {
	if externalDir != "" {
		if data, err := os.ReadFile(filepath.Join(externalDir, "source_profiles.json")); err == nil {
			var f sourceProfilesFile
			if err := json.Unmarshal(data, &f); err != nil {
				return nil, fmt.Errorf("parsing external source_profiles.json: %w", err)
			}
			return f.SourceProfiles, nil
		}
	}

	data, err := Assets.ReadFile("source_profiles.json")
	if err != nil {
		return nil, fmt.Errorf("reading embedded source_profiles.json: %w", err)
	}
	var f sourceProfilesFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing embedded source_profiles.json: %w", err)
	}
	return f.SourceProfiles, nil
}

func LoadSettings(externalDir string) (map[string]string, error) {
	if externalDir != "" {
		if data, err := os.ReadFile(filepath.Join(externalDir, "settings.json")); err == nil {
			var m map[string]string
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, fmt.Errorf("parsing external settings.json: %w", err)
			}
			return m, nil
		}
	}

	data, err := Assets.ReadFile("settings.json")
	if err != nil {
		return nil, fmt.Errorf("reading embedded settings.json: %w", err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing embedded settings.json: %w", err)
	}
	return m, nil
}
