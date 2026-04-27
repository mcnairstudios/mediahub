package tmdb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
)

func TestSearchMovie(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /3/search/movie", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(searchResponse{
			Results: []searchResult{{ID: 550, Title: "Fight Club"}},
		})
	})
	mux.HandleFunc("GET /3/movie/550", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(movieDetailResponse{
			ID:          550,
			Title:       "Fight Club",
			Overview:    "An insomniac office worker...",
			PosterPath:  "/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg",
			ReleaseDate: "1999-10-15",
			VoteAverage: 8.4,
			Runtime:     139,
			Genres:      []genre{{ID: 18, Name: "Drama"}},
			Credits: credits{
				Cast: []creditsPerson{{ID: 1, Name: "Brad Pitt", Character: "Tyler Durden", ProfilePath: "/ajTaN3.jpg"}},
				Crew: []creditsPerson{{ID: 2, Name: "David Fincher", Job: "Director"}},
			},
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	cache := tmdbcache.New()
	c := &Client{
		httpClient: ts.Client(),
		apiKeyFn:   func() string { return "test-key" },
		cache:      cache,
	}

	origGet := c.httpClient.Get
	_ = origGet

	c.httpClient.Transport = &rewriteTransport{base: ts.URL}

	movie, err := c.SearchMovie("Fight Club", 1999)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if movie == nil {
		t.Fatal("expected non-nil movie")
	}
	if movie.Title != "Fight Club" {
		t.Fatalf("expected Fight Club, got %s", movie.Title)
	}
	if movie.Runtime != 139 {
		t.Fatalf("expected 139 min, got %d", movie.Runtime)
	}
	if len(movie.Cast) != 1 || movie.Cast[0].Name != "Brad Pitt" {
		t.Fatalf("expected Brad Pitt in cast, got %v", movie.Cast)
	}
	if len(movie.Crew) != 1 || movie.Crew[0].Job != "Director" {
		t.Fatalf("expected Director in crew, got %v", movie.Crew)
	}

	cached, err := c.SearchMovie("Fight Club", 1999)
	if err != nil {
		t.Fatalf("cached search: %v", err)
	}
	if cached.Title != "Fight Club" {
		t.Fatal("cache miss")
	}
}

func TestSearchTV(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /3/search/tv", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(searchResponse{
			Results: []searchResult{{ID: 1399, Name: "Breaking Bad"}},
		})
	})
	mux.HandleFunc("GET /3/tv/1399", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tvDetailResponse{
			ID:           1399,
			Name:         "Breaking Bad",
			Overview:     "A high school chemistry teacher...",
			PosterPath:   "/ggFHVNu6YYI5L9pCfOacjizRGt.jpg",
			FirstAirDate: "2008-01-20",
			VoteAverage:  8.9,
			Genres:       []genre{{ID: 18, Name: "Drama"}},
			Seasons: []seasonBrief{
				{SeasonNumber: 1, Name: "Season 1", EpisodeCount: 7},
				{SeasonNumber: 2, Name: "Season 2", EpisodeCount: 13},
			},
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	cache := tmdbcache.New()
	c := &Client{
		httpClient: ts.Client(),
		apiKeyFn:   func() string { return "test-key" },
		cache:      cache,
	}
	c.httpClient.Transport = &rewriteTransport{base: ts.URL}

	series, err := c.SearchTV("Breaking Bad")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if series == nil {
		t.Fatal("expected non-nil series")
	}
	if series.Name != "Breaking Bad" {
		t.Fatalf("expected Breaking Bad, got %s", series.Name)
	}
	if len(series.Seasons) != 2 {
		t.Fatalf("expected 2 seasons, got %d", len(series.Seasons))
	}
}

func TestMovieDetail(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /3/movie/550", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") == "" {
			http.Error(w, "missing api_key", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(movieDetailResponse{
			ID:          550,
			Title:       "Fight Club",
			Runtime:     139,
			VoteAverage: 8.4,
			Genres:      []genre{{ID: 18, Name: "Drama"}, {ID: 53, Name: "Thriller"}},
			BelongsToCollection: collectionRef{ID: 0},
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	cache := tmdbcache.New()
	c := &Client{
		httpClient: ts.Client(),
		apiKeyFn:   func() string { return "test-key" },
		cache:      cache,
	}
	c.httpClient.Transport = &rewriteTransport{base: ts.URL}

	movie, err := c.MovieDetail(550)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if len(movie.Genres) != 2 {
		t.Fatalf("expected 2 genres, got %d", len(movie.Genres))
	}
}

func TestNoAPIKey(t *testing.T) {
	cache := tmdbcache.New()
	c := NewClient(func() string { return "" }, cache)

	_, err := c.SearchMovie("test", 0)
	if err == nil {
		t.Fatal("expected error with empty API key")
	}
}

func TestImageURL(t *testing.T) {
	got := ImageURL("/abc.jpg", "w500")
	if got != "https://image.tmdb.org/t/p/w500/abc.jpg" {
		t.Fatalf("unexpected URL: %s", got)
	}

	got = ImageURL("", "w500")
	if got != "" {
		t.Fatalf("expected empty string for empty path, got %s", got)
	}

	got = ImageURL("/abc.jpg", "")
	if got != "https://image.tmdb.org/t/p/w500/abc.jpg" {
		t.Fatalf("expected default w500, got %s", got)
	}
}

func TestSearchMovieNoResults(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /3/search/movie", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(searchResponse{Results: []searchResult{}})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	cache := tmdbcache.New()
	c := &Client{
		httpClient: ts.Client(),
		apiKeyFn:   func() string { return "test-key" },
		cache:      cache,
	}
	c.httpClient.Transport = &rewriteTransport{base: ts.URL}

	movie, err := c.SearchMovie("nonexistent movie xyz", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if movie != nil {
		t.Fatal("expected nil for no results")
	}
}

func TestCleanVODName(t *testing.T) {
	tests := []struct {
		input     string
		wantClean string
		wantYear  string
	}{
		{"Fight Club (1999)", "Fight Club", "1999"},
		{"The Matrix (1999) {4K}", "The Matrix", "1999"},
		{"Inception", "Inception", ""},
		{"Movie {Director's Cut} (2020)", "Movie", "2020"},
	}
	for _, tt := range tests {
		clean, year := cleanVODName(tt.input)
		if clean != tt.wantClean || year != tt.wantYear {
			t.Errorf("cleanVODName(%q) = (%q, %q), want (%q, %q)",
				tt.input, clean, year, tt.wantClean, tt.wantYear)
		}
	}
}

type rewriteTransport struct {
	base string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req)
}
