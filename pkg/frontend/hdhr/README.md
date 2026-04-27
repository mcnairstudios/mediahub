# hdhr

HDHomeRun device emulation frontend. Exposes the HDHR HTTP API so Plex, Channels DVR, and Jellyfin (via HDHR integration) can discover and consume mediahub channels as if they come from a hardware tuner.

## Endpoints

| Route | Description |
|-------|-------------|
| `GET /discover.json` | Device info (DeviceID, FriendlyName, ModelNumber, TunerCount, BaseURL, LineupURL) |
| `GET /lineup_status.json` | Scan status (always reports no scan in progress) |
| `GET /lineup.json` | Channel lineup as JSON (GuideNumber, GuideName, stream URL) |
| `GET /lineup.xml` | Channel lineup as XML |
| `GET /device.xml` | UPnP device description XML |

## Discovery

`DiscoveryResponder` listens on UDP port 65001 for HDHR discovery packets and responds with device info so clients find mediahub on the local network without manual configuration.

## Boundaries

- Depends on `pkg/channel` (Store interface for channel list)
- Depends on `pkg/config` (BaseURL for stream URLs)
- Depends on `pkg/httputil` (JSON/error response helpers)
- Channel URLs point to `/channel/{id}` on the configured BaseURL
- No dependency on session, strategy, or AV packages
