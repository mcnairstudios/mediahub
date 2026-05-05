package spacex

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

var apiBaseOverride string

func apiBase() string {
	if apiBaseOverride != "" {
		return apiBaseOverride
	}
	return "https://ll.thespacedevs.com/2.2.0"
}

// maxPastPages limits how many pages of past launches we fetch (50 per page = 500 launches).
const maxPastPages = 10

// maxUpcomingPages limits upcoming launch pages (50 per page = 250 launches).
const maxUpcomingPages = 5

// pageSize is the number of results per API request.
const pageSize = 50

// paginationDelay is the pause between paginated API requests to respect rate limits.
const paginationDelay = 100 * time.Millisecond

type Config struct {
	ID            string
	Name          string
	IsEnabled     bool
	StreamStore   store.StreamStore
	HTTPClient    *http.Client
	OnRefreshDone func(sourceID, etag string, streamCount int)
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
		BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, source.TypeSpaceX, cfg.IsEnabled, 0),
		cfg:        cfg,
	}
}

// Launch Library 2 API response types.

type ll2Response struct {
	Count   int         `json:"count"`
	Next    *string     `json:"next"`
	Results []ll2Launch `json:"results"`
}

type ll2Launch struct {
	ID     string    `json:"id"`
	Name   string    `json:"name"`
	Net    string    `json:"net"`
	Status any `json:"status"`
	Image  string    `json:"image"`

	// List mode fields (flat strings).
	LSPName     string `json:"lsp_name"`
	Mission     any    `json:"mission"`
	MissionType string `json:"mission_type"`
	Location    string `json:"location"`

	// Detailed mode fields.
	LaunchServiceProvider *ll2LSP      `json:"launch_service_provider"`
	VidURLs               []ll2VidURL  `json:"vidURLs"`
	Pad                   any          `json:"pad"`
}

type ll2Status struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Abbrev string `json:"abbrev"`
}

type ll2LSP struct {
	Name string `json:"name"`
}

type ll2VidURL struct {
	Priority int    `json:"priority"`
	URL      string `json:"url"`
	Title    string `json:"title"`
}

type ll2Mission struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type ll2Pad struct {
	Name     string       `json:"name"`
	Location *ll2Location `json:"location"`
}

type ll2Location struct {
	Name string `json:"name"`
}

// statusAbbrev returns the status abbreviation from either list or detailed mode.
func (l *ll2Launch) statusAbbrev() string {
	switch s := l.Status.(type) {
	case map[string]any:
		if abbrev, ok := s["abbrev"].(string); ok {
			return abbrev
		}
	case string:
		return s
	}
	return ""
}

// providerName returns the launch service provider name from either list or detailed mode.
func (l *ll2Launch) providerName() string {
	if l.LSPName != "" {
		return l.LSPName
	}
	if l.LaunchServiceProvider != nil {
		return l.LaunchServiceProvider.Name
	}
	return "Unknown"
}

// missionDescription returns the mission description if available.
func (l *ll2Launch) missionDescription() string {
	if l.Mission == nil {
		return ""
	}
	switch m := l.Mission.(type) {
	case map[string]any:
		if desc, ok := m["description"].(string); ok {
			return desc
		}
	case string:
		// list mode returns mission as a string (the mission name)
		return ""
	}
	return ""
}

// locationName returns the launch location from either list or detailed mode.
func (l *ll2Launch) locationName() string {
	if l.Location != "" {
		return l.Location
	}
	if m, ok := l.Pad.(map[string]any); ok {
		if loc, ok := m["location"].(map[string]any); ok {
			if name, ok := loc["name"].(string); ok {
				return name
			}
		}
	}
	return ""
}

// bestVideoURL returns the highest-priority video URL (lowest priority number = highest priority).
func (l *ll2Launch) bestVideoURL() string {
	if len(l.VidURLs) == 0 {
		return ""
	}
	best := l.VidURLs[0]
	for _, v := range l.VidURLs[1:] {
		if v.Priority < best.Priority {
			best = v
		}
	}
	return best.URL
}

func (s *Source) Refresh(ctx context.Context) error {
	log.Printf("spacex: refreshing from Launch Library 2")

	// Fetch past launches (detailed mode for video URLs).
	past, err := s.fetchPages(ctx, apiBase()+fmt.Sprintf("/launch/previous/?mode=detailed&limit=%d", pageSize), maxPastPages)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("fetching past launches: %w", err)
	}

	// Fetch upcoming launches (list mode is sufficient — rarely have videos yet).
	upcoming, err := s.fetchPages(ctx, apiBase()+fmt.Sprintf("/launch/upcoming/?mode=list&limit=%d", pageSize), maxUpcomingPages)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("fetching upcoming launches: %w", err)
	}

	all := append(past, upcoming...)
	log.Printf("spacex: fetched %d past + %d upcoming = %d total launches", len(past), len(upcoming), len(all))

	seen := make(map[string]struct{}, len(all))
	var streams []media.Stream
	var keepIDs []string

	for _, l := range all {
		id := deterministicStreamID(s.cfg.ID, l.ID)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		keepIDs = append(keepIDs, id)

		videoURL := l.bestVideoURL()

		year := ""
		displayName := l.Name
		if l.Net != "" {
			if t, err := time.Parse(time.RFC3339Nano, l.Net); err == nil {
				year = t.Format("2006")
				displayName = l.Name + " (" + t.Format("Jan 2, 2006") + ")"
			} else if t, err := time.Parse("2006-01-02T15:04:05Z", l.Net); err == nil {
				year = t.Format("2006")
				displayName = l.Name + " (" + t.Format("Jan 2, 2006") + ")"
			} else if len(l.Net) >= 4 {
				year = l.Net[:4]
			}
		}

		// Build tags from launch status.
		var tags []string
		statusLower := strings.ToLower(l.statusAbbrev())
		if statusLower != "" {
			tags = append(tags, statusLower)
		}

		// Use provider name as group for UI tabs.
		group := l.providerName()

		st := media.Stream{
			ID:          id,
			SourceType:  string(source.TypeSpaceX),
			SourceID:    s.cfg.ID,
			Name:        displayName,
			URL:         videoURL,
			Group:       group,
			TvgLogo:     l.Image,
			VODType:     "movie",
			Year:        year,
			EpisodeName: l.missionDescription(),
			Tags:        tags,
			IsActive:    true,
		}
		streams = append(streams, st)
	}

	log.Printf("spacex: built %d streams", len(streams))

	if len(streams) == 0 {
		s.SetRefreshResult(0)
		return nil
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("upserting streams: %w", err)
	}

	deleted, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, string(source.TypeSpaceX), s.cfg.ID, keepIDs)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("deleting stale streams: %w", err)
	}
	log.Printf("spacex: upserted %d, deleted %d stale", len(streams), len(deleted))

	s.SetRefreshResult(len(streams))
	if s.cfg.OnRefreshDone != nil {
		s.cfg.OnRefreshDone(s.cfg.ID, "", len(streams))
	}
	return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
	streams, err := s.cfg.StreamStore.ListBySource(ctx, string(source.TypeSpaceX), s.cfg.ID)
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
	return s.cfg.StreamStore.DeleteBySource(ctx, string(source.TypeSpaceX), s.cfg.ID)
}

// fetchPages paginates through the Launch Library 2 API starting at startURL.
// It fetches at most maxPages pages, sleeping briefly between requests.
func (s *Source) fetchPages(ctx context.Context, startURL string, maxPages int) ([]ll2Launch, error) {
	var all []ll2Launch
	url := startURL

	for page := 0; page < maxPages && url != ""; page++ {
		if page > 0 {
			time.Sleep(paginationDelay)
		}

		resp, err := s.fetchPage(ctx, url)
		if err != nil {
			return all, err
		}
		all = append(all, resp.Results...)

		if resp.Next != nil {
			nextURL := *resp.Next
			// In tests the next URL from the API might be absolute with the real
			// host, but we need to use the test server. When apiBaseOverride is set,
			// rewrite the next URL to use it.
			if apiBaseOverride != "" && !strings.HasPrefix(nextURL, apiBaseOverride) {
				// Extract the path+query from the next URL.
				if idx := strings.Index(nextURL, "/launch/"); idx >= 0 {
					nextURL = apiBaseOverride + nextURL[idx:]
				}
			}
			url = nextURL
		} else {
			url = ""
		}
	}
	return all, nil
}

func (s *Source) fetchPage(ctx context.Context, url string) (*ll2Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Launch Library 2 API returned %d", resp.StatusCode)
	}

	var result ll2Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}

func deterministicStreamID(sourceID, launchID string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + launchID))
	return fmt.Sprintf("%x", h[:16])
}
