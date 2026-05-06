# pkg/source/wasmsource

Source adapter that bridges WASM plugins into MediaHub's source framework.

## Overview

`wasmsource.Source` implements the `source.Source` interface by delegating to a WASM plugin via the `PluginCaller` interface. This allows WASM plugins to act as first-class stream sources alongside built-in sources like M3U, Xtream, etc.

## How It Works

1. On `Refresh()`, calls the WASM plugin's `refresh` export with the source's config JSON
2. Parses the JSON response containing `{"streams": [...]}`
3. Maps each stream entry to a `media.Stream` with a deterministic ID
4. Calls `BulkUpsert` + `DeleteStaleBySource` on the stream store
5. Reports results via `SetRefreshResult` and `OnRefreshDone`

## Plugin Stream Format

The `refresh` export returns JSON:

```json
{
  "streams": [
    {
      "name": "Channel Name",
      "url": "http://example.com/stream.m3u8",
      "group": "Group Name",
      "logo": "http://example.com/logo.png",
      "tvg_id": "channel.id",
      "tvg_name": "Channel",
      "vod_type": "movie"
    }
  ]
}
```

## Stream ID Generation

Stream IDs are deterministic: `SHA256(sourceID + ":" + url)` truncated to 16 bytes (32 hex chars). This ensures the same stream URL always maps to the same ID, enabling stable upserts across refreshes.

## Testing

The `PluginCaller` interface allows mocking the WASM plugin in tests without needing actual WASM binaries.
