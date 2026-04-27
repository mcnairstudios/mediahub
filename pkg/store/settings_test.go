package store

import (
	"context"
	"testing"
)

func TestSettingsStore_SetAndGet(t *testing.T) {
	s := NewMemorySettingsStore()
	ctx := context.Background()

	err := s.Set(ctx, "theme", "dark")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, err := s.Get(ctx, "theme")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "dark" {
		t.Errorf("got %q, want %q", val, "dark")
	}
}

func TestSettingsStore_GetUnknownKey(t *testing.T) {
	s := NewMemorySettingsStore()
	ctx := context.Background()

	val, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get should not error for unknown key, got: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string for unknown key, got %q", val)
	}
}

func TestSettingsStore_List(t *testing.T) {
	s := NewMemorySettingsStore()
	ctx := context.Background()

	s.Set(ctx, "theme", "dark")
	s.Set(ctx, "language", "en")

	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d settings, want 2", len(all))
	}
	if all["theme"] != "dark" {
		t.Errorf("theme = %q, want %q", all["theme"], "dark")
	}
	if all["language"] != "en" {
		t.Errorf("language = %q, want %q", all["language"], "en")
	}
}

func TestSettingsStore_OverwriteExistingKey(t *testing.T) {
	s := NewMemorySettingsStore()
	ctx := context.Background()

	s.Set(ctx, "theme", "dark")
	s.Set(ctx, "theme", "light")

	val, _ := s.Get(ctx, "theme")
	if val != "light" {
		t.Errorf("got %q, want %q", val, "light")
	}

	all, _ := s.List(ctx)
	if len(all) != 1 {
		t.Errorf("got %d settings, want 1 (no duplicates)", len(all))
	}
}
