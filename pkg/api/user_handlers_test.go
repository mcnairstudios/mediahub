package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/auth"
)

func TestUpdateUser(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/users", map[string]any{
		"username": "testuser",
		"password": "pass123",
		"role":     "standard",
	}, env.adminToken)

	var user auth.User
	decodeBody(resp, &user)

	resp = env.request("PUT", "/api/users/"+user.ID, map[string]any{
		"username": "updated_user",
		"role":     "admin",
	}, env.adminToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var updated auth.User
	decodeBody(resp, &updated)

	if updated.Username != "updated_user" {
		t.Errorf("username = %q, want %q", updated.Username, "updated_user")
	}
	if updated.Role != auth.RoleAdmin {
		t.Errorf("role = %q, want %q", updated.Role, auth.RoleAdmin)
	}
}

func TestUpdateUserNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("PUT", "/api/users/nonexistent", map[string]any{
		"username": "whatever",
	}, env.adminToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateUserDuplicateUsername(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/users", map[string]any{
		"username": "user_a",
		"password": "pass123",
		"role":     "standard",
	}, env.adminToken)

	var userA auth.User
	decodeBody(resp, &userA)

	resp = env.request("PUT", "/api/users/"+userA.ID, map[string]any{
		"username": "admin",
	}, env.adminToken)

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestUpdateUserRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("PUT", "/api/users/someid", map[string]any{
		"username": "hacked",
	}, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestDeleteUser(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/users", map[string]any{
		"username": "to_delete",
		"password": "pass123",
		"role":     "standard",
	}, env.adminToken)

	var user auth.User
	decodeBody(resp, &user)

	resp = env.request("DELETE", "/api/users/"+user.ID, nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	resp = env.request("GET", "/api/users", nil, env.adminToken)
	var users []auth.User
	decodeBody(resp, &users)

	for _, u := range users {
		if u.ID == user.ID {
			t.Fatal("deleted user still present in list")
		}
	}
}

func TestDeleteUserNonexistent(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/users/nonexistent", nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestDeleteUserRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/users/someid", nil, env.standardToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestChangePasswordAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/users", map[string]any{
		"username": "changepass",
		"password": "oldpass",
		"role":     "standard",
	}, env.adminToken)

	var user auth.User
	decodeBody(resp, &user)

	resp = env.request("PUT", "/api/users/"+user.ID+"/password", map[string]any{
		"password": "newpass123",
	}, env.adminToken)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	resp = env.request("POST", "/api/auth/login", map[string]string{
		"username": "changepass",
		"password": "newpass123",
	}, "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected login with new password to succeed, got %d", resp.StatusCode)
	}
}

func TestChangePasswordSelf(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()
	users, _ := env.server.deps.AuthService.ListUsers(ctx)

	var viewerID string
	for _, u := range users {
		if u.Username == "viewer" {
			viewerID = u.ID
			break
		}
	}

	resp := env.request("PUT", "/api/users/"+viewerID+"/password", map[string]any{
		"password": "newviewerpass",
	}, env.standardToken)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	resp = env.request("POST", "/api/auth/login", map[string]string{
		"username": "viewer",
		"password": "newviewerpass",
	}, "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected login with new password to succeed, got %d", resp.StatusCode)
	}
}

func TestChangePasswordOtherUserForbidden(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()
	users, _ := env.server.deps.AuthService.ListUsers(ctx)

	var adminID string
	for _, u := range users {
		if u.Username == "admin" {
			adminID = u.ID
			break
		}
	}

	resp := env.request("PUT", "/api/users/"+adminID+"/password", map[string]any{
		"password": "hacked",
	}, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestChangePasswordMissing(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()
	users, _ := env.server.deps.AuthService.ListUsers(ctx)

	var adminID string
	for _, u := range users {
		if u.Username == "admin" {
			adminID = u.ID
			break
		}
	}

	resp := env.request("PUT", "/api/users/"+adminID+"/password", map[string]any{}, env.adminToken)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestChangePasswordNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("PUT", "/api/users/nonexistent/password", map[string]any{
		"password": "newpass",
	}, env.adminToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
