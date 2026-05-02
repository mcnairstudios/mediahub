package bolt

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	bbolt "go.etcd.io/bbolt"
)

func TestProbeCacheStore_GetSetDelete(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	cache := db.ProbeCacheStore()
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

func TestProbeCacheStore_HashesURL(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	cache := db.ProbeCacheStore()

	probe1 := &media.ProbeResult{Video: &media.VideoInfo{Codec: "h264"}}
	probe2 := &media.ProbeResult{Video: &media.VideoInfo{Codec: "hevc"}}

	cache.Set("http://a.com/stream1", probe1)
	cache.Set("http://b.com/stream2", probe2)

	r1, _ := cache.Get("http://a.com/stream1")
	r2, _ := cache.Get("http://b.com/stream2")

	if r1.Video.Codec != "h264" {
		t.Errorf("expected h264, got %s", r1.Video.Codec)
	}
	if r2.Video.Codec != "hevc" {
		t.Errorf("expected hevc, got %s", r2.Video.Codec)
	}
}

func TestProbeCacheStore_MigrateFromFlatKeys(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate_probe.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}

	url1 := "http://example.com/stream1.ts"
	url2 := "http://example.com/stream2.ts"

	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketProbeCache)
		if err != nil {
			return err
		}

		entries := []struct {
			url   string
			probe *media.ProbeResult
		}{
			{url1, &media.ProbeResult{Video: &media.VideoInfo{Codec: "h264", Width: 1920}}},
			{url2, &media.ProbeResult{Video: &media.VideoInfo{Codec: "hevc", Width: 3840}}},
		}

		for _, e := range entries {
			entry := probeCacheEntry{Result: e.probe, StoredAt: time.Now()}
			data, _ := json.Marshal(entry)
			b.Put([]byte(hashURL(e.url)), data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	cache := db.ProbeCacheStore()

	r1, err := cache.Get(url1)
	if err != nil {
		t.Fatalf("Get url1: %v", err)
	}
	if r1 == nil || r1.Video.Codec != "h264" {
		t.Fatalf("expected h264, got %+v", r1)
	}

	r2, err := cache.Get(url2)
	if err != nil {
		t.Fatalf("Get url2: %v", err)
	}
	if r2 == nil || r2.Video.Codec != "hevc" {
		t.Fatalf("expected hevc, got %+v", r2)
	}
}
