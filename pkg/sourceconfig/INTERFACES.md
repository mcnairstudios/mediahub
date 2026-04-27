# sourceconfig -- Interfaces

## Store

CRUD for source configurations with type-based filtering.

```go
type Store interface {
    Get(ctx context.Context, id string) (*SourceConfig, error)
    List(ctx context.Context) ([]SourceConfig, error)
    ListByType(ctx context.Context, sourceType string) ([]SourceConfig, error)
    Create(ctx context.Context, sc *SourceConfig) error
    Update(ctx context.Context, sc *SourceConfig) error
    Delete(ctx context.Context, id string) error
}
```

| Method | Description |
|--------|-------------|
| `Get` | Retrieve a source config by ID. Returns nil, nil if not found. |
| `List` | Return all source configs |
| `ListByType` | Return source configs matching the given type (e.g. "m3u", "satip") |
| `Create` | Persist a new source config |
| `Update` | Update an existing source config. Returns `ErrNotFound` if missing. |
| `Delete` | Remove a source config by ID |

## Implementations

| Backend | Package |
|---------|---------|
| Memory | `pkg/sourceconfig` (`MemoryStore`) |
| Bolt | `pkg/store/bolt` (`SourceConfigStore`) |
