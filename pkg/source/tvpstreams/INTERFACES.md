# tvpstreams -- Interfaces

## Source (implements source.Source)

```go
type Source struct { ... }
```

| Method | Signature | Description |
|--------|-----------|-------------|
| `Type` | `() SourceType` | Returns `"tvpstreams"` |
| `Info` | `(ctx) SourceInfo` | Returns current metadata (ID, name, stream count, last refresh) |
| `Refresh` | `(ctx) error` | Fetch playlist, parse tvp-* attributes, upsert streams, delete stale |
| `Streams` | `(ctx) ([]string, error)` | Return IDs of all streams belonging to this source |
| `DeleteStreams` | `(ctx) error` | Remove all streams belonging to this source |

## Optional Interfaces

| Interface | Method | Returns |
|-----------|--------|---------|
| `ConditionalRefresher` | `SupportsConditionalRefresh()` | `true` |
| `VPNRoutable` | `UsesVPN()` | `Config.UseWireGuard` |
| `VODProvider` | `SupportsVOD()` | `true` |
| `VODProvider` | `VODTypes()` | `["movie", "series"]` |
| `Clearable` | `Clear(ctx)` | Deletes streams, resets etag and count |

## Config

```go
type Config struct {
    ID           string
    Name         string
    URL          string
    IsEnabled    bool
    UseWireGuard bool
    StreamStore  store.StreamStore
    HTTPClient   *http.Client
    WGClient     *http.Client
    TMDBCache    *tmdb.Cache
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `ID` | Yes | Unique source identifier |
| `Name` | Yes | Display name |
| `URL` | Yes | tvproxy-streams M3U playlist URL |
| `IsEnabled` | No | Whether source is active |
| `UseWireGuard` | No | Route through WireGuard tunnel |
| `StreamStore` | Yes | Store for persisting streams |
| `HTTPClient` | No | HTTP client (defaults to http.DefaultClient) |
| `WGClient` | No | WireGuard-routed HTTP client |
| `TMDBCache` | No | TMDB cache for poster enrichment |

## Constructor

```go
func New(cfg Config) *Source
```

## Internal Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `entryToStream` | `(sourceID, id string, entry m3u.Entry) media.Stream` | Convert M3U entry to stream with tvp-* extraction |
| `enrichFromTMDB` | `(cache *tmdb.Cache, stream *media.Stream)` | Fill empty logo from TMDB poster |
| `parseResolution` | `(res string) (int, int)` | Resolution string to width/height |
| `resolveGroup` | `(vodType, original string) string` | Default group from VOD type if empty |
| `deterministicStreamID` | `(sourceID, url string) string` | SHA-256 hash of sourceID:url |
