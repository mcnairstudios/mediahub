package logocache

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestResolve(t *testing.T) {
	c := New(t.TempDir())

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"empty returns placeholder", "", Placeholder},
		{"local path unchanged", "/images/logo.png", "/images/logo.png"},
		{"data URI unchanged", "data:image/png;base64,abc", "data:image/png;base64,abc"},
		{"http URL rewritten", "http://example.com/logo.png", "/logo?url=http%3A%2F%2Fexample.com%2Flogo.png"},
		{"https URL rewritten", "https://cdn.example.com/img.jpg", "/logo?url=https%3A%2F%2Fcdn.example.com%2Fimg.jpg"},
		{"non-http returns placeholder", "ftp://example.com/logo.png", Placeholder},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Resolve(tt.input)
			if got != tt.expect {
				t.Errorf("Resolve(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestServeHTTP_FetchAndCache(t *testing.T) {
	var fetchCount atomic.Int32
	logoData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	logoData = append(logoData, make([]byte, 300)...)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		w.Header().Set("Content-Type", "image/png")
		w.Write(logoData)
	}))
	defer upstream.Close()

	c := New(t.TempDir())

	req := httptest.NewRequest("GET", "/logo?url="+upstream.URL+"/logo.png", nil)
	rec := httptest.NewRecorder()
	c.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("first request: got %d, want 200", rec.Code)
	}
	if fetchCount.Load() != 1 {
		t.Fatalf("first request: expected 1 upstream fetch, got %d", fetchCount.Load())
	}

	rec2 := httptest.NewRecorder()
	c.ServeHTTP(rec2, httptest.NewRequest("GET", "/logo?url="+upstream.URL+"/logo.png", nil))

	if rec2.Code != http.StatusOK {
		t.Fatalf("second request: got %d, want 200", rec2.Code)
	}
	if fetchCount.Load() != 1 {
		t.Fatalf("second request: expected 1 upstream fetch (cached), got %d", fetchCount.Load())
	}
}

func TestServeHTTP_MissingURL(t *testing.T) {
	c := New(t.TempDir())
	rec := httptest.NewRecorder()
	c.ServeHTTP(rec, httptest.NewRequest("GET", "/logo", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing url: got %d, want 400", rec.Code)
	}
}

func TestServeHTTP_InvalidURL(t *testing.T) {
	c := New(t.TempDir())
	rec := httptest.NewRecorder()
	c.ServeHTTP(rec, httptest.NewRequest("GET", "/logo?url=not-a-url", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid url: got %d, want 400", rec.Code)
	}
}

func TestServeHTTP_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	c := New(t.TempDir())
	rec := httptest.NewRecorder()
	c.ServeHTTP(rec, httptest.NewRequest("GET", "/logo?url="+upstream.URL+"/fail.png", nil))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("upstream error: got %d, want 502", rec.Code)
	}
}

func TestBuildIndex_IgnoresSmallFiles(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "abc123.png"), make([]byte, 50), 0644)
	os.WriteFile(filepath.Join(dir, "def456.png"), make([]byte, 300), 0644)

	c := New(dir)

	c.mu.RLock()
	defer c.mu.RUnlock()

	if _, ok := c.index["abc123"]; ok {
		t.Error("small file should have been removed from index")
	}
	if _, ok := c.index["def456"]; !ok {
		t.Error("large file should be in index")
	}
}
