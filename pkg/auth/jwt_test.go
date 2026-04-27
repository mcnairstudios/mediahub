package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func newTestService() (*JWTService, *MemoryUserStore) {
	store := NewMemoryUserStore()
	svc := NewJWTService(store, "test-secret-key-for-testing")
	return svc, store
}

func TestJWT_CreateUserAndLogin(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	user, err := svc.CreateUser(ctx, "alice", "password123", RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Username != "alice" || user.Role != RoleAdmin || !user.IsAdmin {
		t.Fatalf("unexpected user: %+v", user)
	}

	token, err := svc.Login(ctx, "alice", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestJWT_LoginWrongPassword(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	svc.CreateUser(ctx, "alice", "password123", RoleAdmin)

	_, err := svc.Login(ctx, "alice", "wrongpassword")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestJWT_LoginUnknownUser(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	_, err := svc.Login(ctx, "nobody", "password")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestJWT_ValidateToken(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	svc.CreateUser(ctx, "alice", "pass", RoleStandard)
	token, _ := svc.Login(ctx, "alice", "pass")

	user, err := svc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if user.Username != "alice" || user.Role != RoleStandard {
		t.Fatalf("unexpected user from token: %+v", user)
	}
	if user.IsAdmin {
		t.Fatal("standard user should not be admin")
	}
}

func TestJWT_ValidateExpiredToken(t *testing.T) {
	store := NewMemoryUserStore()
	svc := NewJWTService(store, "test-secret")
	svc.tokenTTL = 1 * time.Millisecond
	ctx := context.Background()

	svc.CreateUser(ctx, "alice", "pass", RoleAdmin)
	token, _ := svc.Login(ctx, "alice", "pass")

	time.Sleep(10 * time.Millisecond)

	_, err := svc.ValidateToken(ctx, token)
	if err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestJWT_ValidateInvalidToken(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	_, err := svc.ValidateToken(ctx, "garbage.token.here")
	if err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestJWT_RefreshToken(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	user, _ := svc.CreateUser(ctx, "alice", "pass", RoleAdmin)

	refreshToken, err := svc.GenerateRefreshToken(user)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	newAccess, err := svc.RefreshToken(ctx, refreshToken)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}

	validated, err := svc.ValidateToken(ctx, newAccess)
	if err != nil {
		t.Fatalf("ValidateToken after refresh: %v", err)
	}
	if validated.Username != "alice" {
		t.Fatalf("expected alice, got %s", validated.Username)
	}
}

func TestJWT_RefreshTokenRejectsAccessToken(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	svc.CreateUser(ctx, "alice", "pass", RoleAdmin)
	accessToken, _ := svc.Login(ctx, "alice", "pass")

	_, err := svc.RefreshToken(ctx, accessToken)
	if err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid when using access token as refresh, got %v", err)
	}
}

func TestJWT_ListUsers(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	svc.CreateUser(ctx, "alice", "pass", RoleAdmin)
	svc.CreateUser(ctx, "bob", "pass", RoleStandard)

	users, err := svc.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestJWT_DeleteUser(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	user, _ := svc.CreateUser(ctx, "alice", "pass", RoleAdmin)

	if err := svc.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	users, _ := svc.ListUsers(ctx)
	if len(users) != 0 {
		t.Fatalf("expected 0 users after delete, got %d", len(users))
	}
}

func TestJWT_ChangePasswordAndLogin(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	user, _ := svc.CreateUser(ctx, "alice", "oldpass", RoleAdmin)

	if err := svc.ChangePassword(ctx, user.ID, "newpass"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	_, err := svc.Login(ctx, "alice", "oldpass")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected old password to fail, got %v", err)
	}

	token, err := svc.Login(ctx, "alice", "newpass")
	if err != nil {
		t.Fatalf("Login with new password: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestJWT_PasswordsAreBcryptHashed(t *testing.T) {
	svc, store := newTestService()
	ctx := context.Background()

	user, _ := svc.CreateUser(ctx, "alice", "mypassword", RoleAdmin)

	hash, err := store.GetPasswordHash(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetPasswordHash: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("mypassword")); err != nil {
		t.Fatal("stored hash is not valid bcrypt")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("wrongpassword")); err == nil {
		t.Fatal("bcrypt should reject wrong password")
	}
}

func TestJWT_TokenContainsCorrectClaims(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	svc.CreateUser(ctx, "alice", "pass", RoleAdmin)
	token, _ := svc.Login(ctx, "alice", "pass")

	parsed, err := jwt.ParseWithClaims(token, &claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key-for-testing"), nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	c := parsed.Claims.(*claims)
	if c.Username != "alice" {
		t.Fatalf("expected username alice, got %s", c.Username)
	}
	if c.Role != RoleAdmin {
		t.Fatalf("expected role admin, got %s", c.Role)
	}
	if c.Type != tokenAccess {
		t.Fatalf("expected access token type, got %s", c.Type)
	}
	if c.ExpiresAt == nil {
		t.Fatal("expected expiry to be set")
	}
}

func TestJWT_DuplicateUsernameRejected(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	_, err := svc.CreateUser(ctx, "alice", "pass1", RoleAdmin)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = svc.CreateUser(ctx, "alice", "pass2", RoleStandard)
	if err != ErrUsernameExists {
		t.Fatalf("expected ErrUsernameExists, got %v", err)
	}
}
