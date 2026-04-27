package channel

import "context"

type Channel struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Number     int      `json:"number"`
	GroupID    string   `json:"group_id,omitempty"`
	StreamIDs  []string `json:"stream_ids,omitempty"`
	LogoURL    string   `json:"logo_url,omitempty"`
	TvgID      string   `json:"tvg_id,omitempty"`
	IsEnabled  bool     `json:"is_enabled"`
	IsFavorite bool     `json:"is_favorite"`
}

type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Store interface {
	Get(ctx context.Context, id string) (*Channel, error)
	List(ctx context.Context) ([]Channel, error)
	Create(ctx context.Context, ch *Channel) error
	Update(ctx context.Context, ch *Channel) error
	Delete(ctx context.Context, id string) error
	AssignStreams(ctx context.Context, channelID string, streamIDs []string) error
	RemoveStreamMappings(ctx context.Context, streamIDs []string) error
}

type GroupStore interface {
	List(ctx context.Context) ([]Group, error)
	Create(ctx context.Context, g *Group) error
	Delete(ctx context.Context, id string) error
}
