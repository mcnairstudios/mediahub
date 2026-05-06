package wasmsource

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/mcnairstudios/mediahub/pkg/wasm"
)

// PluginCaller abstracts the WASM plugin call interface for testability.
type PluginCaller interface {
	Type() string
	CallRefresh(ctx context.Context, configJSON []byte) ([]byte, error)
	CallInteract(ctx context.Context, actionJSON []byte) ([]byte, error)
}

// Config holds the configuration for a WASM-backed source.
type Config struct {
	ID            string
	Name          string
	IsEnabled     bool
	Plugin        PluginCaller
	ConfigJSON    []byte
	StreamStore   store.StreamStore
	OnRefreshDone func(sourceID, etag string, streamCount int)
}

// Source is a source adapter that delegates to a WASM plugin.
type Source struct {
	source.BaseSource
	cfg Config
}

// New creates a new WASM source adapter.
func New(cfg Config) *Source {
	return &Source{
		BaseSource: source.NewBaseSource(
			cfg.ID, cfg.Name,
			source.SourceType(cfg.Plugin.Type()),
			cfg.IsEnabled, 0,
		),
		cfg: cfg,
	}
}

// refreshResult is the JSON structure returned by a plugin's refresh export.
type refreshResult struct {
	Streams []pluginStream `json:"streams"`
}

// pluginStream is a stream entry as returned by a WASM plugin.
type pluginStream struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Group   string `json:"group,omitempty"`
	Logo    string `json:"logo,omitempty"`
	TvgID   string `json:"tvg_id,omitempty"`
	TvgName string `json:"tvg_name,omitempty"`
	VODType string `json:"vod_type,omitempty"`
}

// Refresh calls the WASM plugin's refresh function and stores the resulting streams.
func (s *Source) Refresh(ctx context.Context) error {
	pluginType := s.cfg.Plugin.Type()
	log.Printf("wasm[%s]: refreshing source %s", pluginType, s.cfg.Name)

	data, err := s.cfg.Plugin.CallRefresh(ctx, s.cfg.ConfigJSON)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("wasm refresh: %w", err)
	}

	var result refreshResult
	if err := json.Unmarshal(data, &result); err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("parsing wasm refresh result: %w", err)
	}

	streams := make([]media.Stream, 0, len(result.Streams))
	keepIDs := make([]string, 0, len(result.Streams))

	for _, ps := range result.Streams {
		id := DeterministicStreamID(s.cfg.ID, ps.URL)
		keepIDs = append(keepIDs, id)

		streams = append(streams, media.Stream{
			ID:         id,
			SourceType: pluginType,
			SourceID:   s.cfg.ID,
			Name:       ps.Name,
			URL:        ps.URL,
			Group:      ps.Group,
			TvgID:      ps.TvgID,
			TvgName:    ps.TvgName,
			TvgLogo:    ps.Logo,
			VODType:    ps.VODType,
			IsActive:   true,
		})
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("upserting streams: %w", err)
	}

	deleted, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, pluginType, s.cfg.ID, keepIDs)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("deleting stale streams: %w", err)
	}

	log.Printf("wasm[%s]: upserted %d streams, deleted %d stale for %s",
		pluginType, len(streams), len(deleted), s.cfg.Name)

	s.SetRefreshResult(len(streams))
	if s.cfg.OnRefreshDone != nil {
		s.cfg.OnRefreshDone(s.cfg.ID, "", len(streams))
	}

	return nil
}

// Streams returns the IDs of all streams belonging to this source.
func (s *Source) Streams(ctx context.Context) ([]string, error) {
	streams, err := s.cfg.StreamStore.ListBySource(ctx, s.cfg.Plugin.Type(), s.cfg.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(streams))
	for i, st := range streams {
		ids[i] = st.ID
	}
	return ids, nil
}

// DeleteStreams removes all streams belonging to this source.
func (s *Source) DeleteStreams(ctx context.Context) error {
	return s.cfg.StreamStore.DeleteBySource(ctx, s.cfg.Plugin.Type(), s.cfg.ID)
}

// DeterministicStreamID generates a stable stream ID from source ID and URL.
func DeterministicStreamID(sourceID, url string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + url))
	return fmt.Sprintf("%x", h[:16])
}

// Ensure WASMPlugin satisfies PluginCaller at compile time.
var _ PluginCaller = (*wasm.WASMPlugin)(nil)
