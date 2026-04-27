package wg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

const profileKeyPrefix = "wg_profile_"

type ProfileResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Endpoint   string `json:"endpoint"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
	AllowedIPs string `json:"allowed_ips"`
	DNS        string `json:"dns,omitempty"`
	Address    string `json:"address"`
	IsActive   bool   `json:"is_active"`
}

type StatusResponse struct {
	Connected   bool    `json:"connected"`
	ProfileID   string  `json:"profile_id,omitempty"`
	ProfileName string  `json:"profile_name,omitempty"`
	Endpoint    string  `json:"endpoint,omitempty"`
	ProxyPort   int     `json:"proxy_port,omitempty"`
	TxBytes     int64   `json:"tx_bytes,omitempty"`
	RxBytes     int64   `json:"rx_bytes,omitempty"`
	LatencyMs   float64 `json:"latency_ms,omitempty"`
}

type TestResult struct {
	Success   bool    `json:"success"`
	LatencyMs float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
}

type Service struct {
	settings store.SettingsStore
	tunnel   *Tunnel
	plugin   *Plugin
	mu       sync.RWMutex
}

func NewService(settings store.SettingsStore) *Service {
	return &Service{
		settings: settings,
	}
}

func (s *Service) ListProfiles(ctx context.Context) ([]ProfileResponse, error) {
	all, err := s.settings.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing settings: %w", err)
	}

	var profiles []ProfileResponse
	for k, v := range all {
		if !strings.HasPrefix(k, profileKeyPrefix) {
			continue
		}
		var cfg TunnelConfig
		if err := json.Unmarshal([]byte(v), &cfg); err != nil {
			continue
		}
		profiles = append(profiles, toProfileResponse(cfg))
	}
	return profiles, nil
}

func (s *Service) GetProfile(ctx context.Context, id string) (*ProfileResponse, error) {
	val, err := s.settings.Get(ctx, profileKeyPrefix+id)
	if err != nil {
		return nil, fmt.Errorf("getting profile: %w", err)
	}
	if val == "" {
		return nil, nil
	}

	var cfg TunnelConfig
	if err := json.Unmarshal([]byte(val), &cfg); err != nil {
		return nil, fmt.Errorf("decoding profile: %w", err)
	}

	resp := toProfileResponse(cfg)
	return &resp, nil
}

func (s *Service) CreateProfile(ctx context.Context, cfg TunnelConfig) (*ProfileResponse, error) {
	if cfg.ID == "" {
		cfg.ID = uuid.New().String()
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if cfg.PrivateKey == "" {
		return nil, fmt.Errorf("private key is required")
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if cfg.PublicKey == "" {
		return nil, fmt.Errorf("public key is required")
	}
	if cfg.Address == "" {
		return nil, fmt.Errorf("address is required")
	}

	cfg.IsActive = false

	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("encoding profile: %w", err)
	}

	if err := s.settings.Set(ctx, profileKeyPrefix+cfg.ID, string(data)); err != nil {
		return nil, fmt.Errorf("saving profile: %w", err)
	}

	resp := toProfileResponse(cfg)
	return &resp, nil
}

func (s *Service) UpdateProfile(ctx context.Context, id string, cfg TunnelConfig) (*ProfileResponse, error) {
	existing, err := s.getConfig(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("profile not found")
	}

	if cfg.Name != "" {
		existing.Name = cfg.Name
	}
	if cfg.Endpoint != "" {
		existing.Endpoint = cfg.Endpoint
	}
	if cfg.PublicKey != "" {
		existing.PublicKey = cfg.PublicKey
	}
	if cfg.PrivateKey != "" {
		existing.PrivateKey = cfg.PrivateKey
	}
	if cfg.AllowedIPs != "" {
		existing.AllowedIPs = cfg.AllowedIPs
	}
	if cfg.DNS != "" {
		existing.DNS = cfg.DNS
	}
	if cfg.Address != "" {
		existing.Address = cfg.Address
	}

	data, err := json.Marshal(existing)
	if err != nil {
		return nil, fmt.Errorf("encoding profile: %w", err)
	}

	if err := s.settings.Set(ctx, profileKeyPrefix+id, string(data)); err != nil {
		return nil, fmt.Errorf("saving profile: %w", err)
	}

	s.mu.RLock()
	isActive := s.tunnel != nil && s.tunnel.config.ID == id
	s.mu.RUnlock()

	if isActive {
		s.Deactivate()
		if err := s.Activate(ctx, id); err != nil {
			return nil, fmt.Errorf("reactivating tunnel after update: %w", err)
		}
	}

	resp := toProfileResponse(*existing)
	return &resp, nil
}

func (s *Service) DeleteProfile(ctx context.Context, id string) error {
	s.mu.RLock()
	isActive := s.tunnel != nil && s.tunnel.config.ID == id
	s.mu.RUnlock()

	if isActive {
		s.Deactivate()
	}

	existing, err := s.getConfig(ctx, id)
	if err != nil {
		return err
	}
	if existing != nil {
		existing.IsActive = false
		data, _ := json.Marshal(existing)
		s.settings.Set(ctx, profileKeyPrefix+id, string(data))
	}

	return s.settings.Set(ctx, profileKeyPrefix+id, "")
}

func (s *Service) Activate(ctx context.Context, id string) error {
	cfg, err := s.getConfig(ctx, id)
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("profile not found")
	}

	s.Deactivate()

	profiles, err := s.ListProfiles(ctx)
	if err == nil {
		for _, p := range profiles {
			if p.ID != id {
				s.setActive(ctx, p.ID, false)
			}
		}
	}

	tunnel, err := NewTunnel(*cfg)
	if err != nil {
		return fmt.Errorf("creating tunnel: %w", err)
	}

	plugin, err := New(tunnel.Transport())
	if err != nil {
		tunnel.Close()
		return fmt.Errorf("creating proxy: %w", err)
	}

	s.mu.Lock()
	s.tunnel = tunnel
	s.plugin = plugin
	s.mu.Unlock()

	s.setActive(ctx, id, true)

	return nil
}

func (s *Service) Deactivate() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.plugin != nil {
		s.plugin.Close()
		s.plugin = nil
	}
	if s.tunnel != nil {
		s.tunnel.Close()
		s.tunnel = nil
	}
}

func (s *Service) TestProfile(ctx context.Context, id string) TestResult {
	cfg, err := s.getConfig(ctx, id)
	if err != nil {
		return TestResult{Error: err.Error()}
	}
	if cfg == nil {
		return TestResult{Error: "profile not found"}
	}

	tunnel, err := NewTunnel(*cfg)
	if err != nil {
		return TestResult{Error: fmt.Sprintf("tunnel creation failed: %v", err)}
	}
	defer tunnel.Close()

	client := tunnel.HTTPClient(10 * time.Second)

	start := time.Now()
	resp, err := client.Get("https://www.cloudflare.com/cdn-cgi/trace")
	if err != nil {
		return TestResult{Error: fmt.Sprintf("health check failed: %v", err)}
	}
	resp.Body.Close()
	latency := float64(time.Since(start).Milliseconds())

	if resp.StatusCode != http.StatusOK {
		return TestResult{Error: fmt.Sprintf("unexpected status: %d", resp.StatusCode)}
	}

	return TestResult{
		Success:   true,
		LatencyMs: latency,
	}
}

func (s *Service) Status() StatusResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.tunnel == nil || s.plugin == nil {
		return StatusResponse{Connected: false}
	}

	resp := StatusResponse{
		Connected:   s.plugin.IsConnected(),
		ProfileID:   s.tunnel.config.ID,
		ProfileName: s.tunnel.config.Name,
		Endpoint:    s.tunnel.config.Endpoint,
		ProxyPort:   s.plugin.Port(),
	}

	stats, err := s.tunnel.Stats()
	if err == nil && stats != nil {
		resp.TxBytes = stats.TxBytes
		resp.RxBytes = stats.RxBytes
	}

	return resp
}

func (s *Service) ActivePlugin() *Plugin {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.plugin
}

func (s *Service) Close() {
	s.Deactivate()
}

func (s *Service) RestoreActive(ctx context.Context) error {
	profiles, err := s.ListProfiles(ctx)
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if p.IsActive {
			return s.Activate(ctx, p.ID)
		}
	}
	return nil
}

func (s *Service) getConfig(ctx context.Context, id string) (*TunnelConfig, error) {
	val, err := s.settings.Get(ctx, profileKeyPrefix+id)
	if err != nil {
		return nil, fmt.Errorf("getting profile: %w", err)
	}
	if val == "" {
		return nil, nil
	}

	var cfg TunnelConfig
	if err := json.Unmarshal([]byte(val), &cfg); err != nil {
		return nil, fmt.Errorf("decoding profile: %w", err)
	}
	return &cfg, nil
}

func (s *Service) setActive(ctx context.Context, id string, active bool) {
	cfg, err := s.getConfig(ctx, id)
	if err != nil || cfg == nil {
		return
	}
	cfg.IsActive = active
	data, _ := json.Marshal(cfg)
	s.settings.Set(ctx, profileKeyPrefix+id, string(data))
}

func toProfileResponse(cfg TunnelConfig) ProfileResponse {
	return ProfileResponse{
		ID:         cfg.ID,
		Name:       cfg.Name,
		Endpoint:   cfg.Endpoint,
		PublicKey:   cfg.PublicKey,
		PrivateKey: MaskPrivateKey(cfg.PrivateKey),
		AllowedIPs: cfg.AllowedIPs,
		DNS:        cfg.DNS,
		Address:    cfg.Address,
		IsActive:   cfg.IsActive,
	}
}
