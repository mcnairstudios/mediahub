package store

import (
	"context"
	"sort"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/channel"
)

func TestChannelStore_CreateAndGet(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	ch := &channel.Channel{
		ID:        "ch-1",
		Name:      "BBC One",
		Number:    1,
		GroupID:   "g-1",
		StreamIDs: []string{"s-1", "s-2"},
		IsEnabled: true,
	}

	if err := s.Create(ctx, ch); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "ch-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Name != "BBC One" {
		t.Errorf("Name = %q, want %q", got.Name, "BBC One")
	}
	if got.Number != 1 {
		t.Errorf("Number = %d, want 1", got.Number)
	}
	if len(got.StreamIDs) != 2 {
		t.Errorf("StreamIDs len = %d, want 2", len(got.StreamIDs))
	}
}

func TestChannelStore_GetUnknownID(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get should not error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestChannelStore_List(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", Name: "One"})
	s.Create(ctx, &channel.Channel{ID: "ch-2", Name: "Two"})
	s.Create(ctx, &channel.Channel{ID: "ch-3", Name: "Three"})

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("got %d channels, want 3", len(list))
	}
}

func TestChannelStore_Update(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", Name: "Old Name"})
	s.Update(ctx, &channel.Channel{ID: "ch-1", Name: "New Name"})

	got, _ := s.Get(ctx, "ch-1")
	if got.Name != "New Name" {
		t.Errorf("Name = %q, want %q", got.Name, "New Name")
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Errorf("got %d channels, want 1", len(list))
	}
}

func TestChannelStore_Delete(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", Name: "One"})
	s.Create(ctx, &channel.Channel{ID: "ch-2", Name: "Two"})

	if err := s.Delete(ctx, "ch-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Fatalf("got %d channels, want 1", len(list))
	}
	if list[0].ID != "ch-2" {
		t.Errorf("remaining ID = %q, want %q", list[0].ID, "ch-2")
	}
}

func TestChannelStore_AssignStreams(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", StreamIDs: []string{"s-old"}})

	if err := s.AssignStreams(ctx, "ch-1", []string{"s-1", "s-2", "s-3"}); err != nil {
		t.Fatalf("AssignStreams: %v", err)
	}

	got, _ := s.Get(ctx, "ch-1")
	if len(got.StreamIDs) != 3 {
		t.Fatalf("StreamIDs len = %d, want 3", len(got.StreamIDs))
	}

	sort.Strings(got.StreamIDs)
	if got.StreamIDs[0] != "s-1" || got.StreamIDs[1] != "s-2" || got.StreamIDs[2] != "s-3" {
		t.Errorf("StreamIDs = %v, want [s-1 s-2 s-3]", got.StreamIDs)
	}
}

func TestChannelStore_AssignStreamsUnknownChannel(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	err := s.AssignStreams(ctx, "nonexistent", []string{"s-1"})
	if err != nil {
		t.Fatalf("AssignStreams on unknown channel should not error: %v", err)
	}
}

func TestChannelStore_RemoveStreamMappings(t *testing.T) {
	s := NewMemoryChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", StreamIDs: []string{"s-1", "s-2", "s-3"}})
	s.Create(ctx, &channel.Channel{ID: "ch-2", StreamIDs: []string{"s-2", "s-4"}})

	if err := s.RemoveStreamMappings(ctx, []string{"s-2", "s-3"}); err != nil {
		t.Fatalf("RemoveStreamMappings: %v", err)
	}

	ch1, _ := s.Get(ctx, "ch-1")
	if len(ch1.StreamIDs) != 1 || ch1.StreamIDs[0] != "s-1" {
		t.Errorf("ch-1 StreamIDs = %v, want [s-1]", ch1.StreamIDs)
	}

	ch2, _ := s.Get(ctx, "ch-2")
	if len(ch2.StreamIDs) != 1 || ch2.StreamIDs[0] != "s-4" {
		t.Errorf("ch-2 StreamIDs = %v, want [s-4]", ch2.StreamIDs)
	}
}

func TestGroupStore_CreateAndList(t *testing.T) {
	s := NewMemoryGroupStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Group{ID: "g-1", Name: "News"})
	s.Create(ctx, &channel.Group{ID: "g-2", Name: "Sports"})

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("got %d groups, want 2", len(list))
	}
}

func TestGroupStore_Delete(t *testing.T) {
	s := NewMemoryGroupStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Group{ID: "g-1", Name: "News"})
	s.Create(ctx, &channel.Group{ID: "g-2", Name: "Sports"})

	if err := s.Delete(ctx, "g-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Fatalf("got %d groups, want 1", len(list))
	}
	if list[0].ID != "g-2" {
		t.Errorf("remaining ID = %q, want %q", list[0].ID, "g-2")
	}
}
