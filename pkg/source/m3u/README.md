# pkg/source/m3u — M3U/IPTV Source Plugin

## Purpose
Provides streams from M3U playlists and Xtream Codes APIs. The most common source type for IPTV.

## Responsibilities
- Fetch M3U playlist from URL (with optional WireGuard routing)
- Parse entries using pkg/m3u parser
- Convert M3U entries to media.Stream
- Bulk upsert streams to store
- Support conditional refresh (ETag / If-None-Match)
- Track refresh status and progress

## Implements
- `source.Source` (Info, Refresh, Streams, DeleteStreams, Type)
- `source.ConditionalRefresher`
- `source.VPNRoutable`
- `source.Clearable`

## Does NOT
- Handle Xtream-specific VOD (that's a future enhancement)
- Own the stream store — uses the provided StreamStore interface

## Reference
Port from tvproxy's pkg/service/m3u.go — the refresh logic.
