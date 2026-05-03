package trailers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRefresh_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/movie/upcoming", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tmdbBaseOverride = srv.URL
	defer func() { tmdbBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-source",
		Name:        "Test",
		IsEnabled:   true,
		TMDBKey:     "test-key",
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

func TestRefresh_NoTrailersForMovie(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/movie/upcoming", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{
			Results: []tmdbMovie{
				{ID: 101, Title: "No Trailer Movie", PosterPath: "/poster.jpg", ReleaseDate: "2026-06-15"},
			},
		})
	})
	mux.HandleFunc("/movie/now_playing", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{Results: []tmdbMovie{}})
	})
	mux.HandleFunc("/movie/101/videos", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbVideoList{Results: []tmdbVideo{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tmdbBaseOverride = srv.URL
	defer func() { tmdbBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-source",
		Name:        "Test",
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
		t.Errorf("expected 0 upserted for movie with no trailers, got %d", len(ss.upserted))
	}
}

func TestRefresh_TeaserFallback(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/movie/upcoming", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{
			Results: []tmdbMovie{
				{ID: 101, Title: "Teaser Only", PosterPath: "", ReleaseDate: "2026-01-01"},
			},
		})
	})
	mux.HandleFunc("/movie/now_playing", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{Results: []tmdbMovie{}})
	})
	mux.HandleFunc("/movie/101/videos", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbVideoList{
			Results: []tmdbVideo{
				{Key: "teaser-key", Site: "YouTube", Type: "Teaser", Name: "First Look"},
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
		Name:        "Test",
		IsEnabled:   true,
		TMDBKey:     "test-key",
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 1 {
		t.Fatalf("expected 1 stream for teaser fallback, got %d", len(ss.upserted))
	}
	if ss.upserted[0].URL != "https://www.youtube.com/watch?v=teaser-key" {
		t.Errorf("unexpected URL: %s", ss.upserted[0].URL)
	}
}

func TestRefresh_TrailerPreferredOverTeaser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/movie/upcoming", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{
			Results: []tmdbMovie{
				{ID: 101, Title: "Both Types", ReleaseDate: "2026-01-01"},
			},
		})
	})
	mux.HandleFunc("/movie/now_playing", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{Results: []tmdbMovie{}})
	})
	mux.HandleFunc("/movie/101/videos", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbVideoList{
			Results: []tmdbVideo{
				{Key: "teaser-key", Site: "YouTube", Type: "Teaser", Name: "Teaser"},
				{Key: "trailer-key", Site: "YouTube", Type: "Trailer", Name: "Official Trailer"},
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
		Name:        "Test",
		IsEnabled:   true,
		TMDBKey:     "test-key",
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(ss.upserted))
	}
	if ss.upserted[0].URL != "https://www.youtube.com/watch?v=trailer-key" {
		t.Errorf("expected trailer preferred over teaser, got %s", ss.upserted[0].URL)
	}
}

func TestRefresh_NowPlayingMerged(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/movie/upcoming", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{
			Results: []tmdbMovie{
				{ID: 101, Title: "Upcoming Movie", ReleaseDate: "2026-06-15"},
			},
		})
	})
	mux.HandleFunc("/movie/now_playing", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{
			Results: []tmdbMovie{
				{ID: 102, Title: "Now Playing Movie", ReleaseDate: "2026-04-01"},
			},
		})
	})
	mux.HandleFunc("/movie/101/videos", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbVideoList{
			Results: []tmdbVideo{{Key: "key1", Site: "YouTube", Type: "Trailer", Name: "Trailer"}},
		})
	})
	mux.HandleFunc("/movie/102/videos", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbVideoList{
			Results: []tmdbVideo{{Key: "key2", Site: "YouTube", Type: "Trailer", Name: "Trailer"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tmdbBaseOverride = srv.URL
	defer func() { tmdbBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-source",
		Name:        "Test",
		IsEnabled:   true,
		TMDBKey:     "test-key",
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 2 {
		t.Fatalf("expected 2 streams (upcoming + now_playing), got %d", len(ss.upserted))
	}
}

func TestRefresh_EmptyTitleSkipped(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/movie/upcoming", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{
			Results: []tmdbMovie{
				{ID: 101, Title: "", ReleaseDate: "2026-06-15"},
			},
		})
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
		ID:          "test-source",
		Name:        "Test",
		IsEnabled:   true,
		TMDBKey:     "test-key",
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 0 {
		t.Errorf("expected 0 streams for empty title, got %d", len(ss.upserted))
	}
}

func TestRefresh_PosterURLConstruction(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/movie/upcoming", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{
			Results: []tmdbMovie{
				{ID: 101, Title: "With Poster", PosterPath: "/abc123.jpg", ReleaseDate: "2026-01-01"},
				{ID: 102, Title: "No Poster", PosterPath: "", ReleaseDate: "2026-01-01"},
			},
		})
	})
	mux.HandleFunc("/movie/now_playing", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbMovieList{Results: []tmdbMovie{}})
	})
	mux.HandleFunc("/movie/101/videos", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbVideoList{
			Results: []tmdbVideo{{Key: "k1", Site: "YouTube", Type: "Trailer", Name: "T"}},
		})
	})
	mux.HandleFunc("/movie/102/videos", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbVideoList{
			Results: []tmdbVideo{{Key: "k2", Site: "YouTube", Type: "Trailer", Name: "T"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tmdbBaseOverride = srv.URL
	defer func() { tmdbBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-source",
		Name:        "Test",
		IsEnabled:   true,
		TMDBKey:     "test-key",
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	s.Refresh(context.Background())

	if len(ss.upserted) != 2 {
		t.Fatalf("expected 2, got %d", len(ss.upserted))
	}

	var withPoster, noPoster string
	for _, st := range ss.upserted {
		if st.Name == "With Poster - T" {
			withPoster = st.TvgLogo
		}
		if st.Name == "No Poster - T" {
			noPoster = st.TvgLogo
		}
	}
	if withPoster != "https://image.tmdb.org/t/p/w500/abc123.jpg" {
		t.Errorf("poster URL = %q, want tmdb URL", withPoster)
	}
	if noPoster != "" {
		t.Errorf("expected empty logo for no poster, got %q", noPoster)
	}
}

func TestDeterministicStreamID_Stable(t *testing.T) {
	id1 := deterministicStreamID("src-1", "movie-101")
	id2 := deterministicStreamID("src-1", "movie-101")
	if id1 != id2 {
		t.Error("same inputs should produce the same ID")
	}
}

func TestDeterministicStreamID_DifferentInputs(t *testing.T) {
	id1 := deterministicStreamID("src-1", "movie-101")
	id2 := deterministicStreamID("src-1", "movie-102")
	if id1 == id2 {
		t.Error("different inputs should produce different IDs")
	}
}

func TestNewDefaultHTTPClient(t *testing.T) {
	s := New(Config{ID: "x", Name: "X", IsEnabled: true})
	if s.cfg.HTTPClient == nil {
		t.Fatal("expected default HTTP client to be set")
	}
}
