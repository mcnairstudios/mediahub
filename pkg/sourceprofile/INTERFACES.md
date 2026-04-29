# sourceprofile -- Interfaces

## Store

CRUD for source profiles.

```go
type Store interface {
    Get(ctx context.Context, id string) (*Profile, error)
    List(ctx context.Context) ([]Profile, error)
    Create(ctx context.Context, p *Profile) error
    Update(ctx context.Context, p *Profile) error
    Delete(ctx context.Context, id string) error
}
```

| Method | Description |
|--------|-------------|
| `Get` | Retrieve a source profile by ID. Returns nil, nil if not found. |
| `List` | Return all source profiles |
| `Create` | Persist a new source profile |
| `Update` | Update an existing source profile |
| `Delete` | Remove a source profile by ID |

## Profile

```go
type Profile struct {
    ID                string `json:"id"`
    Name              string `json:"name"`
    Deinterlace       bool   `json:"deinterlace"`
    DeinterlaceMethod string `json:"deinterlace_method,omitempty"`
    AudioLanguage     string `json:"audio_language,omitempty"`
    SubtitleLanguage  string `json:"subtitle_language,omitempty"`
    RTSPProtocols     string `json:"rtsp_protocols,omitempty"`
    RTSPLatency       int    `json:"rtsp_latency,omitempty"`
    HTTPTimeoutSec    int    `json:"http_timeout_sec,omitempty"`
    HTTPUserAgent     string `json:"http_user_agent,omitempty"`
}
```

## Implementations

| Backend | Package |
|---------|---------|
| Bolt | `pkg/store/bolt` |
