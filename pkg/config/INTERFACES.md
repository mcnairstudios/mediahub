# config -- Interfaces

## Config

Plain struct -- no interface. Loaded once at startup via `Load()`.

```go
type Config struct {
    BaseURL      string
    ListenAddr   string
    DataDir      string
    RecordDir    string
    VODOutputDir string
    UserAgent    string
    JellyfinPort int
    DLNAEnabled  bool
    DLNAPort     int
}
```

## Load

```go
func Load() *Config
```

Reads `MEDIAHUB_*` environment variables and returns a populated Config with defaults applied.

| Field | Env Var | Default |
|-------|---------|---------|
| `BaseURL` | `MEDIAHUB_BASE_URL` | (empty) |
| `ListenAddr` | `MEDIAHUB_LISTEN_ADDR` | `:8080` |
| `DataDir` | `MEDIAHUB_DATA_DIR` | `/config` |
| `RecordDir` | `MEDIAHUB_RECORD_DIR` | `/record` |
| `VODOutputDir` | `MEDIAHUB_VOD_OUTPUT_DIR` | (same as RecordDir) |
| `UserAgent` | `MEDIAHUB_USER_AGENT` | `MediaHub` |
| `JellyfinPort` | `MEDIAHUB_JELLYFIN_PORT` | `8096` |
| `DLNAEnabled` | `MEDIAHUB_DLNA_ENABLED` | `true` (set `false` or `0` to disable) |
| `DLNAPort` | `MEDIAHUB_DLNA_PORT` | `8080` |
