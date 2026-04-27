# store -- Interfaces

## StreamStore

Persistence layer for media streams.

```go
type StreamStore interface {
    Get(ctx context.Context, id string) (*media.Stream, error)
    List(ctx context.Context) ([]media.Stream, error)
    ListBySource(ctx context.Context, sourceType, sourceID string) ([]media.Stream, error)
    BulkUpsert(ctx context.Context, streams []media.Stream) error
    DeleteBySource(ctx context.Context, sourceType, sourceID string) error
    DeleteStaleBySource(ctx context.Context, sourceType, sourceID string, keepIDs []string) ([]string, error)
    Save() error
}
```

| Method | Description |
|--------|-------------|
| `Get` | Retrieve a stream by ID |
| `List` | Return all streams |
| `ListBySource` | Return streams for a given source type and source ID |
| `BulkUpsert` | Insert or update streams in bulk |
| `DeleteBySource` | Remove all streams for a given source |
| `DeleteStaleBySource` | Remove streams not in keepIDs, return deleted IDs |
| `Save` | Persist current state to disk |

Implemented by: `MemoryStreamStore`

---

## SettingsStore

Key-value store for application settings.

```go
type SettingsStore interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
    List(ctx context.Context) (map[string]string, error)
}
```

| Method | Description |
|--------|-------------|
| `Get` | Retrieve a setting value by key |
| `Set` | Store a setting |
| `List` | Return all settings as a map |

Implemented by: `MemorySettingsStore`
