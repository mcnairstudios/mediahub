# pkg/source/spacex — SpaceX Launches Source Plugin

## Purpose
Provides streams from SpaceX launch webcasts. Fetches all launches from the r-spacex API and creates playable streams for those with YouTube webcast links.

## Responsibilities
- Fetch all launches from the SpaceX v4 API (`/v4/launches`)
- Filter to launches with YouTube webcast IDs
- Convert launches to media.Stream (group "SpaceX Launches", VODType "movie")
- Use mission patch images as stream logos
- Bulk upsert streams to store, delete stale entries
- Deterministic stream IDs from source ID + launch ID
- Track refresh status and errors

## Implements
- `source.Source` (Info, Refresh, Streams, DeleteStreams, Type)

## Does NOT
- Filter by launch date or success status — includes all launches with webcasts
- Own the stream store — uses the provided StreamStore interface
- Resolve YouTube URLs to direct video links (passes YouTube watch URLs through)
