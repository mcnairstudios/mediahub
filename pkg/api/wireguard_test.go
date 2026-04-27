package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/connectivity/wg"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

func newTestEnvWithWG(t *testing.T) *testEnv {
	t.Helper()
	env := newTestEnv(t)
	settingsStore := store.NewMemorySettingsStore()
	env.server.deps.WGService = wg.NewService(settingsStore)
	return env
}

func TestWGListProfilesEmpty(t *testing.T) {
	env := newTestEnvWithWG(t)
	defer env.close()

	resp := env.request("GET", "/api/wireguard/profiles", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var profiles []wg.ProfileResponse
	decodeBody(resp, &profiles)
	if len(profiles) != 0 {
		t.Fatalf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestWGCreateProfile(t *testing.T) {
	env := newTestEnvWithWG(t)
	defer env.close()

	body := map[string]string{
		"name":        "test-vpn",
		"private_key": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"endpoint":    "1.2.3.4:51820",
		"public_key":  "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		"allowed_ips": "0.0.0.0/0",
		"address":     "10.0.0.2/24",
		"dns":         "1.1.1.1",
	}

	resp := env.request("POST", "/api/wireguard/profiles", body, env.adminToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var profile wg.ProfileResponse
	decodeBody(resp, &profile)
	if profile.Name != "test-vpn" {
		t.Fatalf("expected test-vpn, got %s", profile.Name)
	}
	if profile.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if profile.PrivateKey == "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" {
		t.Fatal("expected private key to be masked")
	}
	if !profile.IsActive {
		t.Log("new profiles are inactive by default")
	}
}

func TestWGCreateProfileValidation(t *testing.T) {
	env := newTestEnvWithWG(t)
	defer env.close()

	resp := env.request("POST", "/api/wireguard/profiles", map[string]string{
		"name": "incomplete",
	}, env.adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestWGCreateAndListProfiles(t *testing.T) {
	env := newTestEnvWithWG(t)
	defer env.close()

	body := map[string]string{
		"name":        "vpn-1",
		"private_key": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"endpoint":    "1.2.3.4:51820",
		"public_key":  "BBBB",
		"address":     "10.0.0.2/24",
	}
	env.request("POST", "/api/wireguard/profiles", body, env.adminToken)

	body["name"] = "vpn-2"
	body["endpoint"] = "5.6.7.8:51820"
	env.request("POST", "/api/wireguard/profiles", body, env.adminToken)

	resp := env.request("GET", "/api/wireguard/profiles", nil, env.adminToken)
	var profiles []wg.ProfileResponse
	decodeBody(resp, &profiles)
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestWGUpdateProfile(t *testing.T) {
	env := newTestEnvWithWG(t)
	defer env.close()

	createResp := env.request("POST", "/api/wireguard/profiles", map[string]string{
		"name":        "original",
		"private_key": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"endpoint":    "1.2.3.4:51820",
		"public_key":  "BBBB",
		"address":     "10.0.0.2/24",
	}, env.adminToken)
	var created wg.ProfileResponse
	decodeBody(createResp, &created)

	updateResp := env.request("PUT", "/api/wireguard/profiles/"+created.ID, map[string]string{
		"name":     "renamed",
		"endpoint": "9.8.7.6:51820",
	}, env.adminToken)
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", updateResp.StatusCode)
	}

	var updated wg.ProfileResponse
	decodeBody(updateResp, &updated)
	if updated.Name != "renamed" {
		t.Fatalf("expected renamed, got %s", updated.Name)
	}
}

func TestWGDeleteProfile(t *testing.T) {
	env := newTestEnvWithWG(t)
	defer env.close()

	createResp := env.request("POST", "/api/wireguard/profiles", map[string]string{
		"name":        "to-delete",
		"private_key": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"endpoint":    "1.2.3.4:51820",
		"public_key":  "BBBB",
		"address":     "10.0.0.2/24",
	}, env.adminToken)
	var created wg.ProfileResponse
	decodeBody(createResp, &created)

	delResp := env.request("DELETE", "/api/wireguard/profiles/"+created.ID, nil, env.adminToken)
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", delResp.StatusCode)
	}

	listResp := env.request("GET", "/api/wireguard/profiles", nil, env.adminToken)
	var profiles []wg.ProfileResponse
	decodeBody(listResp, &profiles)
	if len(profiles) != 0 {
		t.Fatalf("expected 0 profiles after delete, got %d", len(profiles))
	}
}

func TestWGStatusDisconnected(t *testing.T) {
	env := newTestEnvWithWG(t)
	defer env.close()

	resp := env.request("GET", "/api/wireguard/status", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var status wg.StatusResponse
	decodeBody(resp, &status)
	if status.Connected {
		t.Fatal("expected disconnected")
	}
}

func TestWGRequiresAdmin(t *testing.T) {
	env := newTestEnvWithWG(t)
	defer env.close()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/wireguard/profiles"},
		{"POST", "/api/wireguard/profiles"},
		{"GET", "/api/wireguard/status"},
	}

	for _, ep := range endpoints {
		resp := env.request(ep.method, ep.path, nil, env.standardToken)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("%s %s: expected 403 for standard user, got %d", ep.method, ep.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestWGRequiresAuth(t *testing.T) {
	env := newTestEnvWithWG(t)
	defer env.close()

	resp := env.request("GET", "/api/wireguard/profiles", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWGStatusWithoutService(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/wireguard/status", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var status map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&status)
	resp.Body.Close()

	if status["connected"] != false {
		t.Fatal("expected connected=false when no WG service")
	}
}
