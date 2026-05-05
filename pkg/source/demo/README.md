# pkg/source/demo — Demo Streams Source Plugin

## Purpose
Provides a set of hardcoded public test streams for demonstration and testing. Includes Blender open movies (Big Buck Bunny, Sintel, Tears of Steel, Elephant's Dream) and live streams (NASA Live, Bloomberg TV).

## Responsibilities
- Produce a fixed set of publicly available streams on each refresh
- Bulk upsert streams to store, delete stale entries
- Deterministic stream IDs from source ID + stream URL

## Implements
- `source.Source` (Info, Refresh, Streams, DeleteStreams, Type)

## Config
- ID, Name, IsEnabled, StreamStore
- `OnRefreshDone func(sourceID, etag string, streamCount int)` — callback after refresh completes

## Does NOT
- Fetch anything from external APIs — all URLs are hardcoded
- Own the stream store — uses the provided StreamStore interface
