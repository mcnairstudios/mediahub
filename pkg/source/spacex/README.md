# pkg/source/spacex — Space Launches Source Plugin

## Purpose
Provides streams from space launch webcasts across all launch providers (SpaceX, Rocket Lab, ULA, Blue Origin, Arianespace, NASA, ISRO, etc.). Fetches launches from the Launch Library 2 API (thespacedevs.com) and creates playable streams for those with video links. The source type remains `spacex` for backward compatibility with persisted bolt data.

## Responsibilities
- Fetch past launches (with video URLs) and upcoming launches from Launch Library 2
- Paginate through results with rate-limit-respectful delays (100ms between pages)
- Use launch provider name as stream Group (gives tabs in the UI per provider)
- Use launch images as stream logos
- Use the highest-priority video URL as the stream URL
- Include upcoming launches even without videos (URL will be empty)
- Set status as tags (success, failure, go, tbc, etc.)
- Set mission description as EpisodeName
- Bulk upsert streams to store, delete stale entries
- Deterministic stream IDs from source ID + launch UUID

## Implements
- `source.Source` (Info, Refresh, Streams, DeleteStreams, Type)

## Does NOT
- Own the stream store — uses the provided StreamStore interface
- Resolve YouTube URLs to direct video links (passes YouTube watch URLs through)
- Require an API key — Launch Library 2 is free for reasonable usage
