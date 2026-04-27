package xtream

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

var authResponse = AuthResponse{
	UserInfo: UserInfo{
		Username:       "testuser",
		Password:       "testpass",
		Status:         "Active",
		MaxConnections: "5",
	},
	ServerInfo: ServerInfo{
		URL:            "http://example.com",
		Port:           "80",
		ServerProtocol: "http",
	},
}

var testCategories = []Category{
	{ID: "1", Name: "UK Entertainment"},
	{ID: "2", Name: "UK Sports"},
}

var testLiveStreams = []LiveStream{
	{
		Num:          1,
		Name:         "BBC One",
		StreamType:   "live",
		StreamID:     1001,
		StreamIcon:   "http://logo.example.com/bbc1.png",
		EPGChannelID: "bbc1.uk",
		CategoryID:   "1",
	},
	{
		Num:          2,
		Name:         "Sky Sports",
		StreamType:   "live",
		StreamID:     1002,
		StreamIcon:   "http://logo.example.com/sky.png",
		EPGChannelID: "sky.uk",
		CategoryID:   "2",
	},
	{
		Num:          3,
		Name:         "ITV",
		StreamType:   "live",
		StreamID:     1003,
		StreamIcon:   nil,
		EPGChannelID: "itv.uk",
		CategoryID:   "1",
	},
}

var testVODStreams = []VODStream{
	{
		Num:          1,
		Name:         "The Matrix",
		StreamType:   "movie",
		StreamID:     5001,
		StreamIcon:   "http://poster.example.com/matrix.jpg",
		CategoryID:   "10",
		ContainerExt: "mp4",
	},
	{
		Num:          2,
		Name:         "Inception",
		StreamType:   "movie",
		StreamID:     5002,
		StreamIcon:   "http://poster.example.com/inception.jpg",
		CategoryID:   "10",
		ContainerExt: "mkv",
	},
}

var testSeries = []Series{
	{
		Num:        1,
		Name:       "Breaking Bad",
		SeriesID:   3001,
		Cover:      "http://poster.example.com/bb.jpg",
		CategoryID: "20",
	},
}

func newTestServer(authOK bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		username := r.URL.Query().Get("username")
		password := r.URL.Query().Get("password")

		if username != "testuser" || password != "testpass" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{"user_info": map[string]any{"auth": 0}})
			return
		}

		if !authOK {
			resp := AuthResponse{
				UserInfo: UserInfo{Status: "Disabled"},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		switch action {
		case "":
			json.NewEncoder(w).Encode(authResponse)
		case "get_live_categories":
			json.NewEncoder(w).Encode(testCategories)
		case "get_live_streams":
			json.NewEncoder(w).Encode(testLiveStreams)
		case "get_vod_streams":
			json.NewEncoder(w).Encode(testVODStreams)
		case "get_series":
			json.NewEncoder(w).Encode(testSeries)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
}

func TestImplementsSource(t *testing.T) {
	var _ source.Source = (*Source)(nil)
}

func TestImplementsVPNRoutable(t *testing.T) {
	var _ source.VPNRoutable = (*Source)(nil)
}

func TestImplementsVODProvider(t *testing.T) {
	var _ source.VODProvider = (*Source)(nil)
}

func TestImplementsClearable(t *testing.T) {
	var _ source.Clearable = (*Source)(nil)
}

func TestType(t *testing.T) {
	s := New(Config{
		ID:          "test-src",
		Name:        "Test",
		Server:      "http://example.com",
		Username:    "u",
		Password:    "p",
		StreamStore: store.NewMemoryStreamStore(),
	})
	if s.Type() != "xtream" {
		t.Fatalf("expected type xtream, got %s", s.Type())
	}
}

func TestInfo(t *testing.T) {
	s := New(Config{
		ID:          "xt-1",
		Name:        "My Xtream",
		Server:      "http://example.com",
		Username:    "u",
		Password:    "p",
		IsEnabled:   true,
		MaxStreams:  3,
		StreamStore: store.NewMemoryStreamStore(),
	})

	info := s.Info(context.Background())
	if info.ID != "xt-1" {
		t.Fatalf("expected ID xt-1, got %s", info.ID)
	}
	if info.Name != "My Xtream" {
		t.Fatalf("expected Name My Xtream, got %s", info.Name)
	}
	if info.Type != "xtream" {
		t.Fatalf("expected Type xtream, got %s", info.Type)
	}
	if !info.IsEnabled {
		t.Fatal("expected IsEnabled true")
	}
	if info.MaxConcurrentStreams != 3 {
		t.Fatalf("expected MaxConcurrentStreams 3, got %d", info.MaxConcurrentStreams)
	}
	if info.StreamCount != 0 {
		t.Fatalf("expected StreamCount 0, got %d", info.StreamCount)
	}
}

func TestAuthFailure(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "wrong",
		Password:    "wrong",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected auth error")
	}

	info := s.Info(context.Background())
	if info.LastError == "" {
		t.Fatal("expected LastError to be set after auth failure")
	}
}

func TestAuthDisabledAccount(t *testing.T) {
	ts := newTestServer(false)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error for disabled account")
	}
}

func TestRefreshLiveStreams(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, err := ss.ListBySource(context.Background(), "xtream", "xt-1")
	if err != nil {
		t.Fatalf("listing streams: %v", err)
	}
	if len(streams) != 3 {
		t.Fatalf("expected 3 live streams, got %d", len(streams))
	}

	info := s.Info(context.Background())
	if info.StreamCount != 3 {
		t.Fatalf("expected StreamCount 3, got %d", info.StreamCount)
	}
	if info.LastRefreshed == nil {
		t.Fatal("expected LastRefreshed to be set")
	}
	if info.LastError != "" {
		t.Fatalf("expected no error, got %s", info.LastError)
	}
}

func TestRefreshStreamContent(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "xtream", "xt-1")
	byName := make(map[string]struct{})
	for _, stream := range streams {
		byName[stream.Name] = struct{}{}
		if stream.SourceType != "xtream" {
			t.Fatalf("expected SourceType xtream, got %s", stream.SourceType)
		}
		if stream.SourceID != "xt-1" {
			t.Fatalf("expected SourceID xt-1, got %s", stream.SourceID)
		}
		if stream.URL == "" {
			t.Fatalf("expected URL for %s", stream.Name)
		}
		if !stream.IsActive {
			t.Fatalf("expected IsActive for %s", stream.Name)
		}
	}

	for _, name := range []string{"BBC One", "Sky Sports", "ITV"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("expected stream %s not found", name)
		}
	}
}

func TestCategoryMapping(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "xtream", "xt-1")
	groups := make(map[string]string)
	for _, stream := range streams {
		groups[stream.Name] = stream.Group
	}

	if groups["BBC One"] != "UK Entertainment" {
		t.Fatalf("expected BBC One group UK Entertainment, got %s", groups["BBC One"])
	}
	if groups["Sky Sports"] != "UK Sports" {
		t.Fatalf("expected Sky Sports group UK Sports, got %s", groups["Sky Sports"])
	}
}

func TestStreamURLConstruction(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "xtream", "xt-1")
	for _, stream := range streams {
		if stream.Name == "BBC One" {
			expected := ts.URL + "/testuser/testpass/1001"
			if stream.URL != expected {
				t.Fatalf("expected URL %s, got %s", expected, stream.URL)
			}
		}
	}
}

func TestStreamIconParsing(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "xtream", "xt-1")
	logos := make(map[string]string)
	for _, stream := range streams {
		logos[stream.Name] = stream.TvgLogo
	}

	if logos["BBC One"] != "http://logo.example.com/bbc1.png" {
		t.Fatalf("expected BBC One logo, got %s", logos["BBC One"])
	}
	if logos["ITV"] != "" {
		t.Fatalf("expected empty logo for ITV (nil icon), got %s", logos["ITV"])
	}
}

func TestEPGChannelIDMapping(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "xtream", "xt-1")
	tvgIDs := make(map[string]string)
	for _, stream := range streams {
		tvgIDs[stream.Name] = stream.TvgID
	}

	if tvgIDs["BBC One"] != "bbc1.uk" {
		t.Fatalf("expected bbc1.uk, got %s", tvgIDs["BBC One"])
	}
}

func TestStreams(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	ids, err := s.Streams(context.Background())
	if err != nil {
		t.Fatalf("streams failed: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 stream IDs, got %d", len(ids))
	}
}

func TestDeleteStreams(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	if err := s.DeleteStreams(context.Background()); err != nil {
		t.Fatalf("delete streams failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "xtream", "xt-1")
	if len(streams) != 0 {
		t.Fatalf("expected 0 streams after delete, got %d", len(streams))
	}
}

func TestClear(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	info := s.Info(context.Background())
	if info.StreamCount != 3 {
		t.Fatalf("expected 3 streams before clear, got %d", info.StreamCount)
	}

	if err := s.Clear(context.Background()); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "xtream", "xt-1")
	if len(streams) != 0 {
		t.Fatalf("expected 0 streams after clear, got %d", len(streams))
	}

	info = s.Info(context.Background())
	if info.StreamCount != 0 {
		t.Fatalf("expected StreamCount 0 after clear, got %d", info.StreamCount)
	}
}

func TestUsesVPN(t *testing.T) {
	s := New(Config{
		ID:           "xt-1",
		Name:         "Test",
		Server:       "http://example.com",
		Username:     "u",
		Password:     "p",
		UseWireGuard: true,
		StreamStore:  store.NewMemoryStreamStore(),
	})
	if !s.UsesVPN() {
		t.Fatal("expected UsesVPN true")
	}

	s2 := New(Config{
		ID:          "xt-2",
		Name:        "Test2",
		Server:      "http://example.com",
		Username:    "u",
		Password:    "p",
		StreamStore: store.NewMemoryStreamStore(),
	})
	if s2.UsesVPN() {
		t.Fatal("expected UsesVPN false")
	}
}

func TestSupportsVOD(t *testing.T) {
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      "http://example.com",
		Username:    "u",
		Password:    "p",
		StreamStore: store.NewMemoryStreamStore(),
	})
	if !s.SupportsVOD() {
		t.Fatal("expected SupportsVOD true")
	}
	types := s.VODTypes()
	if len(types) != 2 {
		t.Fatalf("expected 2 VOD types, got %d", len(types))
	}
	typeSet := make(map[string]struct{})
	for _, tp := range types {
		typeSet[tp] = struct{}{}
	}
	if _, ok := typeSet["movie"]; !ok {
		t.Fatal("expected movie VOD type")
	}
	if _, ok := typeSet["series"]; !ok {
		t.Fatal("expected series VOD type")
	}
}

func TestDeterministicStreamID(t *testing.T) {
	sourceID := "xt-1"
	streamID := 1001
	id1 := deterministicStreamID(sourceID, streamID)
	id2 := deterministicStreamID(sourceID, streamID)
	if id1 != id2 {
		t.Fatalf("expected deterministic IDs, got %s and %s", id1, id2)
	}
	if id1 == "" {
		t.Fatal("expected non-empty stream ID")
	}

	id3 := deterministicStreamID("other-src", streamID)
	if id1 == id3 {
		t.Fatal("expected different IDs for different source IDs")
	}

	id4 := deterministicStreamID(sourceID, 9999)
	if id1 == id4 {
		t.Fatal("expected different IDs for different stream IDs")
	}
}

func TestStaleStreamsRemoved(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")

		switch action {
		case "":
			json.NewEncoder(w).Encode(authResponse)
		case "get_live_categories":
			json.NewEncoder(w).Encode(testCategories)
		case "get_live_streams":
			callCount++
			if callCount == 1 {
				json.NewEncoder(w).Encode(testLiveStreams)
			} else {
				json.NewEncoder(w).Encode(testLiveStreams[:1])
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())
	streams, _ := ss.ListBySource(context.Background(), "xtream", "xt-1")
	if len(streams) != 3 {
		t.Fatalf("expected 3 streams, got %d", len(streams))
	}

	_ = s.Refresh(context.Background())
	streams, _ = ss.ListBySource(context.Background(), "xtream", "xt-1")
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream after stale removal, got %d", len(streams))
	}
}

func TestVODStreamURLConstruction(t *testing.T) {
	server := "http://example.com"
	url := vodStreamURL(server, "testuser", "testpass", 5001, "mp4")
	expected := "http://example.com/movie/testuser/testpass/5001.mp4"
	if url != expected {
		t.Fatalf("expected %s, got %s", expected, url)
	}

	url2 := vodStreamURL(server, "testuser", "testpass", 5002, "")
	expected2 := "http://example.com/movie/testuser/testpass/5002.mp4"
	if url2 != expected2 {
		t.Fatalf("expected %s, got %s", expected2, url2)
	}
}

func TestSeriesStreamURLConstruction(t *testing.T) {
	server := "http://example.com"
	url := seriesStreamURL(server, "testuser", "testpass", 3001)
	expected := "http://example.com/series/testuser/testpass/3001"
	if url != expected {
		t.Fatalf("expected %s, got %s", expected, url)
	}
}

func TestLiveStreamIconStringParsing(t *testing.T) {
	ls := LiveStream{StreamIcon: "http://logo.example.com/test.png"}
	if ls.Icon() != "http://logo.example.com/test.png" {
		t.Fatalf("expected string icon, got %s", ls.Icon())
	}

	ls2 := LiveStream{StreamIcon: nil}
	if ls2.Icon() != "" {
		t.Fatalf("expected empty icon for nil, got %s", ls2.Icon())
	}

	ls3 := LiveStream{StreamIcon: float64(0)}
	if ls3.Icon() != "" {
		t.Fatalf("expected empty icon for non-string, got %s", ls3.Icon())
	}
}

func TestRefreshHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "xt-1",
		Name:        "Test",
		Server:      ts.URL,
		Username:    "testuser",
		Password:    "testpass",
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func deterministicStreamIDTest(sourceID string, streamID int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:xtream:%d", sourceID, streamID)))
	return fmt.Sprintf("%x", h[:16])
}
