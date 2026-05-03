package store

import (
	"context"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/channel"
)

func TestChannelStore_DeleteNonExistent(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	err := s.Delete(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Delete on nonexistent should not error: %v", err)
	}
}

func TestChannelStore_ListEmpty(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 channels, got %d", len(list))
	}
}

func TestChannelStore_UpdatePreservesCount(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", Name: "One", Number: 1})
	s.Create(ctx, &channel.Channel{ID: "ch-2", Name: "Two", Number: 2})

	s.Update(ctx, &channel.Channel{ID: "ch-1", Name: "Updated", Number: 1})

	list, _ := s.List(ctx)
	if len(list) != 2 {
		t.Fatalf("expected 2 channels after update, got %d", len(list))
	}
}

func TestChannelStore_RemoveStreamMappingsEmpty(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", StreamIDs: []string{"s-1", "s-2"}})

	err := s.RemoveStreamMappings(ctx, []string{})
	if err != nil {
		t.Fatalf("RemoveStreamMappings empty: %v", err)
	}

	got, _ := s.Get(ctx, "ch-1")
	if len(got.StreamIDs) != 2 {
		t.Errorf("expected 2 streams unchanged, got %d", len(got.StreamIDs))
	}
}

func TestChannelStore_RemoveStreamMappingsAllFromChannel(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", StreamIDs: []string{"s-1", "s-2"}})

	s.RemoveStreamMappings(ctx, []string{"s-1", "s-2"})

	got, _ := s.Get(ctx, "ch-1")
	if len(got.StreamIDs) != 0 {
		t.Errorf("expected 0 streams after removing all, got %d", len(got.StreamIDs))
	}
}

func TestGroupStore_GetNonExistent(t *testing.T) {
	s := NewMemoryGroupStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get should not error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestGroupStore_Update(t *testing.T) {
	s := NewMemoryGroupStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Group{ID: "g-1", Name: "Old", SortOrder: 1})
	s.Update(ctx, &channel.Group{ID: "g-1", Name: "New", SortOrder: 2})

	got, _ := s.Get(ctx, "g-1")
	if got.Name != "New" {
		t.Errorf("Name = %q, want New", got.Name)
	}
	if got.SortOrder != 2 {
		t.Errorf("SortOrder = %d, want 2", got.SortOrder)
	}
}

func TestGroupStore_ListEmpty(t *testing.T) {
	s := NewMemoryGroupStore()
	ctx := context.Background()

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(list))
	}
}

func TestGroupStore_DeleteNonExistent(t *testing.T) {
	s := NewMemoryGroupStore()
	ctx := context.Background()

	err := s.Delete(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Delete on nonexistent should not error: %v", err)
	}
}
