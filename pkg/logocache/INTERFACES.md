# logocache interfaces

## Cache

```go
type Cache struct { ... }

func New(cacheDir string) *Cache
func (c *Cache) Resolve(logoURL string) string
func (c *Cache) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

### New

Creates a cache backed by the given directory. Creates the directory if it does not exist. Scans existing files to build an in-memory index on startup.

### Resolve

Rewrites an external logo URL to a local `/logo?url=...` path. Pure function -- no I/O, no network. Returns `Placeholder` for empty or non-HTTP URLs. Passes through local paths (`/...`) and data URIs unchanged.

### ServeHTTP

Implements `http.Handler`. Expects a `url` query parameter containing the external logo URL. Returns 400 if the URL is missing or not HTTP/HTTPS. Returns 502 if the external server fails. On success, serves the cached image with a 24-hour cache header.

## Constants

```go
const Placeholder string  // SVG data URI used when no logo is available
```
