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

func TestSessionDoubleStopDoesNotPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newSession(ctx, cancel, "stream-1", "http://example.com/stream", "Test", "/tmp/out")

	s.Stop()
	s.Stop()

	select {
	case <-s.Done():
	case <-time.After(time.Second):
		t.Fatal("expected done channel to close after Stop")
	}
}

func TestSessionStopDuringActivePush(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newSession(ctx, cancel, "stream-1", "http://example.com/stream", "Test", "/tmp/out")

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			s.FanOut.PushVideo([]byte("frame"), int64(i)*1000, int64(i)*1000, i == 0)
		}
		close(done)
	}()

	time.Sleep(time.Millisecond)
	s.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("push goroutine did not complete")
	}
}

func TestSessionStopClosesClosers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newSession(ctx, cancel, "stream-1", "http://example.com/stream", "Test", "/tmp/out")

	closed := false
	s.AddCloser(closerFunc(func() error { closed = true; return nil }))

	s.Stop()

	if !closed {
		t.Fatal("expected closer to be called on Stop")
	}
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }
