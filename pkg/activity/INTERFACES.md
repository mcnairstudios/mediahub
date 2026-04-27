# Activity Interfaces

## Types

```go
type Viewer struct {
    SessionID  string
    StreamID   string
    StreamName string
    UserID     string
    Username   string
    ClientName string
    Delivery   string    // "mse", "hls", "stream"
    StartedAt  time.Time
    RemoteAddr string
}
```

## Service

```go
func New() *Service
func (s *Service) Add(v *Viewer)
func (s *Service) Remove(sessionID string)
func (s *Service) List() []*Viewer
func (s *Service) Count() int
```

## Integration Points

- `api.OrchestratorDeps.Activity` — injected into the API server
- `handleStartPlayback` — adds viewer on successful playback start
- `handleStopPlayback` — removes viewer before session teardown
- `handleListActivity` — admin-only endpoint, computes duration from StartedAt
- Dashboard — shows active count in stat card (admin only)
- Activity page — auto-refreshes every 5 seconds
