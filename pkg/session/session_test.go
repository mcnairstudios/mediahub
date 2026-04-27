package session

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newSession(ctx, cancel, "stream-1", "http://example.com/stream", "Test Stream", "/tmp/out")

	if s.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if s.StreamID != "stream-1" {
		t.Fatalf("expected StreamID stream-1, got %s", s.StreamID)
	}
	if s.StreamURL != "http://example.com/stream" {
		t.Fatalf("expected StreamURL, got %s", s.StreamURL)
	}
	if s.StreamName != "Test Stream" {
		t.Fatalf("expected StreamName, got %s", s.StreamName)
	}
	if s.OutputDir != "/tmp/out/stream-1" {
		t.Fatalf("expected OutputDir /tmp/out/stream-1, got %s", s.OutputDir)
	}
	if s.FanOut == nil {
		t.Fatal("expected non-nil FanOut")
	}
	if s.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
}

func TestSessionRecorded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newSession(ctx, cancel, "stream-1", "http://example.com/stream", "Test", "/tmp/out")

	if s.IsRecorded() {
		t.Fatal("expected not recorded initially")
	}

	s.SetRecorded(true)
	if !s.IsRecorded() {
		t.Fatal("expected recorded after SetRecorded(true)")
	}

	s.SetRecorded(false)
	if s.IsRecorded() {
		t.Fatal("expected not recorded after SetRecorded(false)")
	}
}

func TestSessionStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newSession(ctx, cancel, "stream-1", "http://example.com/stream", "Test", "/tmp/out")

	s.Stop()

	select {
	case <-s.Done():
	case <-time.After(time.Second):
		t.Fatal("expected done channel to close after Stop")
	}
}

func TestSessionSeek(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newSession(ctx, cancel, "stream-1", "http://example.com/stream", "Test", "/tmp/out")

	var seeked atomic.Int64
	s.SetSeekFunc(func(posMs int64) {
		seeked.Store(posMs)
	})

	s.Seek(5000)
	if seeked.Load() != 5000 {
		t.Fatalf("expected seek to 5000, got %d", seeked.Load())
	}
}

func TestSessionSeekNoFunc(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newSession(ctx, cancel, "stream-1", "http://example.com/stream", "Test", "/tmp/out")

	s.Seek(5000)
}
