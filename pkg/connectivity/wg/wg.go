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
	connected bool
	closed    bool
	mu        sync.RWMutex
}

func New(transport http.RoundTripper) (*Plugin, error) {
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
		Timeout:   60 * time.Second,
	}
}

func (p *Plugin) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.connected
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
