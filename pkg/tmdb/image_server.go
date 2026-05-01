package tmdb

import (
	"net/http"
	"path/filepath"
	"strings"
)

type ImageServer struct {
	imageDir string
}

func NewImageServer(imageDir string) *ImageServer {
	return &ImageServer{imageDir: imageDir}
}

func (s *ImageServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	if strings.Contains(path, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(s.imageDir, path)

	ext := filepath.Ext(fullPath)
	switch ext {
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, fullPath)
}
