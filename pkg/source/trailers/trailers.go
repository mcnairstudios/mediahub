package trailers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

var tmdbBaseOverride string

func tmdbBase() string {
	if tmdbBaseOverride != "" {
		return tmdbBaseOverride
	}
	return "https://api.themoviedb.org/3"
}

type Config struct {
	ID          string
	Name        string
	IsEnabled   bool
	TMDBKey     string
	StreamStore store.StreamStore
	HTTPClient  *http.Client
}

type Source struct {
	source.BaseSource
	cfg Config
}

func New(cfg Config) *Source {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Source{
		BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, source.TypeTrailers, cfg.IsEnabled, 0),
		cfg:        cfg,
	}
}

type tmdbMovieList struct {
	Results []tmdbMovie `json:"results"`
}

type tmdbMovie struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	PosterPath  string  `json:"poster_path"`
	ReleaseDate string  `json:"release_date"`
	VoteAverage float64 `json:"vote_average"`
}

type tmdbVideoList struct {
	Results []tmdbVideo `json:"results"`
}

type tmdbVideo struct {
	Key  string `json:"key"`
	Site string `json:"site"`
	Type string `json:"type"`
	Name string `json:"name"`
}

func (s *Source) Refresh(ctx context.Context) error {
	if s.cfg.TMDBKey == "" {
		s.SetError("no TMDB API key configured")
		return fmt.Errorf("no TMDB API key configured in settings")
	}

	log.Printf("trailers: refreshing source %s", s.cfg.Name)

	movies, err := s.fetchUpcoming(ctx)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("fetching upcoming movies: %w", err)
	}

	nowPlaying, err := s.fetchNowPlaying(ctx)
	if err == nil {
		movies = append(movies, nowPlaying...)
	}

	log.Printf("trailers: found %d movies for %s", len(movies), s.cfg.Name)

	seen := make(map[string]struct{}, len(movies))
	var streams []media.Stream
	var keepIDs []string

	for _, movie := range movies {
		if movie.Title == "" {
			continue
		}

		trailerURL, trailerName := s.findTrailer(ctx, movie.ID)
		if trailerURL == "" {
			continue
		}

		id := deterministicStreamID(s.cfg.ID, fmt.Sprintf("%d", movie.ID))
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		keepIDs = append(keepIDs, id)

		poster := ""
		if movie.PosterPath != "" {
			poster = "https://image.tmdb.org/t/p/w500" + movie.PosterPath
		}

		year := ""
		if len(movie.ReleaseDate) >= 4 {
			year = movie.ReleaseDate[:4]
		}

		name := movie.Title + " - " + trailerName

		st := media.Stream{
			ID:         id,
			SourceType: string(source.TypeTrailers),
			SourceID:   s.cfg.ID,
			Name:       name,
			URL:        trailerURL,
			Group:      "Trailers",
			TvgLogo:    poster,
			VODType:    "movie",
			Year:       year,
			IsActive:   true,
		}
		streams = append(streams, st)
	}

	if len(streams) == 0 {
		log.Printf("trailers: no streams found for %s", s.cfg.Name)
		s.SetRefreshResult(0)
		return nil
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("upserting streams: %w", err)
	}

	deleted, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, string(source.TypeTrailers), s.cfg.ID, keepIDs)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("deleting stale streams: %w", err)
	}
	log.Printf("trailers: upserted %d streams, deleted %d stale for %s", len(streams), len(deleted), s.cfg.Name)

	s.SetRefreshResult(len(streams))
	return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
	streams, err := s.cfg.StreamStore.ListBySource(ctx, string(source.TypeTrailers), s.cfg.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(streams))
	for i, st := range streams {
		ids[i] = st.ID
	}
	return ids, nil
}

func (s *Source) DeleteStreams(ctx context.Context) error {
	return s.cfg.StreamStore.DeleteBySource(ctx, string(source.TypeTrailers), s.cfg.ID)
}

func (s *Source) fetchMovieList(ctx context.Context, endpoint string) ([]tmdbMovie, error) {
	u := fmt.Sprintf("%s/%s?api_key=%s&page=1", tmdbBase(), endpoint, url.QueryEscape(s.cfg.TMDBKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB %s returned %d", endpoint, resp.StatusCode)
	}

	var list tmdbMovieList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", endpoint, err)
	}
	return list.Results, nil
}

func (s *Source) fetchUpcoming(ctx context.Context) ([]tmdbMovie, error) {
	return s.fetchMovieList(ctx, "movie/upcoming")
}

func (s *Source) fetchNowPlaying(ctx context.Context) ([]tmdbMovie, error) {
	return s.fetchMovieList(ctx, "movie/now_playing")
}

func (s *Source) findTrailer(ctx context.Context, tmdbID int) (string, string) {
	u := fmt.Sprintf("%s/movie/%d/videos?api_key=%s", tmdbBase(), tmdbID, url.QueryEscape(s.cfg.TMDBKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", ""
	}

	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", ""
	}

	var videos tmdbVideoList
	if err := json.NewDecoder(resp.Body).Decode(&videos); err != nil {
		return "", ""
	}

	for _, v := range videos.Results {
		if v.Site == "YouTube" && v.Type == "Trailer" {
			return "https://www.youtube.com/watch?v=" + v.Key, v.Name
		}
	}
	for _, v := range videos.Results {
		if v.Site == "YouTube" && v.Type == "Teaser" {
			return "https://www.youtube.com/watch?v=" + v.Key, v.Name
		}
	}
	return "", ""
}

func deterministicStreamID(sourceID, key string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + key))
	return fmt.Sprintf("%x", h[:16])
}
