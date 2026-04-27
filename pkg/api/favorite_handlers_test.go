package api

import (
	"net/http"
	"testing"
)

func TestListFavoritesEmpty(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/favorites", nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var favs []map[string]any
	decodeBody(resp, &favs)
	if len(favs) != 0 {
		t.Fatalf("expected 0 favorites, got %d", len(favs))
	}
}

func TestAddFavorite(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/favorites", map[string]string{
		"stream_id": "stream-1",
	}, env.standardToken)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	resp = env.request("GET", "/api/favorites", nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var favs []map[string]any
	decodeBody(resp, &favs)
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite, got %d", len(favs))
	}
	if favs[0]["stream_id"] != "stream-1" {
		t.Fatalf("expected stream_id=stream-1, got %v", favs[0]["stream_id"])
	}
}

func TestAddFavoriteMissingStreamID(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/favorites", map[string]string{}, env.standardToken)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRemoveFavorite(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	env.request("POST", "/api/favorites", map[string]string{
		"stream_id": "stream-1",
	}, env.standardToken)

	resp := env.request("DELETE", "/api/favorites/stream-1", nil, env.standardToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	resp = env.request("GET", "/api/favorites", nil, env.standardToken)
	var favs []map[string]any
	decodeBody(resp, &favs)
	if len(favs) != 0 {
		t.Fatalf("expected 0 favorites after remove, got %d", len(favs))
	}
}

func TestRemoveNonexistentFavorite(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/favorites/nonexistent", nil, env.standardToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestCheckFavoriteTrue(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	env.request("POST", "/api/favorites", map[string]string{
		"stream_id": "stream-1",
	}, env.standardToken)

	resp := env.request("GET", "/api/favorites/check/stream-1", nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]bool
	decodeBody(resp, &result)
	if !result["is_favorite"] {
		t.Fatal("expected is_favorite=true")
	}
}

func TestCheckFavoriteFalse(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/favorites/check/stream-1", nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]bool
	decodeBody(resp, &result)
	if result["is_favorite"] {
		t.Fatal("expected is_favorite=false")
	}
}

func TestFavoritesRequireAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/favorites", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	resp = env.request("POST", "/api/favorites", map[string]string{
		"stream_id": "stream-1",
	}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestFavoritesUserIsolation(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	env.request("POST", "/api/favorites", map[string]string{
		"stream_id": "stream-1",
	}, env.standardToken)

	env.request("POST", "/api/favorites", map[string]string{
		"stream_id": "stream-2",
	}, env.adminToken)

	resp := env.request("GET", "/api/favorites", nil, env.standardToken)
	var userFavs []map[string]any
	decodeBody(resp, &userFavs)
	if len(userFavs) != 1 {
		t.Fatalf("standard user: expected 1 favorite, got %d", len(userFavs))
	}

	resp = env.request("GET", "/api/favorites", nil, env.adminToken)
	var adminFavs []map[string]any
	decodeBody(resp, &adminFavs)
	if len(adminFavs) != 1 {
		t.Fatalf("admin user: expected 1 favorite, got %d", len(adminFavs))
	}
}
