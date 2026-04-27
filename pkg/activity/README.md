# pkg/activity

In-memory tracking of active viewers/sessions. Tracks who is watching what, via which delivery mode, and for how long.

## Usage

```go
svc := activity.New()

svc.Add(&activity.Viewer{
    SessionID:  "sess-123",
    StreamID:   "stream-456",
    StreamName: "BBC One",
    UserID:     "user-1",
    Username:   "admin",
    Delivery:   "mse",
    StartedAt:  time.Now(),
    RemoteAddr: "192.168.1.10:54321",
})

viewers := svc.List()   // all active viewers
count := svc.Count()    // active viewer count
svc.Remove("sess-123")  // viewer left
```

## API

- `GET /api/activity` — admin only, returns active viewers with computed duration

## Design

- Keyed by session ID (one viewer per session)
- Adding with an existing session ID overwrites (idempotent)
- Removing a nonexistent session ID is a no-op
- Thread-safe via sync.RWMutex
- No persistence — state is ephemeral and resets on restart
