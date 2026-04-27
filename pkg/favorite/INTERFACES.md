# Favorite Interfaces

## Store

```go
type Store interface {
    List(ctx context.Context, userID string) ([]Favorite, error)
    Add(ctx context.Context, userID, streamID string) error
    Remove(ctx context.Context, userID, streamID string) error
    IsFavorite(ctx context.Context, userID, streamID string) (bool, error)
}
```

## Types

```go
type Favorite struct {
    StreamID string    `json:"stream_id"`
    UserID   string    `json:"user_id"`
    AddedAt  time.Time `json:"added_at"`
}
```
