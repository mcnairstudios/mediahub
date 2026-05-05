# pkg/source/radiogarden -- Radio Garden Source Plugin

## Purpose
Provides live radio streams from cities worldwide via the Radio Garden API (radio.garden). Each source instance is configured with a place ID (city) and fetches all radio stations available for that location. The source type is `radiogarden`.

## Responsibilities
- Fetch channels for a configured place from the Radio Garden API
- Extract channel IDs from the API response URL paths
- Build stream URLs that point to the Radio Garden listen endpoint (which 302-redirects to actual Icecast/Shoutcast streams)
- Use the place name as stream Group (gives tabs in the UI per city)
- Bulk upsert streams to store, delete stale entries
- Deterministic stream IDs from source ID + channel ID
- Set User-Agent header on all API requests (required by Radio Garden)

## Implements
- `source.Source` (Info, Refresh, Streams, DeleteStreams, Type)

## Does NOT
- Own the stream store -- uses the provided StreamStore interface
- Resolve the final stream URL during refresh -- stores the Radio Garden redirect URL and lets the demuxer follow the 302
- Require an API key -- Radio Garden API is public
- Fetch all places -- the user selects a place in the UI, the source only fetches channels for that place
