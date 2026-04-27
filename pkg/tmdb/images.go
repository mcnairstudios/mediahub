package tmdb

import (
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ImageCache struct {
	dir    string
	client *http.Client
	mu     sync.Mutex
}

func NewImageCache(dir string) *ImageCache {
	os.MkdirAll(dir, 0755)
	return &ImageCache{
		dir:    dir,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (ic *ImageCache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tmdbPath := r.URL.Query().Get("path")
	size := r.URL.Query().Get("size")

	if tmdbPath == "" {
		http.Error(w, "missing path parameter", http.StatusBadRequest)
		return
	}
	if size == "" {
		size = "w500"
	}

	filename := ic.hashKey(size + tmdbPath)
	if cached := ic.findCached(filename); cached != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeFile(w, r, cached)
		return
	}

	imageURL := "https://image.tmdb.org/t/p/" + size + tmdbPath
	resp, err := ic.client.Get(imageURL)
	if err != nil {
		http.Error(w, "fetch failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "upstream error", resp.StatusCode)
		return
	}

	ext := detectExtension(resp.Header.Get("Content-Type"), tmdbPath)
	cached := filepath.Join(ic.dir, filename+ext)

	ic.mu.Lock()
	defer ic.mu.Unlock()

	if existing := ic.findCached(filename); existing != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeFile(w, r, existing)
		return
	}

	f, err := os.Create(cached)
	if err != nil {
		http.Error(w, "cache write failed", http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(cached)
		http.Error(w, "cache write failed", http.StatusInternalServerError)
		return
	}
	f.Close()

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, cached)
}

func (ic *ImageCache) hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", h[:16])
}

func (ic *ImageCache) findCached(filename string) string {
	matches, _ := filepath.Glob(filepath.Join(ic.dir, filename+".*"))
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func detectExtension(contentType, path string) string {
	if contentType != "" {
		ct := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
		exts, _ := mime.ExtensionsByType(ct)
		if len(exts) > 0 {
			for _, e := range exts {
				if e == ".png" || e == ".jpg" || e == ".jpeg" || e == ".webp" {
					return e
				}
			}
			return exts[0]
		}
	}
	ext := filepath.Ext(strings.SplitN(path, "?", 2)[0])
	if ext != "" && len(ext) <= 5 {
		return ext
	}
	return ".jpg"
}
