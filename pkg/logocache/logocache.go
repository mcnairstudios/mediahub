package logocache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const Placeholder = `data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='200' height='200' viewBox='0 0 200 200'%3E%3Crect width='200' height='200' rx='20' fill='%23374151'/%3E%3Ctext x='100' y='115' font-family='sans-serif' font-size='80' fill='%239CA3AF' text-anchor='middle'%3ETV%3C/text%3E%3C/svg%3E`

type Cache struct {
	dir    string
	client *http.Client
	index  map[string]string
	mu     sync.RWMutex
}

func New(cacheDir string) *Cache {
	c := &Cache{
		dir:    cacheDir,
		client: &http.Client{Timeout: 10 * time.Second},
		index:  make(map[string]string),
	}
	os.MkdirAll(cacheDir, 0755)
	c.buildIndex()
	return c
}

func (c *Cache) buildIndex() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Size() < 200 {
			os.Remove(filepath.Join(c.dir, name))
			continue
		}
		hash := strings.TrimSuffix(name, filepath.Ext(name))
		c.index[hash] = name
	}
}

func (c *Cache) Resolve(logoURL string) string {
	if logoURL == "" {
		return Placeholder
	}
	if strings.HasPrefix(logoURL, "/") || strings.HasPrefix(logoURL, "data:") {
		return logoURL
	}
	if strings.HasPrefix(logoURL, "http://") || strings.HasPrefix(logoURL, "https://") {
		return "/logo?url=" + url.QueryEscape(logoURL)
	}
	return Placeholder
}

func (c *Cache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logoURL := r.URL.Query().Get("url")
	if logoURL == "" {
		http.Error(w, "missing url parameter", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(logoURL, "http://") && !strings.HasPrefix(logoURL, "https://") {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}

	hash := hashURL(logoURL)

	c.mu.RLock()
	filename, ok := c.index[hash]
	c.mu.RUnlock()

	if ok {
		cachedPath := filepath.Join(c.dir, filename)
		if _, err := os.Stat(cachedPath); err == nil {
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeFile(w, r, cachedPath)
			return
		}
		c.mu.Lock()
		delete(c.index, hash)
		c.mu.Unlock()
	}

	filename = c.fetch(r.Context(), logoURL, hash)
	if filename == "" {
		http.Error(w, "failed to fetch logo", http.StatusBadGateway)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filepath.Join(c.dir, filename))
}

func (c *Cache) fetch(ctx context.Context, logoURL, hash string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, logoURL, nil)
	if err != nil {
		return ""
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "image/") {
		return ""
	}

	ext := detectExtension(ct, logoURL)
	filename := hash + ext
	path := filepath.Join(c.dir, filename)

	f, err := os.Create(path)
	if err != nil {
		return ""
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(path)
		return ""
	}
	f.Close()

	c.mu.Lock()
	c.index[hash] = filename
	c.mu.Unlock()

	return filename
}

func hashURL(u string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(u)))[:16]
}

func detectExtension(contentType, u string) string {
	if contentType != "" {
		ct := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
		exts, _ := mime.ExtensionsByType(ct)
		if len(exts) > 0 {
			for _, e := range exts {
				if e == ".png" || e == ".jpg" || e == ".jpeg" || e == ".svg" || e == ".webp" || e == ".gif" {
					return e
				}
			}
			return exts[0]
		}
	}
	ext := filepath.Ext(strings.SplitN(u, "?", 2)[0])
	if ext != "" && len(ext) <= 5 {
		return ext
	}
	return ".png"
}
