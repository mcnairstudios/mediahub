# DLNA Frontend Interfaces

## Required (consumed by this package)

### ChannelLister

Provides channel and group data for DIDL-Lite browse responses.

```go
type ChannelLister interface {
    ListChannels(ctx context.Context) ([]ChannelItem, error)
    GetChannel(ctx context.Context, id string) (*ChannelItem, error)
    ListGroups(ctx context.Context) ([]GroupItem, error)
}
```

Implementor adapts from `channel.Store` and `channel.GroupStore`.

### SettingsChecker

Controls whether DLNA is active. When disabled, all HTTP endpoints return 404 and SSDP stops advertising.

```go
type SettingsChecker interface {
    IsEnabled(ctx context.Context) bool
}
```

Implementor reads from `store.SettingsStore` (key: `dlna_enabled`).

## Provided (exported by this package)

### Server

- `NewServer(channels ChannelLister, settings SettingsChecker, baseURL string, port int, log zerolog.Logger) *Server`
- `RegisterRoutes(mux *http.ServeMux)` -- registers all DLNA HTTP routes
- `UDN() string` -- deterministic UUID for SSDP advertisement
- Individual handler methods: `DeviceDescription`, `ContentDirectorySCPD`, `ConnectionManagerSCPD`, `ContentDirectoryControl`, `ConnectionManagerControl`

### SSDPAdvertiser

- `NewSSDPAdvertiser(server *Server, baseURL string, port int, announceInterval time.Duration, log zerolog.Logger) *SSDPAdvertiser`
- `Run(ctx context.Context)` -- blocking; send alive, listen for M-SEARCH, send byebye on cancel

### Types

- `ChannelItem` -- ID, Name, LogoURL, GroupID
- `GroupItem` -- ID, Name
- `SoapEnvelope`, `SoapBody`, `BrowseRequest` -- SOAP XML parsing
