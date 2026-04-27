package store

import (
	"context"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

type StreamStore interface {
	Get(ctx context.Context, id string) (*media.Stream, error)
	List(ctx context.Context) ([]media.Stream, error)
	ListBySource(ctx context.Context, sourceType, sourceID string) ([]media.Stream, error)
	BulkUpsert(ctx context.Context, streams []media.Stream) error
	DeleteBySource(ctx context.Context, sourceType, sourceID string) error
	DeleteStaleBySource(ctx context.Context, sourceType, sourceID string, keepIDs []string) ([]string, error)
	Save() error
}

type SettingsStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	List(ctx context.Context) (map[string]string, error)
}
