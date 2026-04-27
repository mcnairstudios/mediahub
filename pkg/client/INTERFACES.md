# client -- Public API

## Store

CRUD for client configurations.

```go
type Store interface {
    Get(ctx context.Context, id string) (*Client, error)
    List(ctx context.Context) ([]Client, error)
    Create(ctx context.Context, c *Client) error
    Update(ctx context.Context, c *Client) error
    Delete(ctx context.Context, id string) error
}
```

| Method | Description |
|--------|-------------|
| `Get` | Retrieve a client by ID |
| `List` | Return all clients |
| `Create` | Persist a new client |
| `Update` | Update an existing client |
| `Delete` | Remove a client by ID |

Implemented by: `store/bolt.ClientStore`

---

## Detector

Matches incoming requests to configured clients by port and header rules.

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewDetector` | `(clients []Client) *Detector` | Create a detector with clients sorted by priority (descending) |
| `Detect` | `(port int, headers map[string]string) *Client` | Return the first matching client, or nil |

## Match (package-level)

```go
func Match(c Client, port int, headers map[string]string) bool
```

Test whether a single client matches the given port and headers. Rules use AND logic -- all must match.

---

## Types

### Client

```go
type Client struct {
    ID         string      `json:"id"`
    Name       string      `json:"name"`
    Priority   int         `json:"priority"`
    ListenPort int         `json:"listen_port,omitempty"`
    IsEnabled  bool        `json:"is_enabled"`
    IsSystem   bool        `json:"is_system"`
    MatchRules []MatchRule `json:"match_rules,omitempty"`
    Profile    Profile     `json:"profile"`
}
```

### Profile

```go
type Profile struct {
    Delivery     string `json:"delivery"`
    VideoCodec   string `json:"video_codec"`
    AudioCodec   string `json:"audio_codec"`
    Container    string `json:"container"`
    HWAccel      string `json:"hwaccel"`
    OutputHeight int    `json:"output_height,omitempty"`
}
```

### MatchRule

```go
type MatchRule struct {
    HeaderName string `json:"header_name"`
    MatchType  string `json:"match_type"`
    MatchValue string `json:"match_value"`
}
```
