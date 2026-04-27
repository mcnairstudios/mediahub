package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

func TestTick_StartsDueRecording(t *testing.T) {
	rs := store.NewMemoryRecordingStore()
	ctx := context.Background()

	rec := &recording.Recording{
		ID:             "rec-1",
		StreamID:       "stream-1",
		Title:          "Evening News",
		Status:         recording.StatusScheduled,
		ScheduledStart: time.Now().Add(-time.Minute),
		ScheduledStop:  time.Now().Add(time.Hour),
	}
	rs.Create(ctx, rec)

	var started []string
	var mu sync.Mutex

	s := New(rs)
	s.SetStartFunc(func(streamID, title string) error {
		mu.Lock()
		started = append(started, streamID)
		mu.Unlock()
		return nil
	})
	s.SetStopFunc(func(streamID string) error { return nil })

	s.Tick(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(started) != 1 {
		t.Fatalf("expected 1 started, got %d", len(started))
	}
	if started[0] != "stream-1" {
		t.Errorf("expected stream-1, got %s", started[0])
	}

	updated, _ := rs.Get(ctx, "rec-1")
	if updated.Status != recording.StatusRecording {
		t.Errorf("expected recording status, got %s", updated.Status)
	}
}

func TestTick_StopsExpiredRecording(t *testing.T) {
	rs := store.NewMemoryRecordingStore()
	ctx := context.Background()

	rec := &recording.Recording{
		ID:             "rec-1",
		StreamID:       "stream-1",
		Title:          "Evening News",
		Status:         recording.StatusRecording,
		StartedAt:      time.Now().Add(-2 * time.Hour),
		ScheduledStart: time.Now().Add(-2 * time.Hour),
		ScheduledStop:  time.Now().Add(-time.Minute),
	}
	rs.Create(ctx, rec)

	var stopped []string
	var mu sync.Mutex

	s := New(rs)
	s.SetStartFunc(func(streamID, title string) error { return nil })
	s.SetStopFunc(func(streamID string) error {
		mu.Lock()
		stopped = append(stopped, streamID)
		mu.Unlock()
		return nil
	})

	s.Tick(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(stopped) != 1 {
		t.Fatalf("expected 1 stopped, got %d", len(stopped))
	}
	if stopped[0] != "stream-1" {
		t.Errorf("expected stream-1, got %s", stopped[0])
	}

	updated, _ := rs.Get(ctx, "rec-1")
	if updated.Status != recording.StatusCompleted {
		t.Errorf("expected completed status, got %s", updated.Status)
	}
}

func TestTick_ScheduledRecordingListing(t *testing.T) {
	rs := store.NewMemoryRecordingStore()
	ctx := context.Background()

	for i, id := range []string{"rec-1", "rec-2", "rec-3"} {
		rec := &recording.Recording{
			ID:             id,
			StreamID:       "stream-1",
			Title:          "Show " + id,
			Status:         recording.StatusScheduled,
			ScheduledStart: time.Now().Add(time.Duration(i+1) * time.Hour),
			ScheduledStop:  time.Now().Add(time.Duration(i+2) * time.Hour),
		}
		rs.Create(ctx, rec)
	}

	scheduled, err := rs.ListScheduled(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scheduled) != 3 {
		t.Fatalf("expected 3 scheduled, got %d", len(scheduled))
	}
}

func TestTick_CancelRemovesScheduledRecording(t *testing.T) {
	rs := store.NewMemoryRecordingStore()
	ctx := context.Background()

	rec := &recording.Recording{
		ID:             "rec-1",
		StreamID:       "stream-1",
		Title:          "Cancelled Show",
		Status:         recording.StatusScheduled,
		ScheduledStart: time.Now().Add(time.Hour),
		ScheduledStop:  time.Now().Add(2 * time.Hour),
	}
	rs.Create(ctx, rec)

	rs.Delete(ctx, "rec-1")

	scheduled, err := rs.ListScheduled(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scheduled) != 0 {
		t.Fatalf("expected 0 scheduled after cancel, got %d", len(scheduled))
	}
}

func TestTick_IgnoresFutureRecordings(t *testing.T) {
	rs := store.NewMemoryRecordingStore()
	ctx := context.Background()

	rec := &recording.Recording{
		ID:             "rec-1",
		StreamID:       "stream-1",
		Title:          "Future Show",
		Status:         recording.StatusScheduled,
		ScheduledStart: time.Now().Add(time.Hour),
		ScheduledStop:  time.Now().Add(2 * time.Hour),
	}
	rs.Create(ctx, rec)

	var started []string
	s := New(rs)
	s.SetStartFunc(func(streamID, title string) error {
		started = append(started, streamID)
		return nil
	})
	s.SetStopFunc(func(streamID string) error { return nil })

	s.Tick(ctx)

	if len(started) != 0 {
		t.Errorf("expected no starts for future recording, got %d", len(started))
	}

	updated, _ := rs.Get(ctx, "rec-1")
	if updated.Status != recording.StatusScheduled {
		t.Errorf("expected still scheduled, got %s", updated.Status)
	}
}

func TestTick_FailedStartMarksRecordingFailed(t *testing.T) {
	rs := store.NewMemoryRecordingStore()
	ctx := context.Background()

	rec := &recording.Recording{
		ID:             "rec-1",
		StreamID:       "stream-1",
		Title:          "Bad Stream",
		Status:         recording.StatusScheduled,
		ScheduledStart: time.Now().Add(-time.Minute),
		ScheduledStop:  time.Now().Add(time.Hour),
	}
	rs.Create(ctx, rec)

	s := New(rs)
	s.SetStartFunc(func(streamID, title string) error {
		return context.DeadlineExceeded
	})
	s.SetStopFunc(func(streamID string) error { return nil })

	s.Tick(ctx)

	updated, _ := rs.Get(ctx, "rec-1")
	if updated.Status != recording.StatusFailed {
		t.Errorf("expected failed status, got %s", updated.Status)
	}
}

func TestStartStop(t *testing.T) {
	rs := store.NewMemoryRecordingStore()
	s := New(rs)
	s.SetStartFunc(func(streamID, title string) error { return nil })
	s.SetStopFunc(func(streamID string) error { return nil })

	ctx := context.Background()
	s.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	s.Stop()
}

func TestStartIdempotent(t *testing.T) {
	rs := store.NewMemoryRecordingStore()
	s := New(rs)

	ctx := context.Background()
	s.Start(ctx)
	s.Start(ctx)
	s.Stop()
}

func TestStopWithoutStart(t *testing.T) {
	rs := store.NewMemoryRecordingStore()
	s := New(rs)
	s.Stop()
}

func TestNoStartFuncDoesNotPanic(t *testing.T) {
	rs := store.NewMemoryRecordingStore()
	ctx := context.Background()

	rec := &recording.Recording{
		ID:             "rec-1",
		StreamID:       "stream-1",
		Title:          "No Func",
		Status:         recording.StatusScheduled,
		ScheduledStart: time.Now().Add(-time.Minute),
		ScheduledStop:  time.Now().Add(time.Hour),
	}
	rs.Create(ctx, rec)

	s := New(rs)
	s.Tick(ctx)

	updated, _ := rs.Get(ctx, "rec-1")
	if updated.Status != recording.StatusScheduled {
		t.Errorf("expected still scheduled when no start func, got %s", updated.Status)
	}
}
