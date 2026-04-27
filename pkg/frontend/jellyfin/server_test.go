package jellyfin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/media"
)

type mockAuthService struct {
	users []*auth.User
}

func (m *mockAuthService) Login(_ context.Context, username, password string) (string, error) {
	for _, u := range m.users {
		if u.Username == username {
			return "jwt-token-123", nil
		}
	}
	return "", assert.AnError
}

func (m *mockAuthService) ValidateToken(_ context.Context, token string) (*auth.User, error) {
	if len(m.users) > 0 {
		return m.users[0], nil
	}
	return nil, assert.AnError
}

func (m *mockAuthService) RefreshToken(_ context.Context, token string) (string, error) {
	return token, nil
}

func (m *mockAuthService) CreateUser(_ context.Context, username, password string, role auth.Role) (*auth.User, error) {
	return nil, nil
}

func (m *mockAuthService) ListUsers(_ context.Context) ([]*auth.User, error) {
	return m.users, nil
}

func (m *mockAuthService) DeleteUser(_ context.Context, id string) error { return nil }

func (m *mockAuthService) ChangePassword(_ context.Context, id, newPassword string) error {
	return nil
}

type mockChannelStore struct {
	channels []channel.Channel
}

func (m *mockChannelStore) Get(_ context.Context, id string) (*channel.Channel, error) {
	for _, ch := range m.channels {
		if ch.ID == id {
			return &ch, nil
		}
	}
	return nil, nil
}

func (m *mockChannelStore) List(_ context.Context) ([]channel.Channel, error) {
	return m.channels, nil
}

func (m *mockChannelStore) Create(_ context.Context, ch *channel.Channel) error   { return nil }
func (m *mockChannelStore) Update(_ context.Context, ch *channel.Channel) error   { return nil }
func (m *mockChannelStore) Delete(_ context.Context, id string) error             { return nil }
func (m *mockChannelStore) AssignStreams(_ context.Context, _ string, _ []string) error {
	return nil
}
func (m *mockChannelStore) RemoveStreamMappings(_ context.Context, _ []string) error { return nil }

type mockStreamStore struct {
	streams []media.Stream
}

func (m *mockStreamStore) Get(_ context.Context, id string) (*media.Stream, error) {
	for _, s := range m.streams {
		if s.ID == id {
			return &s, nil
		}
	}
	return nil, nil
}
func (m *mockStreamStore) List(_ context.Context) ([]media.Stream, error) { return m.streams, nil }
func (m *mockStreamStore) ListBySource(_ context.Context, _, _ string) ([]media.Stream, error) {
	return nil, nil
}
func (m *mockStreamStore) BulkUpsert(_ context.Context, _ []media.Stream) error { return nil }
func (m *mockStreamStore) DeleteBySource(_ context.Context, _, _ string) error   { return nil }
func (m *mockStreamStore) DeleteStaleBySource(_ context.Context, _, _ string, _ []string) ([]string, error) {
	return nil, nil
}
func (m *mockStreamStore) Save() error { return nil }

func newTestServer() *Server {
	return &Server{
		serverID:   "testserverid1234567890abcdef1234",
		serverName: "Test",
		log:        zerolog.Nop(),
		state:      &persistedState{Tokens: make(map[string]string)},
	}
}

func newTestServerWithAuth() *Server {
	s := newTestServer()
	s.auth = &mockAuthService{
		users: []*auth.User{
			{ID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Username: "admin", IsAdmin: true},
		},
	}
	return s
}

func newTestServerFull() *Server {
	s := newTestServerWithAuth()
	s.channels = &mockChannelStore{
		channels: []channel.Channel{
			{ID: "11111111-2222-3333-4444-555555555555", Name: "BBC One", LogoURL: "https://example.com/bbc.png", TvgID: "bbc1"},
			{ID: "11111111-2222-3333-4444-666666666666", Name: "ITV", TvgID: "itv1"},
		},
	}
	s.streams = &mockStreamStore{
		streams: []media.Stream{
			{ID: "aaaaaaaa-1111-2222-3333-444444444444", Name: "Test Movie", VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080, Duration: 7200},
		},
	}
	return s
}

func TestSystemInfoPublic(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/System/Info/Public", nil)
	w := httptest.NewRecorder()

	srv.systemInfoPublic(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var info PublicSystemInfo
	require.NoError(t, json.NewDecoder(w.Body).Decode(&info))

	assert.Equal(t, "Test", info.ServerName)
	assert.Equal(t, "10.10.6", info.Version)
	assert.Equal(t, "Jellyfin Server", info.ProductName)
	assert.Equal(t, "Linux", info.OperatingSystem)
	assert.True(t, info.StartupWizardCompleted)
	assert.Equal(t, srv.serverID, info.ID)
	assert.Equal(t, 32, len(info.ID))
}

func TestSystemInfoPublicJSON(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/System/Info/Public", nil)
	w := httptest.NewRecorder()

	srv.systemInfoPublic(w, req)

	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var raw map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&raw))

	requiredKeys := []string{"LocalAddress", "ServerName", "Version", "ProductName", "OperatingSystem", "Id", "StartupWizardCompleted"}
	for _, k := range requiredKeys {
		_, ok := raw[k]
		assert.True(t, ok, "missing key %s", k)
	}
}

func TestSystemInfo(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/System/Info", nil)
	w := httptest.NewRecorder()

	srv.systemInfo(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var info SystemInfo
	require.NoError(t, json.NewDecoder(w.Body).Decode(&info))

	assert.Equal(t, 8096, info.WebSocketPortNumber)
	assert.True(t, info.SupportsLibraryMonitor)
	assert.True(t, info.CanSelfRestart)
}

func TestPing(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/System/Ping", nil)
	w := httptest.NewRecorder()

	srv.ping(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Jellyfin Server", w.Body.String())
}

func TestBrandingConfig(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/Branding/Configuration", nil)
	w := httptest.NewRecorder()

	srv.brandingConfig(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var cfg BrandingConfiguration
	require.NoError(t, json.NewDecoder(w.Body).Decode(&cfg))
	assert.False(t, cfg.SplashscreenEnabled)
}

func TestAuthenticateByName(t *testing.T) {
	srv := newTestServerWithAuth()
	body := `{"Username":"admin","Pw":"password"}`
	req := httptest.NewRequest(http.MethodPost, "/Users/AuthenticateByName", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.authenticateByName(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result AuthenticationResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	assert.NotEmpty(t, result.AccessToken)
	assert.Equal(t, 32, len(result.AccessToken))
	assert.Equal(t, srv.serverID, result.ServerID)
	assert.NotNil(t, result.User)
	assert.Equal(t, "admin", result.User.Name)
	assert.Equal(t, 32, len(result.User.ID))
	assert.True(t, result.User.Policy.IsAdministrator)
	assert.NotNil(t, result.SessionInfo)
}

func TestAuthenticateByNameInvalid(t *testing.T) {
	srv := newTestServerWithAuth()
	body := `{"Username":"nonexistent","Pw":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/Users/AuthenticateByName", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	srv.authenticateByName(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTokenExtraction(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		name     string
		setup    func(*http.Request)
		expected string
	}{
		{
			name: "X-MediaBrowser-Token header",
			setup: func(r *http.Request) {
				r.Header.Set("X-MediaBrowser-Token", "abc123")
			},
			expected: "abc123",
		},
		{
			name: "X-Emby-Token header",
			setup: func(r *http.Request) {
				r.Header.Set("X-Emby-Token", "emby456")
			},
			expected: "emby456",
		},
		{
			name: "api_key query param",
			setup: func(r *http.Request) {
				q := r.URL.Query()
				q.Set("api_key", "query789")
				r.URL.RawQuery = q.Encode()
			},
			expected: "query789",
		},
		{
			name: "Authorization header with Token",
			setup: func(r *http.Request) {
				r.Header.Set("Authorization", `MediaBrowser Token="headertoken"`)
			},
			expected: "headertoken",
		},
		{
			name: "X-Emby-Authorization header with Token",
			setup: func(r *http.Request) {
				r.Header.Set("X-Emby-Authorization", `MediaBrowser Client="Test", Token="embyauth"`)
			},
			expected: "embyauth",
		},
		{
			name:     "no token",
			setup:    func(r *http.Request) {},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			tt.setup(req)
			token := srv.extractToken(req)
			assert.Equal(t, tt.expected, token)
		})
	}
}

func TestUserViews(t *testing.T) {
	srv := newTestServerFull()
	req := httptest.NewRequest(http.MethodGet, "/UserViews", nil)
	w := httptest.NewRecorder()

	srv.userViews(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result BaseItemDtoQueryResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	assert.Equal(t, 2, result.TotalRecordCount)
	assert.Equal(t, 2, len(result.Items))

	assert.Equal(t, "Movies", result.Items[0].Name)
	assert.Equal(t, "CollectionFolder", result.Items[0].Type)
	assert.Equal(t, "movies", result.Items[0].CollectionType)
	assert.True(t, result.Items[0].IsFolder)

	assert.Equal(t, "TV Shows", result.Items[1].Name)
	assert.Equal(t, "CollectionFolder", result.Items[1].Type)
	assert.Equal(t, "tvshows", result.Items[1].CollectionType)
}

func TestUserViewsIDsAre32CharHex(t *testing.T) {
	srv := newTestServerFull()
	req := httptest.NewRequest(http.MethodGet, "/UserViews", nil)
	w := httptest.NewRecorder()

	srv.userViews(w, req)

	var result BaseItemDtoQueryResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	for _, item := range result.Items {
		assert.Equal(t, 32, len(item.ID), "ID %q should be 32 chars", item.ID)
		assert.Equal(t, 32, len(item.ServerID), "ServerID should be 32 chars")
	}
}

func TestLiveTvChannels(t *testing.T) {
	srv := newTestServerFull()
	req := httptest.NewRequest(http.MethodGet, "/LiveTv/Channels", nil)
	w := httptest.NewRecorder()

	srv.liveTvChannels(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result BaseItemDtoQueryResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	assert.Equal(t, 2, result.TotalRecordCount)
	assert.Equal(t, "BBC One", result.Items[0].Name)
	assert.Equal(t, "LiveTvChannel", result.Items[0].Type)
	assert.Equal(t, "1", result.Items[0].ChannelNumber)
	assert.Equal(t, "logo", result.Items[0].ChannelPrimaryImageTag)

	assert.Equal(t, "ITV", result.Items[1].Name)
	assert.Equal(t, "", result.Items[1].ChannelPrimaryImageTag)
}

func TestItemDetail(t *testing.T) {
	srv := newTestServerFull()
	itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
	req := httptest.NewRequest(http.MethodGet, "/Items/"+itemID, nil)
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.itemDetail(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var item BaseItemDto
	require.NoError(t, json.NewDecoder(w.Body).Decode(&item))

	assert.Equal(t, "Test Movie", item.Name)
	assert.Equal(t, "Movie", item.Type)
	assert.Equal(t, "Video", item.MediaType)
	assert.Equal(t, 1920, item.Width)
	assert.Equal(t, 1080, item.Height)
	assert.NotZero(t, item.RunTimeTicks)
}

func TestPlaybackInfo(t *testing.T) {
	srv := newTestServerFull()
	itemID := "aaaaaaaa11112222333344444444"
	req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.playbackInfo(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	assert.NotEmpty(t, result["PlaySessionId"])
	sources, ok := result["MediaSources"].([]any)
	require.True(t, ok)
	assert.Len(t, sources, 1)

	ms := sources[0].(map[string]any)
	assert.Equal(t, "hls", ms["TranscodingSubProtocol"])
	assert.Contains(t, ms["TranscodingUrl"], "master.m3u8")
}

func TestQuickConnectDisabled(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/QuickConnect/Enabled", nil)
	w := httptest.NewRecorder()

	srv.quickConnectEnabled(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "false", strings.TrimSpace(w.Body.String()))
}

func TestEmptyResult(t *testing.T) {
	result := emptyResult()
	assert.NotNil(t, result.Items)
	assert.Equal(t, 0, len(result.Items))
	assert.Equal(t, 0, result.TotalRecordCount)
}

func TestListItemsEmpty(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/Items?parentId="+viewMoviesID, nil)
	w := httptest.NewRecorder()

	srv.listItems(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result BaseItemDtoQueryResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, 0, result.TotalRecordCount)
}

func TestListItemsWithStreams(t *testing.T) {
	srv := newTestServerFull()
	req := httptest.NewRequest(http.MethodGet, "/Items?parentId="+viewMoviesID, nil)
	w := httptest.NewRecorder()

	srv.listItems(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result BaseItemDtoQueryResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, 1, result.TotalRecordCount)
	assert.Equal(t, "Test Movie", result.Items[0].Name)
}

func TestRequireAuthMiddleware(t *testing.T) {
	srv := newTestServerWithAuth()

	called := false
	handler := srv.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("no token returns 401", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.False(t, called)
	})

	t.Run("unknown token auto-registers", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-MediaBrowser-Token", "unknown-token-value")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, called)

		if v, ok := srv.tokens.Load("unknown-token-value"); ok {
			assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", v.(string))
		} else {
			t.Fatal("token was not auto-registered")
		}
	})

	t.Run("known token passes through", func(t *testing.T) {
		called = false
		srv.tokens.Store("known-token", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-MediaBrowser-Token", "known-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, called)
	})
}

func TestHandlerRouting(t *testing.T) {
	srv := newTestServerFull()
	handler := srv.Handler()

	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/System/Info/Public", http.StatusOK},
		{"GET", "/System/Info", http.StatusOK},
		{"GET", "/System/Ping", http.StatusOK},
		{"POST", "/System/Ping", http.StatusOK},
		{"GET", "/Branding/Configuration", http.StatusOK},
		{"GET", "/QuickConnect/Enabled", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, tt.status, w.Code)
		})
	}
}

func TestServerIDIs32CharHex(t *testing.T) {
	srv := NewServer(ServerDeps{
		ServerName: "Test",
		StateDir:   t.TempDir(),
		Auth:       &mockAuthService{},
		Log:        zerolog.Nop(),
	})

	assert.Equal(t, 32, len(srv.serverID))

	for _, c := range srv.serverID {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'), "invalid hex char %c", c)
	}
}
