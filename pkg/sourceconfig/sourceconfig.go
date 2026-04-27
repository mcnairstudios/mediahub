package sourceconfig

import "context"

type SourceConfig struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	IsEnabled bool              `json:"is_enabled"`
	Config    map[string]string `json:"config"`
}

type Store interface {
	Get(ctx context.Context, id string) (*SourceConfig, error)
	List(ctx context.Context) ([]SourceConfig, error)
	ListByType(ctx context.Context, sourceType string) ([]SourceConfig, error)
	Create(ctx context.Context, sc *SourceConfig) error
	Update(ctx context.Context, sc *SourceConfig) error
	Delete(ctx context.Context, id string) error
}
