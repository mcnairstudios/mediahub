# cache -- Interfaces

## Cache

Generic key-value cache with typed registration.

```go
type Cache interface {
    Type() CacheType
    Get(ctx context.Context, key string) (any, bool)
    Set(ctx context.Context, key string, value any) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
}
```

| Method | Description |
|--------|-------------|
| `Type` | Return the cache type identifier (tmdb, probe, logo) |
| `Get` | Retrieve a cached value by key |
| `Set` | Store a value under the given key |
| `Delete` | Remove a single cached entry |
| `Clear` | Remove all entries from this cache |

Caches are registered in a `Registry` which holds one `Cache` per `CacheType`.

### Registry public API

| Method | Signature | Description |
|--------|-----------|-------------|
| `Register` | `(c Cache)` | Add a cache to the registry |
| `Get` | `(cacheType CacheType) (Cache, bool)` | Retrieve a cache by type |
| `Types` | `() []CacheType` | List all registered cache types |
