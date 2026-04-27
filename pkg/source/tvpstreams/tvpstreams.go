package tvpstreams

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/m3u"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/mtls"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

const tmdbImageBase = "https://image.tmdb.org/t/p/w500"

type Config struct {
	ID              string
	Name            string
	URL             string
	IsEnabled       bool
	UseWireGuard    bool
	DataDir         string
	EnrollmentToken string
	TLSEnrolled     bool
	StreamStore     store.StreamStore
	HTTPClient      *http.Client
	WGClient        *http.Client
	TMDBCache       *tmdb.Cache
	OnEnrolled      func(sourceID string) error
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
	return "tvpstreams"
}

func (s *Source) Info(_ context.Context) source.SourceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return source.SourceInfo{
		ID:            s.cfg.ID,
		Type:          "tvpstreams",
		Name:          s.cfg.Name,
		IsEnabled:     s.cfg.IsEnabled,
		StreamCount:   s.streamCount,
		LastRefreshed: s.lastRefreshed,
		LastError:     s.lastError,
	}
}

func (s *Source) Refresh(ctx context.Context) error {
	if s.cfg.EnrollmentToken != "" && !s.cfg.TLSEnrolled && s.cfg.DataDir != "" {
		result, err := mtls.Enroll(s.cfg.URL, s.cfg.EnrollmentToken)
		if err != nil {
			s.mu.Lock()
			s.lastError = fmt.Sprintf("mTLS enrollment failed: %v", err)
			s.mu.Unlock()
			return fmt.Errorf("mTLS enrollment: %w", err)
		}
		if err := mtls.SaveCerts(s.cfg.DataDir, s.cfg.ID, result); err != nil {
			s.mu.Lock()
			s.lastError = fmt.Sprintf("saving mTLS certs: %v", err)
			s.mu.Unlock()
			return fmt.Errorf("saving mTLS certs: %w", err)
		}
		s.cfg.TLSEnrolled = true
		s.cfg.EnrollmentToken = ""
		if s.cfg.OnEnrolled != nil {
			if err := s.cfg.OnEnrolled(s.cfg.ID); err != nil {
				s.mu.Lock()
				s.lastError = fmt.Sprintf("persisting enrollment state: %v", err)
				s.mu.Unlock()
			}
		}
	}

	client := s.cfg.HTTPClient
	if s.cfg.TLSEnrolled && s.cfg.DataDir != "" && mtls.HasCerts(s.cfg.DataDir, s.cfg.ID) {
		tlsClient, err := mtls.HTTPClient(s.cfg.DataDir, s.cfg.ID)
		if err != nil {
			s.mu.Lock()
			s.lastError = fmt.Sprintf("loading mTLS client: %v", err)
			s.mu.Unlock()
			return fmt.Errorf("loading mTLS client: %w", err)
		}
		client = tlsClient
	} else if s.cfg.UseWireGuard && s.cfg.WGClient != nil {
		client = s.cfg.WGClient
	}

	s.mu.RLock()
	etag := s.etag
	s.mu.RUnlock()

	result, err := httputil.FetchConditional(ctx, client, s.cfg.URL, etag, "")
	if err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("fetching tvpstreams playlist: %w", err)
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
		return fmt.Errorf("parsing tvpstreams playlist: %w", err)
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

		stream := entryToStream(s.cfg.ID, id, entry)

		if s.cfg.TMDBCache != nil && stream.TMDBID != "" {
			enrichFromTMDB(s.cfg.TMDBCache, &stream)
		}

		streams = append(streams, stream)
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("upserting streams: %w", err)
	}

	if _, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, "tvpstreams", s.cfg.ID, keepIDs); err != nil {
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
	streams, err := s.cfg.StreamStore.ListBySource(ctx, "tvpstreams", s.cfg.ID)
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
	return s.cfg.StreamStore.DeleteBySource(ctx, "tvpstreams", s.cfg.ID)
}

func (s *Source) SupportsConditionalRefresh() bool {
	return true
}

func (s *Source) UsesVPN() bool {
	return s.cfg.UseWireGuard
}

func (s *Source) SupportsVOD() bool {
	return true
}

func (s *Source) VODTypes() []string {
	return []string{"movie", "series"}
}

func (s *Source) TLSInfo() source.TLSStatus {
	status := source.TLSStatus{Enrolled: s.cfg.TLSEnrolled}
	if s.cfg.TLSEnrolled && s.cfg.DataDir != "" {
		status.Fingerprint = mtls.Fingerprint(s.cfg.DataDir, s.cfg.ID)
	}
	return status
}

func (s *Source) Clear(ctx context.Context) error {
	if err := s.cfg.StreamStore.DeleteBySource(ctx, "tvpstreams", s.cfg.ID); err != nil {
		return err
	}

	s.mu.Lock()
	s.streamCount = 0
	s.etag = ""
	s.lastError = ""
	s.mu.Unlock()

	return nil
}

func entryToStream(sourceID, id string, entry m3u.Entry) media.Stream {
	attrs := entry.Attributes

	vodType := attrs["tvp-type"]
	w, h := parseResolution(attrs["tvp-resolution"])
	season, _ := strconv.Atoi(attrs["tvp-season"])
	episode, _ := strconv.Atoi(attrs["tvp-episode"])

	return media.Stream{
		ID:             id,
		SourceType:     "tvpstreams",
		SourceID:       sourceID,
		Name:           entry.Name,
		URL:            entry.URL,
		Group:          resolveGroup(vodType, entry.Group),
		TvgID:          entry.TvgID,
		TvgName:        entry.TvgName,
		TvgLogo:        entry.TvgLogo,
		IsActive:       true,
		VideoCodec:     attrs["tvp-codec"],
		AudioCodec:     attrs["tvp-audio"],
		Width:          w,
		Height:         h,
		VODType:        vodType,
		TMDBID:         attrs["tvp-tmdb"],
		Year:           attrs["tvp-year"],
		Season:         season,
		Episode:        episode,
		EpisodeName:    attrs["tvp-episode-name"],
		CollectionName: attrs["tvp-collection"],
		CollectionID:   attrs["tvp-collection-id"],
		IsLocal:        attrs["tvp-local"] == "true",
	}
}

func enrichFromTMDB(cache *tmdb.Cache, stream *media.Stream) {
	if stream.VODType == "movie" {
		if movie, ok := cache.GetMovie(stream.TMDBID); ok {
			if stream.TvgLogo == "" && movie.PosterPath != "" {
				stream.TvgLogo = tmdbImageBase + movie.PosterPath
			}
		}
	} else if stream.VODType == "episode" || stream.VODType == "series" {
		if series, ok := cache.GetSeries(stream.TMDBID); ok {
			if stream.TvgLogo == "" && series.PosterPath != "" {
				stream.TvgLogo = tmdbImageBase + series.PosterPath
			}
		}
	}
}

func parseResolution(res string) (int, int) {
	switch strings.ToLower(res) {
	case "4k", "2160p":
		return 3840, 2160
	case "1080p":
		return 1920, 1080
	case "720p":
		return 1280, 720
	case "480p":
		return 854, 480
	case "360p":
		return 640, 360
	default:
		return 0, 0
	}
}

func resolveGroup(vodType, original string) string {
	if original != "" {
		return original
	}
	switch vodType {
	case "movie":
		return "Movies"
	case "series", "episode":
		return "TV Series"
	default:
		return original
	}
}

func deterministicStreamID(sourceID, url string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + url))
	return fmt.Sprintf("%x", h[:16])
}
