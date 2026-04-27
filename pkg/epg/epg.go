package epg

import (
	"context"
	"time"
)

type Source struct {
	ID            string
	Name          string
	URL           string
	IsEnabled     bool
	UseWireGuard  bool
	LastRefreshed *time.Time
	ChannelCount  int
	ProgramCount  int
	LastError     string
	ETag          string
}

type Program struct {
	ChannelID   string
	Title       string
	Subtitle    string
	Description string
	StartTime   time.Time
	EndTime     time.Time
	Categories  []string
	Rating      string
	EpisodeNum  string
	IsNew       bool
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
