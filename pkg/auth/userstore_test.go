package auth

import (
	"context"
	"testing"
)

func TestMemoryUserStore_CreateAndGet(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	user := &User{ID: "1", Username: "alice", Role: RoleAdmin, IsAdmin: true}
	if err := store.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(ctx, "1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Username != "alice" || got.Role != RoleAdmin || !got.IsAdmin {
		t.Fatalf("unexpected user: %+v", got)
	}
}

func TestMemoryUserStore_GetByUsername(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	user := &User{ID: "1", Username: "bob", Role: RoleStandard}
	if err := store.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.GetByUsername(ctx, "bob")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if got.ID != "1" {
		t.Fatalf("expected ID=1, got %s", got.ID)
	}

	_, err = store.GetByUsername(ctx, "nonexistent")
	if err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestMemoryUserStore_List(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	store.Create(ctx, &User{ID: "1", Username: "alice"})
	store.Create(ctx, &User{ID: "2", Username: "bob"})

	users, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestMemoryUserStore_Delete(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	store.Create(ctx, &User{ID: "1", Username: "alice"})

	if err := store.Delete(ctx, "1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Get(ctx, "1")
	if err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound after delete, got %v", err)
	}

	if err := store.Delete(ctx, "nonexistent"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound for missing user, got %v", err)
	}
}

func TestMemoryUserStore_UpdatePassword(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	store.Create(ctx, &User{ID: "1", Username: "alice"})

	if err := store.UpdatePassword(ctx, "1", "hashed123"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}

	hash, err := store.GetPasswordHash(ctx, "1")
	if err != nil {
		t.Fatalf("GetPasswordHash: %v", err)
	}
	if hash != "hashed123" {
		t.Fatalf("expected hashed123, got %s", hash)
	}

	if err := store.UpdatePassword(ctx, "nonexistent", "x"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestMemoryUserStore_DuplicateUsername(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	store.Create(ctx, &User{ID: "1", Username: "alice"})

	err := store.Create(ctx, &User{ID: "2", Username: "alice"})
	if err != ErrUsernameExists {
		t.Fatalf("expected ErrUsernameExists, got %v", err)
	}
}

func TestMemoryUserStore_GetReturnsACopy(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	store.Create(ctx, &User{ID: "1", Username: "alice"})

	got, _ := store.Get(ctx, "1")
	got.Username = "modified"

	got2, _ := store.Get(ctx, "1")
	if got2.Username != "alice" {
		t.Fatal("Get returned a reference, not a copy")
	}
}
