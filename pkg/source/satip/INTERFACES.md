# SAT>IP Source Plugin Interfaces

## Constructor
```go
type Config struct {
    ID              string
    Name            string
    Host            string
    HTTPPort        int              // default 8875
    IsEnabled       bool
    MaxStreams      int
    TransmitterFile string           // DVB scan file for frequency data
    StreamStore     store.StreamStore
}

func New(cfg Config) *Source
```

## Implements source.Source
```go
func (s *Source) Info(ctx context.Context) source.SourceInfo
func (s *Source) Refresh(ctx context.Context) error         // stub — full scan later
func (s *Source) Streams(ctx context.Context) ([]string, error)
func (s *Source) DeleteStreams(ctx context.Context) error
func (s *Source) Type() source.SourceType                   // returns "satip"
```

## Implements source.Clearable
```go
func (s *Source) Clear(ctx context.Context) error
```

## Implements source.Discoverable
```go
func (s *Source) Discover(ctx context.Context) ([]source.DiscoveredDevice, error)  // stub
```

## Internal Helpers
```go
func deterministicStreamID(sourceID string, serviceID uint16) string
func streamGroup(serviceType uint8) string  // "SD", "HD", or "Radio"
```

## Refresh Flow (Future)
1. Connect to SAT>IP device at Host:554 (RTSP) and Host:HTTPPort (HTTP)
2. Run DVB-SI scan (NIT for mux list, SDT for services, PMT for PIDs)
3. Build RTSP URLs with tuning parameters
4. Convert to media.Stream (set SourceType="satip", SourceID=config.ID)
5. BulkUpsert to StreamStore, remove stale streams
6. Update status (stream count, last refreshed)
