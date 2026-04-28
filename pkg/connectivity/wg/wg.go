package wg

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Plugin struct {
	listener  net.Listener
	server    *http.Server
	port      int
	transport http.RoundTripper
	tunnel    *Tunnel
	connected bool
	closed    bool
	mu        sync.RWMutex
}

func resolveURLLocally(targetURL string) (resolvedURL string, originalHost string, err error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return "", "", err
	}

	host := u.Hostname()
	if net.ParseIP(host) != nil {
		return targetURL, "", nil
	}

	addrs, err := net.LookupHost(host)
	if err != nil {
		return "", "", fmt.Errorf("dns: %w", err)
	}
	if len(addrs) == 0 {
		return "", "", fmt.Errorf("dns: no addresses for %q", host)
	}

	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	originalHost = host
	u.Host = net.JoinHostPort(addrs[0], port)
	return u.String(), originalHost, nil
}

type PluginConfig struct {
	UserAgent    string
	BypassHeader string
	BypassSecret string
}

func New(transport http.RoundTripper, tunnel *Tunnel, cfg PluginConfig) (*Plugin, error) {
	if transport == nil {
		transport = http.DefaultTransport
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("binding wg proxy: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	p := &Plugin{
		listener:  listener,
		port:      port,
		transport: transport,
		tunnel:    tunnel,
		connected: true,
	}

	proxyClient := &http.Client{Transport: transport}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetURL := r.URL.Query().Get("url")
		if targetURL == "" {
			http.Error(w, "missing url param", http.StatusBadRequest)
			return
		}

		outReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		if cfg.UserAgent != "" {
			outReq.Header.Set("User-Agent", cfg.UserAgent)
		}
		if cfg.BypassHeader != "" && cfg.BypassSecret != "" {
			outReq.Header.Set(cfg.BypassHeader, cfg.BypassSecret)
		}
		if rng := r.Header.Get("Range"); rng != "" {
			outReq.Header.Set("Range", rng)
		}

		resp, err := proxyClient.Do(outReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	p.server = &http.Server{Handler: handler}

	go func() {
		if err := p.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			p.mu.Lock()
			p.connected = false
			p.mu.Unlock()
		}
	}()

	return p, nil
}

func (p *Plugin) Name() string {
	return "wireguard"
}

func (p *Plugin) ProxyURL(upstreamURL string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/?url=%s", p.port, url.QueryEscape(upstreamURL))
}

func (p *Plugin) HTTPClient() *http.Client {
	return &http.Client{
		Transport: &proxyTransport{proxyBase: fmt.Sprintf("http://127.0.0.1:%d", p.port)},
		Timeout:   30 * time.Minute,
	}
}

func (p *Plugin) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.connected || p.tunnel == nil {
		return p.connected
	}
	stats, err := p.tunnel.Stats()
	if err != nil {
		return false
	}
	return stats.LastHandshakeSec > 0
}

func (p *Plugin) Stats() (*PeerStats, error) {
	if p.tunnel == nil {
		return nil, fmt.Errorf("no tunnel")
	}
	return p.tunnel.Stats()
}

func (p *Plugin) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	p.connected = false
	return p.server.Shutdown(context.Background())
}

func (p *Plugin) Port() int {
	return p.port
}

type proxyTransport struct {
	proxyBase string
}

func (t *proxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	targetURL := req.URL.String()
	proxyReq, err := http.NewRequestWithContext(
		req.Context(),
		req.Method,
		t.proxyBase+"/?url="+url.QueryEscape(targetURL),
		req.Body,
	)
	if err != nil {
		return nil, err
	}
	for k, v := range req.Header {
		proxyReq.Header[k] = v
	}
	return http.DefaultClient.Do(proxyReq)
}
