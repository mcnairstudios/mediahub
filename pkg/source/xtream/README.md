# pkg/source/xtream -- Xtream Codes Source Plugin

## Purpose
Provides live streams from Xtream Codes IPTV providers. Authenticates with the provider's player API, fetches live stream lists with categories, and converts them to media.Stream entries.

## Responsibilities
- Authenticate with Xtream Codes player_api.php
- Fetch live categories and map them to stream groups
- Fetch live streams and convert to media.Stream (SourceType="xtream")
- Construct stream URLs from server + credentials + stream_id
- Bulk upsert streams to store, delete stale entries on refresh
- Track refresh status, stream count, and errors
- Support VPN routing via WireGuard client

## Xtream Codes API
- `{server}/player_api.php?username={u}&password={p}` -- auth + server info
- `{server}/player_api.php?username={u}&password={p}&action=get_live_streams` -- live streams
- `{server}/player_api.php?username={u}&password={p}&action=get_live_categories` -- categories
- `{server}/player_api.php?username={u}&password={p}&action=get_vod_streams` -- VOD movies
- `{server}/player_api.php?username={u}&password={p}&action=get_series` -- TV series

## Stream URL Format
- Live: `{server}/{username}/{password}/{stream_id}`
- VOD: `{server}/movie/{username}/{password}/{stream_id}.{ext}`
- Series: `{server}/series/{username}/{password}/{stream_id}`

## Implements
- `source.Source` (Info, Refresh, Streams, DeleteStreams, Type)
- `source.VPNRoutable`
- `source.VODProvider` (movies + series)
- `source.Clearable`

## Does NOT
- Fetch VOD or series content during refresh (live streams only for now)
- Own the stream store -- uses the provided StreamStore interface
- Support conditional refresh (Xtream has no ETag mechanism)

## API Endpoints
- `POST /api/sources/xtream` -- create
- `PUT /api/sources/xtream/{id}` -- update
- `DELETE /api/sources/xtream/{id}` -- delete
- `POST /api/sources/{id}/refresh` -- refresh (shared endpoint)

## Config Keys (sourceconfig.Config map)
- `server` -- base URL of the Xtream provider
- `username` -- account username
- `password` -- account password
- `use_wireguard` -- "true"/"false"
- `max_streams` -- max concurrent streams (0 = unlimited)
