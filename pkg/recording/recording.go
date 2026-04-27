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
	ID             string    `json:"id"`
	StreamID       string    `json:"stream_id"`
	StreamName     string    `json:"stream_name,omitempty"`
	ChannelID      string    `json:"channel_id,omitempty"`
	ChannelName    string    `json:"channel_name,omitempty"`
	Title          string    `json:"title"`
	UserID         string    `json:"user_id"`
	Status         Status    `json:"status"`
	StartedAt      time.Time `json:"started_at,omitempty"`
	StoppedAt      time.Time `json:"stopped_at,omitempty"`
	ScheduledStart time.Time `json:"scheduled_start,omitempty"`
	ScheduledStop  time.Time `json:"scheduled_stop,omitempty"`
	FilePath       string    `json:"file_path,omitempty"`
	FileSize       int64     `json:"file_size,omitempty"`
	Container      string    `json:"container,omitempty"`
	VideoCodec     string    `json:"video_codec,omitempty"`
	AudioCodec     string    `json:"audio_codec,omitempty"`
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
