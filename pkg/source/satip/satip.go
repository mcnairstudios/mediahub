package satip

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/source/satip/scan"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/rs/zerolog/log"
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
	DiSEqCSource    int
	StreamStore     store.StreamStore
	OnScanProgress  func(done, total, channels int)
}

type Source struct {
	source.BaseSource
	cfg Config
}

func New(cfg Config) *Source {
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = defaultHTTPPort
	}
	return &Source{
		BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, "satip", cfg.IsEnabled, cfg.MaxStreams),
		cfg:        cfg,
	}
}

func (s *Source) SetScanProgress(fn func(done, total, channels int)) {
	s.cfg.OnScanProgress = fn
}

func (s *Source) Refresh(ctx context.Context) error {
	host := s.cfg.Host
	if host == "" {
		return fmt.Errorf("satip source %s: host not configured", s.cfg.ID)
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, "554")
	}

	cfg := scan.Config{
		SeedTimeout:     60 * time.Second,
		MuxTimeout:      60 * time.Second,
		Timeout:         60 * time.Second,
		Parallel:        s.cfg.MaxStreams,
		TransmitterFile: s.cfg.TransmitterFile,
		DiSEqCSource:    s.cfg.DiSEqCSource,
		Log:             log.With().Str("source", s.cfg.ID).Logger(),
		OnMuxScanned: func(done, total int) {
			if s.cfg.OnScanProgress != nil {
				s.cfg.OnScanProgress(done, total, 0)
			}
		},
	}

	result, err := scan.Scan(host, s.cfg.HTTPPort, cfg)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("satip scan: %w", err)
	}

	var streams []media.Stream
	for _, ch := range result.Channels {
		rtspURL := ch.RTSPURL(host)
		stream := media.Stream{
			ID:         deterministicStreamID(s.cfg.ID, ch.ServiceID),
			SourceType: "satip",
			SourceID:   s.cfg.ID,
			Name:       ch.Name,
			URL:        rtspURL,
			Group:      streamGroup(ch.ServiceType),
			IsActive:   !ch.Encrypted,
			Encrypted:  ch.Encrypted,
		}
		streams = append(streams, stream)
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("upserting streams: %w", err)
	}

	keepIDs := make([]string, len(streams))
	for i, st := range streams {
		keepIDs[i] = st.ID
	}
	if _, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, "satip", s.cfg.ID, keepIDs); err != nil {
		log.Warn().Err(err).Str("source", s.cfg.ID).Msg("failed to delete stale streams")
	}

	s.SetRefreshResult(len(streams))
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

	s.ClearState()
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
