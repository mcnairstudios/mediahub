package youtube

import (
	"context"
	"net/http"
	"testing"
	"time"

	ytlib "github.com/kkdai/youtube/v2"
)

func TestIsYouTubeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"standard watch URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"http watch URL", "http://www.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"no www", "https://youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"mobile", "https://m.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"short link", "https://youtu.be/dQw4w9WgXcQ", true},
		{"shorts", "https://www.youtube.com/shorts/abc123", true},
		{"live", "https://www.youtube.com/live/abc123", true},
		{"watch no v param", "https://www.youtube.com/watch", false},
		{"empty youtu.be", "https://youtu.be/", false},
		{"channel page", "https://www.youtube.com/channel/abc", false},
		{"other site", "https://example.com/watch?v=abc", false},
		{"rtsp URL", "rtsp://192.168.1.1:554/stream", false},
		{"empty string", "", false},
		{"not a URL", "not-a-url", false},
		{"extra params", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=30", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsYouTubeURL(tt.url)
			if got != tt.want {
				t.Errorf("IsYouTubeURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestIsBetterFormat(t *testing.T) {
	tests := []struct {
		name    string
		aH, bH int
		want    bool
	}{
		{"720 vs 360", 720, 360, true},
		{"1080 vs 720", 1080, 720, true},
		{"360 vs 720", 360, 720, false},
		{"1080 vs 1440", 1080, 1440, true},
		{"1440 vs 1080", 1440, 1080, false},
		{"2160 vs 1440 both over", 2160, 1440, false},
		{"1440 vs 2160 both over", 1440, 2160, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := formatWithHeight(tt.aH)
			b := formatWithHeight(tt.bH)
			got := isBetterFormat(a, b)
			if got != tt.want {
				t.Errorf("isBetterFormat(h=%d, h=%d) = %v, want %v", tt.aH, tt.bH, got, tt.want)
			}
		})
	}
}

func formatWithHeight(h int) ytlib.Format {
	return ytlib.Format{Height: h}
}

// TestResolveStreamURL_Integration tests actual YouTube resolution.
// Skipped when network is unavailable.
func TestResolveStreamURL_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Quick network check.
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://www.youtube.com")
	if err != nil {
		t.Skipf("skipping: network unavailable: %v", err)
	}
	resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Rick Astley - Never Gonna Give You Up (well-known, stable video).
	streamURL, err := ResolveStreamURL(ctx, "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("ResolveStreamURL failed: %v", err)
	}

	if streamURL == "" {
		t.Fatal("ResolveStreamURL returned empty URL")
	}

	t.Logf("resolved URL (truncated): %.100s...", streamURL)
}
