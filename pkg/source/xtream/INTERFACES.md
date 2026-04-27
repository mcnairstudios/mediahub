# pkg/source/xtream -- Interface Contract

## Implements

| Interface | Method | Description |
|-----------|--------|-------------|
| `source.Source` | `Info(ctx) SourceInfo` | Returns ID, type, name, enabled, stream count, last refreshed, error |
| `source.Source` | `Refresh(ctx) error` | Auth + fetch categories + fetch live streams + upsert + delete stale |
| `source.Source` | `Streams(ctx) ([]string, error)` | Returns IDs of all streams for this source |
| `source.Source` | `DeleteStreams(ctx) error` | Removes all streams for this source |
| `source.Source` | `Type() SourceType` | Returns "xtream" |
| `source.VPNRoutable` | `UsesVPN() bool` | Returns UseWireGuard config value |
| `source.VODProvider` | `SupportsVOD() bool` | Returns true |
| `source.VODProvider` | `VODTypes() []string` | Returns ["movie", "series"] |
| `source.Clearable` | `Clear(ctx) error` | Deletes streams and resets internal state |

## Dependencies

| Dependency | Interface | Used For |
|-----------|-----------|----------|
| `store.StreamStore` | `BulkUpsert`, `ListBySource`, `DeleteBySource`, `DeleteStaleBySource` | Stream persistence |
| `*http.Client` | `Do` | Xtream API requests |
| `*http.Client` (WG) | `Do` | VPN-routed requests (optional) |

## Internal Types

| Type | Description |
|------|-------------|
| `Config` | Source configuration (ID, name, server, credentials, flags, stores) |
| `AuthResponse` | Xtream auth response (user info + server info) |
| `Category` | Xtream category (ID + name) |
| `LiveStream` | Xtream live stream entry (name, stream_id, icon, epg_channel_id, category_id) |
| `VODStream` | Xtream VOD movie entry (name, stream_id, container extension) |
| `Series` | Xtream TV series entry (name, series_id, cover) |

## API Routes

| Method | Path | Auth | Handler |
|--------|------|------|---------|
| POST | /api/sources/xtream | Admin | handleCreateXtreamSource |
| PUT | /api/sources/xtream/{id} | Admin | handleUpdateXtreamSource |
| DELETE | /api/sources/xtream/{id} | Admin | handleDeleteXtreamSource |
