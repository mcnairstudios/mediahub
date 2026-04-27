package epg

import (
	"context"
	"time"
)

type Source struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	IsEnabled     bool       `json:"is_enabled"`
	UseWireGuard  bool       `json:"use_wireguard"`
	LastRefreshed *time.Time `json:"last_refreshed,omitempty"`
	ChannelCount  int        `json:"channel_count"`
	ProgramCount  int        `json:"program_count"`
	LastError     string     `json:"last_error,omitempty"`
	ETag          string     `json:"etag,omitempty"`
}

type Program struct {
	ChannelID   string    `json:"channel_id"`
	Title       string    `json:"title"`
	Subtitle    string    `json:"subtitle,omitempty"`
	Description string    `json:"description,omitempty"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Categories  []string  `json:"categories,omitempty"`
	Rating      string    `json:"rating,omitempty"`
	EpisodeNum  string    `json:"episode_num,omitempty"`
	IsNew       bool      `json:"is_new"`
}

type SourceStore interface {
	Get(ctx context.Context, id string) (*Source, error)
	List(ctx context.Context) ([]Source, error)
	Create(ctx context.Context, s *Source) error
	Update(ctx context.Context, s *Source) error
	Delete(ctx context.Context, id string) error
}

type ProgramStore interface {
	NowPlaying(ctx context.Context, channelID string) (*Program, error)
	Range(ctx context.Context, channelID string, start, end time.Time) ([]Program, error)
	BulkInsert(ctx context.Context, programs []Program) error
	DeleteBySource(ctx context.Context, sourceID string) error
}
