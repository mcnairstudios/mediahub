# Jellyfin Frontend

Emulates a Jellyfin server for native clients (Android TV, iOS/Swiftfin, Apple TV). Translates Jellyfin API calls into mediahub store operations.

## What it does

- Server discovery (System/Info/Public, Branding)
- User authentication (AuthenticateByName with token persistence)
- Library browsing (UserViews, Items with filtering/sorting/pagination)
- Content detail (item metadata, media sources, TMDB enrichment)
- Playback info (PlaybackInfo returns HLS master.m3u8 URL)
- Live TV channels listing
- Image serving (stub, returns 404 until image pipeline wired)

## Architecture

The Server struct accepts mediahub interfaces (auth.Service, channel.Store, store.StreamStore, etc.) via ServerDeps. No concrete implementations are imported.

Token management uses sync.Map for thread-safe in-memory lookup plus JSON file persistence for restart survival. Unknown tokens are auto-registered to the first user.

## ID Format

All IDs are 32-character dashless hex strings (Kotlin SDK parses these as UUID).

Prefixed IDs encode entity type:
- `cccc` prefix: series (hash of series name)
- `cccd` prefix: season (hash of series name + season number)
- `bbbb` prefix: channel group

## Key Constraints

- All array fields use `omitempty` (nil arrays crash Kotlin SDK)
- Items array in query results is never nil (always `[]BaseItemDto{}`)
- Server version reports `10.10.6` for client compatibility

## Port

Ported from tvproxy's proven Jellyfin emulation (pkg/jellyfin/). Working on Android TV FireTV client v0.19.8 for login, movie browsing, TV series browsing.
