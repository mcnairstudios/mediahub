# Trailers Source Plugin Interfaces

## Constructor
```go
type Config struct {
    ID          string
    Name        string
    IsEnabled   bool
    TMDBKey     string
    StreamStore store.StreamStore
    HTTPClient  *http.Client    // defaults to 30s timeout if nil
}

func New(cfg Config) *Source
```

## Implements source.Source
```go
func (s *Source) Info(ctx context.Context) source.SourceInfo
func (s *Source) Refresh(ctx context.Context) error
func (s *Source) Streams(ctx context.Context) ([]string, error)
func (s *Source) DeleteStreams(ctx context.Context) error
func (s *Source) Type() source.SourceType  // returns "trailers"
```

## Refresh Flow
1. Validate TMDBKey is set (error if empty)
2. Fetch upcoming movies from TMDB `/movie/upcoming`
3. Fetch now-playing movies from TMDB `/movie/now_playing` (append to list, skip on error)
4. For each movie, query `/movie/{id}/videos` for YouTube Trailer (fallback to Teaser)
5. Build media.Stream with deterministic ID, poster URL, year, group="Trailers"
6. BulkUpsert to StreamStore
7. DeleteStaleBySource to remove entries no longer in TMDB results
8. Update status (stream count, last refreshed)
