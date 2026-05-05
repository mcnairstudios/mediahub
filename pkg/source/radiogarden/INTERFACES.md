# Radio Garden Source Plugin Interfaces

## Constructor
```go
type Config struct {
    ID            string
    Name          string
    IsEnabled     bool
    PlaceID       string           // Radio Garden place ID (e.g. "0eZoYyEW" for London)
    PlaceName     string           // Display name (e.g. "London")
    StreamStore   store.StreamStore
    HTTPClient    *http.Client     // defaults to 30s timeout if nil
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
func (s *Source) Type() source.SourceType  // returns "radiogarden"
```

## Refresh Flow
1. Fetch channels for the configured PlaceID from `/page/{placeID}/channels`
2. Extract channel ID from each channel's URL path (last segment)
3. Build media.Stream with deterministic ID, Radio Garden listen URL, place name as group
4. BulkUpsert to StreamStore
5. DeleteStaleBySource to remove entries no longer in API results
6. Update status (stream count, last refreshed)

## Stream URL Strategy
Stream URLs are stored as `https://radio.garden/api/ara/content/listen/{channelID}/channel.mp3`. This endpoint returns a 302 redirect to the actual Icecast/Shoutcast MP3 stream URL. The demuxer follows the redirect automatically.
