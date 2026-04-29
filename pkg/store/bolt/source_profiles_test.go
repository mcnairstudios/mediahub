package bolt

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/sourceprofile"
)

func TestSourceProfileStore_CRUD(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SourceProfileStore()
	ctx := context.Background()

	p := &sourceprofile.Profile{
		ID:                "sp-1",
		Name:              "SAT>IP DVB-T",
		Deinterlace:       true,
		DeinterlaceMethod: "auto",
		RTSPProtocols:     "tcp",
		HTTPTimeoutSec:    10,
	}

	if err := store.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.Get(ctx, "sp-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected profile, got nil")
	}
	if got.Name != "SAT>IP DVB-T" {
		t.Errorf("name = %s, want SAT>IP DVB-T", got.Name)
	}
	if !got.Deinterlace {
		t.Error("deinterlace should be true")
	}
	if got.DeinterlaceMethod != "auto" {
		t.Errorf("deinterlace_method = %s, want auto", got.DeinterlaceMethod)
	}
	if got.RTSPProtocols != "tcp" {
		t.Errorf("rtsp_protocols = %s, want tcp", got.RTSPProtocols)
	}
	if got.HTTPTimeoutSec != 10 {
		t.Errorf("http_timeout_sec = %d, want 10", got.HTTPTimeoutSec)
	}

	got.Name = "Updated"
	got.HTTPTimeoutSec = 30
	if err := store.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	updated, _ := store.Get(ctx, "sp-1")
	if updated.Name != "Updated" {
		t.Errorf("updated name = %s, want Updated", updated.Name)
	}
	if updated.HTTPTimeoutSec != 30 {
		t.Errorf("updated http_timeout_sec = %d, want 30", updated.HTTPTimeoutSec)
	}

	profiles, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("list len = %d, want 1", len(profiles))
	}

	if err := store.Delete(ctx, "sp-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	deleted, _ := store.Get(ctx, "sp-1")
	if deleted != nil {
		t.Error("expected nil after delete")
	}
}

func TestSourceProfileStore_GetNonExistent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SourceProfileStore()
	got, err := store.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent profile")
	}
}

func TestSourceProfileStore_UpdateNotFound(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SourceProfileStore()
	err = store.Update(context.Background(), &sourceprofile.Profile{ID: "nonexistent", Name: "test"})
	if err != ErrSourceProfileNotFound {
		t.Errorf("expected ErrSourceProfileNotFound, got %v", err)
	}
}

func TestSourceProfileStore_ListEmpty(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.SourceProfileStore()
	profiles, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("list len = %d, want 0", len(profiles))
	}
}

func TestSourceProfileStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ctx := context.Background()
	db.SourceProfileStore().Create(ctx, &sourceprofile.Profile{
		ID:             "sp-1",
		Name:           "Persisted",
		HTTPTimeoutSec: 30,
	})
	db.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()

	got, err := db2.SourceProfileStore().Get(ctx, "sp-1")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got == nil {
		t.Fatal("source profile should persist across close/reopen")
	}
	if got.Name != "Persisted" {
		t.Errorf("Name = %q, want %q", got.Name, "Persisted")
	}
	if got.HTTPTimeoutSec != 30 {
		t.Errorf("HTTPTimeoutSec = %d, want 30", got.HTTPTimeoutSec)
	}
}
