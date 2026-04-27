# pkg/source/tvpstreams -- tvproxy-streams Source Plugin

## Purpose
Provides curated VOD content (movies, TV series, collections) from tvproxy-streams instances. Unlike plain M3U/IPTV sources, tvproxy-streams embeds rich metadata via custom `tvp-*` M3U attributes: TMDB IDs, codec info, resolution, season/episode numbers, and collection grouping.

## Responsibilities
- Fetch M3U playlist from tvproxy-streams URL (with optional WireGuard routing)
- Parse entries using pkg/m3u parser
- Extract tvp-* custom attributes from each entry's Attributes map
- Convert to media.Stream with VOD metadata (type, TMDB ID, season, episode, collection)
- Derive width/height from tvp-resolution (1080p, 4K, 720p, etc.)
- Optionally enrich streams with TMDB cache data (poster URLs)
- Mark local streams (tvp-local=true) for priority sync
- Support conditional refresh (ETag / If-None-Match)
- Bulk upsert to store, delete stale streams

## Implements
- `source.Source` (Info, Refresh, Streams, DeleteStreams, Type)
- `source.ConditionalRefresher`
- `source.VPNRoutable`
- `source.VODProvider` (movie, series)
- `source.Clearable`

## tvp-* Attributes
| Attribute | Description | Example |
|-----------|-------------|---------|
| `tvp-type` | Content type | movie, series, episode |
| `tvp-year` | Release year | 1999 |
| `tvp-tmdb` | TMDB ID | 603 |
| `tvp-season` | Season number | 1 |
| `tvp-episode` | Episode number | 1 |
| `tvp-episode-name` | Episode title | Pilot |
| `tvp-resolution` | Video resolution | 1080p, 4K, 720p |
| `tvp-codec` | Video codec | h264, hevc, av1 |
| `tvp-audio` | Audio codec | aac, ac3, eac3 |
| `tvp-collection` | Collection name | The Matrix Collection |
| `tvp-collection-id` | Collection TMDB ID | 2344 |
| `tvp-local` | Locally hosted flag | true |

## Does NOT
- Fetch from TMDB API -- uses the provided TMDBCache for enrichment only
- Own the stream store -- uses the provided StreamStore interface
- Handle playback -- streams are URLs to media files served by tvproxy-streams
