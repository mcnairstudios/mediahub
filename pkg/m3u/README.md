# pkg/m3u

M3U playlist parser for IPTV streams. Parses `#EXTINF` lines with `tvg-*` attributes into `Entry` structs.

## Usage

```go
entries, err := m3u.Parse(reader)
```

## Entry fields

| Field | Source |
|-------|--------|
| Name | Display name after the comma in `#EXTINF` |
| URL | First non-comment, non-empty line after `#EXTINF` |
| Group | `group-title` attribute |
| TvgID | `tvg-id` attribute |
| TvgName | `tvg-name` attribute |
| TvgLogo | `tvg-logo` attribute |
| Duration | Integer after `#EXTINF:` (-1 for live) |
| Attributes | All other `key="value"` pairs from the `#EXTINF` line |

## Design

- Zero external dependencies (stdlib only: bufio, strings, io, strconv)
- Does not depend on `pkg/media/` -- callers convert `Entry` to their own types
- Malformed lines are skipped gracefully (no errors on bad input)
- `#EXTM3U` header is optional
- 1MB line buffer for playlists with long URLs
