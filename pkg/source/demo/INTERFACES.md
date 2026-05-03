# Demo Source Plugin Interfaces

## Constructor
```go
type Config struct {
    ID          string
    Name        string
    IsEnabled   bool
    StreamStore store.StreamStore
}

func New(cfg Config) *Source
```

## Implements source.Source
```go
func (s *Source) Info(ctx context.Context) source.SourceInfo
func (s *Source) Refresh(ctx context.Context) error
func (s *Source) Streams(ctx context.Context) ([]string, error)
func (s *Source) DeleteStreams(ctx context.Context) error
func (s *Source) Type() source.SourceType  // returns "demo"
```

## Refresh Flow
1. Iterate hardcoded demo stream list (4 VOD movies + 2 live streams)
2. Build media.Stream for each with deterministic ID, group ("Demo - Movies" or "Demo - Live")
3. BulkUpsert to StreamStore
4. DeleteStaleBySource to remove entries no longer in the hardcoded list
5. Update status (stream count, last refreshed)
