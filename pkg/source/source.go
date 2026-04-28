// Package source defines the interfaces for media stream sources.
//
// A Source represents any provider of media streams: M3U playlists,
// Xtream Codes accounts, HDHomeRun devices, SAT>IP tuners, or
// tvproxy-streams instances. The package uses optional interfaces
// (Discoverable, Retunable, etc.) so each source type only implements
// the capabilities it actually has.
package source

import (
	"context"
	"time"
)

// SourceType identifies the kind of source (e.g. "m3u", "xtream", "hdhr", "satip").
type SourceType string

// SourceInfo holds the metadata for a configured source.
type SourceInfo struct {
	ID                   string     `json:"id"`
	Type                 SourceType `json:"type"`
	Name                 string     `json:"name"`
	IsEnabled            bool       `json:"is_enabled"`
	StreamCount          int        `json:"stream_count"`
	LastRefreshed        *time.Time `json:"last_refreshed,omitempty"`
	LastError            string     `json:"last_error,omitempty"`
	SourceProfileID      string     `json:"source_profile_id,omitempty"`
	MaxConcurrentStreams int        `json:"max_concurrent_streams,omitempty"`
}

// Source is the core interface that every source type must implement.
type Source interface {
	// Info returns the current metadata for this source.
	Info(ctx context.Context) SourceInfo

	// Refresh fetches the latest stream list from the upstream provider.
	Refresh(ctx context.Context) error

	// Streams returns the IDs of all streams belonging to this source.
	Streams(ctx context.Context) ([]string, error)

	// DeleteStreams removes all streams belonging to this source.
	DeleteStreams(ctx context.Context) error

	// Type returns the source type identifier.
	Type() SourceType
}

// StatusReporter provides refresh progress for long-running operations.
type StatusReporter interface {
	RefreshStatus(id string) RefreshStatus
}

// RefreshStatus represents the progress of an ongoing refresh operation.
type RefreshStatus struct {
	State    string `json:"state"`
	Message  string `json:"message,omitempty"`
	Total    int    `json:"total,omitempty"`
	Progress int    `json:"progress,omitempty"`
}
