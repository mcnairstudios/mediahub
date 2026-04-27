package sourceconfig

import (
	"context"
	"sort"
	"testing"
)

func TestMemoryStore_CreateAndGet(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	sc := &SourceConfig{
		ID:        "src-1",
		Type:      "m3u",
		Name:      "Test",
		IsEnabled: true,
		Config:    map[string]string{"url": "http://example.com"},
	}
	if err := s.Create(ctx, sc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "src-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.Name != "Test" {
		t.Fatalf("Get = %v, want Name=Test", got)
	}
}

func TestMemoryStore_GetUnknown(t *testing.T) {
	s := NewMemoryStore()
	got, err := s.Get(context.Background(), "nope")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestMemoryStore_List(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Create(ctx, &SourceConfig{ID: "1", Type: "m3u", Config: map[string]string{}})
	s.Create(ctx, &SourceConfig{ID: "2", Type: "hdhr", Config: map[string]string{}})

	list, _ := s.List(ctx)
	if len(list) != 2 {
		t.Fatalf("got %d, want 2", len(list))
	}
}

func TestMemoryStore_ListByType(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Create(ctx, &SourceConfig{ID: "1", Type: "m3u", Name: "A", Config: map[string]string{}})
	s.Create(ctx, &SourceConfig{ID: "2", Type: "hdhr", Name: "B", Config: map[string]string{}})
	s.Create(ctx, &SourceConfig{ID: "3", Type: "m3u", Name: "C", Config: map[string]string{}})

	list, _ := s.ListByType(ctx, "m3u")
	if len(list) != 2 {
		t.Fatalf("got %d m3u sources, want 2", len(list))
	}
	names := []string{list[0].Name, list[1].Name}
	sort.Strings(names)
	if names[0] != "A" || names[1] != "C" {
		t.Errorf("names = %v, want [A C]", names)
	}
}

func TestMemoryStore_Update(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Create(ctx, &SourceConfig{ID: "1", Name: "Old", Config: map[string]string{}})
	err := s.Update(ctx, &SourceConfig{ID: "1", Name: "New", Config: map[string]string{}})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := s.Get(ctx, "1")
	if got.Name != "New" {
		t.Errorf("Name = %q, want New", got.Name)
	}
}

func TestMemoryStore_UpdateNotFound(t *testing.T) {
	s := NewMemoryStore()
	err := s.Update(context.Background(), &SourceConfig{ID: "nope", Config: map[string]string{}})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Create(ctx, &SourceConfig{ID: "1", Config: map[string]string{}})
	s.Delete(ctx, "1")

	got, _ := s.Get(ctx, "1")
	if got != nil {
		t.Errorf("expected nil after delete")
	}
}
