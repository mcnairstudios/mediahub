package bolt

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

func tempDB(t *testing.T) (*DB, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db, path
}

func TestOpenClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file should exist after Open")
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestStreamStore_CreateAndGet(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	if err := s.Save(); err != nil {
		t.Errorf("Save should be no-op, got: %v", err)
	}
}

func TestSettingsStore_SetAndGet(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SettingsStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SettingsStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SettingsStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SettingsStore()
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

func TestDataSurvivesCloseAndReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ctx := context.Background()

	db.StreamStore().BulkUpsert(ctx, []media.Stream{
		{ID: "s1", Name: "Persisted Stream", SourceType: "m3u", SourceID: "a1"},
	})
	db.SettingsStore().Set(ctx, "persist_key", "persist_value")

	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()

	got, err := db2.StreamStore().Get(ctx, "s1")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got == nil {
		t.Fatal("stream should persist across close/reopen")
	}
	if got.Name != "Persisted Stream" {
		t.Errorf("got name %q, want %q", got.Name, "Persisted Stream")
	}

	val, err := db2.SettingsStore().Get(ctx, "persist_key")
	if err != nil {
		t.Fatalf("Get setting after reopen: %v", err)
	}
	if val != "persist_value" {
		t.Errorf("got %q, want %q", val, "persist_value")
	}
}
