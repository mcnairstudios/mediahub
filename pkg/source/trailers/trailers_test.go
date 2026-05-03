package trailers

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
	mux := http.NewServeMux()
	mux.HandleFunc("/movie/upcoming", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{
			Results: []tmdbMovie{
				{ID: 101, Title: "Test Movie", PosterPath: "/poster1.jpg", ReleaseDate: "2026-06-15"},
				{ID: 102, Title: "Another Film", PosterPath: "/poster2.jpg", ReleaseDate: "2025-12-01"},
			},
		})
	})
	mux.HandleFunc("/movie/now_playing", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{Results: []tmdbMovie{}})
	})
	mux.HandleFunc("/movie/101/videos", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbVideoList{
			Results: []tmdbVideo{
				{Key: "abc123", Site: "YouTube", Type: "Trailer", Name: "Official Trailer"},
			},
		})
	})
	mux.HandleFunc("/movie/102/videos", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbVideoList{
			Results: []tmdbVideo{
				{Key: "def456", Site: "YouTube", Type: "Teaser", Name: "Teaser"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tmdbBaseOverride = srv.URL
	defer func() { tmdbBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-source",
		Name:        "Test Trailers",
		IsEnabled:   true,
		TMDBKey:     "test-key",
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if len(ss.upserted) != 2 {
		t.Fatalf("expected 2 upserted streams, got %d", len(ss.upserted))
	}

	st := ss.upserted[0]
	if st.SourceType != "trailers" {
		t.Errorf("expected source_type=trailers, got %s", st.SourceType)
	}
	if st.VODType != "movie" {
		t.Errorf("expected vod_type=movie, got %s", st.VODType)
	}
	if st.Group != "Trailers" {
		t.Errorf("expected group=Trailers, got %s", st.Group)
	}
	if st.Year != "2026" {
		t.Errorf("expected year=2026, got %s", st.Year)
	}
	if st.URL != "https://www.youtube.com/watch?v=abc123" {
		t.Errorf("unexpected URL: %s", st.URL)
	}

	info := s.Info(context.Background())
	if info.StreamCount != 2 {
		t.Errorf("expected StreamCount=2, got %d", info.StreamCount)
	}
}

func TestRefreshNoKey(t *testing.T) {
	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-nokey",
		Name:        "No Key",
		IsEnabled:   true,
		StreamStore: ss,
	})

	err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error for missing TMDB key")
	}
}

func TestRefreshEmptyResults(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/movie/upcoming", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{Results: []tmdbMovie{}})
	})
	mux.HandleFunc("/movie/now_playing", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{Results: []tmdbMovie{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tmdbBaseOverride = srv.URL
	defer func() { tmdbBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-empty",
		Name:        "Empty",
		IsEnabled:   true,
		TMDBKey:     "test-key",
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if len(ss.upserted) != 0 {
		t.Errorf("expected 0 upserted streams, got %d", len(ss.upserted))
	}
}

func TestStreamsAndDelete(t *testing.T) {
	ss := &mockStreamStore{
		streams: []media.Stream{
			{ID: "a", SourceType: string(source.TypeTrailers), SourceID: "src1"},
			{ID: "b", SourceType: string(source.TypeTrailers), SourceID: "src1"},
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
	if s.Type() != "trailers" {
		t.Errorf("expected type=trailers, got %s", s.Type())
	}
}
