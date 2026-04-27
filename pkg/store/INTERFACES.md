# Store Interfaces

## Design
Stores are pluggable. Each domain has its own interface. Backends are swappable — bolt, SQLite, JSON files, in-memory. Different stores can use different backends based on their access patterns.

## Store Interfaces

### StreamStore (pkg/store)
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

### SettingsStore (pkg/store)
```go
type SettingsStore interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
    List(ctx context.Context) (map[string]string, error)
}
```

### Other Domain Stores
| Interface | Package | Key Methods |
|-----------|---------|-------------|
| `channel.Store` | pkg/channel | Get, List, Create, Update, Delete, AssignStreams, RemoveStreamMappings |
| `channel.GroupStore` | pkg/channel | List, Create, Delete |
| `epg.SourceStore` | pkg/epg | Get, List, Create, Update, Delete |
| `epg.ProgramStore` | pkg/epg | NowPlaying, Range, BulkInsert, DeleteBySource |
| `recording.Store` | pkg/recording | Get, List, Create, Update, Delete, ListByStatus, ListScheduled |
| `auth.UserStore` | pkg/auth | Get, GetByUsername, List, Create, Delete, UpdatePassword |
| `favorite.Store` | pkg/favorite | List, Add, Remove, IsFavorite |

## Backend Factory

```go
type BackendType string

const (
    BackendMemory BackendType = "memory"
    BackendBolt   BackendType = "bolt"
    BackendJSON   BackendType = "json"
    BackendSQLite BackendType = "sqlite"
)

type Factory interface {
    StreamStore(backend BackendType) (StreamStore, error)
    SettingsStore(backend BackendType) (SettingsStore, error)
    ChannelStore(backend BackendType) (channel.Store, error)
    GroupStore(backend BackendType) (channel.GroupStore, error)
    EPGSourceStore(backend BackendType) (epg.SourceStore, error)
    ProgramStore(backend BackendType) (epg.ProgramStore, error)
    RecordingStore(backend BackendType) (recording.Store, error)
    UserStore(backend BackendType) (auth.UserStore, error)
    FavoriteStore(backend BackendType) (favorite.Store, error)
}
```

## Implementations

| Backend | Best For | Status |
|---------|----------|--------|
| Memory | Testing, MVP | Done (pkg/store/memory.go + domain memory stores) |
| Bolt | Streams, channels, settings (fast k-v) | Done (pkg/store/bolt/ — StreamStore + SettingsStore) |
| JSON | Small configs, portability | Future |
| SQLite | EPG programs (time-range queries, bulk) | Future |

## Usage in main.go
```go
factory := store.NewFactory(cfg.DataDir)
streamStore, _ := factory.StreamStore(store.BackendBolt)
settingsStore, _ := factory.SettingsStore(store.BackendJSON)
programStore, _ := factory.ProgramStore(store.BackendSQLite)
// Mix and match backends per store
```
