# source -- Interfaces

## Source

Core interface every source type must implement.

```go
type Source interface {
    Info(ctx context.Context) SourceInfo
    Refresh(ctx context.Context) error
    Streams(ctx context.Context) ([]string, error)
    DeleteStreams(ctx context.Context) error
    Type() SourceType
}
```

| Method | Description |
|--------|-------------|
| `Info` | Return current metadata for this source |
| `Refresh` | Fetch the latest stream list from upstream |
| `Streams` | Return IDs of all streams belonging to this source |
| `DeleteStreams` | Remove all streams belonging to this source |
| `Type` | Return the source type identifier |

---

## Optional Interfaces

Sources implement these only when they support the capability.

### Discoverable

```go
type Discoverable interface {
    Discover(ctx context.Context) ([]DiscoveredDevice, error)
}
```

Network discovery of devices (HDHomeRun SSDP, SAT>IP tuners).

### Retunable

```go
type Retunable interface {
    Retune(ctx context.Context) error
}
```

Retune the connection without a full refresh (SAT>IP frequency changes).

### ConditionalRefresher

```go
type ConditionalRefresher interface {
    SupportsConditionalRefresh() bool
}
```

Supports ETag/If-Modified-Since to skip unchanged refreshes.

### VPNRoutable

```go
type VPNRoutable interface {
    UsesVPN() bool
}
```

Streams must be routed through a VPN tunnel.

### VODProvider

```go
type VODProvider interface {
    SupportsVOD() bool
    VODTypes() []string
}
```

Source provides video-on-demand content alongside live streams.

### EPGProvider

```go
type EPGProvider interface {
    ProvidesEPG() bool
}
```

Source provides its own EPG data.

### Clearable

```go
type Clearable interface {
    Clear(ctx context.Context) error
}
```

Clear all cached data and state without deleting the source configuration.

---

## StatusReporter

```go
type StatusReporter interface {
    RefreshStatus(id string) RefreshStatus
}
```

Provides refresh progress for long-running operations.

---

## Registry

Factory registry for creating sources by type.

| Method | Signature | Description |
|--------|-----------|-------------|
| `Register` | `(st SourceType, factory Factory)` | Register a factory for a source type |
| `Create` | `(ctx context.Context, st SourceType, sourceID string) (Source, error)` | Create a source from its factory |
| `Types` | `() []SourceType` | List all registered source types |

`Factory` signature: `func(ctx context.Context, sourceID string) (Source, error)`
