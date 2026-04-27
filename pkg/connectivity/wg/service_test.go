package wg

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

type memSettings struct {
	mu   sync.RWMutex
	data map[string]string
}

func newMemSettings() *memSettings {
	return &memSettings{data: make(map[string]string)}
}

func (m *memSettings) Get(_ context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[key], nil
}

func (m *memSettings) Set(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if value == "" {
		delete(m.data, key)
	} else {
		m.data[key] = value
	}
	return nil
}

func (m *memSettings) List(_ context.Context) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]string, len(m.data))
	for k, v := range m.data {
		out[k] = v
	}
	return out, nil
}

func TestServiceCreateProfile(t *testing.T) {
	svc := NewService(newMemSettings())
	ctx := context.Background()

	resp, err := svc.CreateProfile(ctx, TunnelConfig{
		Name:       "test-vpn",
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		Endpoint:   "1.2.3.4:51820",
		PublicKey:  "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		AllowedIPs: "0.0.0.0/0",
		Address:    "10.0.0.2/24",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if resp.Name != "test-vpn" {
		t.Fatalf("expected test-vpn, got %s", resp.Name)
	}
	if resp.IsActive {
		t.Fatal("expected new profile to be inactive")
	}
	if resp.PrivateKey == "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" {
		t.Fatal("expected private key to be masked")
	}
}

func TestServiceCreateProfileValidation(t *testing.T) {
	svc := NewService(newMemSettings())
	ctx := context.Background()

	tests := []struct {
		name string
		cfg  TunnelConfig
	}{
		{"missing name", TunnelConfig{PrivateKey: "x", Endpoint: "y", PublicKey: "z", Address: "a"}},
		{"missing private key", TunnelConfig{Name: "x", Endpoint: "y", PublicKey: "z", Address: "a"}},
		{"missing endpoint", TunnelConfig{Name: "x", PrivateKey: "y", PublicKey: "z", Address: "a"}},
		{"missing public key", TunnelConfig{Name: "x", PrivateKey: "y", Endpoint: "z", Address: "a"}},
		{"missing address", TunnelConfig{Name: "x", PrivateKey: "y", Endpoint: "z", PublicKey: "a"}},
	}

	for _, tt := range tests {
		_, err := svc.CreateProfile(ctx, tt.cfg)
		if err == nil {
			t.Fatalf("expected error for %s", tt.name)
		}
	}
}

func TestServiceListProfiles(t *testing.T) {
	svc := NewService(newMemSettings())
	ctx := context.Background()

	profiles, err := svc.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected 0 profiles, got %d", len(profiles))
	}

	svc.CreateProfile(ctx, TunnelConfig{
		Name:       "vpn-1",
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		Endpoint:   "1.2.3.4:51820",
		PublicKey:  "BBBB",
		Address:    "10.0.0.2/24",
	})
	svc.CreateProfile(ctx, TunnelConfig{
		Name:       "vpn-2",
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		Endpoint:   "5.6.7.8:51820",
		PublicKey:  "CCCC",
		Address:    "10.0.0.3/24",
	})

	profiles, err = svc.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestServiceGetProfile(t *testing.T) {
	svc := NewService(newMemSettings())
	ctx := context.Background()

	created, _ := svc.CreateProfile(ctx, TunnelConfig{
		Name:       "vpn-1",
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		Endpoint:   "1.2.3.4:51820",
		PublicKey:  "BBBB",
		Address:    "10.0.0.2/24",
	})

	got, err := svc.GetProfile(ctx, created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected profile, got nil")
	}
	if got.Name != "vpn-1" {
		t.Fatalf("expected vpn-1, got %s", got.Name)
	}
}

func TestServiceGetProfileNotFound(t *testing.T) {
	svc := NewService(newMemSettings())
	ctx := context.Background()

	got, err := svc.GetProfile(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent profile")
	}
}

func TestServiceUpdateProfile(t *testing.T) {
	svc := NewService(newMemSettings())
	ctx := context.Background()

	created, _ := svc.CreateProfile(ctx, TunnelConfig{
		Name:       "vpn-1",
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		Endpoint:   "1.2.3.4:51820",
		PublicKey:  "BBBB",
		Address:    "10.0.0.2/24",
	})

	updated, err := svc.UpdateProfile(ctx, created.ID, TunnelConfig{
		Name:     "vpn-renamed",
		Endpoint: "9.8.7.6:51820",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "vpn-renamed" {
		t.Fatalf("expected vpn-renamed, got %s", updated.Name)
	}
	if updated.Endpoint != "9.8.7.6:51820" {
		t.Fatalf("expected 9.8.7.6:51820, got %s", updated.Endpoint)
	}
}

func TestServiceUpdateProfileNotFound(t *testing.T) {
	svc := NewService(newMemSettings())
	ctx := context.Background()

	_, err := svc.UpdateProfile(ctx, "nonexistent", TunnelConfig{Name: "x"})
	if err == nil {
		t.Fatal("expected error for nonexistent profile")
	}
}

func TestServiceDeleteProfile(t *testing.T) {
	svc := NewService(newMemSettings())
	ctx := context.Background()

	created, _ := svc.CreateProfile(ctx, TunnelConfig{
		Name:       "vpn-1",
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		Endpoint:   "1.2.3.4:51820",
		PublicKey:  "BBBB",
		Address:    "10.0.0.2/24",
	})

	err := svc.DeleteProfile(ctx, created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := svc.GetProfile(ctx, created.ID)
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestServiceStatusDisconnected(t *testing.T) {
	svc := NewService(newMemSettings())

	status := svc.Status()
	if status.Connected {
		t.Fatal("expected disconnected when no tunnel")
	}
	if status.ProfileID != "" {
		t.Fatal("expected empty profile ID")
	}
}

func TestServiceMaskInListProfiles(t *testing.T) {
	settings := newMemSettings()
	svc := NewService(settings)
	ctx := context.Background()

	svc.CreateProfile(ctx, TunnelConfig{
		Name:       "vpn-1",
		PrivateKey: "cGFzc3dvcmRwYXNzd29yZHBhc3N3b3JkcGFzc3dvcmQ=",
		Endpoint:   "1.2.3.4:51820",
		PublicKey:  "BBBB",
		Address:    "10.0.0.2/24",
	})

	profiles, _ := svc.ListProfiles(ctx)
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].PrivateKey == "cGFzc3dvcmRwYXNzd29yZHBhc3N3b3JkcGFzc3dvcmQ=" {
		t.Fatal("expected private key to be masked in list response")
	}
}

func TestServicePrivateKeyStoredUnmasked(t *testing.T) {
	settings := newMemSettings()
	svc := NewService(settings)
	ctx := context.Background()

	originalKey := "cGFzc3dvcmRwYXNzd29yZHBhc3N3b3JkcGFzc3dvcmQ="
	created, _ := svc.CreateProfile(ctx, TunnelConfig{
		Name:       "vpn-1",
		PrivateKey: originalKey,
		Endpoint:   "1.2.3.4:51820",
		PublicKey:  "BBBB",
		Address:    "10.0.0.2/24",
	})

	val, _ := settings.Get(ctx, profileKeyPrefix+created.ID)
	var stored TunnelConfig
	json.Unmarshal([]byte(val), &stored)

	if stored.PrivateKey != originalKey {
		t.Fatalf("expected unmasked key in store, got %s", stored.PrivateKey)
	}
}

func TestServiceRestoreActiveNoActive(t *testing.T) {
	svc := NewService(newMemSettings())
	ctx := context.Background()

	svc.CreateProfile(ctx, TunnelConfig{
		Name:       "vpn-1",
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		Endpoint:   "1.2.3.4:51820",
		PublicKey:  "BBBB",
		Address:    "10.0.0.2/24",
	})

	err := svc.RestoreActive(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.Status().Connected {
		t.Fatal("expected disconnected when no active profile")
	}
}

func TestServiceClose(t *testing.T) {
	svc := NewService(newMemSettings())
	svc.Close()

	status := svc.Status()
	if status.Connected {
		t.Fatal("expected disconnected after close")
	}
}

func TestServiceActivePluginNil(t *testing.T) {
	svc := NewService(newMemSettings())
	if svc.ActivePlugin() != nil {
		t.Fatal("expected nil plugin when no tunnel")
	}
}

func TestToProfileResponseMasksKey(t *testing.T) {
	cfg := TunnelConfig{
		ID:         "test",
		Name:       "test",
		PrivateKey: "cGFzc3dvcmRwYXNzd29yZHBhc3N3b3JkcGFzc3dvcmQ=",
		Endpoint:   "1.2.3.4:51820",
		PublicKey:  "BBBB",
		Address:    "10.0.0.2/24",
	}

	resp := toProfileResponse(cfg)
	if resp.PrivateKey == cfg.PrivateKey {
		t.Fatal("expected masked key in response")
	}
}
