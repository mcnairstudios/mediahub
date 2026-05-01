package hls

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/output"
)

type SubprocessPlugin struct {
	segDir     string
	generation atomic.Int64
	stopped    atomic.Bool
}

func NewSubprocessPlugin(outputDir string) (*SubprocessPlugin, error) {
	segDir := filepath.Join(outputDir, "segments")
	if err := os.MkdirAll(segDir, 0755); err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(segDir)
	if err != nil {
		return nil, err
	}
	p := &SubprocessPlugin{segDir: resolved}
	p.generation.Store(1)
	return p, nil
}

func (p *SubprocessPlugin) Mode() output.DeliveryMode {
	return output.DeliveryHLS
}

func (p *SubprocessPlugin) PushVideo(_ []byte, _, _ int64, _ bool) error {
	return nil
}

func (p *SubprocessPlugin) PushAudio(_ []byte, _, _ int64) error {
	return nil
}

func (p *SubprocessPlugin) PushSubtitle(_ []byte, _ int64, _ int64) error {
	return nil
}

func (p *SubprocessPlugin) EndOfStream() {
	p.stopped.Store(true)
}

func (p *SubprocessPlugin) ResetForSeek() {
	p.generation.Add(1)
}

func (p *SubprocessPlugin) Stop() {
	p.stopped.Store(true)
}

func (p *SubprocessPlugin) Status() output.PluginStatus {
	return output.PluginStatus{
		Mode:    output.DeliveryHLS,
		Healthy: !p.stopped.Load(),
	}
}

func (p *SubprocessPlugin) Generation() int64 {
	return p.generation.Load()
}

func (p *SubprocessPlugin) WaitReady(ctx context.Context) error {
	playlistPath := filepath.Join(p.segDir, "playlist.m3u8")

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := os.Stat(playlistPath); err == nil {
				return nil
			}
		}
	}
}

func (p *SubprocessPlugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := r.URL.Path

	if path == "/playlist.m3u8" || path == "playlist.m3u8" {
		p.servePlaylist(w, r)
		return
	}

	if strings.HasSuffix(path, ".m4s") || strings.HasSuffix(path, ".mp4") {
		p.serveSegment(w, r, path)
		return
	}

	if strings.HasSuffix(path, ".ts") {
		p.serveSegment(w, r, path)
		return
	}

	http.NotFound(w, r)
}

func (p *SubprocessPlugin) servePlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := p.WaitReady(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	playlistPath := filepath.Join(p.segDir, "playlist.m3u8")
	data, err := os.ReadFile(playlistPath)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Write(data) //nolint:errcheck
}

func (p *SubprocessPlugin) serveSegment(w http.ResponseWriter, _ *http.Request, path string) {
	name := filepath.Base(path)

	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, string(os.PathSeparator)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	cleaned := filepath.Clean(name)
	if cleaned != name || strings.Contains(cleaned, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	segPath := filepath.Join(p.segDir, name)

	resolved, err := filepath.EvalSymlinks(segPath)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	if !strings.HasPrefix(resolved, p.segDir) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	data, err := os.ReadFile(segPath)
	if err != nil {
		http.NotFound(w, nil)
		return
	}

	contentType := "video/mp4"
	if strings.HasSuffix(name, ".ts") {
		contentType = "video/mp2t"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Write(data) //nolint:errcheck
}
