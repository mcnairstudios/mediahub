package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/favorite"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
)

type testEnv struct {
	server       *Server
	httpServer   *httptest.Server
	adminToken   string
	standardToken string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	userStore := auth.NewMemoryUserStore()
	authService := auth.NewJWTService(userStore, "test-secret-key-for-api-tests")

	ctx := context.Background()
	_, err := authService.CreateUser(ctx, "admin", "adminpass", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	_, err = authService.CreateUser(ctx, "viewer", "viewerpass", auth.RoleStandard)
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	adminToken, err := authService.Login(ctx, "admin", "adminpass")
	if err != nil {
		t.Fatalf("admin login: %v", err)
	}
	standardToken, err := authService.Login(ctx, "viewer", "viewerpass")
	if err != nil {
		t.Fatalf("viewer login: %v", err)
	}

	streamStore := store.NewMemoryStreamStore()
	streamStore.BulkUpsert(ctx, []media.Stream{
		{ID: "stream-1", Name: "BBC One", URL: "http://example.com/bbc1", SourceType: "m3u", SourceID: "src-1", IsActive: true},
		{ID: "stream-2", Name: "BBC Two", URL: "http://example.com/bbc2", SourceType: "m3u", SourceID: "src-1", IsActive: true},
	})

	channelStore := store.NewMemoryChannelStore()
	settingsStore := store.NewMemorySettingsStore()
	settingsStore.Set(ctx, "base_url", "http://localhost:8080")

	recordingStore := store.NewMemoryRecordingStore()
	epgSourceStore := store.NewMemoryEPGSourceStore()

	sourceConfigStore := sourceconfig.NewMemoryStore()

	programStore := store.NewMemoryProgramStore()
	groupStore := store.NewMemoryGroupStore()

	deps := OrchestratorDeps{
		StreamStore:       streamStore,
		ChannelStore:      channelStore,
		SettingsStore:     settingsStore,
		SourceConfigStore: sourceConfigStore,
		SessionMgr:        session.NewManager(t.TempDir()),
		Detector:          client.NewDetector(nil),
		OutputReg:         output.NewRegistry(),
		SourceReg:         source.NewRegistry(),
		RecordingStore:    recordingStore,
		AuthService:       authService,
		EPGSourceStore:    epgSourceStore,
		ProgramStore:      programStore,
		GroupStore:        groupStore,
		FavoriteStore:     favorite.NewMemoryStore(),
		Strategy: func(in strategy.Input, out strategy.Output) strategy.Decision {
			return strategy.Resolve(in, out)
		},
	}

	srv := NewServer(deps)
	ts := httptest.NewServer(srv.Handler())

	return &testEnv{
		server:        srv,
		httpServer:    ts,
		adminToken:    adminToken,
		standardToken: standardToken,
	}
}

func (e *testEnv) close() {
	e.httpServer.Close()
}

func (e *testEnv) request(method, path string, body any, token string) *http.Response {
	var bodyReader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, _ := http.NewRequest(method, e.httpServer.URL+path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	return resp
}

func decodeBody(resp *http.Response, v any) {
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(v)
}

func TestLoginValid(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "adminpass",
	}, "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	decodeBody(resp, &result)

	if _, ok := result["access_token"]; !ok {
		t.Fatal("response missing access_token")
	}
	if result["access_token"] == "" {
		t.Fatal("access_token is empty")
	}
}

func TestLoginInvalid(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "wrongpass",
	}, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLoginMissingFields(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/auth/login", map[string]string{
		"username": "admin",
	}, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestListStreamsNoAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/streams", nil, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListStreamsWithAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/streams", nil, env.standardToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var streams []media.Stream
	decodeBody(resp, &streams)

	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}
}

func TestListStreamsFilterBySource(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()
	env.server.deps.StreamStore.BulkUpsert(ctx, []media.Stream{
		{ID: "stream-3", Name: "Channel X", URL: "http://example.com/x", SourceType: "xtream", SourceID: "src-2", IsActive: true},
	})

	resp := env.request("GET", "/api/streams?source_type=m3u&source_id=src-1", nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var streams []media.Stream
	decodeBody(resp, &streams)

	if len(streams) != 2 {
		t.Fatalf("expected 2 streams from m3u/src-1, got %d", len(streams))
	}

	resp = env.request("GET", "/api/streams?source_type=xtream&source_id=src-2", nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	decodeBody(resp, &streams)
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream from xtream/src-2, got %d", len(streams))
	}
	if streams[0].Name != "Channel X" {
		t.Fatalf("expected 'Channel X', got %q", streams[0].Name)
	}

	resp = env.request("GET", "/api/streams", nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	decodeBody(resp, &streams)
	if len(streams) != 3 {
		t.Fatalf("expected 3 total streams, got %d", len(streams))
	}
}

func TestAdminEndpointWithStandardUser(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/users", nil, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestAdminEndpointWithAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/users", nil, env.adminToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var users []auth.User
	decodeBody(resp, &users)

	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/settings", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET settings: expected 200, got %d", resp.StatusCode)
	}

	var initial map[string]string
	decodeBody(resp, &initial)

	if initial["base_url"] != "http://localhost:8080" {
		t.Fatalf("expected base_url=http://localhost:8080, got %q", initial["base_url"])
	}

	resp = env.request("PUT", "/api/settings", map[string]string{
		"base_url":    "http://192.168.1.100:8080",
		"dlna_enabled": "true",
	}, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT settings: expected 200, got %d", resp.StatusCode)
	}

	resp = env.request("GET", "/api/settings", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET settings after update: expected 200, got %d", resp.StatusCode)
	}

	var updated map[string]string
	decodeBody(resp, &updated)

	if updated["base_url"] != "http://192.168.1.100:8080" {
		t.Fatalf("expected updated base_url, got %q", updated["base_url"])
	}
	if updated["dlna_enabled"] != "true" {
		t.Fatalf("expected dlna_enabled=true, got %q", updated["dlna_enabled"])
	}
}

func TestUpdateSettingsRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("PUT", "/api/settings", map[string]string{
		"base_url": "http://evil.com",
	}, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCreateUser(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/users", map[string]any{
		"username": "newuser",
		"password": "newpass123",
		"role":     "standard",
	}, env.adminToken)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var user auth.User
	decodeBody(resp, &user)

	if user.Username != "newuser" {
		t.Fatalf("expected username=newuser, got %q", user.Username)
	}
	if user.Role != auth.RoleStandard {
		t.Fatalf("expected role=standard, got %q", user.Role)
	}
}

func TestCreateUserRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/users", map[string]any{
		"username": "newuser",
		"password": "newpass123",
	}, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestListChannels(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/channels", nil, env.standardToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestListRecordings(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/recordings", nil, env.standardToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestListEPGSources(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/epg/sources", nil, env.standardToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStopPlaybackNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/play/nonexistent", nil, env.standardToken)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestInvalidToken(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/streams", nil, "invalid-token-here")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListSourcesEmpty(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/sources", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var sources []map[string]any
	decodeBody(resp, &sources)
	if len(sources) != 0 {
		t.Fatalf("expected 0 sources, got %d", len(sources))
	}
}

func TestCreateM3USource(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/sources/m3u", map[string]any{
		"name": "UK IPTV",
		"url":  "http://example.com/playlist.m3u",
	}, env.adminToken)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var sc sourceconfig.SourceConfig
	decodeBody(resp, &sc)

	if sc.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if sc.Name != "UK IPTV" {
		t.Errorf("Name = %q, want %q", sc.Name, "UK IPTV")
	}
	if sc.Type != "m3u" {
		t.Errorf("Type = %q, want %q", sc.Type, "m3u")
	}
	if !sc.IsEnabled {
		t.Error("IsEnabled should be true")
	}
	if sc.Config["url"] != "http://example.com/playlist.m3u" {
		t.Errorf("Config[url] = %q, want %q", sc.Config["url"], "http://example.com/playlist.m3u")
	}
}

func TestCreateM3USourceMissingFields(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/sources/m3u", map[string]any{
		"name": "Missing URL",
	}, env.adminToken)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateM3USourceRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/sources/m3u", map[string]any{
		"name": "UK IPTV",
		"url":  "http://example.com/playlist.m3u",
	}, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCreateAndListSources(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	env.request("POST", "/api/sources/m3u", map[string]any{
		"name": "Source One",
		"url":  "http://example.com/one.m3u",
	}, env.adminToken)

	env.request("POST", "/api/sources/m3u", map[string]any{
		"name": "Source Two",
		"url":  "http://example.com/two.m3u",
	}, env.adminToken)

	resp := env.request("GET", "/api/sources", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var sources []map[string]any
	decodeBody(resp, &sources)
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
}

func TestDeleteM3USource(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/sources/m3u", map[string]any{
		"name": "To Delete",
		"url":  "http://example.com/del.m3u",
	}, env.adminToken)

	var sc sourceconfig.SourceConfig
	decodeBody(resp, &sc)

	resp = env.request("DELETE", "/api/sources/m3u/"+sc.ID, nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	resp = env.request("GET", "/api/sources", nil, env.adminToken)
	var sources []map[string]any
	decodeBody(resp, &sources)
	if len(sources) != 0 {
		t.Fatalf("expected 0 sources after delete, got %d", len(sources))
	}
}

func TestUpdateM3USource(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/sources/m3u", map[string]any{
		"name": "Original",
		"url":  "http://example.com/orig.m3u",
	}, env.adminToken)

	var sc sourceconfig.SourceConfig
	decodeBody(resp, &sc)

	resp = env.request("PUT", "/api/sources/m3u/"+sc.ID, map[string]any{
		"name": "Updated Name",
	}, env.adminToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var updated sourceconfig.SourceConfig
	decodeBody(resp, &updated)
	if updated.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", updated.Name, "Updated Name")
	}
}

func TestUpdateM3USourceNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("PUT", "/api/sources/m3u/nonexistent", map[string]any{
		"name": "Whatever",
	}, env.adminToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestSourceStatusEndpoint(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()
	env.server.deps.SourceConfigStore.Create(ctx, &sourceconfig.SourceConfig{
		ID:        "status-src",
		Type:      "m3u",
		Name:      "Status Test",
		IsEnabled: true,
		Config:    map[string]string{"url": "http://example.com/test.m3u"},
	})

	resp := env.request("GET", "/api/sources/status-src/status", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var status map[string]any
	decodeBody(resp, &status)
	if status["state"] != "idle" {
		t.Errorf("state = %q, want %q", status["state"], "idle")
	}
}

func TestSourceStatusNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/sources/nonexistent/status", nil, env.adminToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCreateEPGSource(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/epg/sources", map[string]any{
		"name": "UK EPG",
		"url":  "http://example.com/uk.xml",
	}, env.adminToken)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var src map[string]any
	decodeBody(resp, &src)

	if src["id"] == "" {
		t.Fatal("expected non-empty ID")
	}
	if src["name"] != "UK EPG" {
		t.Errorf("name = %q, want %q", src["name"], "UK EPG")
	}
	if src["url"] != "http://example.com/uk.xml" {
		t.Errorf("url = %q, want %q", src["url"], "http://example.com/uk.xml")
	}
}

func TestCreateEPGSourceMissingFields(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/epg/sources", map[string]any{
		"name": "No URL",
	}, env.adminToken)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateEPGSourceRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/epg/sources", map[string]any{
		"name": "UK EPG",
		"url":  "http://example.com/uk.xml",
	}, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestUpdateEPGSource(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/epg/sources", map[string]any{
		"name": "Original",
		"url":  "http://example.com/orig.xml",
	}, env.adminToken)

	var src map[string]any
	decodeBody(resp, &src)
	id := src["id"].(string)

	resp = env.request("PUT", "/api/epg/sources/"+id, map[string]any{
		"name": "Updated EPG",
	}, env.adminToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var updated map[string]any
	decodeBody(resp, &updated)
	if updated["name"] != "Updated EPG" {
		t.Errorf("name = %q, want %q", updated["name"], "Updated EPG")
	}
}

func TestUpdateEPGSourceNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("PUT", "/api/epg/sources/nonexistent", map[string]any{
		"name": "Whatever",
	}, env.adminToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteEPGSource(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/epg/sources", map[string]any{
		"name": "To Delete",
		"url":  "http://example.com/del.xml",
	}, env.adminToken)

	var src map[string]any
	decodeBody(resp, &src)
	id := src["id"].(string)

	resp = env.request("DELETE", "/api/epg/sources/"+id, nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	resp = env.request("GET", "/api/epg/sources", nil, env.adminToken)
	var sources []map[string]any
	decodeBody(resp, &sources)
	if len(sources) != 0 {
		t.Fatalf("expected 0 sources after delete, got %d", len(sources))
	}
}

func TestRefreshEPGSourceAsync(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/epg/sources", map[string]any{
		"name": "Refreshable",
		"url":  "http://example.com/test.xml",
	}, env.adminToken)

	var src map[string]any
	decodeBody(resp, &src)
	id := src["id"].(string)

	resp = env.request("POST", "/api/epg/sources/"+id+"/refresh", nil, env.adminToken)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
}

func TestRefreshEPGSourceNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/epg/sources/nonexistent/refresh", nil, env.adminToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCreateChannel(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channels", map[string]any{
		"name":       "BBC One",
		"number":     1,
		"stream_ids": []string{"stream-1"},
	}, env.adminToken)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var ch map[string]any
	decodeBody(resp, &ch)

	if ch["id"] == "" {
		t.Fatal("expected non-empty ID")
	}
	if ch["name"] != "BBC One" {
		t.Errorf("name = %q, want %q", ch["name"], "BBC One")
	}
	if ch["number"] != float64(1) {
		t.Errorf("number = %v, want 1", ch["number"])
	}
}

func TestCreateChannelMissingName(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channels", map[string]any{
		"number": 1,
	}, env.adminToken)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateChannelRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channels", map[string]any{
		"name":   "BBC One",
		"number": 1,
	}, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCreateChannelDuplicateNumber(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	env.request("POST", "/api/channels", map[string]any{
		"name":   "BBC One",
		"number": 1,
	}, env.adminToken)

	resp := env.request("POST", "/api/channels", map[string]any{
		"name":   "BBC Two",
		"number": 1,
	}, env.adminToken)

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestUpdateChannel(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channels", map[string]any{
		"name":   "Original",
		"number": 1,
	}, env.adminToken)

	var ch map[string]any
	decodeBody(resp, &ch)
	id := ch["id"].(string)

	resp = env.request("PUT", "/api/channels/"+id, map[string]any{
		"name": "Updated Channel",
	}, env.adminToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var updated map[string]any
	decodeBody(resp, &updated)
	if updated["name"] != "Updated Channel" {
		t.Errorf("name = %q, want %q", updated["name"], "Updated Channel")
	}
}

func TestUpdateChannelNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("PUT", "/api/channels/nonexistent", map[string]any{
		"name": "Whatever",
	}, env.adminToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateChannelDuplicateNumber(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	env.request("POST", "/api/channels", map[string]any{
		"name":   "Channel One",
		"number": 1,
	}, env.adminToken)

	resp := env.request("POST", "/api/channels", map[string]any{
		"name":   "Channel Two",
		"number": 2,
	}, env.adminToken)

	var ch map[string]any
	decodeBody(resp, &ch)
	id := ch["id"].(string)

	resp = env.request("PUT", "/api/channels/"+id, map[string]any{
		"number": 1,
	}, env.adminToken)

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestDeleteChannel(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channels", map[string]any{
		"name":   "To Delete",
		"number": 1,
	}, env.adminToken)

	var ch map[string]any
	decodeBody(resp, &ch)
	id := ch["id"].(string)

	resp = env.request("DELETE", "/api/channels/"+id, nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	resp = env.request("GET", "/api/channels", nil, env.adminToken)
	var channels []map[string]any
	decodeBody(resp, &channels)
	if len(channels) != 0 {
		t.Fatalf("expected 0 channels after delete, got %d", len(channels))
	}
}

func TestAssignStreams(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channels", map[string]any{
		"name":   "Channel",
		"number": 1,
	}, env.adminToken)

	var ch map[string]any
	decodeBody(resp, &ch)
	id := ch["id"].(string)

	resp = env.request("POST", "/api/channels/"+id+"/streams", map[string]any{
		"stream_ids": []string{"stream-1", "stream-2"},
	}, env.adminToken)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestAssignStreamsNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channels/nonexistent/streams", map[string]any{
		"stream_ids": []string{"stream-1"},
	}, env.adminToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListGroups(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/channel-groups", nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var groups []map[string]any
	decodeBody(resp, &groups)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(groups))
	}
}

func TestCreateGroup(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channel-groups", map[string]any{
		"name": "News",
	}, env.adminToken)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var g map[string]any
	decodeBody(resp, &g)
	if g["name"] != "News" {
		t.Errorf("name = %q, want %q", g["name"], "News")
	}
}

func TestCreateGroupMissingName(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channel-groups", map[string]any{}, env.adminToken)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateGroupRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channel-groups", map[string]any{
		"name": "News",
	}, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestDeleteGroup(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channel-groups", map[string]any{
		"name": "To Delete",
	}, env.adminToken)

	var g map[string]any
	decodeBody(resp, &g)
	id := g["id"].(string)

	resp = env.request("DELETE", "/api/channel-groups/"+id, nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	resp = env.request("GET", "/api/channel-groups", nil, env.adminToken)
	var groups []map[string]any
	decodeBody(resp, &groups)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups after delete, got %d", len(groups))
	}
}

func TestCreateAndListChannels(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	env.request("POST", "/api/channels", map[string]any{
		"name":   "BBC One",
		"number": 1,
	}, env.adminToken)

	env.request("POST", "/api/channels", map[string]any{
		"name":   "BBC Two",
		"number": 2,
	}, env.adminToken)

	resp := env.request("GET", "/api/channels", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var channels []map[string]any
	decodeBody(resp, &channels)
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
}

func TestSourceStreamCount(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	ctx := context.Background()
	env.server.deps.SourceConfigStore.Create(ctx, &sourceconfig.SourceConfig{
		ID:        "src-1",
		Type:      "m3u",
		Name:      "Test M3U",
		IsEnabled: true,
		Config:    map[string]string{"url": "http://example.com/test.m3u"},
	})

	resp := env.request("GET", "/api/sources", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var sources []map[string]any
	decodeBody(resp, &sources)
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	count, ok := sources[0]["stream_count"].(float64)
	if !ok {
		t.Fatalf("stream_count not a number: %v", sources[0]["stream_count"])
	}
	if count != 2 {
		t.Fatalf("expected stream_count=2, got %v", count)
	}
}

func TestRecordingsEmptyReturnsArray(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/recordings", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	resp.Body.Close()

	trimmed := strings.TrimSpace(string(body))
	if trimmed != "[]" {
		t.Fatalf("expected [], got %q", trimmed)
	}
}

func TestScheduledRecordingsEmptyReturnsArray(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/recordings/schedule", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	resp.Body.Close()

	trimmed := strings.TrimSpace(string(body))
	if trimmed != "[]" {
		t.Fatalf("expected [], got %q", trimmed)
	}
}

func TestActivityEmptyReturnsArray(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/activity", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	resp.Body.Close()

	trimmed := strings.TrimSpace(string(body))
	if trimmed != "[]" {
		t.Fatalf("expected [], got %q", trimmed)
	}
}

func TestActivityRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/activity", nil, env.standardToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestChannelGroupsEmptyReturnsArray(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/channel-groups", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	resp.Body.Close()

	trimmed := strings.TrimSpace(string(body))
	if trimmed != "[]" {
		t.Fatalf("expected [], got %q", trimmed)
	}
}

func TestChannelsEmptyReturnsArray(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/channels", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	resp.Body.Close()

	trimmed := strings.TrimSpace(string(body))
	if trimmed != "[]" {
		t.Fatalf("expected [], got %q (should be empty array, not null)", trimmed)
	}
}

func TestSettingsReturnKeyValue(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/settings", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var settings map[string]string
	decodeBody(resp, &settings)

	if _, ok := settings["base_url"]; !ok {
		t.Fatal("expected base_url key in settings")
	}
}

func TestRefreshSourceRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/sources/some-id/refresh", map[string]any{
		"source_type": "m3u",
	}, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestSeekNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/nonexistent/seek", map[string]any{
		"position_ms": 5000,
	}, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	var result map[string]any
	decodeBody(resp, &result)
	if _, ok := result["error"]; !ok {
		t.Fatal("expected error field in response")
	}
}

func TestDeleteGroupRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/channel-groups", map[string]any{
		"name": "Test",
	}, env.adminToken)

	var g map[string]any
	decodeBody(resp, &g)
	id := g["id"].(string)

	resp = env.request("DELETE", "/api/channel-groups/"+id, nil, env.standardToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestScheduleRecording(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	chResp := env.request("POST", "/api/channels", map[string]any{
		"name":       "BBC One",
		"number":     1,
		"stream_ids": []string{"stream-1"},
	}, env.adminToken)

	var ch map[string]any
	decodeBody(chResp, &ch)
	channelID := ch["id"].(string)

	start := time.Now().Add(time.Hour)
	stop := start.Add(2 * time.Hour)

	resp := env.request("POST", "/api/recordings/schedule", map[string]any{
		"channel_id": channelID,
		"title":      "Test Recording",
		"start_at":   start.Format(time.RFC3339),
		"stop_at":    stop.Format(time.RFC3339),
	}, env.standardToken)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var rec map[string]any
	decodeBody(resp, &rec)

	if rec["id"] == "" {
		t.Fatal("expected non-empty recording ID")
	}
	if rec["title"] != "Test Recording" {
		t.Errorf("title = %q, want %q", rec["title"], "Test Recording")
	}
	if rec["status"] != "scheduled" {
		t.Errorf("status = %q, want %q", rec["status"], "scheduled")
	}
}

func TestScheduleRecordingMissingFields(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/recordings/schedule", map[string]any{
		"title": "Missing Channel",
	}, env.standardToken)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestScheduleRecordingStopBeforeStart(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	chResp := env.request("POST", "/api/channels", map[string]any{
		"name":       "BBC One",
		"number":     1,
		"stream_ids": []string{"stream-1"},
	}, env.adminToken)

	var ch map[string]any
	decodeBody(chResp, &ch)
	channelID := ch["id"].(string)

	stop := time.Now().Add(time.Hour)
	start := stop.Add(2 * time.Hour)

	resp := env.request("POST", "/api/recordings/schedule", map[string]any{
		"channel_id": channelID,
		"title":      "Bad Times",
		"start_at":   start.Format(time.RFC3339),
		"stop_at":    stop.Format(time.RFC3339),
	}, env.standardToken)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCancelScheduledRecording(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	chResp := env.request("POST", "/api/channels", map[string]any{
		"name":       "BBC One",
		"number":     1,
		"stream_ids": []string{"stream-1"},
	}, env.adminToken)

	var ch map[string]any
	decodeBody(chResp, &ch)
	channelID := ch["id"].(string)

	start := time.Now().Add(time.Hour)
	stop := start.Add(2 * time.Hour)

	createResp := env.request("POST", "/api/recordings/schedule", map[string]any{
		"channel_id": channelID,
		"title":      "To Cancel",
		"start_at":   start.Format(time.RFC3339),
		"stop_at":    stop.Format(time.RFC3339),
	}, env.standardToken)

	var rec map[string]any
	decodeBody(createResp, &rec)
	recID, ok := rec["id"].(string)
	if !ok || recID == "" {
		t.Fatalf("expected non-empty recording ID, got %v", rec["id"])
	}

	resp := env.request("DELETE", "/api/recordings/schedule/"+recID, nil, env.standardToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestCancelNonexistentScheduledRecording(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/recordings/schedule/nonexistent", nil, env.standardToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestCreateXtreamSource(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/sources/xtream", map[string]any{
		"name":     "Xtream",
		"server":   "http://example.com:8080",
		"username": "user",
		"password": "pass",
	}, env.adminToken)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var sc sourceconfig.SourceConfig
	decodeBody(resp, &sc)

	if sc.Type != "xtream" {
		t.Errorf("type = %q, want %q", sc.Type, "xtream")
	}
	if sc.Config["server"] != "http://example.com:8080" {
		t.Errorf("server = %q, want %q", sc.Config["server"], "http://example.com:8080")
	}
}

func TestCreateXtreamSourceMissingFields(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/sources/xtream", map[string]any{
		"name": "Incomplete",
	}, env.adminToken)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateTVPStreamsSource(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/sources/tvpstreams", map[string]any{
		"name": "TVP Streams",
		"url":  "https://streams.example.com",
	}, env.adminToken)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var sc sourceconfig.SourceConfig
	decodeBody(resp, &sc)

	if sc.Type != "tvpstreams" {
		t.Errorf("type = %q, want %q", sc.Type, "tvpstreams")
	}
}

func TestDeleteXtreamSource(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/sources/xtream", map[string]any{
		"name":     "Delete Me",
		"server":   "http://example.com:8080",
		"username": "user",
		"password": "pass",
	}, env.adminToken)

	var sc sourceconfig.SourceConfig
	decodeBody(resp, &sc)

	resp = env.request("DELETE", "/api/sources/xtream/"+sc.ID, nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestDeleteTVPStreamsSource(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/sources/tvpstreams", map[string]any{
		"name": "Delete Me",
		"url":  "https://streams.example.com",
	}, env.adminToken)

	var sc sourceconfig.SourceConfig
	decodeBody(resp, &sc)

	resp = env.request("DELETE", "/api/sources/tvpstreams/"+sc.ID, nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestErrorResponseFormat(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "wrongpass",
	}, "")

	var result map[string]string
	decodeBody(resp, &result)

	errMsg, ok := result["error"]
	if !ok {
		t.Fatal("error response missing 'error' field")
	}
	if errMsg == "" {
		t.Fatal("error message is empty")
	}
}

func TestStreamsReturnCorrectFields(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/streams", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var streams []map[string]any
	decodeBody(resp, &streams)

	if len(streams) < 1 {
		t.Fatal("expected at least 1 stream")
	}

	s := streams[0]
	if _, ok := s["id"]; !ok {
		t.Error("stream missing 'id' field")
	}
	if _, ok := s["name"]; !ok {
		t.Error("stream missing 'name' field")
	}
	if _, ok := s["url"]; !ok {
		t.Error("stream missing 'url' field")
	}
	if _, ok := s["source_type"]; !ok {
		t.Error("stream missing 'source_type' field")
	}
	if _, ok := s["source_id"]; !ok {
		t.Error("stream missing 'source_id' field")
	}
}

func TestEPGSourcesEmptyReturnsArray(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/epg/sources", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	resp.Body.Close()

	trimmed := strings.TrimSpace(string(body))
	if trimmed != "[]" {
		t.Fatalf("expected [], got %q", trimmed)
	}
}

func TestRefreshTokenValid(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	loginResp := env.request("POST", "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "adminpass",
	}, "")

	var loginResult map[string]any
	decodeBody(loginResp, &loginResult)

	refreshToken, ok := loginResult["refresh_token"].(string)
	if !ok || refreshToken == "" {
		t.Skip("no refresh_token in login response")
	}

	resp := env.request("POST", "/api/auth/refresh", map[string]string{
		"token": refreshToken,
	}, "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	decodeBody(resp, &result)
	if _, ok := result["access_token"]; !ok {
		t.Fatal("response missing access_token")
	}
}

func TestRefreshTokenInvalid(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/auth/refresh", map[string]string{
		"token": "invalid-token",
	}, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRefreshTokenMissing(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/auth/refresh", map[string]string{}, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSourcesRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/sources", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSettingsRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/settings", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRecordingsRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/recordings", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestEPGSourcesRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/epg/sources", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
