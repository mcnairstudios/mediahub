package recording

import (
	"context"
	"time"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusRecording Status = "recording"
	StatusCompleted Status = "completed"
	StatusScheduled Status = "scheduled"
	StatusFailed    Status = "failed"
)

type Recording struct {
	ID             string
	StreamID       string
	StreamName     string
	ChannelID      string
	ChannelName    string
	Title          string
	UserID         string
	Status         Status
	StartedAt      time.Time
	StoppedAt      time.Time
	ScheduledStart time.Time
	ScheduledStop  time.Time
	FilePath       string
	FileSize       int64
	Container      string
	VideoCodec     string
	AudioCodec     string
}

type Store interface {
	Get(ctx context.Context, id string) (*Recording, error)
	List(ctx context.Context, userID string, isAdmin bool) ([]Recording, error)
	Create(ctx context.Context, r *Recording) error
	Update(ctx context.Context, r *Recording) error
	Delete(ctx context.Context, id string) error
	ListByStatus(ctx context.Context, status Status) ([]Recording, error)
	ListScheduled(ctx context.Context) ([]Recording, error)
}
