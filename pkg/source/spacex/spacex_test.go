package spacex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

type mockStreamStore struct {
	store.StreamStore
	upserted []media.Stream
	deleted  []string
	streams  []media.Stream
}

func (m *mockStreamStore) BulkUpsert(_ context.Context, streams []media.Stream) error {
	m.upserted = append(m.upserted, streams...)
	m.streams = streams
	return nil
}

func (m *mockStreamStore) DeleteStaleBySource(_ context.Context, _, _ string, _ []string) ([]string, error) {
	return m.deleted, nil
}

func (m *mockStreamStore) ListBySource(_ context.Context, _, _ string) ([]media.Stream, error) {
	return m.streams, nil
}

func (m *mockStreamStore) DeleteBySource(_ context.Context, _, _ string) error {
	m.streams = nil
	return nil
}

// samplePastResponse returns a Launch Library 2 previous launches response with vidURLs.
func samplePastResponse() ll2Response {
	return ll2Response{
		Count: 2,
		Next:  nil,
		Results: []ll2Launch{
			{
				ID:   "uuid-crew5",
				Name: "Falcon 9 Block 5 | Crew-5",
				Net:  "2022-10-05T16:00:00Z",
				Status: ll2Status{
					ID: 3, Name: "Launch Successful", Abbrev: "Success",
				},
				Image: "https://img.example.com/crew5.jpg",
				LaunchServiceProvider: &ll2LSP{Name: "SpaceX"},
				Mission: map[string]any{
					"name":        "Crew-5",
					"description": "Fifth operational crew rotation mission.",
				},
				VidURLs: []ll2VidURL{
					{Priority: 10, URL: "https://www.youtube.com/watch?v=abc123", Title: "Official Webcast"},
					{Priority: 5, URL: "https://www.youtube.com/watch?v=best123", Title: "High Priority"},
				},
			},
			{
				ID:      "uuid-nowebcast",
				Name:    "Falcon 9 Block 5 | No Webcast",
				Net:     "2022-09-20T00:00:00Z",
				Status:  ll2Status{ID: 3, Abbrev: "Success"},
				LSPName: "SpaceX",
			},
		},
	}
}

// sampleUpcomingResponse returns a Launch Library 2 upcoming launches response.
func sampleUpcomingResponse() ll2Response {
	return ll2Response{
		Count: 1,
		Next:  nil,
		Results: []ll2Launch{
			{
				ID:      "uuid-upcoming1",
				Name:    "Electron | PREFIRE-2",
				Net:     "2025-06-15T12:00:00Z",
				Status:  ll2Status{ID: 1, Abbrev: "Go"},
				Image:   "https://img.example.com/electron.jpg",
				LSPName: "Rocket Lab",
				Mission: "PREFIRE-2",
			},
		},
	}
}

func TestRefresh(t *testing.T) {
	past := samplePastResponse()
	upcoming := sampleUpcomingResponse()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case contains(r.URL.Path, "/launch/previous/"):
			json.NewEncoder(w).Encode(past)
		case contains(r.URL.Path, "/launch/upcoming/"):
			json.NewEncoder(w).Encode(upcoming)
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
		Name:        "Space Launches",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// 2 past + 1 upcoming = 3 streams (no-webcast launch still included, just with empty URL).
	if len(ss.upserted) != 3 {
		t.Fatalf("expected 3 upserted streams, got %d", len(ss.upserted))
	}

	// Find the Crew-5 stream.
	var crew5 *media.Stream
	for i := range ss.upserted {
		if ss.upserted[i].Name == "Falcon 9 Block 5 | Crew-5 (Oct 5, 2022)" {
			crew5 = &ss.upserted[i]
			break
		}
	}
	if crew5 == nil {
		t.Fatal("could not find Crew-5 stream")
	}
	if crew5.SourceType != "spacex" {
		t.Errorf("expected source_type=spacex, got %s", crew5.SourceType)
	}
	// Best video URL should be the one with lowest priority number (5).
	if crew5.URL != "https://www.youtube.com/watch?v=best123" {
		t.Errorf("expected best video URL (priority 5), got %s", crew5.URL)
	}
	if crew5.TvgLogo != "https://img.example.com/crew5.jpg" {
		t.Errorf("expected launch image as logo, got %s", crew5.TvgLogo)
	}
	if crew5.Year != "2022" {
		t.Errorf("expected year=2022, got %s", crew5.Year)
	}
	if crew5.Group != "SpaceX" {
		t.Errorf("expected group=SpaceX, got %s", crew5.Group)
	}
	if crew5.EpisodeName != "Fifth operational crew rotation mission." {
		t.Errorf("expected mission description as episode name, got %q", crew5.EpisodeName)
	}
	if crew5.VODType != "movie" {
		t.Errorf("expected VODType=movie, got %s", crew5.VODType)
	}

	// Find the Rocket Lab upcoming stream.
	var rl *media.Stream
	for i := range ss.upserted {
		if ss.upserted[i].Group == "Rocket Lab" {
			rl = &ss.upserted[i]
			break
		}
	}
	if rl == nil {
		t.Fatal("could not find Rocket Lab stream")
	}
	if rl.URL != "" {
		t.Errorf("expected empty URL for upcoming launch without videos, got %s", rl.URL)
	}
	if rl.TvgLogo != "https://img.example.com/electron.jpg" {
		t.Errorf("expected launch image, got %s", rl.TvgLogo)
	}

	info := s.Info(context.Background())
	if info.StreamCount != 3 {
		t.Errorf("expected StreamCount=3, got %d", info.StreamCount)
	}
}

func TestStreamsAndDelete(t *testing.T) {
	ss := &mockStreamStore{
		streams: []media.Stream{
			{ID: "a", SourceType: string(source.TypeSpaceX), SourceID: "src1"},
			{ID: "b", SourceType: string(source.TypeSpaceX), SourceID: "src1"},
		},
	}
	s := New(Config{ID: "src1", Name: "Test", IsEnabled: true, StreamStore: ss})

	ids, err := s.Streams(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}

	if err := s.DeleteStreams(context.Background()); err != nil {
		t.Fatal(err)
	}
	if ss.streams != nil {
		t.Error("expected streams to be nil after delete")
	}
}

func TestType(t *testing.T) {
	s := New(Config{ID: "x", Name: "X", IsEnabled: true})
	if s.Type() != "spacex" {
		t.Errorf("expected type=spacex, got %s", s.Type())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
