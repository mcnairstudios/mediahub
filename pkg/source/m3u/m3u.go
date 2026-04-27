package m3u

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/m3u"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

type Config struct {
	ID           string
	Name         string
	URL          string
	IsEnabled    bool
	UseWireGuard bool
	MaxStreams   int
	UserAgent    string
	StreamStore  store.StreamStore
	HTTPClient   *http.Client
	WGClient     *http.Client
}

type Source struct {
	cfg           Config
	etag          string
	streamCount   int
	lastRefreshed *time.Time
	lastError     string
	mu            sync.RWMutex
}

func New(cfg Config) *Source {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &Source{cfg: cfg}
}

func (s *Source) Type() source.SourceType {
	return "m3u"
}

func (s *Source) Info(_ context.Context) source.SourceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return source.SourceInfo{
		ID:                  s.cfg.ID,
		Type:                "m3u",
		Name:                s.cfg.Name,
		IsEnabled:           s.cfg.IsEnabled,
		StreamCount:         s.streamCount,
		LastRefreshed:       s.lastRefreshed,
		LastError:           s.lastError,
		MaxConcurrentStreams: s.cfg.MaxStreams,
	}
}

func (s *Source) Refresh(ctx context.Context) error {
	client := s.cfg.HTTPClient
	if s.cfg.UseWireGuard && s.cfg.WGClient != nil {
		client = s.cfg.WGClient
	}

	s.mu.RLock()
	etag := s.etag
	s.mu.RUnlock()

	result, err := httputil.FetchConditional(ctx, client, s.cfg.URL, etag, s.cfg.UserAgent)
	if err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("fetching m3u: %w", err)
	}

	if !result.Changed {
		s.mu.Lock()
		now := time.Now()
		s.lastRefreshed = &now
		s.lastError = ""
		s.mu.Unlock()
		return nil
	}
	defer result.Body.Close()

	entries, err := m3u.Parse(result.Body)
	if err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("parsing m3u: %w", err)
	}

	seen := make(map[string]struct{}, len(entries))
	streams := make([]media.Stream, 0, len(entries))
	keepIDs := make([]string, 0, len(entries))

	for _, entry := range entries {
		id := deterministicStreamID(s.cfg.ID, entry.URL)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		keepIDs = append(keepIDs, id)

		streams = append(streams, media.Stream{
			ID:         id,
			SourceType: "m3u",
			SourceID:   s.cfg.ID,
			Name:       entry.Name,
			URL:        entry.URL,
			Group:      entry.Group,
			TvgID:      entry.TvgID,
			TvgName:    entry.TvgName,
			TvgLogo:    entry.TvgLogo,
			IsActive:   true,
		})
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("upserting streams: %w", err)
	}

	if _, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, "m3u", s.cfg.ID, keepIDs); err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("deleting stale streams: %w", err)
	}

	s.mu.Lock()
	s.etag = result.ETag
	s.streamCount = len(streams)
	now := time.Now()
	s.lastRefreshed = &now
	s.lastError = ""
	s.mu.Unlock()

	return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
	streams, err := s.cfg.StreamStore.ListBySource(ctx, "m3u", s.cfg.ID)
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(streams))
	for i, stream := range streams {
		ids[i] = stream.ID
	}
	return ids, nil
}

func (s *Source) DeleteStreams(ctx context.Context) error {
	return s.cfg.StreamStore.DeleteBySource(ctx, "m3u", s.cfg.ID)
}

func (s *Source) SupportsConditionalRefresh() bool {
	return true
}

func (s *Source) UsesVPN() bool {
	return s.cfg.UseWireGuard
}

func (s *Source) Clear(ctx context.Context) error {
	if err := s.cfg.StreamStore.DeleteBySource(ctx, "m3u", s.cfg.ID); err != nil {
		return err
	}

	s.mu.Lock()
	s.streamCount = 0
	s.etag = ""
	s.lastError = ""
	s.mu.Unlock()

	return nil
}

func deterministicStreamID(sourceID, url string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + url))
	return fmt.Sprintf("%x", h[:16])
}
