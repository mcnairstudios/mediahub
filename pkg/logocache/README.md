# logocache

Logo caching proxy. Fetches channel logos from external URLs on demand, caches them to disk, and serves them locally. Prevents repeated external fetches and handles slow or unreliable logo sources.

## Usage

```go
cache := logocache.New("/config/logocache")

// Rewrite external URL to local proxy URL (no I/O)
localURL := cache.Resolve("http://external.com/logo.png")
// => "/logo?url=http%3A%2F%2Fexternal.com%2Flogo.png"

// Register HTTP handler
mux.HandleFunc("GET /logo", cache.ServeHTTP)
```

## Behavior

- `Resolve()` rewrites external HTTP/HTTPS URLs to `/logo?url=...` format. Empty strings and non-HTTP URLs return an SVG placeholder. Local paths and data URIs pass through unchanged.
- `ServeHTTP` handles `/logo?url=...` requests: checks disk cache first, fetches from external URL on miss, saves to disk, serves the image.
- Cache key: first 16 hex chars of SHA256 hash of the original URL.
- Content-Type: preserved from the original response via file extension detection.
- Cache-Control: `public, max-age=86400` (24 hours).
- Files under 200 bytes are treated as corrupt and removed on startup.

## Route

`GET /logo?url=<encoded-url>` -- public, no auth required.
