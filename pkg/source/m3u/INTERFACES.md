# M3U Source Plugin Interfaces

## Constructor
```go
type Config struct {
    ID           string
    Name         string
    URL          string
    IsEnabled    bool
    UseWireGuard bool
    MaxStreams   int
    UserAgent    string
    StreamStore  store.StreamStore
    HTTPClient   *http.Client    // regular client
    WGClient     *http.Client    // WireGuard client (nil if not used)
}

func New(cfg Config) *Source
```

## Implements source.Source
```go
func (s *Source) Info(ctx context.Context) source.SourceInfo
func (s *Source) Refresh(ctx context.Context) error
func (s *Source) Streams(ctx context.Context) ([]string, error)
func (s *Source) DeleteStreams(ctx context.Context) error
func (s *Source) Type() source.SourceType  // returns "m3u"
```

## Implements source.ConditionalRefresher
```go
func (s *Source) SupportsConditionalRefresh() bool  // true
```

## Implements source.VPNRoutable
```go
func (s *Source) UsesVPN() bool  // returns UseWireGuard
```

## Implements source.Clearable
```go
func (s *Source) Clear(ctx context.Context) error
```

## Refresh Flow
1. Select HTTP client (WG or regular) based on UseWireGuard
2. Fetch URL with ETag conditional
3. If 304 Not Modified → skip, update status
4. Parse M3U response
5. Convert entries to media.Stream (set SourceType="m3u", SourceID=config.ID)
6. BulkUpsert to StreamStore
7. Update status (stream count, last refreshed)
