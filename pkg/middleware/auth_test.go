package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/auth"
)

type mockAuthService struct {
	validateFunc func(ctx context.Context, token string) (*auth.User, error)
}

func (m *mockAuthService) Login(ctx context.Context, username, password string) (string, error) {
	return "", nil
}

func (m *mockAuthService) ValidateToken(ctx context.Context, token string) (*auth.User, error) {
	return m.validateFunc(ctx, token)
}

func (m *mockAuthService) RefreshToken(ctx context.Context, token string) (string, error) {
	return "", nil
}

func (m *mockAuthService) CreateUser(ctx context.Context, username, password string, role auth.Role) (*auth.User, error) {
	return nil, nil
}

func (m *mockAuthService) ListUsers(ctx context.Context) ([]*auth.User, error) { return nil, nil }
func (m *mockAuthService) UpdateUser(ctx context.Context, id string, username string, role auth.Role) (*auth.User, error) {
	return nil, nil
}
func (m *mockAuthService) DeleteUser(ctx context.Context, id string) error { return nil }
func (m *mockAuthService) ChangePassword(ctx context.Context, id, newPassword string) error {
	return nil
}

func TestAuthenticate_ValidToken(t *testing.T) {
	expectedUser := &auth.User{ID: "u1", Username: "alice", IsAdmin: false, Role: auth.RoleStandard}
	svc := &mockAuthService{
		validateFunc: func(_ context.Context, token string) (*auth.User, error) {
			if token == "valid-token" {
				return expectedUser, nil
			}
			return nil, errors.New("invalid")
		},
	}

	var capturedUser *auth.User
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware(svc)
	handler := mw.Authenticate(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedUser == nil {
		t.Fatal("expected user on context")
	}
	if capturedUser.ID != "u1" {
		t.Fatalf("expected user ID u1, got %s", capturedUser.ID)
	}
}

func TestAuthenticate_MissingToken(t *testing.T) {
	svc := &mockAuthService{
		validateFunc: func(_ context.Context, _ string) (*auth.User, error) {
			return nil, errors.New("should not be called")
		},
	}

	mw := NewAuthMiddleware(svc)
	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthenticate_InvalidToken(t *testing.T) {
	svc := &mockAuthService{
		validateFunc: func(_ context.Context, _ string) (*auth.User, error) {
			return nil, errors.New("expired")
		},
	}

	mw := NewAuthMiddleware(svc)
	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthenticate_BearerPrefixParsing(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		wantCode   int
	}{
		{"lowercase bearer", "bearer valid-token", http.StatusOK},
		{"uppercase BEARER", "BEARER valid-token", http.StatusOK},
		{"mixed case BeArEr", "BeArEr valid-token", http.StatusOK},
		{"no prefix", "valid-token", http.StatusUnauthorized},
		{"wrong prefix", "Basic valid-token", http.StatusUnauthorized},
		{"empty after bearer", "Bearer ", http.StatusUnauthorized},
		{"bearer only", "Bearer", http.StatusUnauthorized},
	}

	svc := &mockAuthService{
		validateFunc: func(_ context.Context, token string) (*auth.User, error) {
			if token == "valid-token" {
				return &auth.User{ID: "u1"}, nil
			}
			return nil, errors.New("invalid")
		},
	}
	mw := NewAuthMiddleware(svc)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tt.authHeader)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d", tt.wantCode, rec.Code)
			}
		})
	}
}

func TestRequireAdmin_AdminUser(t *testing.T) {
	svc := &mockAuthService{
		validateFunc: func(_ context.Context, _ string) (*auth.User, error) {
			return &auth.User{ID: "a1", Username: "admin", IsAdmin: true, Role: auth.RoleAdmin}, nil
		},
	}

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware(svc)
	handler := mw.RequireAdmin(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !called {
		t.Fatal("inner handler should have been called")
	}
}

func TestRequireAdmin_NonAdminUser(t *testing.T) {
	svc := &mockAuthService{
		validateFunc: func(_ context.Context, _ string) (*auth.User, error) {
			return &auth.User{ID: "u1", Username: "alice", IsAdmin: false, Role: auth.RoleStandard}, nil
		},
	}

	mw := NewAuthMiddleware(svc)
	handler := mw.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for non-admin")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer user-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestUserFromContext_NoUser(t *testing.T) {
	user := UserFromContext(context.Background())
	if user != nil {
		t.Fatal("expected nil user from empty context")
	}
}
