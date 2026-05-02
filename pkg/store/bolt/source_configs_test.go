package bolt

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	bbolt "go.etcd.io/bbolt"
)

func TestSourceConfigStore_MigrateFromFlatKeys(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate_sc.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}

	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketSourceConfigs)
		if err != nil {
			return err
		}

		configs := []sourceconfig.SourceConfig{
			{ID: "src-1", Type: "m3u", Name: "UK IPTV", Config: map[string]string{"url": "http://example.com"}},
			{ID: "src-2", Type: "hdhr", Name: "HDHR", Config: map[string]string{}},
			{ID: "src-3", Type: "m3u", Name: "US IPTV", Config: map[string]string{}},
		}
		for _, sc := range configs {
			data, _ := json.Marshal(sc)
			b.Put([]byte(sc.ID), data)
		}
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

	s := db.SourceConfigStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "src-1")
	if err != nil {
		t.Fatalf("Get src-1: %v", err)
	}
	if got == nil || got.Name != "UK IPTV" {
		t.Fatalf("expected UK IPTV, got %+v", got)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("got %d configs, want 3", len(list))
	}

	m3u, err := s.ListByType(ctx, "m3u")
	if err != nil {
		t.Fatalf("ListByType: %v", err)
	}
	if len(m3u) != 2 {
		t.Fatalf("got %d m3u configs, want 2", len(m3u))
	}
	names := []string{m3u[0].Name, m3u[1].Name}
	sort.Strings(names)
	if names[0] != "UK IPTV" || names[1] != "US IPTV" {
		t.Errorf("names = %v, want [UK IPTV US IPTV]", names)
	}
}

func TestSourceConfigStore_MigrateFromFlatKeysIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate_sc2.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}
	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketSourceConfigs)
		if err != nil {
			return err
		}
		sc := sourceconfig.SourceConfig{ID: "src-1", Type: "m3u", Name: "Test", Config: map[string]string{}}
		data, _ := json.Marshal(sc)
		return b.Put([]byte("src-1"), data)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := db.SourceConfigStore()
	ctx := context.Background()

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Fatalf("got %d, want 1", len(list))
	}
	db.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()

	s2 := db2.SourceConfigStore()
	list2, _ := s2.List(ctx)
	if len(list2) != 1 {
		t.Fatalf("got %d after reopen, want 1", len(list2))
	}
}
