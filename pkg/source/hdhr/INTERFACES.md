# HDHR Source Plugin Interfaces

## Constructor
```go
type Device struct {
    Host            string
    DeviceID        string
    Model           string
    FirmwareVersion string
    TunerCount      int
}

type Config struct {
    ID          string
    Name        string
    IsEnabled   bool
    Devices     []Device
    StreamStore store.StreamStore
    HTTPClient  *http.Client
}

func New(cfg Config) *Source
```

## Implements source.Source
```go
func (s *Source) Info(ctx context.Context) source.SourceInfo
func (s *Source) Refresh(ctx context.Context) error
func (s *Source) Streams(ctx context.Context) ([]string, error)
func (s *Source) DeleteStreams(ctx context.Context) error
func (s *Source) Type() source.SourceType  // returns "hdhr"
```

## Implements source.Discoverable
```go
func (s *Source) Discover(ctx context.Context) ([]source.DiscoveredDevice, error)
```

## Implements source.Retunable
```go
func (s *Source) Retune(ctx context.Context) error
```

## Implements source.Clearable
```go
func (s *Source) Clear(ctx context.Context) error
```

## Refresh Flow
1. For each device, fetch /discover.json to get device info and lineup URL
2. Fetch /lineup.json (or custom LineupURL from discover response)
3. Filter entries: skip DRM=1 and empty URL
4. Classify group: Radio (no video codec), HD (HD=1), SD (default)
5. Normalize video/audio codecs to lowercase ffmpeg names
6. Convert to media.Stream (SourceType="hdhr", SourceID=config.ID)
7. BulkUpsert to StreamStore, delete stale entries
8. Update status (stream count, last refreshed)

## Discovery Flow
1. UDP broadcast on port 65001 with HDHR discovery packet
2. Collect responding IPs (3-second timeout)
3. Fetch /discover.json from each IP for device metadata
4. Return DiscoveredDevice list with AlreadyAdded flag

## Retune Flow
1. POST /lineup.post?scan=start&source=Antenna to first device
2. Poll /lineup_status.json until ScanInProgress=0
3. Run full Refresh to pick up newly found channels
