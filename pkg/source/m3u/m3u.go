package m3u

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"strconv"
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
	WGProfileID  string
	MaxStreams   int
	UserAgent    string
	BypassHeader string
	BypassSecret string
	InitialETag  string
	StreamStore  store.StreamStore
	HTTPClient   *http.Client
	WGClient     *http.Client
	OnRefreshDone func(sourceID, etag string, streamCount int)
}

type Source struct {
	source.BaseSource
	cfg  Config
	etag string
}

func New(cfg Config) *Source {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	s := &Source{
		BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, source.TypeM3U, cfg.IsEnabled, cfg.MaxStreams),
		cfg:        cfg,
	}
	if cfg.InitialETag != "" {
		s.etag = cfg.InitialETag
	}
	return s
}

func (s *Source) Refresh(ctx context.Context) error {
	client := source.HTTPClientFor(s.cfg.HTTPClient, s.cfg.WGClient, s.cfg.UseWireGuard)

	log.Printf("m3u: refreshing source %s from %s (wg=%v)", s.cfg.Name, s.cfg.URL, s.cfg.UseWireGuard)
	log.Printf("m3u: using client (timeout=%v)", client.Timeout)

	etag := s.etag

	var extraHeaders map[string]string
	if s.cfg.BypassHeader != "" && s.cfg.BypassSecret != "" {
		extraHeaders = map[string]string{s.cfg.BypassHeader: s.cfg.BypassSecret}
	}

	fetchStart := time.Now()
	result, err := httputil.FetchConditional(ctx, client, s.cfg.URL, etag, s.cfg.UserAgent, extraHeaders)
	fetchDuration := time.Since(fetchStart)
	if err != nil {
		log.Printf("m3u: fetch error for %s after %s: %v", s.cfg.Name, fetchDuration, err)
		s.SetError(err.Error())
		return fmt.Errorf("fetching m3u: %w", err)
	}

	log.Printf("m3u: fetch completed for %s in %s (changed=%v etag=%q)", s.cfg.Name, fetchDuration, result.Changed, result.ETag)

	if !result.Changed {
		log.Printf("m3u: 304 not modified for %s", s.cfg.Name)
		s.SetRefreshed()
		return nil
	}
	defer result.Body.Close()

	entries, err := m3u.Parse(result.Body)
	if err != nil {
		log.Printf("m3u: parse error for %s: %v", s.cfg.Name, err)
		s.SetError(err.Error())
		return fmt.Errorf("parsing m3u: %w", err)
	}
	log.Printf("m3u: parsed %d entries for %s", len(entries), s.cfg.Name)

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

		st := media.Stream{
			ID:         id,
			SourceType: string(source.TypeM3U),
			SourceID:   s.cfg.ID,
			Name:       entry.Name,
			URL:        entry.URL,
			Group:      entry.Group,
			TvgID:      entry.TvgID,
			TvgName:    entry.TvgName,
			TvgLogo:    entry.TvgLogo,
			IsActive:   true,
		}

		if tvpType := entry.Attributes["tvp-type"]; tvpType != "" {
			st.VODType = tvpType
			st.IsLocal = true
		}
		if tvpSeries := entry.Attributes["tvp-series"]; tvpSeries != "" {
			st.SeriesName = tvpSeries
		}
		if tvpCollection := entry.Attributes["tvp-collection"]; tvpCollection != "" {
			st.CollectionName = tvpCollection
		}
		if tvpCollectionID := entry.Attributes["tvp-collection-id"]; tvpCollectionID != "" {
			st.CollectionID = tvpCollectionID
		}
		if tvpSeason := entry.Attributes["tvp-season"]; tvpSeason != "" {
			if v, err := strconv.Atoi(tvpSeason); err == nil {
				st.Season = v
			}
		}
		if tvpEpisode := entry.Attributes["tvp-episode"]; tvpEpisode != "" {
			if v, err := strconv.Atoi(tvpEpisode); err == nil {
				st.Episode = v
			}
		}
		st.SeasonName = entry.Attributes["tvp-season-name"]
		st.EpisodeName = entry.Attributes["tvp-episode-name"]
		st.Year = entry.Attributes["tvp-year"]

		var tags []string
		for _, tagKey := range []string{"tvp-vcodec", "tvp-acodec", "tvp-resolution", "tvp-audio"} {
			if v := entry.Attributes[tagKey]; v != "" {
				tags = append(tags, v)
			}
		}
		for k, v := range entry.Attributes {
			if len(k) > 8 && k[:8] == "edition-" {
				tags = append(tags, v)
			}
		}
		if len(tags) > 0 {
			st.Tags = tags
		}

		streams = append(streams, st)
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
		log.Printf("m3u: upsert error for %s: %v", s.cfg.Name, err)
		s.SetError(err.Error())
		return fmt.Errorf("upserting streams: %w", err)
	}

	deleted, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, string(source.TypeM3U), s.cfg.ID, keepIDs)
	if err != nil {
		log.Printf("m3u: delete stale error for %s: %v", s.cfg.Name, err)
		s.SetError(err.Error())
		return fmt.Errorf("deleting stale streams: %w", err)
	}
	log.Printf("m3u: upserted %d streams, deleted %d stale for %s", len(streams), len(deleted), s.cfg.Name)

	s.etag = result.ETag
	s.SetRefreshResult(len(streams))

	if s.cfg.OnRefreshDone != nil {
		s.cfg.OnRefreshDone(s.cfg.ID, result.ETag, len(streams))
	}

	return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
	streams, err := s.cfg.StreamStore.ListBySource(ctx, string(source.TypeM3U), s.cfg.ID)
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
	return s.cfg.StreamStore.DeleteBySource(ctx, string(source.TypeM3U), s.cfg.ID)
}

func (s *Source) SupportsConditionalRefresh() bool {
	return true
}

func (s *Source) UsesVPN() bool {
	return s.cfg.UseWireGuard
}

func (s *Source) Clear(ctx context.Context) error {
	if err := s.cfg.StreamStore.DeleteBySource(ctx, string(source.TypeM3U), s.cfg.ID); err != nil {
		return err
	}

	s.etag = ""
	s.ClearState()
	return nil
}

func deterministicStreamID(sourceID, url string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + url))
	return fmt.Sprintf("%x", h[:16])
}
