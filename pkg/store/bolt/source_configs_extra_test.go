package bolt

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
)

func TestSourceConfigStore_FullCRUDCycle(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SourceConfigStore()
	ctx := context.Background()

	sc := &sourceconfig.SourceConfig{
		ID:   "src-1",
		Type: "m3u",
		Name: "UK IPTV",
		Config: map[string]string{
			"url": "http://example.com/playlist.m3u",
		},
	}

	if err := store.Create(ctx, sc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(ctx, "src-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected config, got nil")
	}
	if got.Config["url"] != "http://example.com/playlist.m3u" {
		t.Errorf("Config[url] = %q", got.Config["url"])
	}

	got.Name = "Updated UK IPTV"
	if err := store.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, _ := store.Get(ctx, "src-1")
	if updated.Name != "Updated UK IPTV" {
		t.Errorf("after update Name = %q, want Updated UK IPTV", updated.Name)
	}

	if err := store.Delete(ctx, "src-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	deleted, _ := store.Get(ctx, "src-1")
	if deleted != nil {
		t.Error("expected nil after delete")
	}
}

func TestSourceConfigStore_ListByTypeMultiple(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SourceConfigStore()
	ctx := context.Background()

	store.Create(ctx, &sourceconfig.SourceConfig{ID: "s1", Type: "m3u", Name: "M3U 1", Config: map[string]string{}})
	store.Create(ctx, &sourceconfig.SourceConfig{ID: "s2", Type: "m3u", Name: "M3U 2", Config: map[string]string{}})
	store.Create(ctx, &sourceconfig.SourceConfig{ID: "s3", Type: "hdhr", Name: "HDHR", Config: map[string]string{}})
	store.Create(ctx, &sourceconfig.SourceConfig{ID: "s4", Type: "spacex", Name: "SpaceX", Config: map[string]string{}})

	m3u, _ := store.ListByType(ctx, "m3u")
	if len(m3u) != 2 {
		t.Fatalf("expected 2 m3u configs, got %d", len(m3u))
	}

	hdhr, _ := store.ListByType(ctx, "hdhr")
	if len(hdhr) != 1 {
		t.Fatalf("expected 1 hdhr config, got %d", len(hdhr))
	}

	none, _ := store.ListByType(ctx, "satip")
	if len(none) != 0 {
		t.Fatalf("expected 0 satip configs, got %d", len(none))
	}
}

func TestSourceConfigStore_EmptyList(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SourceConfigStore()
	list, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0, got %d", len(list))
	}
}
