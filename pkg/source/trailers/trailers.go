package trailers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

const (
	defaultFeedURL         = "https://trailers.apple.com/trailers/home/feeds/just_added.json"
	defaultItunesSearchURL = "https://itunes.apple.com/search"
	sourceType             = "trailers"
)

var (
	feedURLOverride         string
	itunesSearchURLOverride string
)

func getFeedURL() string {
	if feedURLOverride != "" {
		return feedURLOverride
	}
	return defaultFeedURL
}

func getItunesSearchURL() string {
	if itunesSearchURLOverride != "" {
		return itunesSearchURLOverride
	}
	return defaultItunesSearchURL
}

type Config struct {
	ID          string
	Name        string
	IsEnabled   bool
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
		BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, sourceType, cfg.IsEnabled, 0),
		cfg:        cfg,
	}
}

type feedEntry struct {
	Title       string `json:"title"`
	ReleaseDate string `json:"releasedate"`
	Poster      string `json:"poster"`
	Location    string `json:"location"`
}

type itunesResult struct {
	Results []struct {
		PreviewURL  string `json:"previewUrl"`
		ArtworkURL  string `json:"artworkUrl100"`
		ReleaseDate string `json:"releaseDate"`
	} `json:"results"`
}

func (s *Source) Refresh(ctx context.Context) error {
	log.Printf("trailers: refreshing source %s", s.cfg.Name)

	entries, err := s.fetchFeed(ctx)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("fetching apple trailers feed: %w", err)
	}
	log.Printf("trailers: fetched %d entries from feed for %s", len(entries), s.cfg.Name)

	seen := make(map[string]struct{}, len(entries))
	var streams []media.Stream
	var keepIDs []string

	for _, entry := range entries {
		if entry.Title == "" {
			continue
		}

		previewURL, artworkURL, year := s.lookupITunes(ctx, entry.Title)
		if previewURL == "" {
			continue
		}

		id := deterministicStreamID(s.cfg.ID, previewURL)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		keepIDs = append(keepIDs, id)

		poster := entry.Poster
		if poster != "" && !strings.HasPrefix(poster, "http") {
			poster = "https://trailers.apple.com" + poster
		}
		if artworkURL != "" {
			poster = strings.Replace(artworkURL, "100x100bb", "600x600bb", 1)
		}

		st := media.Stream{
			ID:         id,
			SourceType: sourceType,
			SourceID:   s.cfg.ID,
			Name:       entry.Title + " - Trailer",
			URL:        previewURL,
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

	deleted, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, sourceType, s.cfg.ID, keepIDs)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("deleting stale streams: %w", err)
	}
	log.Printf("trailers: upserted %d streams, deleted %d stale for %s", len(streams), len(deleted), s.cfg.Name)

	s.SetRefreshResult(len(streams))
	return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
	streams, err := s.cfg.StreamStore.ListBySource(ctx, sourceType, s.cfg.ID)
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
	return s.cfg.StreamStore.DeleteBySource(ctx, sourceType, s.cfg.ID)
}

func (s *Source) fetchFeed(ctx context.Context) ([]feedEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getFeedURL(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple feed returned %d", resp.StatusCode)
	}

	var entries []feedEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decoding feed: %w", err)
	}
	return entries, nil
}

func (s *Source) lookupITunes(ctx context.Context, title string) (previewURL, artworkURL, year string) {
	params := url.Values{
		"term":   {title},
		"entity": {"movie"},
		"limit":  {"1"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getItunesSearchURL()+"?"+params.Encode(), nil)
	if err != nil {
		return "", "", ""
	}

	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return "", "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", ""
	}

	var result itunesResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", ""
	}

	if len(result.Results) == 0 {
		return "", "", ""
	}

	r := result.Results[0]
	if r.ReleaseDate != "" && len(r.ReleaseDate) >= 4 {
		year = r.ReleaseDate[:4]
	}
	return r.PreviewURL, r.ArtworkURL, year
}

func deterministicStreamID(sourceID, url string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + url))
	return fmt.Sprintf("%x", h[:16])
}
