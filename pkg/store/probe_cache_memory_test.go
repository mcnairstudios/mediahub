package store

import (
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

func TestMemoryProbeCache_GetSetDelete(t *testing.T) {
	cache := NewMemoryProbeCache()
	url := "http://example.com/stream.ts"

	result, err := cache.Get(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil for cache miss")
	}

	probe := &media.ProbeResult{
		Video: &media.VideoInfo{
			Codec:  "h264",
			Width:  1920,
			Height: 1080,
		},
		AudioTracks: []media.AudioTrack{
			{Codec: "aac", Channels: 2, SampleRate: 48000},
		},
	}

	if err := cache.Set(url, probe); err != nil {
		t.Fatalf("unexpected error on set: %v", err)
	}

	result, err = cache.Get(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected cached result")
	}
	if result.Video.Codec != "h264" {
		t.Errorf("expected h264, got %s", result.Video.Codec)
	}
	if result.Video.Width != 1920 {
		t.Errorf("expected width 1920, got %d", result.Video.Width)
	}
	if len(result.AudioTracks) != 1 {
		t.Fatalf("expected 1 audio track, got %d", len(result.AudioTracks))
	}

	if err := cache.Delete(url); err != nil {
		t.Fatalf("unexpected error on delete: %v", err)
	}

	result, err = cache.Get(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestMemoryProbeCache_DifferentURLs(t *testing.T) {
	cache := NewMemoryProbeCache()

	probe1 := &media.ProbeResult{
		Video: &media.VideoInfo{Codec: "h264"},
	}
	probe2 := &media.ProbeResult{
		Video: &media.VideoInfo{Codec: "hevc"},
	}

	cache.Set("http://a.com/1", probe1)
	cache.Set("http://b.com/2", probe2)

	r1, _ := cache.Get("http://a.com/1")
	r2, _ := cache.Get("http://b.com/2")

	if r1.Video.Codec != "h264" {
		t.Errorf("expected h264, got %s", r1.Video.Codec)
	}
	if r2.Video.Codec != "hevc" {
		t.Errorf("expected hevc, got %s", r2.Video.Codec)
	}
}

func TestMemoryProbeCache_DeleteNonexistent(t *testing.T) {
	cache := NewMemoryProbeCache()
	if err := cache.Delete("http://nonexistent.com"); err != nil {
		t.Fatalf("delete of nonexistent key should not error: %v", err)
	}
}
