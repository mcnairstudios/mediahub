package wg

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/connectivity"
)

func TestPluginSatisfiesInterface(t *testing.T) {
	var _ connectivity.Plugin = (*Plugin)(nil)
}

func TestNameReturnsWireguard(t *testing.T) {
	p := &Plugin{}
	if p.Name() != "wireguard" {
		t.Fatalf("expected wireguard, got %s", p.Name())
	}
}

func TestProxyURLFormat(t *testing.T) {
	p := &Plugin{port: 12345}
	got := p.ProxyURL("http://example.com/stream.ts")
	expected := "http://127.0.0.1:12345/?url=" + url.QueryEscape("http://example.com/stream.ts")
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestProxyURLEncodesSpecialChars(t *testing.T) {
	p := &Plugin{port: 9999}
	upstream := "http://example.com/stream?token=abc&quality=hd"
	got := p.ProxyURL(upstream)
	expected := fmt.Sprintf("http://127.0.0.1:9999/?url=%s", url.QueryEscape(upstream))
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestStartAndStop(t *testing.T) {
	p, err := New(nil, nil, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	if p.port == 0 {
		t.Fatal("expected non-zero port")
	}
	if !p.IsConnected() {
		t.Fatal("expected connected after start")
	}

	err = p.Close()
	if err != nil {
		t.Fatalf("unexpected error on close: %v", err)
	}
	if p.IsConnected() {
		t.Fatal("expected not connected after close")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	p, err := New(nil, nil, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := p.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
	if p.IsConnected() {
		t.Fatal("expected not connected after double close")
	}
}

func TestIsConnectedReflectsState(t *testing.T) {
	p, err := New(nil, nil, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !p.IsConnected() {
		t.Fatal("expected connected after start")
	}

	p.Close()

	if p.IsConnected() {
		t.Fatal("expected not connected after close")
	}
}

func TestHTTPClientNonNil(t *testing.T) {
	p, err := New(nil, nil, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	client := p.HTTPClient()
	if client == nil {
		t.Fatal("expected non-nil http client")
	}
}

func TestProxyForwardsToUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "upstream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello from upstream"))
	}))
	defer upstream.Close()

	p, err := New(nil, nil, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	proxyURL := p.ProxyURL(upstream.URL)
	resp, err := http.Get(proxyURL)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Test") != "upstream" {
		t.Fatal("expected upstream header to be forwarded")
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello from upstream" {
		t.Fatalf("expected upstream body, got %s", string(body))
	}
}

func TestProxyForwardsRangeHeader(t *testing.T) {
	var gotRange string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		w.WriteHeader(http.StatusPartialContent)
	}))
	defer upstream.Close()

	p, err := New(nil, nil, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	proxyURL := p.ProxyURL(upstream.URL)
	req, _ := http.NewRequest("GET", proxyURL, nil)
	req.Header.Set("Range", "bytes=100-200")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	resp.Body.Close()

	if gotRange != "bytes=100-200" {
		t.Fatalf("expected Range header forwarded, got %q", gotRange)
	}
}

func TestProxyMissingURLParam(t *testing.T) {
	p, err := New(nil, nil, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", p.Port()))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing url param, got %d", resp.StatusCode)
	}
}

func TestProxyWithCustomTransport(t *testing.T) {
	var transportUsed bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	customTransport := &recordingTransport{
		inner: http.DefaultTransport,
		onRoundTrip: func() {
			transportUsed = true
		},
	}

	p, err := New(customTransport, nil, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	proxyURL := p.ProxyURL(upstream.URL)
	resp, err := http.Get(proxyURL)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	resp.Body.Close()

	if !transportUsed {
		t.Fatal("expected custom transport to be used")
	}
}

type recordingTransport struct {
	inner       http.RoundTripper
	onRoundTrip func()
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.onRoundTrip()
	return r.inner.RoundTrip(req)
}

func TestProxyUpstreamError(t *testing.T) {
	p, err := New(nil, nil, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	proxyURL := p.ProxyURL("http://127.0.0.1:1")
	resp, err := http.Get(proxyURL)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502 for unreachable upstream, got %d", resp.StatusCode)
	}
}

func TestResolveURLLocallyIP(t *testing.T) {
	resolved, host, err := resolveURLLocally("http://192.168.1.1:8080/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "" {
		t.Fatalf("expected empty originalHost for IP, got %q", host)
	}
	if resolved != "http://192.168.1.1:8080/path" {
		t.Fatalf("expected unchanged URL for IP, got %q", resolved)
	}
}

func TestResolveURLLocallyHostname(t *testing.T) {
	resolved, host, err := resolveURLLocally("http://localhost:8080/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "localhost" {
		t.Fatalf("expected originalHost=localhost, got %q", host)
	}
	if resolved == "http://localhost:8080/path" {
		t.Fatalf("expected URL to be resolved to IP, got %q", resolved)
	}
}

func TestResolveURLLocallyBadHost(t *testing.T) {
	_, _, err := resolveURLLocally("http://this-host-does-not-exist-abc123.invalid/path")
	if err == nil {
		t.Fatal("expected error for unresolvable host")
	}
}

func TestProxyForwardsHostHeader(t *testing.T) {
	var gotHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p, err := New(nil, nil, PluginConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	proxyURL := p.ProxyURL(upstream.URL)
	resp, err := http.Get(proxyURL)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	resp.Body.Close()

	if gotHost == "" {
		t.Fatal("expected Host header to be forwarded")
	}
}

func TestProxyForwardsIncomingHeaders(t *testing.T) {
	var gotUA string
	var gotBypass string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotBypass = r.Header.Get("X-Bypass")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p, err := New(nil, nil, PluginConfig{
		UserAgent:    "TestAgent/1.0",
		BypassHeader: "X-Bypass",
		BypassSecret: "secret123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	proxyURL := p.ProxyURL(upstream.URL)
	req, _ := http.NewRequest("GET", proxyURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	resp.Body.Close()

	if gotUA != "TestAgent/1.0" {
		t.Fatalf("expected User-Agent from config, got %q", gotUA)
	}
	if gotBypass != "secret123" {
		t.Fatalf("expected X-Bypass from config, got %q", gotBypass)
	}
}
