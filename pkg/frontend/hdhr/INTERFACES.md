# hdhr -- Interfaces

## Dependencies

### channel.Store

Used to list enabled channels for the lineup.

```go
type Store interface {
    List(ctx context.Context) ([]Channel, error)
    // other methods not used by hdhr
}
```

| Method | Used by |
|--------|---------|
| `List` | `buildLineup` -- iterates enabled channels to produce lineup entries |

### config.Config

| Field | Used by |
|-------|---------|
| `BaseURL` | All handlers -- base URL for stream URLs and lineup URL |

## Exported Types

### Server

HTTP server exposing HDHR endpoints. Created via `NewServer(channelStore, cfg)`.

| Method | Description |
|--------|-------------|
| `Handler()` | Returns `http.Handler` for mounting into a parent mux or listener |

### DiscoveryResponder

UDP listener for HDHR discovery protocol on port 65001. Created via `NewDiscoveryResponder(baseURL, logger)`.

| Method | Description |
|--------|-------------|
| `Run(ctx)` | Blocking -- listens for discovery packets until context is cancelled |

### Response Types

| Type | Format | Endpoint |
|------|--------|----------|
| `DiscoverResponse` | JSON | `/discover.json` |
| `LineupEntry` | JSON/XML | `/lineup.json`, `/lineup.xml` |
| `LineupStatus` | JSON | `/lineup_status.json` |
| `DeviceXML` | XML | `/device.xml` |
