package trailers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
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
	feedEntries := []feedEntry{
		{Title: "Test Movie", Poster: "/images/test.jpg", Location: "/trailers/studio/test/"},
		{Title: "Another Film", Poster: "https://example.com/poster.jpg", Location: "/trailers/studio/another/"},
		{Title: "", Poster: "", Location: ""},
	}

	itunesResp := itunesResult{
		Results: []struct {
			PreviewURL  string `json:"previewUrl"`
			ArtworkURL  string `json:"artworkUrl100"`
			ReleaseDate string `json:"releaseDate"`
		}{
			{PreviewURL: "https://video.itunes.apple.com/trailer.mp4", ArtworkURL: "https://is1-ssl.mzstatic.com/image/100x100bb.jpg", ReleaseDate: "2026-01-15T08:00:00Z"},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(feedEntries)
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		term := r.URL.Query().Get("term")
		resp := itunesResp
		if term == "Another Film" {
			resp = itunesResult{
				Results: []struct {
					PreviewURL  string `json:"previewUrl"`
					ArtworkURL  string `json:"artworkUrl100"`
					ReleaseDate string `json:"releaseDate"`
				}{
					{PreviewURL: "https://video.itunes.apple.com/another.mp4", ArtworkURL: "https://is1-ssl.mzstatic.com/image2/100x100bb.jpg", ReleaseDate: "2025-06-01T08:00:00Z"},
				},
			}
		}
		json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-source",
		Name:        "Test Trailers",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	feedURLOverride = srv.URL + "/feed"
	itunesSearchURLOverride = srv.URL + "/search"
	defer func() {
		feedURLOverride = ""
		itunesSearchURLOverride = ""
	}()

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
	if st.URL != "https://video.itunes.apple.com/trailer.mp4" {
		t.Errorf("unexpected URL: %s", st.URL)
	}

	info := s.Info(context.Background())
	if info.StreamCount != 2 {
		t.Errorf("expected StreamCount=2, got %d", info.StreamCount)
	}
}

func TestRefreshEmptyFeed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]feedEntry{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-empty",
		Name:        "Empty Trailers",
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	feedURLOverride = srv.URL + "/feed"
	itunesSearchURLOverride = srv.URL + "/search"
	defer func() {
		feedURLOverride = ""
		itunesSearchURLOverride = ""
	}()

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
			{ID: "a", SourceType: sourceType, SourceID: "src1"},
			{ID: "b", SourceType: sourceType, SourceID: "src1"},
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
