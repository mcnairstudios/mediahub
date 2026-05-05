package source

import (
	"context"
	"time"
)

type SourceType string

const (
	TypeM3U        SourceType = "m3u"
	TypeXtream     SourceType = "xtream"
	TypeSATIP      SourceType = "satip"
	TypeHDHR       SourceType = "hdhr"
	TypeTVPStreams SourceType = "tvpstreams"
	TypeTrailers   SourceType = "trailers"
	TypeDemo       SourceType = "demo"
	TypeSpaceX      SourceType = "spacex"
	TypeRadioGarden SourceType = "radiogarden"
)

const (
	StateIdle     = "idle"
	StateScanning = "scanning"
	StateDone     = "done"
	StateError    = "error"
)

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

type Source interface {
	Info(ctx context.Context) SourceInfo
	Refresh(ctx context.Context) error
	Streams(ctx context.Context) ([]string, error)
	DeleteStreams(ctx context.Context) error
	Type() SourceType
}

type StatusReporter interface {
	RefreshStatus(id string) RefreshStatus
}

type RefreshStatus struct {
	State    string `json:"state"`
	Message  string `json:"message,omitempty"`
	Total    int    `json:"total,omitempty"`
	Progress int    `json:"progress,omitempty"`
}
