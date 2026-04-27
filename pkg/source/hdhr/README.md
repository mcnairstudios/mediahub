# pkg/source/hdhr -- HDHomeRun Source Plugin

## Purpose
Provides streams from HDHomeRun network tuner devices. Discovers devices on the LAN via UDP broadcast, fetches channel lineups from each device's HTTP API, and converts them to media.Stream entries.

## Responsibilities
- Discover HDHomeRun devices on the local network (UDP broadcast on port 65001)
- Fetch /discover.json and /lineup.json from each device
- Filter out DRM-protected and empty-URL entries
- Convert lineup entries to media.Stream with codec metadata
- Classify channels into HD/SD/Radio groups
- Support multiple devices per source
- Trigger hardware channel scans (retune) on devices
- Bulk upsert streams and remove stale entries

## Implements
- `source.Source` (Info, Refresh, Streams, DeleteStreams, Type)
- `source.Discoverable` (UDP broadcast discovery of HDHR devices)
- `source.Retunable` (trigger hardware channel scan on first device)
- `source.Clearable` (remove all streams without deleting source config)

## Does NOT
- Provide EPG data (HDHR devices have their own guide service, but this plugin only handles streams)
- Own the stream store -- uses the provided StreamStore interface
- Handle DRM content -- DRM=1 entries are silently skipped

## Reference
Ported from tvproxy's pkg/service/hdhr_source.go.
