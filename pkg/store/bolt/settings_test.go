package bolt

import (
	"context"
	"testing"

	bbolt "go.etcd.io/bbolt"
)

func TestSettingsStore_MigrateFromFlatKeys(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate_settings.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}

	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketSettings)
		if err != nil {
			return err
		}
		b.Put([]byte("theme"), []byte("dark"))
		b.Put([]byte("language"), []byte("en"))
		b.Put([]byte("default_hwaccel"), []byte("vaapi"))
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	s := db.SettingsStore()
	ctx := context.Background()

	val, err := s.Get(ctx, "theme")
	if err != nil {
		t.Fatalf("Get theme: %v", err)
	}
	if val != "dark" {
		t.Errorf("theme = %q, want dark", val)
	}

	val, err = s.Get(ctx, "default_hwaccel")
	if err != nil {
		t.Fatalf("Get default_hwaccel: %v", err)
	}
	if val != "vaapi" {
		t.Errorf("default_hwaccel = %q, want vaapi", val)
	}

	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d settings, want 3", len(all))
	}
	if all["theme"] != "dark" {
		t.Errorf("theme = %q, want dark", all["theme"])
	}
	if all["language"] != "en" {
		t.Errorf("language = %q, want en", all["language"])
	}
}

func TestSettingsStore_MigrateFromFlatKeysIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate_settings2.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}
	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketSettings)
		if err != nil {
			return err
		}
		return b.Put([]byte("theme"), []byte("dark"))
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	s := db.SettingsStore()
	ctx := context.Background()

	all, _ := s.List(ctx)
	if len(all) != 1 {
		t.Fatalf("got %d, want 1", len(all))
	}
	db.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()

	s2 := db2.SettingsStore()
	all2, _ := s2.List(ctx)
	if len(all2) != 1 {
		t.Fatalf("got %d after reopen, want 1", len(all2))
	}
}
