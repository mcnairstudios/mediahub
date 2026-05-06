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

### AccountInfoProvider

```go
type AccountInfoProvider interface {
    GetAccountInfo(ctx context.Context) (any, error)
}
```

Fetch account details from the upstream provider (e.g. Xtream Codes server info, connection limits, VOD/series counts).

---

## StatusReporter

```go
type StatusReporter interface {
    RefreshStatus(id string) RefreshStatus
}
```

Provides refresh progress for long-running operations.

---

## BaseSource

Embeddable struct providing shared state management for source plugins.

```go
type BaseSource struct { /* unexported fields */ }

func NewBaseSource(id, name string, typ SourceType, isEnabled bool, maxStreams int) BaseSource
func (b *BaseSource) Info(ctx context.Context) SourceInfo
func (b *BaseSource) Type() SourceType
func (b *BaseSource) SetRefreshResult(count int)
func (b *BaseSource) SetRefreshed()
func (b *BaseSource) SetError(msg string)
func (b *BaseSource) AddStreamCount(delta int)
func (b *BaseSource) ClearState()
func (b *BaseSource) ID() string
func (b *BaseSource) Name() string
```

| Method | Description |
|--------|-------------|
| `Info` | Return SourceInfo under read lock |
| `Type` | Return the source type identifier |
| `SetRefreshResult` | Set stream count + mark refreshed + clear error |
| `SetRefreshed` | Mark refreshed without changing stream count |
| `SetError` | Record an error string |
| `AddStreamCount` | Atomically add to stream count (for background fetchers) |
| `ClearState` | Reset stream count and error |

---

## HTTPClientFor

```go
func HTTPClientFor(defaultClient, wgClient *http.Client, useWG bool) *http.Client
```

Returns the WireGuard client if `useWG` is true and `wgClient` is non-nil, otherwise returns `defaultClient` (or `http.DefaultClient` if nil).

---

## Registry

Factory registry for creating sources by type.

| Method | Signature | Description |
|--------|-----------|-------------|
| `Register` | `(st SourceType, factory Factory)` | Register a factory for a source type |
| `RegisterPlugin` | `(reg PluginRegistration)` | Register a full plugin (descriptor + optional factory/routes/JS) |
| `Plugin` | `(st SourceType) *PluginRegistration` | Return the registration for a source type, or nil |
| `Plugins` | `() []*PluginRegistration` | Return all registered plugins, sorted by type |
| `Create` | `(ctx context.Context, st SourceType, sourceID string) (Source, error)` | Create a source from its factory |
| `Types` | `() []SourceType` | List all registered source types |

`Factory` signature: `func(ctx context.Context, sourceID string) (Source, error)`

### DefaultRegistry

```go
var DefaultRegistry = NewRegistry()
```

Package-level registry for self-registering plugins. Plugins call `DefaultRegistry.RegisterPlugin()` in their `init()` function to register themselves at import time.

---

## PluginDescriptor

Declares metadata about a source plugin. Serialized to JSON for the frontend.

```go
type PluginDescriptor struct {
    Type         SourceType    `json:"type"`
    Label        string        `json:"label"`
    ShortLabel   string        `json:"short_label"`
    Color        string        `json:"color"`
    Icon         string        `json:"icon,omitempty"`
    Version      string        `json:"version"`
    Description  string        `json:"description,omitempty"`
    ConfigFields []ConfigField `json:"config_fields"`
}
```

| Field | Description |
|-------|-------------|
| `Type` | Unique source type identifier (e.g. `"m3u"`, `"hdhr"`) |
| `Label` | Human-readable name shown in UI (e.g. `"M3U Playlist"`) |
| `ShortLabel` | Abbreviated label for compact views (e.g. `"M3U"`) |
| `Color` | CSS hex color for UI badges (e.g. `"#4caf50"`) |
| `Icon` | Optional icon identifier |
| `Version` | Plugin version string (e.g. `"1.0.0"`) |
| `Description` | Short description shown when adding a source |
| `ConfigFields` | Ordered list of configuration fields for the add/edit form |

---

## ConfigField

Describes a single configuration field rendered in the source add/edit form.

```go
type ConfigField struct {
    Key         string    `json:"key"`
    Label       string    `json:"label"`
    Type        FieldType `json:"type"`
    Required    bool      `json:"required,omitempty"`
    Default     string    `json:"default,omitempty"`
    Placeholder string    `json:"placeholder,omitempty"`
    HelpText    string    `json:"help_text,omitempty"`
    Options     []Option  `json:"options,omitempty"`
    Component   string    `json:"component,omitempty"`
}
```

### FieldType values

| Constant | Value | Description |
|----------|-------|-------------|
| `FieldText` | `"text"` | Single-line text input |
| `FieldPassword` | `"password"` | Masked password input |
| `FieldURL` | `"url"` | URL input with validation |
| `FieldNumber` | `"number"` | Numeric input |
| `FieldBool` | `"bool"` | Toggle/checkbox |
| `FieldSelect` | `"select"` | Dropdown, uses `Options` |
| `FieldHidden` | `"hidden"` | Hidden field, not shown in UI |
| `FieldCustom` | `"custom"` | Custom UI component, uses `Component` |

### Option

```go
type Option struct {
    Value string `json:"value"`
    Label string `json:"label"`
}
```

Value/label pair for `FieldSelect` dropdowns.

---

## PluginRegistration

Bundles everything needed to register a source plugin with the registry.

```go
type PluginRegistration struct {
    Descriptor   PluginDescriptor
    Factory      Factory
    CustomRoutes []CustomRoute
    FrontendJS   []byte
}
```

| Field | Description |
|-------|-------------|
| `Descriptor` | Plugin metadata and config field definitions |
| `Factory` | Optional factory function to create source instances |
| `CustomRoutes` | Optional additional API endpoints provided by the plugin |
| `FrontendJS` | Optional JavaScript bytes served to the browser for custom UI components |

---

## CustomRoute

Defines an additional API endpoint provided by a plugin.

```go
type CustomRoute struct {
    Method  string
    Pattern string // relative path, e.g. "places" -> /api/sources/{type}/places
    Handler any
}
```

| Field | Description |
|-------|-------------|
| `Method` | HTTP method (e.g. `"GET"`, `"POST"`) |
| `Pattern` | Relative path appended to `/api/sources/{type}/` |
| `Handler` | `http.HandlerFunc` stored as `any` to avoid importing `net/http` in this package |
