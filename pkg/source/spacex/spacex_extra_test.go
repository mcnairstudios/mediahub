package spacex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(past, upcoming ll2Response) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/launch/previous/"):
			json.NewEncoder(w).Encode(past)
		case strings.Contains(r.URL.Path, "/launch/upcoming/"):
			json.NewEncoder(w).Encode(upcoming)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func emptyResponse() ll2Response {
	return ll2Response{Count: 0, Results: []ll2Launch{}}
}

func TestRefresh_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-space",
		Name:        "Space",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error for API failure")
	}

	info := s.Info(context.Background())
	if info.LastError == "" {
		t.Error("expected LastError to be set")
	}
}

func TestRefresh_EmptyLaunches(t *testing.T) {
	srv := newTestServer(emptyResponse(), emptyResponse())
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-space",
		Name:        "Space",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if len(ss.upserted) != 0 {
		t.Errorf("expected 0 upserted, got %d", len(ss.upserted))
	}

	info := s.Info(context.Background())
	if info.StreamCount != 0 {
		t.Errorf("expected StreamCount=0, got %d", info.StreamCount)
	}
}

func TestRefresh_LaunchImage(t *testing.T) {
	past := ll2Response{
		Count: 1,
		Results: []ll2Launch{
			{
				ID:    "uuid-1",
				Name:  "Crew-5",
				Net:   "2022-10-05T16:00:00Z",
				Image: "https://img.example.com/launch.jpg",
				LaunchServiceProvider: &ll2LSP{Name: "SpaceX"},
				VidURLs: []ll2VidURL{
					{Priority: 0, URL: "https://youtube.com/watch?v=abc"},
				},
			},
		},
	}

	srv := newTestServer(past, emptyResponse())
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-space",
		Name:        "Space",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 1 {
		t.Fatalf("expected 1 upserted, got %d", len(ss.upserted))
	}
	if ss.upserted[0].TvgLogo != "https://img.example.com/launch.jpg" {
		t.Errorf("expected launch image as logo, got %q", ss.upserted[0].TvgLogo)
	}
}

func TestRefresh_DuplicateLaunchIDs(t *testing.T) {
	past := ll2Response{
		Count: 2,
		Results: []ll2Launch{
			{
				ID:      "same-uuid",
				Name:    "Launch A",
				Net:     "2022-10-05T16:00:00Z",
				LSPName: "SpaceX",
				VidURLs: []ll2VidURL{
					{Priority: 0, URL: "https://youtube.com/watch?v=abc"},
				},
			},
			{
				ID:      "same-uuid",
				Name:    "Launch B (duplicate)",
				Net:     "2022-10-05T16:00:00Z",
				LSPName: "SpaceX",
				VidURLs: []ll2VidURL{
					{Priority: 0, URL: "https://youtube.com/watch?v=def"},
				},
			},
		},
	}

	srv := newTestServer(past, emptyResponse())
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-space",
		Name:        "Space",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 1 {
		t.Fatalf("expected 1 upserted (deduped), got %d", len(ss.upserted))
	}
	if ss.upserted[0].Name != "Launch A (Oct 5, 2022)" {
		t.Errorf("expected first launch to win, got %q", ss.upserted[0].Name)
	}
}

func TestRefresh_NoDateSkipsYear(t *testing.T) {
	past := ll2Response{
		Count: 1,
		Results: []ll2Launch{
			{
				ID:      "uuid-nodate",
				Name:    "No Date",
				Net:     "",
				LSPName: "SpaceX",
				VidURLs: []ll2VidURL{
					{Priority: 0, URL: "https://youtube.com/watch?v=abc"},
				},
			},
		},
	}

	srv := newTestServer(past, emptyResponse())
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-space",
		Name:        "Space",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 1 {
		t.Fatalf("expected 1 upserted, got %d", len(ss.upserted))
	}
	if ss.upserted[0].Year != "" {
		t.Errorf("expected empty year for no date, got %q", ss.upserted[0].Year)
	}
}

func TestRefresh_StatusAsTags(t *testing.T) {
	past := ll2Response{
		Count: 1,
		Results: []ll2Launch{
			{
				ID:      "uuid-fail",
				Name:    "Failed Launch",
				Net:     "2023-01-01T00:00:00Z",
				Status:  ll2Status{ID: 4, Abbrev: "Failure"},
				LSPName: "NewCo",
				VidURLs: []ll2VidURL{
					{Priority: 0, URL: "https://youtube.com/watch?v=abc"},
				},
			},
		},
	}

	srv := newTestServer(past, emptyResponse())
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-space",
		Name:        "Space",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 1 {
		t.Fatalf("expected 1 upserted, got %d", len(ss.upserted))
	}
	found := false
	for _, tag := range ss.upserted[0].Tags {
		if tag == "failure" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'failure' tag, got %v", ss.upserted[0].Tags)
	}
}

func TestRefresh_ProviderAsGroup(t *testing.T) {
	past := ll2Response{
		Count: 2,
		Results: []ll2Launch{
			{
				ID:      "uuid-rl",
				Name:    "Electron | Kineis",
				Net:     "2024-06-20T00:00:00Z",
				LSPName: "Rocket Lab",
				VidURLs: []ll2VidURL{
					{Priority: 0, URL: "https://youtube.com/watch?v=rl1"},
				},
			},
			{
				ID:   "uuid-ula",
				Name: "Atlas V | USSF-12",
				Net:  "2024-06-25T00:00:00Z",
				LaunchServiceProvider: &ll2LSP{Name: "United Launch Alliance"},
				VidURLs: []ll2VidURL{
					{Priority: 0, URL: "https://youtube.com/watch?v=ula1"},
				},
			},
		},
	}

	srv := newTestServer(past, emptyResponse())
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-space",
		Name:        "Space",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 2 {
		t.Fatalf("expected 2 upserted, got %d", len(ss.upserted))
	}

	groups := map[string]bool{}
	for _, st := range ss.upserted {
		groups[st.Group] = true
	}
	if !groups["Rocket Lab"] {
		t.Error("expected a stream with group 'Rocket Lab'")
	}
	if !groups["United Launch Alliance"] {
		t.Error("expected a stream with group 'United Launch Alliance'")
	}
}

func TestRefresh_Pagination(t *testing.T) {
	page1 := ll2Response{
		Count: 2,
		Results: []ll2Launch{
			{
				ID:      "uuid-p1",
				Name:    "Launch Page 1",
				Net:     "2024-01-01T00:00:00Z",
				LSPName: "SpaceX",
				VidURLs: []ll2VidURL{
					{Priority: 0, URL: "https://youtube.com/watch?v=p1"},
				},
			},
		},
	}
	page2 := ll2Response{
		Count: 2,
		Results: []ll2Launch{
			{
				ID:      "uuid-p2",
				Name:    "Launch Page 2",
				Net:     "2024-01-02T00:00:00Z",
				LSPName: "Rocket Lab",
				VidURLs: []ll2VidURL{
					{Priority: 0, URL: "https://youtube.com/watch?v=p2"},
				},
			},
		},
	}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/launch/previous/"):
			callCount++
			if strings.Contains(r.URL.RawQuery, "offset=1") {
				json.NewEncoder(w).Encode(page2)
			} else {
				// First page: set next URL to page 2.
				next := "https://ll.thespacedevs.com/2.2.0/launch/previous/?mode=detailed&limit=1&offset=1"
				page1Copy := page1
				page1Copy.Next = &next
				json.NewEncoder(w).Encode(page1Copy)
			}
		case strings.Contains(r.URL.Path, "/launch/upcoming/"):
			json.NewEncoder(w).Encode(emptyResponse())
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-space",
		Name:        "Space",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 2 {
		t.Fatalf("expected 2 upserted (from 2 pages), got %d", len(ss.upserted))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestRefresh_UpcomingWithoutVideo(t *testing.T) {
	upcoming := ll2Response{
		Count: 1,
		Results: []ll2Launch{
			{
				ID:      "uuid-up1",
				Name:    "Starship | Flight 10",
				Net:     "2025-12-01T00:00:00Z",
				Status:  ll2Status{ID: 1, Abbrev: "Go"},
				LSPName: "SpaceX",
				Image:   "https://img.example.com/starship.jpg",
			},
		},
	}

	srv := newTestServer(emptyResponse(), upcoming)
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-space",
		Name:        "Space",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 1 {
		t.Fatalf("expected 1 upserted, got %d", len(ss.upserted))
	}
	st := ss.upserted[0]
	if st.URL != "" {
		t.Errorf("expected empty URL for upcoming launch, got %s", st.URL)
	}
	if st.Name != "Starship | Flight 10 (Dec 1, 2025)" {
		t.Errorf("unexpected name: %s", st.Name)
	}
	if st.TvgLogo != "https://img.example.com/starship.jpg" {
		t.Errorf("expected launch image, got %s", st.TvgLogo)
	}

	foundGo := false
	for _, tag := range st.Tags {
		if tag == "go" {
			foundGo = true
		}
	}
	if !foundGo {
		t.Errorf("expected 'go' tag for upcoming launch, got %v", st.Tags)
	}
}

func TestDeterministicStreamID_Stable(t *testing.T) {
	id1 := deterministicStreamID("src-1", "launch-abc")
	id2 := deterministicStreamID("src-1", "launch-abc")
	if id1 != id2 {
		t.Error("same inputs should produce the same ID")
	}
}

func TestDeterministicStreamID_DifferentSource(t *testing.T) {
	id1 := deterministicStreamID("src-1", "launch-abc")
	id2 := deterministicStreamID("src-2", "launch-abc")
	if id1 == id2 {
		t.Error("different source IDs should produce different stream IDs")
	}
}

func TestNewDefaultHTTPClient(t *testing.T) {
	s := New(Config{ID: "x", Name: "X", IsEnabled: true})
	if s.cfg.HTTPClient == nil {
		t.Fatal("expected default HTTP client to be set")
	}
}

func TestBestVideoURL(t *testing.T) {
	l := ll2Launch{
		VidURLs: []ll2VidURL{
			{Priority: 10, URL: "https://youtube.com/watch?v=low"},
			{Priority: 1, URL: "https://youtube.com/watch?v=high"},
			{Priority: 5, URL: "https://youtube.com/watch?v=mid"},
		},
	}
	if got := l.bestVideoURL(); got != "https://youtube.com/watch?v=high" {
		t.Errorf("expected highest priority (lowest number) URL, got %s", got)
	}
}

func TestBestVideoURL_Empty(t *testing.T) {
	l := ll2Launch{}
	if got := l.bestVideoURL(); got != "" {
		t.Errorf("expected empty URL for no videos, got %s", got)
	}
}

func TestProviderName_LSPName(t *testing.T) {
	l := ll2Launch{LSPName: "Rocket Lab"}
	if got := l.providerName(); got != "Rocket Lab" {
		t.Errorf("expected 'Rocket Lab', got %s", got)
	}
}

func TestProviderName_DetailedMode(t *testing.T) {
	l := ll2Launch{LaunchServiceProvider: &ll2LSP{Name: "ULA"}}
	if got := l.providerName(); got != "ULA" {
		t.Errorf("expected 'ULA', got %s", got)
	}
}

func TestProviderName_Unknown(t *testing.T) {
	l := ll2Launch{}
	if got := l.providerName(); got != "Unknown" {
		t.Errorf("expected 'Unknown', got %s", got)
	}
}

func TestMissionDescription(t *testing.T) {
	l := ll2Launch{
		Mission: map[string]any{
			"description": "A test mission.",
		},
	}
	if got := l.missionDescription(); got != "A test mission." {
		t.Errorf("expected description, got %q", got)
	}
}

func TestMissionDescription_StringMission(t *testing.T) {
	l := ll2Launch{Mission: "Starlink Group 10-38"}
	if got := l.missionDescription(); got != "" {
		t.Errorf("expected empty for string mission, got %q", got)
	}
}

func TestMissionDescription_Nil(t *testing.T) {
	l := ll2Launch{Mission: nil}
	if got := l.missionDescription(); got != "" {
		t.Errorf("expected empty for nil mission, got %q", got)
	}
}
