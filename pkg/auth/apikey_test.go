package auth

import (
	"context"
	"testing"
)

func newTestServiceWithAPIKeys() (*JWTService, *MemoryUserStore, *MemoryAPIKeyStore) {
	store := NewMemoryUserStore()
	apiKeys := NewMemoryAPIKeyStore()
	svc := NewJWTService(store, "test-secret-key-for-testing")
	svc.SetAPIKeyStore(apiKeys)
	return svc, store, apiKeys
}

func TestAPIKey_CreateAndValidate(t *testing.T) {
	svc, _, _ := newTestServiceWithAPIKeys()
	ctx := context.Background()

	user, _ := svc.CreateUser(ctx, "alice", "pass", "", RoleAdmin)

	apiKey, err := svc.CreateAPIKey(ctx, user.ID, "my-key")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if apiKey.Key == "" {
		t.Fatal("expected non-empty key")
	}
	if apiKey.UserID != user.ID {
		t.Fatalf("expected userID %s, got %s", user.ID, apiKey.UserID)
	}
	if apiKey.Name != "my-key" {
		t.Fatalf("expected name my-key, got %s", apiKey.Name)
	}

	validated, err := svc.ValidateAPIKey(ctx, apiKey.Key)
	if err != nil {
		t.Fatalf("ValidateAPIKey: %v", err)
	}
	if validated.Username != "alice" {
		t.Fatalf("expected alice, got %s", validated.Username)
	}
	if validated.Role != RoleAdmin {
		t.Fatalf("expected admin role, got %s", validated.Role)
	}
}

func TestAPIKey_ValidateInvalidKey(t *testing.T) {
	svc, _, _ := newTestServiceWithAPIKeys()
	ctx := context.Background()

	_, err := svc.ValidateAPIKey(ctx, "nonexistent-key")
	if err != ErrAPIKeyNotFound {
		t.Fatalf("expected ErrAPIKeyNotFound, got %v", err)
	}
}

func TestAPIKey_ListByUser(t *testing.T) {
	svc, _, _ := newTestServiceWithAPIKeys()
	ctx := context.Background()

	alice, _ := svc.CreateUser(ctx, "alice", "pass", "", RoleAdmin)
	bob, _ := svc.CreateUser(ctx, "bob", "pass", "", RoleStandard)

	svc.CreateAPIKey(ctx, alice.ID, "alice-key-1")
	svc.CreateAPIKey(ctx, alice.ID, "alice-key-2")
	svc.CreateAPIKey(ctx, bob.ID, "bob-key-1")

	aliceKeys, err := svc.ListAPIKeys(ctx, alice.ID)
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	if len(aliceKeys) != 2 {
		t.Fatalf("expected 2 keys for alice, got %d", len(aliceKeys))
	}

	bobKeys, _ := svc.ListAPIKeys(ctx, bob.ID)
	if len(bobKeys) != 1 {
		t.Fatalf("expected 1 key for bob, got %d", len(bobKeys))
	}
}

func TestAPIKey_Revoke(t *testing.T) {
	svc, _, _ := newTestServiceWithAPIKeys()
	ctx := context.Background()

	user, _ := svc.CreateUser(ctx, "alice", "pass", "", RoleAdmin)
	apiKey, _ := svc.CreateAPIKey(ctx, user.ID, "my-key")

	if err := svc.RevokeAPIKey(ctx, user.ID, apiKey.ID); err != nil {
		t.Fatalf("RevokeAPIKey: %v", err)
	}

	_, err := svc.ValidateAPIKey(ctx, apiKey.Key)
	if err != ErrAPIKeyNotFound {
		t.Fatalf("expected ErrAPIKeyNotFound after revoke, got %v", err)
	}

	keys, _ := svc.ListAPIKeys(ctx, user.ID)
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys after revoke, got %d", len(keys))
	}
}

func TestAPIKey_RevokeOtherUsersKey(t *testing.T) {
	svc, _, _ := newTestServiceWithAPIKeys()
	ctx := context.Background()

	alice, _ := svc.CreateUser(ctx, "alice", "pass", "", RoleAdmin)
	bob, _ := svc.CreateUser(ctx, "bob", "pass", "", RoleStandard)
	aliceKey, _ := svc.CreateAPIKey(ctx, alice.ID, "alice-key")

	err := svc.RevokeAPIKey(ctx, bob.ID, aliceKey.ID)
	if err != ErrAPIKeyNotFound {
		t.Fatalf("expected ErrAPIKeyNotFound when revoking other user's key, got %v", err)
	}
}

func TestAPIKey_CreateForNonexistentUser(t *testing.T) {
	svc, _, _ := newTestServiceWithAPIKeys()
	ctx := context.Background()

	_, err := svc.CreateAPIKey(ctx, "nonexistent-id", "my-key")
	if err == nil {
		t.Fatal("expected error creating key for nonexistent user")
	}
}

func TestAPIKey_ValidateAfterUserDeleted(t *testing.T) {
	svc, _, _ := newTestServiceWithAPIKeys()
	ctx := context.Background()

	user, _ := svc.CreateUser(ctx, "alice", "pass", "", RoleAdmin)
	apiKey, _ := svc.CreateAPIKey(ctx, user.ID, "my-key")

	svc.DeleteUser(ctx, user.ID)

	_, err := svc.ValidateAPIKey(ctx, apiKey.Key)
	if err == nil {
		t.Fatal("expected error validating key after user deleted")
	}
}
