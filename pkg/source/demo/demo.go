package demo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

type demoStream struct {
	Name    string
	URL     string
	Group   string
	VODType string
}

var demoStreams = []demoStream{
	{Name: "Big Buck Bunny", URL: "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4", Group: "Demo - Movies", VODType: "movie"},
	{Name: "Sintel", URL: "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/Sintel.mp4", Group: "Demo - Movies", VODType: "movie"},
	{Name: "Tears of Steel", URL: "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/TearsOfSteel.mp4", Group: "Demo - Movies", VODType: "movie"},
	{Name: "Elephant's Dream", URL: "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ElephantsDream.mp4", Group: "Demo - Movies", VODType: "movie"},
	{Name: "NASA Live", URL: "https://ntv1.akamaized.net/hls/live/2014075/NASA-NTV1-HLS/master.m3u8", Group: "Demo - Live", VODType: ""},
	{Name: "Bloomberg TV", URL: "https://www.bloomberg.com/media-manifest/streams/us.m3u8", Group: "Demo - Live", VODType: ""},
}

type Config struct {
	ID            string
	Name          string
	IsEnabled     bool
	StreamStore   store.StreamStore
	OnRefreshDone func(sourceID, etag string, streamCount int)
}

type Source struct {
	source.BaseSource
	cfg Config
}

func New(cfg Config) *Source {
	return &Source{
		BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, source.TypeDemo, cfg.IsEnabled, 0),
		cfg:        cfg,
	}
}

func (s *Source) Refresh(ctx context.Context) error {
	log.Printf("demo: refreshing source %s", s.cfg.Name)

	var streams []media.Stream
	var keepIDs []string

	for _, ds := range demoStreams {
		id := deterministicStreamID(s.cfg.ID, ds.URL)
		keepIDs = append(keepIDs, id)

		streams = append(streams, media.Stream{
			ID:         id,
			SourceType: string(source.TypeDemo),
			SourceID:   s.cfg.ID,
			Name:       ds.Name,
			URL:        ds.URL,
			Group:      ds.Group,
			VODType:    ds.VODType,
			IsActive:   true,
		})
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("upserting streams: %w", err)
	}

	deleted, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, string(source.TypeDemo), s.cfg.ID, keepIDs)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("deleting stale streams: %w", err)
	}
	log.Printf("demo: upserted %d streams, deleted %d stale for %s", len(streams), len(deleted), s.cfg.Name)

	s.SetRefreshResult(len(streams))
	if s.cfg.OnRefreshDone != nil {
		s.cfg.OnRefreshDone(s.cfg.ID, "", len(streams))
	}
	return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
	streams, err := s.cfg.StreamStore.ListBySource(ctx, string(source.TypeDemo), s.cfg.ID)
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
	return s.cfg.StreamStore.DeleteBySource(ctx, string(source.TypeDemo), s.cfg.ID)
}

func deterministicStreamID(sourceID, url string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + url))
	return fmt.Sprintf("%x", h[:16])
}
