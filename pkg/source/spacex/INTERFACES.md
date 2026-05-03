# SpaceX Source Plugin Interfaces

## Constructor
```go
type Config struct {
    ID          string
    Name        string
    IsEnabled   bool
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
func (s *Source) Type() source.SourceType  // returns "spacex"
```

## Refresh Flow
1. Fetch all launches from SpaceX API `/v4/launches`
2. Filter to launches with a `youtube_id` in links
3. Build media.Stream with deterministic ID, YouTube URL, mission patch as logo, year from launch date
4. BulkUpsert to StreamStore
5. DeleteStaleBySource to remove entries no longer in API results
6. Update status (stream count, last refreshed)
