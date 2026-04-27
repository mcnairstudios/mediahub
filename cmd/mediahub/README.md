# mediahub

Application entry point. Wires bolt-backed stores, output plugins, source registry, cache, background workers, and starts the HTTP server with graceful shutdown.

## Build

```bash
CGO_ENABLED=1 go build -o ./mediahub ./cmd/mediahub/
```

## Run

```bash
MEDIAHUB_DATA_DIR=/tmp/mediahub-data MEDIAHUB_RECORD_DIR=/tmp/mediahub-records ./mediahub
```

The data directory is created automatically if it does not exist. The bolt database (`mediahub.db`) is stored inside `MEDIAHUB_DATA_DIR`.

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MEDIAHUB_BASE_URL` | (empty) | Public base URL for generated links |
| `MEDIAHUB_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `MEDIAHUB_DATA_DIR` | `/config` | Directory for persistent data (bolt DB) |
| `MEDIAHUB_RECORD_DIR` | `/record` | Directory for recordings |
| `MEDIAHUB_VOD_OUTPUT_DIR` | (same as RECORD_DIR) | Directory for VOD output |
| `MEDIAHUB_USER_AGENT` | `MediaHub` | User-Agent header for upstream requests |
| `MEDIAHUB_JELLYFIN_PORT` | `8096` | Jellyfin emulation listen port |

## Default credentials

Username: `admin` / Password: `admin`

An admin user is seeded automatically only when the user store is empty (first run). Subsequent starts skip seeding.

## Registered plugins

**Output**: MSE (fMP4 segments), HLS (MPEG-TS segments), Stream (mpegts/mp4 file), Record (mp4 file)

**Source types**: m3u, hdhr, satip (placeholder factories; sources created via API with their config)

## Background workers

- **source-refresh**: runs every 6 hours (placeholder until source instances are persisted)

## Shutdown

Handles SIGINT and SIGTERM. Stops the worker scheduler and all active sessions before exiting.
