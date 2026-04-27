# m3u -- Public API

No interfaces defined. This package provides M3U playlist parsing.

## Parse

```go
func Parse(r io.Reader) ([]Entry, error)
```

Parse an M3U playlist from a reader. Returns a slice of entries, each with name, URL, group, tvg-id, tvg-name, tvg-logo, duration, and arbitrary attributes extracted from `#EXTINF` lines.
