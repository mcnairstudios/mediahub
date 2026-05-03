package bolt

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/auth"
)

func TestUserStore_FullCRUDCycle(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.UserStore()
	ctx := context.Background()

	u := &auth.User{
		ID:       "u-1",
		Username: "alice",
		Role:     auth.RoleAdmin,
		Email:    "alice@example.com",
	}

	if err := store.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(ctx, "u-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", got.Email)
	}

	got.Email = "alice2@example.com"
	if err := store.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, _ := store.Get(ctx, "u-1")
	if updated.Email != "alice2@example.com" {
		t.Errorf("after update Email = %q", updated.Email)
	}

	if err := store.Delete(ctx, "u-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Get(ctx, "u-1")
	if err != auth.ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound after delete, got %v", err)
	}
}

func TestUserStore_UpdateDuplicateUsername(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.UserStore()
	ctx := context.Background()

	store.Create(ctx, &auth.User{ID: "u-1", Username: "alice", Role: auth.RoleAdmin})
	store.Create(ctx, &auth.User{ID: "u-2", Username: "bob", Role: auth.RoleStandard})

	err = store.Update(ctx, &auth.User{ID: "u-2", Username: "alice", Role: auth.RoleStandard})
	if err != auth.ErrUsernameExists {
		t.Fatalf("expected ErrUsernameExists on update, got %v", err)
	}
}

func TestUserStore_GetByEmail(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.UserStore()
	ctx := context.Background()

	store.Create(ctx, &auth.User{ID: "u-1", Username: "alice", Email: "Alice@Example.Com", Role: auth.RoleAdmin})

	got, err := store.GetByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.ID != "u-1" {
		t.Errorf("ID = %q, want u-1", got.ID)
	}

	_, err = store.GetByEmail(ctx, "nobody@example.com")
	if err != auth.ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_UpdateNonExistent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.UserStore()
	err = store.Update(context.Background(), &auth.User{ID: "nonexistent", Username: "ghost"})
	if err != auth.ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound on update, got %v", err)
	}
}

func TestUserStore_ListMultipleUsers(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.UserStore()
	ctx := context.Background()

	store.Create(ctx, &auth.User{ID: "u-1", Username: "alice", Role: auth.RoleAdmin})
	store.Create(ctx, &auth.User{ID: "u-2", Username: "bob", Role: auth.RoleStandard})
	store.Create(ctx, &auth.User{ID: "u-3", Username: "charlie", Role: auth.RoleStandard})

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 users, got %d", len(list))
	}
}

func TestUserStore_EmptyList(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.UserStore()
	list, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 users, got %d", len(list))
	}
}

func TestUserStore_UpdateSameUsernameSameUser(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.UserStore()
	ctx := context.Background()

	store.Create(ctx, &auth.User{ID: "u-1", Username: "alice", Role: auth.RoleAdmin})

	err = store.Update(ctx, &auth.User{ID: "u-1", Username: "alice", Role: auth.RoleStandard})
	if err != nil {
		t.Fatalf("updating same user with same username should succeed: %v", err)
	}

	got, _ := store.Get(ctx, "u-1")
	if got.Role != auth.RoleStandard {
		t.Errorf("Role = %q, want standard", got.Role)
	}
}
