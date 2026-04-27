package wg

import (
	"net/netip"
	"testing"
)

func TestBase64ToHex(t *testing.T) {
	hex, err := base64ToHex("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hex) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(hex))
	}
}

func TestBase64ToHexInvalid(t *testing.T) {
	_, err := base64ToHex("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestResolveEndpointIP(t *testing.T) {
	resolved, err := resolveEndpoint("1.2.3.4:51820")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "1.2.3.4:51820" {
		t.Fatalf("expected 1.2.3.4:51820, got %s", resolved)
	}
}

func TestResolveEndpointIPv6(t *testing.T) {
	resolved, err := resolveEndpoint("[::1]:51820")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "[::1]:51820" {
		t.Fatalf("expected [::1]:51820, got %s", resolved)
	}
}

func TestResolveEndpointBadFormat(t *testing.T) {
	_, err := resolveEndpoint("no-port")
	if err == nil {
		t.Fatal("expected error for bad endpoint format")
	}
}

func TestParseAddressWithCIDR(t *testing.T) {
	addr, err := parseAddress("10.0.0.2/24")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := netip.MustParseAddr("10.0.0.2")
	if addr != expected {
		t.Fatalf("expected %s, got %s", expected, addr)
	}
}

func TestParseAddressPlain(t *testing.T) {
	addr, err := parseAddress("10.0.0.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := netip.MustParseAddr("10.0.0.2")
	if addr != expected {
		t.Fatalf("expected %s, got %s", expected, addr)
	}
}

func TestParseAddressInvalid(t *testing.T) {
	_, err := parseAddress("not-an-ip")
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

func TestTunnelConfigFields(t *testing.T) {
	cfg := TunnelConfig{
		ID:         "test-id",
		Name:       "test-profile",
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		Endpoint:   "1.2.3.4:51820",
		PublicKey:  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		AllowedIPs: "0.0.0.0/0",
		DNS:        "1.1.1.1",
		Address:    "10.0.0.2/24",
		IsActive:   true,
	}

	if cfg.ID != "test-id" {
		t.Fatalf("expected test-id, got %s", cfg.ID)
	}
	if cfg.Name != "test-profile" {
		t.Fatalf("expected test-profile, got %s", cfg.Name)
	}
	if !cfg.IsActive {
		t.Fatal("expected IsActive true")
	}
}

func TestPeerStatsFields(t *testing.T) {
	stats := PeerStats{
		TxBytes:           1024,
		RxBytes:           2048,
		LastHandshakeSec:  1000,
		LastHandshakeNsec: 500,
		Endpoint:          "1.2.3.4:51820",
	}

	if stats.TxBytes != 1024 {
		t.Fatalf("expected TxBytes 1024, got %d", stats.TxBytes)
	}
	if stats.RxBytes != 2048 {
		t.Fatalf("expected RxBytes 2048, got %d", stats.RxBytes)
	}
	if stats.Endpoint != "1.2.3.4:51820" {
		t.Fatalf("expected endpoint 1.2.3.4:51820, got %s", stats.Endpoint)
	}
}

func TestMaskPrivateKey(t *testing.T) {
	key := "cGFzc3dvcmRwYXNzd29yZHBhc3N3b3JkcGFzc3dvcmQ="
	masked := MaskPrivateKey(key)
	if masked == key {
		t.Fatal("expected masked key to differ from original")
	}
	if len(masked) < 8 {
		t.Fatal("expected masked key to have reasonable length")
	}
	if masked[:4] != key[:4] {
		t.Fatal("expected first 4 chars to be visible")
	}
}

func TestMaskPrivateKeyShort(t *testing.T) {
	key := "abc"
	masked := MaskPrivateKey(key)
	if masked != "***" {
		t.Fatalf("expected ***, got %s", masked)
	}
}
