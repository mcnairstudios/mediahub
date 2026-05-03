package spacex

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	return "https://api.spacexdata.com/v4"
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
		BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, source.TypeSpaceX, cfg.IsEnabled, 0),
		cfg:        cfg,
	}
}

type launch struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	DateUTC string `json:"date_utc"`
	Success *bool  `json:"success"`
	Details string `json:"details"`
	Links   struct {
		Patch struct {
			Small string `json:"small"`
			Large string `json:"large"`
		} `json:"patch"`
		Webcast   string `json:"webcast"`
		YouTubeID string `json:"youtube_id"`
	} `json:"links"`
}

func (s *Source) Refresh(ctx context.Context) error {
	log.Printf("spacex: refreshing source %s", s.cfg.Name)

	launches, err := s.fetchLaunches(ctx)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("fetching launches: %w", err)
	}

	seen := make(map[string]struct{}, len(launches))
	var streams []media.Stream
	var keepIDs []string

	for _, l := range launches {
		if l.Links.YouTubeID == "" {
			continue
		}

		id := deterministicStreamID(s.cfg.ID, l.ID)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		keepIDs = append(keepIDs, id)

		webcastURL := "https://www.youtube.com/watch?v=" + l.Links.YouTubeID

		poster := l.Links.Patch.Large
		if poster == "" {
			poster = l.Links.Patch.Small
		}

		year := ""
		if len(l.DateUTC) >= 4 {
			year = l.DateUTC[:4]
		}

		st := media.Stream{
			ID:         id,
			SourceType: string(source.TypeSpaceX),
			SourceID:   s.cfg.ID,
			Name:       l.Name,
			URL:        webcastURL,
			Group:      "SpaceX Launches",
			TvgLogo:    poster,
			VODType:    "movie",
			Year:       year,
			IsActive:   true,
		}
		streams = append(streams, st)
	}

	log.Printf("spacex: found %d launches with webcasts for %s", len(streams), s.cfg.Name)

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
	log.Printf("spacex: upserted %d streams, deleted %d stale for %s", len(streams), len(deleted), s.cfg.Name)

	s.SetRefreshResult(len(streams))
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

func (s *Source) fetchLaunches(ctx context.Context) ([]launch, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase()+"/launches", nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SpaceX API returned %d", resp.StatusCode)
	}

	var launches []launch
	if err := json.NewDecoder(resp.Body).Decode(&launches); err != nil {
		return nil, fmt.Errorf("decoding launches: %w", err)
	}
	return launches, nil
}

func deterministicStreamID(sourceID, launchID string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + launchID))
	return fmt.Sprintf("%x", h[:16])
}
