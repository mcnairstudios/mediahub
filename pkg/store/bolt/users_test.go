package bolt

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	bbolt "go.etcd.io/bbolt"
)

func TestUserStore_MigrateFromFlatKeys(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate_users.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}

	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketUsers)
		if err != nil {
			return err
		}

		users := []boltStoredUser{
			{User: auth.User{ID: "u-1", Username: "alice", Role: auth.RoleAdmin}},
			{User: auth.User{ID: "u-2", Username: "bob", Role: auth.RoleStandard}, PasswordHash: "$2a$10$hash"},
		}
		for _, su := range users {
			data, _ := json.Marshal(su)
			b.Put([]byte(su.User.ID), data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed flat keys: %v", err)
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "u-1")
	if err != nil {
		t.Fatalf("Get u-1: %v", err)
	}
	if got == nil || got.Username != "alice" {
		t.Fatalf("expected alice, got %+v", got)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d users, want 2", len(list))
	}

	hash, err := s.GetPasswordHash(ctx, "u-2")
	if err != nil {
		t.Fatalf("GetPasswordHash: %v", err)
	}
	if hash != "$2a$10$hash" {
		t.Errorf("hash = %q, want %q", hash, "$2a$10$hash")
	}

	got2, err := s.GetByUsername(ctx, "bob")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if got2.ID != "u-2" {
		t.Errorf("ID = %q, want u-2", got2.ID)
	}
}

func TestUserStore_MigrateFromFlatKeysIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate_users2.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}

	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketUsers)
		if err != nil {
			return err
		}
		su := boltStoredUser{User: auth.User{ID: "u-1", Username: "alice"}}
		data, _ := json.Marshal(su)
		return b.Put([]byte("u-1"), data)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	s := db.UserStore()
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

	s2 := db2.UserStore()
	list2, _ := s2.List(ctx)
	if len(list2) != 1 {
		t.Fatalf("got %d after reopen, want 1", len(list2))
	}
}
