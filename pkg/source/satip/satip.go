package satip

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

const defaultHTTPPort = 8875

type Config struct {
	ID              string
	Name            string
	Host            string
	HTTPPort        int
	IsEnabled       bool
	MaxStreams      int
	TransmitterFile string
	StreamStore     store.StreamStore
}

type Source struct {
	cfg           Config
	streamCount   int
	lastRefreshed *time.Time
	lastError     string
	mu            sync.RWMutex
}

func New(cfg Config) *Source {
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = defaultHTTPPort
	}
	return &Source{cfg: cfg}
}

func (s *Source) Type() source.SourceType {
	return "satip"
}

func (s *Source) Info(_ context.Context) source.SourceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return source.SourceInfo{
		ID:                  s.cfg.ID,
		Type:                "satip",
		Name:                s.cfg.Name,
		IsEnabled:           s.cfg.IsEnabled,
		StreamCount:         s.streamCount,
		LastRefreshed:       s.lastRefreshed,
		LastError:           s.lastError,
		MaxConcurrentStreams: s.cfg.MaxStreams,
	}
}

func (s *Source) Refresh(_ context.Context) error {
	return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
	streams, err := s.cfg.StreamStore.ListBySource(ctx, "satip", s.cfg.ID)
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
	return s.cfg.StreamStore.DeleteBySource(ctx, "satip", s.cfg.ID)
}

func (s *Source) Clear(ctx context.Context) error {
	if err := s.cfg.StreamStore.DeleteBySource(ctx, "satip", s.cfg.ID); err != nil {
		return err
	}

	s.mu.Lock()
	s.streamCount = 0
	s.lastError = ""
	s.mu.Unlock()

	return nil
}

func (s *Source) Discover(_ context.Context) ([]source.DiscoveredDevice, error) {
	return nil, nil
}

func deterministicStreamID(sourceID string, serviceID uint16) string {
	content := fmt.Sprintf("%s:%d", sourceID, serviceID)
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:16])
}

func streamGroup(serviceType uint8) string {
	switch serviceType {
	case 0x02, 0x07, 0x0A:
		return "Radio"
	case 0x11, 0x19, 0x1F, 0x20:
		return "HD"
	default:
		return "SD"
	}
}
