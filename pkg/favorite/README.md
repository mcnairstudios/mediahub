# favorite

Per-user stream favorites. Users can star/unstar streams. Favorites are isolated per user.

## Store Interface

```go
type Store interface {
    List(ctx context.Context, userID string) ([]Favorite, error)
    Add(ctx context.Context, userID, streamID string) error
    Remove(ctx context.Context, userID, streamID string) error
    IsFavorite(ctx context.Context, userID, streamID string) (bool, error)
}
```

## Implementations

| Backend | Package | Notes |
|---------|---------|-------|
| Memory | `pkg/favorite` | For testing |
| Bolt | `pkg/store/bolt` | Nested buckets: `favorites/{userID}/{streamID}` |

## API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/favorites` | user | List user's favorites |
| POST | `/api/favorites` | user | Add favorite (body: `{"stream_id": "..."}`) |
| DELETE | `/api/favorites/{streamID}` | user | Remove favorite |
| GET | `/api/favorites/check/{streamID}` | user | Check if stream is favorited |

## Behavior

- Add is idempotent (adding twice is a no-op)
- Remove is idempotent (removing non-existent returns 204)
- Favorites are per-user (users cannot see each other's favorites)
