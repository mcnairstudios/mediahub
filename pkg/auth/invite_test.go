package auth

import (
	"context"
	"testing"
	"time"
)

func newTestServiceWithInvites() (*JWTService, *MemoryUserStore, *MemoryInviteStore) {
	store := NewMemoryUserStore()
	invites := NewMemoryInviteStore()
	svc := NewJWTService(store, "test-secret-key-for-testing")
	svc.SetInviteStore(invites)
	return svc, store, invites
}

func TestInvite_CreateAndAccept(t *testing.T) {
	svc, _, _ := newTestServiceWithInvites()
	ctx := context.Background()

	invite, err := svc.CreateInvite(ctx, RoleStandard, 1*time.Hour)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if invite.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if invite.Role != RoleStandard {
		t.Fatalf("expected role standard, got %s", invite.Role)
	}
	if invite.Used {
		t.Fatal("invite should not be used")
	}
	if invite.ExpiresAt.Before(time.Now()) {
		t.Fatal("invite should not be expired")
	}

	user, err := svc.AcceptInvite(ctx, invite.Token, "newuser", "password123")
	if err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	if user.Username != "newuser" {
		t.Fatalf("expected username newuser, got %s", user.Username)
	}
	if user.Role != RoleStandard {
		t.Fatalf("expected role standard, got %s", user.Role)
	}

	_, err = svc.Login(ctx, "newuser", "password123")
	if err != nil {
		t.Fatalf("Login after accept: %v", err)
	}
}

func TestInvite_AcceptUsedInvite(t *testing.T) {
	svc, _, _ := newTestServiceWithInvites()
	ctx := context.Background()

	invite, _ := svc.CreateInvite(ctx, RoleStandard, 1*time.Hour)
	svc.AcceptInvite(ctx, invite.Token, "user1", "pass1")

	_, err := svc.AcceptInvite(ctx, invite.Token, "user2", "pass2")
	if err != ErrInviteUsed {
		t.Fatalf("expected ErrInviteUsed, got %v", err)
	}
}

func TestInvite_AcceptExpiredInvite(t *testing.T) {
	svc, _, _ := newTestServiceWithInvites()
	ctx := context.Background()

	invite, _ := svc.CreateInvite(ctx, RoleStandard, 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	_, err := svc.AcceptInvite(ctx, invite.Token, "user1", "pass1")
	if err != ErrInviteExpired {
		t.Fatalf("expected ErrInviteExpired, got %v", err)
	}
}

func TestInvite_AcceptInvalidToken(t *testing.T) {
	svc, _, _ := newTestServiceWithInvites()
	ctx := context.Background()

	_, err := svc.AcceptInvite(ctx, "nonexistent-token", "user1", "pass1")
	if err != ErrInviteNotFound {
		t.Fatalf("expected ErrInviteNotFound, got %v", err)
	}
}

func TestInvite_ListAndDelete(t *testing.T) {
	svc, _, _ := newTestServiceWithInvites()
	ctx := context.Background()

	svc.CreateInvite(ctx, RoleStandard, 1*time.Hour)
	svc.CreateInvite(ctx, RoleAdmin, 1*time.Hour)

	invites, err := svc.ListInvites(ctx)
	if err != nil {
		t.Fatalf("ListInvites: %v", err)
	}
	if len(invites) != 2 {
		t.Fatalf("expected 2 invites, got %d", len(invites))
	}

	if err := svc.DeleteInvite(ctx, invites[0].Token); err != nil {
		t.Fatalf("DeleteInvite: %v", err)
	}

	invites, _ = svc.ListInvites(ctx)
	if len(invites) != 1 {
		t.Fatalf("expected 1 invite after delete, got %d", len(invites))
	}
}

func TestInvite_DefaultExpiry(t *testing.T) {
	svc, _, _ := newTestServiceWithInvites()
	ctx := context.Background()

	invite, err := svc.CreateInvite(ctx, RoleStandard, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	expectedExpiry := time.Now().Add(24 * time.Hour)
	diff := invite.ExpiresAt.Sub(expectedExpiry)
	if diff > 1*time.Second || diff < -1*time.Second {
		t.Fatalf("expected ~24h expiry, got %v", invite.ExpiresAt)
	}
}

func TestInvite_AdminRole(t *testing.T) {
	svc, _, _ := newTestServiceWithInvites()
	ctx := context.Background()

	invite, _ := svc.CreateInvite(ctx, RoleAdmin, 1*time.Hour)
	user, err := svc.AcceptInvite(ctx, invite.Token, "adminuser", "pass")
	if err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	if user.Role != RoleAdmin {
		t.Fatalf("expected admin role, got %s", user.Role)
	}
	if !user.IsAdmin {
		t.Fatal("expected IsAdmin=true")
	}
}

func TestInvite_DuplicateUsername(t *testing.T) {
	svc, _, _ := newTestServiceWithInvites()
	ctx := context.Background()

	svc.CreateUser(ctx, "alice", "pass", "", RoleStandard)

	invite, _ := svc.CreateInvite(ctx, RoleStandard, 1*time.Hour)
	_, err := svc.AcceptInvite(ctx, invite.Token, "alice", "pass2")
	if err != ErrUsernameExists {
		t.Fatalf("expected ErrUsernameExists, got %v", err)
	}
}
