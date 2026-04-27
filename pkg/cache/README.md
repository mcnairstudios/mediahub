# cache

Cache plugin interfaces for mediahub. Cache plugins enrich media items with
metadata from external sources (TMDB, probe data, logos).

## Design

- `Cache` interface: Get/Set/Delete/Clear keyed by string, typed by `CacheType`
- `Registry`: thread-safe registration and lookup of cache implementations
- Zero external dependencies — interfaces and registry only

## Cache Types

| Type    | Purpose                                      |
|---------|----------------------------------------------|
| `tmdb`  | TMDB metadata (posters, synopses, ratings)   |
| `probe` | Stream probe results (codecs, resolution)    |
| `logo`  | Channel/network logo images                  |

## Usage

```go
reg := cache.NewRegistry()
reg.Register(myTMDBCache)

if c, ok := reg.Get(cache.CacheTMDB); ok {
    val, found := c.Get(ctx, "movie:550")
}
```
