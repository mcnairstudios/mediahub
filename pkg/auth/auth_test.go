package auth

import (
	"context"
	"testing"
)

func TestRoleConstants(t *testing.T) {
	if RoleAdmin != "admin" {
		t.Errorf("RoleAdmin = %q, want %q", RoleAdmin, "admin")
	}
	if RoleStandard != "standard" {
		t.Errorf("RoleStandard = %q, want %q", RoleStandard, "standard")
	}
	if RoleJellyfin != "jellyfin" {
		t.Errorf("RoleJellyfin = %q, want %q", RoleJellyfin, "jellyfin")
	}
}

func TestUserStruct(t *testing.T) {
	u := User{
		ID:       "abc-123",
		Username: "testuser",
		IsAdmin:  true,
		Role:     RoleAdmin,
	}
	if u.ID != "abc-123" {
		t.Errorf("ID = %q, want %q", u.ID, "abc-123")
	}
	if u.Username != "testuser" {
		t.Errorf("Username = %q, want %q", u.Username, "testuser")
	}
	if !u.IsAdmin {
		t.Error("IsAdmin = false, want true")
	}
	if u.Role != RoleAdmin {
		t.Errorf("Role = %q, want %q", u.Role, RoleAdmin)
	}
}

type mockService struct{}

func (m *mockService) Login(_ context.Context, _, _ string) (string, error)        { return "", nil }
func (m *mockService) ValidateToken(_ context.Context, _ string) (*User, error)    { return nil, nil }
func (m *mockService) RefreshToken(_ context.Context, _ string) (string, error)    { return "", nil }
func (m *mockService) CreateUser(_ context.Context, _, _ string, _ Role) (*User, error) {
	return nil, nil
}
func (m *mockService) ListUsers(_ context.Context) ([]*User, error)          { return nil, nil }
func (m *mockService) DeleteUser(_ context.Context, _ string) error          { return nil }
func (m *mockService) ChangePassword(_ context.Context, _, _ string) error   { return nil }

func TestMockSatisfiesService(t *testing.T) {
	var _ Service = (*mockService)(nil)
}

type mockUserStore struct{}

func (m *mockUserStore) Get(_ context.Context, _ string) (*User, error)            { return nil, nil }
func (m *mockUserStore) GetByUsername(_ context.Context, _ string) (*User, error)   { return nil, nil }
func (m *mockUserStore) List(_ context.Context) ([]*User, error)                   { return nil, nil }
func (m *mockUserStore) Create(_ context.Context, _ *User) error                   { return nil }
func (m *mockUserStore) Delete(_ context.Context, _ string) error                  { return nil }
func (m *mockUserStore) UpdatePassword(_ context.Context, _, _ string) error       { return nil }

func TestMockSatisfiesUserStore(t *testing.T) {
	var _ UserStore = (*mockUserStore)(nil)
}
