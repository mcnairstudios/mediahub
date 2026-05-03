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

func TestRefresh(t *testing.T) {
	launches := []launch{
		{
			ID:   "launch1",
			Name: "Crew-5",
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
				}{Small: "https://img.example.com/small.png", Large: "https://img.example.com/large.png"},
				Webcast:   "https://youtu.be/abc123",
				YouTubeID: "abc123",
			},
		},
		{
			ID:   "launch2",
			Name: "No Webcast",
		},
		{
			ID:   "launch3",
			Name: "Starlink 4-35",
			DateUTC: "2022-09-24T00:00:00.000Z",
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
		Name:        "SpaceX Launches",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if len(ss.upserted) != 2 {
		t.Fatalf("expected 2 upserted streams (skipped no-webcast), got %d", len(ss.upserted))
	}

	st := ss.upserted[0]
	if st.SourceType != "spacex" {
		t.Errorf("expected source_type=spacex, got %s", st.SourceType)
	}
	if st.Name != "Crew-5" {
		t.Errorf("expected name=Crew-5, got %s", st.Name)
	}
	if st.URL != "https://www.youtube.com/watch?v=abc123" {
		t.Errorf("unexpected URL: %s", st.URL)
	}
	if st.TvgLogo != "https://img.example.com/large.png" {
		t.Errorf("expected large patch as logo, got %s", st.TvgLogo)
	}
	if st.Year != "2022" {
		t.Errorf("expected year=2022, got %s", st.Year)
	}

	info := s.Info(context.Background())
	if info.StreamCount != 2 {
		t.Errorf("expected StreamCount=2, got %d", info.StreamCount)
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
