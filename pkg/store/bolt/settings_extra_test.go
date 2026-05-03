package bolt

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSettingsStore_SetAndGetCycle(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SettingsStore()
	ctx := context.Background()

	if err := store.Set(ctx, "theme", "dark"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, err := store.Get(ctx, "theme")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "dark" {
		t.Errorf("val = %q, want dark", val)
	}
}

func TestSettingsStore_GetMissingKey(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SettingsStore()
	val, err := store.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string for nonexistent key, got %q", val)
	}
}

func TestSettingsStore_OverwriteValue(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SettingsStore()
	ctx := context.Background()

	store.Set(ctx, "hwaccel", "none")
	store.Set(ctx, "hwaccel", "vaapi")

	val, _ := store.Get(ctx, "hwaccel")
	if val != "vaapi" {
		t.Errorf("val = %q, want vaapi", val)
	}

	all, _ := store.List(ctx)
	if len(all) != 1 {
		t.Errorf("expected 1 setting after overwrite, got %d", len(all))
	}
}

func TestSettingsStore_ListMultipleKeys(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SettingsStore()
	ctx := context.Background()

	store.Set(ctx, "theme", "dark")
	store.Set(ctx, "language", "en")
	store.Set(ctx, "hwaccel", "vaapi")
	store.Set(ctx, "container", "mp4")

	all, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 settings, got %d", len(all))
	}
	if all["theme"] != "dark" {
		t.Errorf("theme = %q, want dark", all["theme"])
	}
	if all["container"] != "mp4" {
		t.Errorf("container = %q, want mp4", all["container"])
	}
}

func TestSettingsStore_EmptyList(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SettingsStore()
	all, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected 0 settings, got %d", len(all))
	}
}

func TestSettingsStore_EmptyStringValue(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SettingsStore()
	ctx := context.Background()

	store.Set(ctx, "key", "")

	val, _ := store.Get(ctx, "key")
	if val != "" {
		t.Errorf("expected empty value, got %q", val)
	}
}
