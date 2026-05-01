package bolt

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/auth"
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
