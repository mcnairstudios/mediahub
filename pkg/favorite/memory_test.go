package favorite

import (
	"context"
	"testing"
)

func TestMemoryStore_AddAndList(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Add(ctx, "user1", "stream-a"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Add(ctx, "user1", "stream-b"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	favs, err := s.List(ctx, "user1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(favs) != 2 {
		t.Fatalf("expected 2 favorites, got %d", len(favs))
	}
}

func TestMemoryStore_AddIdempotent(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Add(ctx, "user1", "stream-a")
	s.Add(ctx, "user1", "stream-a")

	favs, _ := s.List(ctx, "user1")
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite after duplicate add, got %d", len(favs))
	}
}

func TestMemoryStore_Remove(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Add(ctx, "user1", "stream-a")
	s.Add(ctx, "user1", "stream-b")

	if err := s.Remove(ctx, "user1", "stream-a"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	favs, _ := s.List(ctx, "user1")
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite after remove, got %d", len(favs))
	}
}

func TestMemoryStore_RemoveNonExistent(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Remove(ctx, "user1", "nonexistent"); err != nil {
		t.Fatalf("Remove non-existent: %v", err)
	}
}

func TestMemoryStore_IsFavorite(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Add(ctx, "user1", "stream-a")

	ok, err := s.IsFavorite(ctx, "user1", "stream-a")
	if err != nil {
		t.Fatalf("IsFavorite: %v", err)
	}
	if !ok {
		t.Fatal("expected stream-a to be favorite")
	}

	ok, err = s.IsFavorite(ctx, "user1", "stream-b")
	if err != nil {
		t.Fatalf("IsFavorite: %v", err)
	}
	if ok {
		t.Fatal("expected stream-b to NOT be favorite")
	}
}

func TestMemoryStore_UserIsolation(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Add(ctx, "user1", "stream-a")
	s.Add(ctx, "user2", "stream-b")

	favs1, _ := s.List(ctx, "user1")
	if len(favs1) != 1 {
		t.Fatalf("user1: expected 1 favorite, got %d", len(favs1))
	}
	if favs1[0].StreamID != "stream-a" {
		t.Fatalf("user1: expected stream-a, got %s", favs1[0].StreamID)
	}

	favs2, _ := s.List(ctx, "user2")
	if len(favs2) != 1 {
		t.Fatalf("user2: expected 1 favorite, got %d", len(favs2))
	}
	if favs2[0].StreamID != "stream-b" {
		t.Fatalf("user2: expected stream-b, got %s", favs2[0].StreamID)
	}
}

func TestMemoryStore_ListEmpty(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	favs, err := s.List(ctx, "nobody")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if favs != nil {
		t.Fatalf("expected nil for unknown user, got %v", favs)
	}
}
