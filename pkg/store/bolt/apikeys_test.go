package bolt

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	bbolt "go.etcd.io/bbolt"
)

func newTestAPIKeyStore(t *testing.T) *APIKeyStore {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(dir)
	})
	return db.APIKeyStore()
}

func TestAPIKeyStore_CreateAndGetByKey(t *testing.T) {
	store := newTestAPIKeyStore(t)
	ctx := context.Background()

	key := &auth.APIKey{
		ID:        "key-id-1",
		Key:       "secret-key-value",
		UserID:    "user-1",
		Name:      "test key",
		CreatedAt: time.Now(),
	}

	if err := store.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.GetByKey(ctx, "secret-key-value")
	if err != nil {
		t.Fatalf("GetByKey: %v", err)
	}
	if got.ID != "key-id-1" {
		t.Fatalf("expected ID key-id-1, got %s", got.ID)
	}
	if got.UserID != "user-1" {
		t.Fatalf("expected userID user-1, got %s", got.UserID)
	}
}

func TestAPIKeyStore_GetByKeyNotFound(t *testing.T) {
	store := newTestAPIKeyStore(t)
	ctx := context.Background()

	_, err := store.GetByKey(ctx, "nonexistent")
	if err != auth.ErrAPIKeyNotFound {
		t.Fatalf("expected ErrAPIKeyNotFound, got %v", err)
	}
}

func TestAPIKeyStore_ListByUser(t *testing.T) {
	store := newTestAPIKeyStore(t)
	ctx := context.Background()

	store.Create(ctx, &auth.APIKey{ID: "k1", Key: "key1", UserID: "alice", Name: "key1", CreatedAt: time.Now()})
	store.Create(ctx, &auth.APIKey{ID: "k2", Key: "key2", UserID: "alice", Name: "key2", CreatedAt: time.Now()})
	store.Create(ctx, &auth.APIKey{ID: "k3", Key: "key3", UserID: "bob", Name: "key3", CreatedAt: time.Now()})

	aliceKeys, err := store.ListByUser(ctx, "alice")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(aliceKeys) != 2 {
		t.Fatalf("expected 2 keys for alice, got %d", len(aliceKeys))
	}

	bobKeys, _ := store.ListByUser(ctx, "bob")
	if len(bobKeys) != 1 {
		t.Fatalf("expected 1 key for bob, got %d", len(bobKeys))
	}

	noKeys, _ := store.ListByUser(ctx, "charlie")
	if len(noKeys) != 0 {
		t.Fatalf("expected 0 keys for charlie, got %d", len(noKeys))
	}
}

func TestAPIKeyStore_Delete(t *testing.T) {
	store := newTestAPIKeyStore(t)
	ctx := context.Background()

	store.Create(ctx, &auth.APIKey{ID: "k1", Key: "key1", UserID: "alice", Name: "key1", CreatedAt: time.Now()})

	if err := store.Delete(ctx, "k1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.GetByKey(ctx, "key1")
	if err != auth.ErrAPIKeyNotFound {
		t.Fatalf("expected ErrAPIKeyNotFound after delete, got %v", err)
	}
}

func TestAPIKeyStore_DeleteNotFound(t *testing.T) {
	store := newTestAPIKeyStore(t)
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if err != auth.ErrAPIKeyNotFound {
		t.Fatalf("expected ErrAPIKeyNotFound, got %v", err)
	}
}

func TestAPIKeyStore_MigrateFromFlatKeys(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate_apikeys.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}

	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketAPIKeys)
		if err != nil {
			return err
		}

		keys := []auth.APIKey{
			{ID: "k1", Key: "secret1", UserID: "alice", Name: "key1", CreatedAt: time.Now()},
			{ID: "k2", Key: "secret2", UserID: "alice", Name: "key2", CreatedAt: time.Now()},
			{ID: "k3", Key: "secret3", UserID: "bob", Name: "key3", CreatedAt: time.Now()},
		}
		for _, ak := range keys {
			data, _ := json.Marshal(ak)
			b.Put([]byte(ak.ID), data)
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

	s := db.APIKeyStore()
	ctx := context.Background()

	got, err := s.GetByKey(ctx, "secret1")
	if err != nil {
		t.Fatalf("GetByKey: %v", err)
	}
	if got.ID != "k1" {
		t.Errorf("ID = %q, want k1", got.ID)
	}

	aliceKeys, err := s.ListByUser(ctx, "alice")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(aliceKeys) != 2 {
		t.Fatalf("got %d keys for alice, want 2", len(aliceKeys))
	}

	bobKeys, err := s.ListByUser(ctx, "bob")
	if err != nil {
		t.Fatalf("ListByUser bob: %v", err)
	}
	if len(bobKeys) != 1 {
		t.Fatalf("got %d keys for bob, want 1", len(bobKeys))
	}
}
