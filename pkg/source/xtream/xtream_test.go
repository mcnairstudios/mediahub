package xtream

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
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

var testVODCategories = []Category{
	{ID: "10", Name: "Movies"},
	{ID: "11", Name: "Documentaries"},
}

var testVODStreams = []VODStream{
	{
		Num:          1,
		Name:         "The Matrix (1999)",
		StreamType:   "movie",
		StreamID:     5001,
		StreamIcon:   "https://image.tmdb.org/t/p/w600_and_h900_bestv2/abc.jpg",
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

var testSeriesCategories = []Category{
	{ID: "20", Name: "TV Drama"},
}

var testSeries = []Series{
	{
		Num:        1,
		Name:       "Breaking Bad (2008)",
		SeriesID:   3001,
		Cover:      "http://poster.example.com/bb.jpg",
		CategoryID: "20",
	},
}

var testSeriesInfo = SeriesInfo{
	Seasons: map[string][]SeriesEpisode{
		"1": {
			{
				ID:           "90001",
				EpisodeNum:   1,
				Title:        "Pilot",
				ContainerExt: "mkv",
				Info:         SeriesEpisodeInfo{Season: 1},
			},
			{
				ID:           "90002",
				EpisodeNum:   2,
				Title:        "Cat's in the Bag...",
				ContainerExt: "mkv",
				Info:         SeriesEpisodeInfo{Season: 1},
			},
		},
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
		case "get_vod_categories":
			json.NewEncoder(w).Encode(testVODCategories)
		case "get_vod_streams":
			json.NewEncoder(w).Encode(testVODStreams)
		case "get_series_categories":
			json.NewEncoder(w).Encode(testSeriesCategories)
		case "get_series":
			json.NewEncoder(w).Encode(testSeries)
		case "get_series_info":
			json.NewEncoder(w).Encode(testSeriesInfo)
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

func TestImplementsAccountInfoProvider(t *testing.T) {
	var _ source.AccountInfoProvider = (*Source)(nil)
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
	if len(streams) != 5 {
		t.Fatalf("expected 5 streams (3 live + 2 VOD), got %d", len(streams))
	}

	info := s.Info(context.Background())
	if info.StreamCount != 5 {
		t.Fatalf("expected StreamCount 5, got %d", info.StreamCount)
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
	if len(ids) != 5 {
		t.Fatalf("expected 5 stream IDs (3 live + 2 VOD), got %d", len(ids))
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
	if info.StreamCount != 5 {
		t.Fatalf("expected 5 streams before clear, got %d", info.StreamCount)
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
		case "get_vod_categories", "get_series_categories":
			json.NewEncoder(w).Encode([]Category{})
		case "get_vod_streams":
			json.NewEncoder(w).Encode([]VODStream{})
		case "get_series":
			json.NewEncoder(w).Encode([]Series{})
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

func TestGetAccountInfo(t *testing.T) {
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

	result, err := s.GetAccountInfo(context.Background())
	if err != nil {
		t.Fatalf("get account info failed: %v", err)
	}
	info, ok := result.(*AccountInfo)
	if !ok {
		t.Fatalf("expected *AccountInfo, got %T", result)
	}
	if info.Status != "Active" {
		t.Fatalf("expected status Active, got %s", info.Status)
	}
	if info.MaxConnections != "5" {
		t.Fatalf("expected max connections 5, got %s", info.MaxConnections)
	}
	if info.LiveCategories != 2 {
		t.Fatalf("expected 2 live categories, got %d", info.LiveCategories)
	}
	if info.LiveStreams != 3 {
		t.Fatalf("expected 3 live streams, got %d", info.LiveStreams)
	}
	if info.VODStreams != 2 {
		t.Fatalf("expected 2 VOD streams, got %d", info.VODStreams)
	}
	if info.SeriesCount != 1 {
		t.Fatalf("expected 1 series, got %d", info.SeriesCount)
	}
}

func TestGetAccountInfoAuthFailure(t *testing.T) {
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

	_, err := s.GetAccountInfo(context.Background())
	if err == nil {
		t.Fatal("expected error for auth failure")
	}
}

func TestFetchVODStreams(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	vod, err := fetchVODStreams(context.Background(), ts.Client(), ts.URL, "testuser", "testpass")
	if err != nil {
		t.Fatalf("fetchVODStreams failed: %v", err)
	}
	if len(vod) != 2 {
		t.Fatalf("expected 2 VOD streams, got %d", len(vod))
	}
	if vod[0].Name != "The Matrix (1999)" {
		t.Fatalf("expected The Matrix (1999), got %s", vod[0].Name)
	}
}

func TestFetchSeries(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	series, err := fetchSeries(context.Background(), ts.Client(), ts.URL, "testuser", "testpass")
	if err != nil {
		t.Fatalf("fetchSeries failed: %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(series))
	}
	if series[0].Name != "Breaking Bad (2008)" {
		t.Fatalf("expected Breaking Bad (2008), got %s", series[0].Name)
	}
}

func TestRefreshVODStreams(t *testing.T) {
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
	var movies []media.Stream
	for _, stream := range streams {
		if stream.VODType == "movie" {
			movies = append(movies, stream)
		}
	}
	if len(movies) != 2 {
		t.Fatalf("expected 2 movie streams, got %d", len(movies))
	}

	byName := make(map[string]media.Stream)
	for _, m := range movies {
		byName[m.Name] = m
	}

	matrix, ok := byName["The Matrix"]
	if !ok {
		t.Fatal("expected The Matrix movie stream")
	}
	if matrix.Year != "1999" {
		t.Fatalf("expected year 1999, got %s", matrix.Year)
	}
	if matrix.VODType != "movie" {
		t.Fatalf("expected VODType movie, got %s", matrix.VODType)
	}
	if matrix.Group != "Movies" {
		t.Fatalf("expected group Movies, got %s", matrix.Group)
	}
	expectedURL := ts.URL + "/movie/testuser/testpass/5001.mp4"
	if matrix.URL != expectedURL {
		t.Fatalf("expected URL %s, got %s", expectedURL, matrix.URL)
	}
	if matrix.TvgLogo != "https://image.tmdb.org/t/p/w600_and_h900_bestv2/abc.jpg" {
		t.Fatalf("expected TMDB logo URL, got %s", matrix.TvgLogo)
	}

	inception, ok := byName["Inception"]
	if !ok {
		t.Fatal("expected Inception movie stream")
	}
	if inception.Year != "" {
		t.Fatalf("expected empty year for Inception, got %s", inception.Year)
	}
}

func TestRefreshSeriesEpisodes(t *testing.T) {
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

	time.Sleep(500 * time.Millisecond)

	streams, _ := ss.ListBySource(context.Background(), "xtream", "xt-1")
	var episodes []media.Stream
	for _, stream := range streams {
		if stream.VODType == "series" {
			episodes = append(episodes, stream)
		}
	}
	if len(episodes) != 2 {
		t.Fatalf("expected 2 series episode streams, got %d", len(episodes))
	}

	byName := make(map[string]media.Stream)
	for _, ep := range episodes {
		byName[ep.EpisodeName] = ep
	}

	pilot, ok := byName["Pilot"]
	if !ok {
		t.Fatal("expected Pilot episode")
	}
	if pilot.SeriesName != "Breaking Bad" {
		t.Fatalf("expected SeriesName Breaking Bad, got %s", pilot.SeriesName)
	}
	if pilot.Season != 1 {
		t.Fatalf("expected Season 1, got %d", pilot.Season)
	}
	if pilot.Episode != 1 {
		t.Fatalf("expected Episode 1, got %d", pilot.Episode)
	}
	if pilot.Year != "2008" {
		t.Fatalf("expected year 2008, got %s", pilot.Year)
	}
	if pilot.Name != "Breaking Bad - S01E01 - Pilot" {
		t.Fatalf("expected formatted name, got %s", pilot.Name)
	}
	expectedURL := ts.URL + "/series/testuser/testpass/90001.mkv"
	if pilot.URL != expectedURL {
		t.Fatalf("expected URL %s, got %s", expectedURL, pilot.URL)
	}
}

func TestParseNameAndYear(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantYear string
	}{
		{"The Matrix (1999)", "The Matrix", "1999"},
		{"Breaking Bad (2008)", "Breaking Bad", "2008"},
		{"Inception", "Inception", ""},
		{"No Year Here", "No Year Here", ""},
		{"Title (2026)", "Title", "2026"},
		{"Title (abc)", "Title (abc)", ""},
	}

	for _, tt := range tests {
		name, year := parseNameAndYear(tt.input)
		if name != tt.wantName {
			t.Errorf("parseNameAndYear(%q) name = %q, want %q", tt.input, name, tt.wantName)
		}
		if year != tt.wantYear {
			t.Errorf("parseNameAndYear(%q) year = %q, want %q", tt.input, year, tt.wantYear)
		}
	}
}

func TestSeriesEpisodeURLConstruction(t *testing.T) {
	url := seriesEpisodeURL("http://example.com", "user", "pass", 90001, "mkv")
	expected := "http://example.com/series/user/pass/90001.mkv"
	if url != expected {
		t.Fatalf("expected %s, got %s", expected, url)
	}

	url2 := seriesEpisodeURL("http://example.com", "user", "pass", 90002, "")
	expected2 := "http://example.com/series/user/pass/90002.mkv"
	if url2 != expected2 {
		t.Fatalf("expected %s, got %s", expected2, url2)
	}
}

func TestVODStreamIcon(t *testing.T) {
	vs := VODStream{StreamIcon: "http://poster.example.com/test.jpg"}
	if vs.Icon() != "http://poster.example.com/test.jpg" {
		t.Fatalf("expected string icon, got %s", vs.Icon())
	}

	vs2 := VODStream{StreamIcon: nil}
	if vs2.Icon() != "" {
		t.Fatalf("expected empty icon for nil, got %s", vs2.Icon())
	}

	vs3 := VODStream{StreamIcon: float64(0)}
	if vs3.Icon() != "" {
		t.Fatalf("expected empty icon for non-string, got %s", vs3.Icon())
	}
}

func TestFetchSeriesInfo(t *testing.T) {
	ts := newTestServer(true)
	defer ts.Close()

	info, err := fetchSeriesInfo(context.Background(), ts.Client(), ts.URL, "testuser", "testpass", 3001)
	if err != nil {
		t.Fatalf("fetchSeriesInfo failed: %v", err)
	}
	episodes, ok := info.Seasons["1"]
	if !ok {
		t.Fatal("expected season 1")
	}
	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes in season 1, got %d", len(episodes))
	}
	if episodes[0].Title != "Pilot" {
		t.Fatalf("expected Pilot, got %s", episodes[0].Title)
	}
}

func deterministicStreamIDTest(sourceID string, streamID int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:xtream:%d", sourceID, streamID)))
	return fmt.Sprintf("%x", h[:16])
}
