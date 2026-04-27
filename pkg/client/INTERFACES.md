# client -- Public API

This package defines types only (no interfaces). The `Detector` struct provides header-based client detection.

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
