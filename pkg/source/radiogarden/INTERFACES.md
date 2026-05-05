# Radio Garden Source Plugin Interfaces

## Constructor
```go
type Place struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

type Config struct {
    ID            string
    Name          string
    IsEnabled     bool
    Places        []Place          // multiple Radio Garden places (cities)
    StreamStore   store.StreamStore
    HTTPClient    *http.Client     // defaults to 30s timeout if nil
    OnRefreshDone func(sourceID, etag string, streamCount int)
}

func New(cfg Config) *Source
```

`ParsePlaces(jsonStr string) ([]Place, error)` parses a JSON array string into a slice of Place.

## Implements source.Source
```go
func (s *Source) Info(ctx context.Context) source.SourceInfo
func (s *Source) Refresh(ctx context.Context) error
func (s *Source) Streams(ctx context.Context) ([]string, error)
func (s *Source) DeleteStreams(ctx context.Context) error
func (s *Source) Type() source.SourceType  // returns "radiogarden"
```

## Refresh Flow
1. Iterate over all configured Places
2. For each place, fetch channels from `/page/{placeID}/channels`
3. Extract channel ID from each channel's URL path (last segment)
4. Build media.Stream with deterministic ID, Radio Garden listen URL, place name as Group
5. Deduplicate streams across places (same channel may appear in multiple cities)
6. BulkUpsert to StreamStore
7. DeleteStaleBySource to remove entries no longer in API results
8. Call OnRefreshDone callback if set

## Stream URL Strategy
Stream URLs are stored as `https://radio.garden/api/ara/content/listen/{channelID}/channel.mp3`. This endpoint returns a 302 redirect to the actual Icecast/Shoutcast MP3 stream URL. The demuxer follows the redirect automatically.
