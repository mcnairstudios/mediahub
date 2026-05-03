package spacex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRefresh_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-spacex",
		Name:        "SpaceX",
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]launch{})
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-spacex",
		Name:        "SpaceX",
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

func TestRefresh_SmallPatchFallback(t *testing.T) {
	launches := []launch{
		{
			ID:      "launch1",
			Name:    "Crew-5",
			DateUTC: "2022-10-05T16:00:00.000Z",
			Links: struct {
				Patch struct {
					Small string `json:"small"`
					Large string `json:"large"`
				} `json:"patch"`
				Webcast   string `json:"webcast"`
				YouTubeID string `json:"youtube_id"`
			}{
				Patch: struct {
					Small string `json:"small"`
					Large string `json:"large"`
				}{Small: "https://img.example.com/small.png"},
				Webcast:   "https://youtu.be/abc123",
				YouTubeID: "abc123",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(launches)
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-spacex",
		Name:        "SpaceX",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 1 {
		t.Fatalf("expected 1 upserted, got %d", len(ss.upserted))
	}
	if ss.upserted[0].TvgLogo != "https://img.example.com/small.png" {
		t.Errorf("expected small patch as fallback logo, got %q", ss.upserted[0].TvgLogo)
	}
}

func TestRefresh_DuplicateLaunchIDs(t *testing.T) {
	launches := []launch{
		{
			ID:      "same-id",
			Name:    "Launch A",
			DateUTC: "2022-10-05T16:00:00.000Z",
			Links: struct {
				Patch struct {
					Small string `json:"small"`
					Large string `json:"large"`
				} `json:"patch"`
				Webcast   string `json:"webcast"`
				YouTubeID string `json:"youtube_id"`
			}{
				Webcast:   "https://youtu.be/abc123",
				YouTubeID: "abc123",
			},
		},
		{
			ID:      "same-id",
			Name:    "Launch B (duplicate)",
			DateUTC: "2022-10-05T16:00:00.000Z",
			Links: struct {
				Patch struct {
					Small string `json:"small"`
					Large string `json:"large"`
				} `json:"patch"`
				Webcast   string `json:"webcast"`
				YouTubeID string `json:"youtube_id"`
			}{
				Webcast:   "https://youtu.be/def456",
				YouTubeID: "def456",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(launches)
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-spacex",
		Name:        "SpaceX",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 1 {
		t.Fatalf("expected 1 upserted (deduped), got %d", len(ss.upserted))
	}
	if ss.upserted[0].Name != "Launch A" {
		t.Errorf("expected first launch to win, got %q", ss.upserted[0].Name)
	}
}

func TestRefresh_NoDateSkipsYear(t *testing.T) {
	launches := []launch{
		{
			ID:      "launch1",
			Name:    "No Date",
			DateUTC: "",
			Links: struct {
				Patch struct {
					Small string `json:"small"`
					Large string `json:"large"`
				} `json:"patch"`
				Webcast   string `json:"webcast"`
				YouTubeID string `json:"youtube_id"`
			}{
				Webcast:   "https://youtu.be/abc123",
				YouTubeID: "abc123",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(launches)
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-spacex",
		Name:        "SpaceX",
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
