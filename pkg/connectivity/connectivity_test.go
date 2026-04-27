package connectivity

import (
	"errors"
	"net/http"
	"sort"
	"testing"
)

type mockPlugin struct {
	name      string
	proxyURL  string
	connected bool
	closed    bool
}

func (m *mockPlugin) Name() string              { return m.name }
func (m *mockPlugin) ProxyURL(url string) string { return m.proxyURL + "?url=" + url }
func (m *mockPlugin) HTTPClient() *http.Client   { return &http.Client{} }
func (m *mockPlugin) IsConnected() bool          { return m.connected }
func (m *mockPlugin) Close() error {
	m.closed = true
	return nil
}

func TestMockSatisfiesPlugin(t *testing.T) {
	var _ Plugin = (*mockPlugin)(nil)
}

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	p := &mockPlugin{name: "wireguard"}
	reg.Register(p)

	got, ok := reg.Get("wireguard")
	if !ok {
		t.Fatal("expected to find wireguard plugin")
	}
	if got.Name() != "wireguard" {
		t.Fatalf("expected name wireguard, got %s", got.Name())
	}
}

func TestRegistryUnknownReturnsFalse(t *testing.T) {
	reg := NewRegistry()

	_, ok := reg.Get("unknown")
	if ok {
		t.Fatal("expected false for unknown plugin")
	}
}

func TestRegistrySetActiveAndActive(t *testing.T) {
	reg := NewRegistry()
	p := &mockPlugin{name: "wireguard"}
	reg.Register(p)

	if reg.Active() != nil {
		t.Fatal("expected nil active plugin before SetActive")
	}

	if err := reg.SetActive("wireguard"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	active := reg.Active()
	if active == nil {
		t.Fatal("expected active plugin after SetActive")
	}
	if active.Name() != "wireguard" {
		t.Fatalf("expected active wireguard, got %s", active.Name())
	}
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockPlugin{name: "wireguard"})
	reg.Register(&mockPlugin{name: "tailscale"})
	reg.Register(&mockPlugin{name: "tor"})

	names := reg.List()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}

	sort.Strings(names)
	expected := []string{"tailscale", "tor", "wireguard"}
	for i, e := range expected {
		if names[i] != e {
			t.Fatalf("expected %s at index %d, got %s", e, i, names[i])
		}
	}
}

func TestRegistrySetActiveUnknownReturnsError(t *testing.T) {
	reg := NewRegistry()

	err := reg.SetActive("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}
	if !errors.Is(err, ErrUnknownPlugin) {
		t.Fatalf("expected ErrUnknownPlugin, got %v", err)
	}
}

func TestProxyURLOnMock(t *testing.T) {
	p := &mockPlugin{name: "wireguard", proxyURL: "http://127.0.0.1:9999"}
	got := p.ProxyURL("http://example.com/stream")
	expected := "http://127.0.0.1:9999?url=http://example.com/stream"
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestHTTPClientOnMock(t *testing.T) {
	p := &mockPlugin{name: "wireguard"}
	client := p.HTTPClient()
	if client == nil {
		t.Fatal("expected non-nil http client")
	}
}

func TestIsConnectedOnMock(t *testing.T) {
	p := &mockPlugin{name: "wireguard", connected: false}
	if p.IsConnected() {
		t.Fatal("expected not connected")
	}
	p.connected = true
	if !p.IsConnected() {
		t.Fatal("expected connected")
	}
}

func TestRegistryActiveNilWhenEmpty(t *testing.T) {
	reg := NewRegistry()
	if reg.Active() != nil {
		t.Fatal("expected nil active on empty registry")
	}
}

func TestRegistryListEmpty(t *testing.T) {
	reg := NewRegistry()
	names := reg.List()
	if len(names) != 0 {
		t.Fatalf("expected 0 names, got %d", len(names))
	}
}
