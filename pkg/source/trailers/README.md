# pkg/source/trailers — TMDB Trailers Source Plugin

## Purpose
Provides streams from TMDB movie trailers. Fetches upcoming and now-playing movies, resolves their YouTube trailer URLs, and creates playable streams.

## Responsibilities
- Fetch upcoming and now-playing movie lists from TMDB API
- Resolve YouTube trailer URLs via TMDB videos endpoint (prefers Trailer, falls back to Teaser)
- Convert movies with trailers to media.Stream (group "Trailers", VODType "movie")
- Bulk upsert streams to store, delete stale entries
- Deterministic stream IDs from source ID + TMDB movie ID
- Track refresh status and errors

## Implements
- `source.Source` (Info, Refresh, Streams, DeleteStreams, Type)

## Config
- ID, Name, IsEnabled, TMDBKey, StreamStore, HTTPClient
- `OnRefreshDone func(sourceID, etag string, streamCount int)` — callback after refresh completes

## Does NOT
- Cache TMDB responses across refreshes
- Own the stream store — uses the provided StreamStore interface
- Resolve YouTube URLs to direct video links (passes YouTube watch URLs through)
