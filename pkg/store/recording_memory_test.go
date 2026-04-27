package store

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/recording"
)

func TestRecordingStore_CreateAndGet(t *testing.T) {
	s := NewMemoryRecordingStore()
	ctx := context.Background()

	r := &recording.Recording{
		ID:         "rec-1",
		StreamID:   "s-1",
		StreamName: "BBC One",
		ChannelID:  "ch-1",
		Title:      "News at Ten",
		UserID:     "user-1",
		Status:     recording.StatusScheduled,
		StartedAt:  time.Now(),
	}

	if err := s.Create(ctx, r); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "rec-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Title != "News at Ten" {
		t.Errorf("Title = %q, want %q", got.Title, "News at Ten")
	}
	if got.Status != recording.StatusScheduled {
		t.Errorf("Status = %q, want %q", got.Status, recording.StatusScheduled)
	}
}

func TestRecordingStore_GetUnknownID(t *testing.T) {
	s := NewMemoryRecordingStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get should not error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestRecordingStore_Update(t *testing.T) {
	s := NewMemoryRecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1", Status: recording.StatusScheduled})
	s.Update(ctx, &recording.Recording{ID: "rec-1", Status: recording.StatusRecording})

	got, _ := s.Get(ctx, "rec-1")
	if got.Status != recording.StatusRecording {
		t.Errorf("Status = %q, want %q", got.Status, recording.StatusRecording)
	}
}

func TestRecordingStore_Delete(t *testing.T) {
	s := NewMemoryRecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1"})
	s.Create(ctx, &recording.Recording{ID: "rec-2"})

	if err := s.Delete(ctx, "rec-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, _ := s.Get(ctx, "rec-1")
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}

	got2, _ := s.Get(ctx, "rec-2")
	if got2 == nil {
		t.Error("rec-2 should still exist")
	}
}

func TestRecordingStore_ListAdminSeesAll(t *testing.T) {
	s := NewMemoryRecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1", UserID: "user-1"})
	s.Create(ctx, &recording.Recording{ID: "rec-2", UserID: "user-2"})
	s.Create(ctx, &recording.Recording{ID: "rec-3", UserID: "user-1"})

	list, err := s.List(ctx, "user-1", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("admin got %d recordings, want 3", len(list))
	}
}

func TestRecordingStore_ListNonAdminSeesOwn(t *testing.T) {
	s := NewMemoryRecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1", UserID: "user-1"})
	s.Create(ctx, &recording.Recording{ID: "rec-2", UserID: "user-2"})
	s.Create(ctx, &recording.Recording{ID: "rec-3", UserID: "user-1"})

	list, err := s.List(ctx, "user-1", false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("non-admin got %d recordings, want 2", len(list))
	}

	ids := []string{list[0].ID, list[1].ID}
	sort.Strings(ids)
	if ids[0] != "rec-1" || ids[1] != "rec-3" {
		t.Errorf("IDs = %v, want [rec-1 rec-3]", ids)
	}
}

func TestRecordingStore_ListByStatus(t *testing.T) {
	s := NewMemoryRecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1", Status: recording.StatusScheduled})
	s.Create(ctx, &recording.Recording{ID: "rec-2", Status: recording.StatusRecording})
	s.Create(ctx, &recording.Recording{ID: "rec-3", Status: recording.StatusScheduled})
	s.Create(ctx, &recording.Recording{ID: "rec-4", Status: recording.StatusCompleted})

	list, err := s.ListByStatus(ctx, recording.StatusScheduled)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d recordings, want 2", len(list))
	}

	ids := []string{list[0].ID, list[1].ID}
	sort.Strings(ids)
	if ids[0] != "rec-1" || ids[1] != "rec-3" {
		t.Errorf("IDs = %v, want [rec-1 rec-3]", ids)
	}
}

func TestRecordingStore_ListScheduled(t *testing.T) {
	s := NewMemoryRecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1", Status: recording.StatusScheduled})
	s.Create(ctx, &recording.Recording{ID: "rec-2", Status: recording.StatusRecording})
	s.Create(ctx, &recording.Recording{ID: "rec-3", Status: recording.StatusScheduled})

	list, err := s.ListScheduled(ctx)
	if err != nil {
		t.Fatalf("ListScheduled: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("got %d scheduled, want 2", len(list))
	}
}
