# tmdb -- Interfaces

## Client

TMDB API client with in-memory caching.

```go
func NewClient(apiKeyFn func() string, cache *tmdbcache.Cache) *Client
```

### Search

```go
func (c *Client) SearchMovie(query string, year int) (*tmdbcache.Movie, error)
func (c *Client) SearchTV(query string) (*tmdbcache.Series, error)
```

| Method | Description |
|--------|-------------|
| `SearchMovie` | Search TMDB for a movie by title and optional year. Returns the first match with full detail (cast, crew, genres). Cached by query+year. |
| `SearchTV` | Search TMDB for a TV series. Cleans VOD name (strips year/edition tags). Returns first match with seasons. Cached by query. |

### Detail

```go
func (c *Client) MovieDetail(tmdbID int) (*tmdbcache.Movie, error)
func (c *Client) TVDetail(tmdbID int) (*tmdbcache.Series, error)
func (c *Client) SeasonDetail(tvID, seasonNum int) (*tmdbcache.Season, error)
```

| Method | Description |
|--------|-------------|
| `MovieDetail` | Full movie detail: title, overview, poster, backdrop, rating, runtime, genres, certification, cast (top 20), key crew (director/writer). |
| `TVDetail` | Full TV series detail: name, overview, poster, backdrop, rating, genres, season list. |
| `SeasonDetail` | Season with episode list (number, name, overview, still, air date, runtime). |

### Sync

```go
func (c *Client) SyncStream(streamName, mediaType, tmdbIDStr string)
func (c *Client) SyncBatch(items []SyncItem)
```

Background metadata sync for streams. `SyncBatch` runs in a goroutine with 250ms rate limiting.

### Utility

```go
func ImageURL(path string, size string) string
```

Constructs a TMDB CDN image URL. Empty path returns empty string. Default size is `w500`.

## ImageCache

Disk-backed HTTP handler for TMDB images.

```go
func NewImageCache(dir string) *ImageCache
func (ic *ImageCache) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

Query params: `path` (TMDB image path), `size` (default w500). Fetches on first request, serves from disk cache after. Immutable `Cache-Control` headers.
