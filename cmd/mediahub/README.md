# mediahub

Application entry point. Wires stores, services, and registries together, seeds a default admin user, and starts the HTTP server.

## Build

```bash
go build -o ./mediahub ./cmd/mediahub/
```

## Run

```bash
./mediahub
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MEDIAHUB_BASE_URL` | (empty) | Public base URL for generated links |
| `MEDIAHUB_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `MEDIAHUB_DATA_DIR` | `/config` | Directory for persistent data |
| `MEDIAHUB_RECORD_DIR` | `/record` | Directory for recordings |
| `MEDIAHUB_VOD_OUTPUT_DIR` | (same as RECORD_DIR) | Directory for VOD output |
| `MEDIAHUB_USER_AGENT` | `MediaHub` | User-Agent header for upstream requests |
| `MEDIAHUB_JELLYFIN_PORT` | `8096` | Jellyfin emulation listen port |

## Default credentials

Username: `admin` / Password: `admin`
