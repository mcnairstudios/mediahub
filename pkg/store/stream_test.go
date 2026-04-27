package store

import (
	"context"
	"sort"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

func TestStreamStore_CreateAndGet(t *testing.T) {
	s := NewMemoryStreamStore()
	ctx := context.Background()

	stream := media.Stream{
		ID:         "stream-1",
		SourceType: "m3u",
		SourceID:   "account-1",
		Name:       "BBC One",
		URL:        "http://example.com/bbc1",
	}

	err := s.BulkUpsert(ctx, []media.Stream{stream})
	if err != nil {
		t.Fatalf("BulkUpsert: %v", err)
	}

	got, err := s.Get(ctx, "stream-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "BBC One" {
		t.Errorf("got name %q, want %q", got.Name, "BBC One")
	}
	if got.URL != "http://example.com/bbc1" {
		t.Errorf("got URL %q, want %q", got.URL, "http://example.com/bbc1")
	}
}

func TestStreamStore_BulkUpsertUpdatesExisting(t *testing.T) {
	s := NewMemoryStreamStore()
	ctx := context.Background()

	original := media.Stream{
		ID:   "stream-1",
		Name: "BBC One",
		URL:  "http://example.com/bbc1",
	}
	s.BulkUpsert(ctx, []media.Stream{original})

	updated := media.Stream{
		ID:   "stream-1",
		Name: "BBC One HD",
		URL:  "http://example.com/bbc1hd",
	}
	s.BulkUpsert(ctx, []media.Stream{updated})

	got, err := s.Get(ctx, "stream-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "BBC One HD" {
		t.Errorf("got name %q, want %q", got.Name, "BBC One HD")
	}
	if got.URL != "http://example.com/bbc1hd" {
		t.Errorf("got URL %q, want %q", got.URL, "http://example.com/bbc1hd")
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Errorf("got %d streams, want 1", len(list))
	}
}

func TestStreamStore_ListBySource(t *testing.T) {
	s := NewMemoryStreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "a1", Name: "One"},
		{ID: "s2", SourceType: "m3u", SourceID: "a1", Name: "Two"},
		{ID: "s3", SourceType: "m3u", SourceID: "a2", Name: "Three"},
		{ID: "s4", SourceType: "satip", SourceID: "b1", Name: "Four"},
	}
	s.BulkUpsert(ctx, streams)

	got, err := s.ListBySource(ctx, "m3u", "a1")
	if err != nil {
		t.Fatalf("ListBySource: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d streams, want 2", len(got))
	}

	names := []string{got[0].Name, got[1].Name}
	sort.Strings(names)
	if names[0] != "One" || names[1] != "Two" {
		t.Errorf("got names %v, want [One Two]", names)
	}
}

func TestStreamStore_DeleteBySource(t *testing.T) {
	s := NewMemoryStreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "a1"},
		{ID: "s2", SourceType: "m3u", SourceID: "a1"},
		{ID: "s3", SourceType: "m3u", SourceID: "a2"},
	}
	s.BulkUpsert(ctx, streams)

	err := s.DeleteBySource(ctx, "m3u", "a1")
	if err != nil {
		t.Fatalf("DeleteBySource: %v", err)
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Fatalf("got %d streams, want 1", len(list))
	}
	if list[0].ID != "s3" {
		t.Errorf("remaining stream ID = %q, want %q", list[0].ID, "s3")
	}
}

func TestStreamStore_DeleteStaleBySource(t *testing.T) {
	s := NewMemoryStreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "a1", Name: "Keep"},
		{ID: "s2", SourceType: "m3u", SourceID: "a1", Name: "Delete"},
		{ID: "s3", SourceType: "m3u", SourceID: "a1", Name: "Also Delete"},
		{ID: "s4", SourceType: "m3u", SourceID: "a2", Name: "Other Source"},
	}
	s.BulkUpsert(ctx, streams)

	deleted, err := s.DeleteStaleBySource(ctx, "m3u", "a1", []string{"s1"})
	if err != nil {
		t.Fatalf("DeleteStaleBySource: %v", err)
	}

	sort.Strings(deleted)
	if len(deleted) != 2 {
		t.Fatalf("got %d deleted, want 2", len(deleted))
	}
	if deleted[0] != "s2" || deleted[1] != "s3" {
		t.Errorf("deleted = %v, want [s2 s3]", deleted)
	}

	list, _ := s.List(ctx)
	if len(list) != 2 {
		t.Fatalf("got %d streams, want 2", len(list))
	}
}

func TestStreamStore_List(t *testing.T) {
	s := NewMemoryStreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", Name: "One"},
		{ID: "s2", Name: "Two"},
		{ID: "s3", Name: "Three"},
	}
	s.BulkUpsert(ctx, streams)

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("got %d streams, want 3", len(list))
	}
}

func TestStreamStore_GetUnknownID(t *testing.T) {
	s := NewMemoryStreamStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get should not error for unknown ID, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unknown ID, got %+v", got)
	}
}

func TestStreamStore_SaveIsNoop(t *testing.T) {
	s := NewMemoryStreamStore()
	if err := s.Save(); err != nil {
		t.Errorf("Save should be no-op, got: %v", err)
	}
}
