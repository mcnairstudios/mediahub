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

func newTestInviteStore(t *testing.T) *InviteStore {
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
	return db.InviteStore()
}

func TestInviteStore_CreateAndGet(t *testing.T) {
	store := newTestInviteStore(t)
	ctx := context.Background()

	invite := &auth.Invite{
		Token:     "test-token-123",
		Role:      auth.RoleStandard,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := store.Create(ctx, invite); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(ctx, "test-token-123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Token != "test-token-123" {
		t.Fatalf("expected token test-token-123, got %s", got.Token)
	}
	if got.Role != auth.RoleStandard {
		t.Fatalf("expected role standard, got %s", got.Role)
	}
}

func TestInviteStore_GetNotFound(t *testing.T) {
	store := newTestInviteStore(t)
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if err != auth.ErrInviteNotFound {
		t.Fatalf("expected ErrInviteNotFound, got %v", err)
	}
}

func TestInviteStore_List(t *testing.T) {
	store := newTestInviteStore(t)
	ctx := context.Background()

	store.Create(ctx, &auth.Invite{Token: "t1", Role: auth.RoleStandard, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
	store.Create(ctx, &auth.Invite{Token: "t2", Role: auth.RoleAdmin, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})

	invites, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(invites) != 2 {
		t.Fatalf("expected 2 invites, got %d", len(invites))
	}
}

func TestInviteStore_Update(t *testing.T) {
	store := newTestInviteStore(t)
	ctx := context.Background()

	invite := &auth.Invite{Token: "t1", Role: auth.RoleStandard, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	store.Create(ctx, invite)

	invite.Used = true
	if err := store.Update(ctx, invite); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := store.Get(ctx, "t1")
	if !got.Used {
		t.Fatal("expected invite to be marked as used")
	}
}

func TestInviteStore_Delete(t *testing.T) {
	store := newTestInviteStore(t)
	ctx := context.Background()

	store.Create(ctx, &auth.Invite{Token: "t1", Role: auth.RoleStandard, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})

	if err := store.Delete(ctx, "t1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Get(ctx, "t1")
	if err != auth.ErrInviteNotFound {
		t.Fatalf("expected ErrInviteNotFound after delete, got %v", err)
	}
}

func TestInviteStore_DeleteNotFound(t *testing.T) {
	store := newTestInviteStore(t)
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if err != auth.ErrInviteNotFound {
		t.Fatalf("expected ErrInviteNotFound, got %v", err)
	}
}

func TestInviteStore_MigrateFromFlatKeys(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate_invites.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}

	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketInvites)
		if err != nil {
			return err
		}

		invites := []auth.Invite{
			{Token: "tok-1", Role: auth.RoleStandard, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
			{Token: "tok-2", Role: auth.RoleAdmin, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
		}
		for _, inv := range invites {
			data, _ := json.Marshal(inv)
			b.Put([]byte(inv.Token), data)
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

	s := db.InviteStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "tok-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Role != auth.RoleStandard {
		t.Errorf("Role = %q, want standard", got.Role)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d invites, want 2", len(list))
	}
}
