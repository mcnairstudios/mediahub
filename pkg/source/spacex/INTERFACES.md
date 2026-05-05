# SpaceX Source Plugin Interfaces

## Constructor
```go
type Config struct {
    ID            string
    Name          string
    IsEnabled     bool
    StreamStore   store.StreamStore
    HTTPClient    *http.Client    // defaults to 30s timeout if nil
    OnRefreshDone func(sourceID, etag string, streamCount int)
}

func New(cfg Config) *Source
```

## Implements source.Source
```go
func (s *Source) Info(ctx context.Context) source.SourceInfo
func (s *Source) Refresh(ctx context.Context) error
func (s *Source) Streams(ctx context.Context) ([]string, error)
func (s *Source) DeleteStreams(ctx context.Context) error
func (s *Source) Type() source.SourceType  // returns "spacex"
```

## Refresh Flow
1. Fetch past launches (paginated, up to 10 pages of 50) from Launch Library 2 API (`/launch/previous/`)
2. Fetch upcoming launches (paginated, up to 5 pages of 50) from Launch Library 2 API (`/launch/upcoming/`)
3. Rate-limit-respectful pagination (100ms between pages)
4. Build media.Stream with deterministic ID, video URL, launch image as logo, provider name as Group
5. BulkUpsert to StreamStore
6. DeleteStaleBySource to remove entries no longer in API results
7. Call OnRefreshDone callback if set
