package channel

import "context"

type Channel struct {
	ID         string
	Name       string
	Number     int
	GroupID    string
	StreamIDs  []string
	LogoURL    string
	TvgID      string
	IsEnabled  bool
	IsFavorite bool
}

type Group struct {
	ID   string
	Name string
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
