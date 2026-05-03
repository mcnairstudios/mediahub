# MediaHub Plan

## Architecture
See top of file for Source Stream Profile design (unchanged).

## Design Decisions (from user)

1. **Two ways to create channels**: "Add Channel" button on channels page, OR "+" on a stream in streams page. Both open the same modal with different pre-populated fields.
2. **One stream per channel** for now. No multi-stream failover.
3. **Combined Library** with tabs (Movies/TV Series), not separate nav items.
4. **Everything goes through the pipeline** — IPTV HLS, SAT>IP, everything. Same processing path.
5. **EPG matching at source level** — link an EPG source to a stream source. Channel names/numbers auto-match. Not per-channel manual matching.
6. **Record from guide** — every EPG entry has a record dot. Records the channel's stream for the program's time window (start→stop from EPG).
7. **User roles** — standard users see admin channel assignments but eventually users will manage their own channel selections. DLNA/Jellyfin filter by user's channel groups.

## Sprint: User Feedback (Current)

### Stream 1: Frontend UX Overhaul
1. **Left menu reorganization** [M] — Mirror tvproxy: Dashboard, Activity, Channels, Movies, TV Series, EPG Guide, Recordings, Favorites | Admin: Sources, Channel Groups, Source Profiles, Clients, Settings, Users, WireGuard, HDHR Devices, Logos | Developer: Probe, Play URL, API Keys, Debug. Remove TMDB as standalone page.
2. **Edit panels as popups** [S] — All edit forms in modal overlays, not inline at bottom. Confirm/cancel buttons. Reference tvproxy modal pattern.
3. **Invites merged into Users** [S] — "Invite User" button on Users page, not separate nav item. Remove Invites from nav.
4. **TMDB counts on dashboard** [S] — Remove TMDB nav item. Show queue/resolved/image counts as dashboard widget.
5. **Dashboard EPG per-source** [S] — Show each EPG source with individual timestamp + status color (green/yellow/red).
6. **Dashboard source cards** [S] — Click navigates to that source's streams (DONE but verify working).

### Stream 2: Channels, Guide & EPG
7. **Channels overhaul** [M] — Match tvproxy: stream assignment UI, logo picker, EPG ID selector, group assignment. Edit in modal popup.
8. **Channel groups** [M] — Drag reorder, bulk channel assignment to groups. Reference tvproxy channel_group.go.
9. **Guide = MY channels** [M] — Guide shows user's chosen channels (not all EPG), grouped by channel group. Time grid with program tiles. Reference tvproxy EPG guide page.
10. **Auto EPG matching** [M] — Match EPG channels to stream channels by tvg_id (exact) then name (fuzzy). Manual override. Optional epg_source_id on source config.
11. **Series link in EPG** [M] — Parse XMLTV series-id/CRID. Show series link icon next to record button in guide. Group related episodes.
12. **Stream/Logo/EPG selection** [S] — When adding channels, use tvproxy-style selection UIs for picking streams, logos, and EPG IDs.

### Stream 3: Recordings, Activity & Playback
13. **Recording page overhaul** [M] — Three sections: Active, Scheduled, Completed. Show channel name, program title, duration, file size. Handle stale entries. Scheduled recordings from guide register with cron scheduler, cancellable.
14. **Activity page** [S] — Active viewers (tuner usage), recent logins (20min window), stream/session info. Reference tvproxy ActivityService.
15. **IPTV playback fix** [S] ✅ — Strategy defaults to copy when in_video empty. 4 test cases cover unknown codec scenarios. Post-probe updates stream store with discovered codec.
16. **SAT>IP audio sync** [S] — AC3/MP2 decode errors cause audio underflow every ~25s. Investigate and fix.
17. **Arbitrary play URL** [S] — Debug menu input: paste URL, detect codec, play. Uses existing detection logic.

### Stream 4: Data Integrity & Backend
18. **Save persistence audit** [M] — Agent to check every save/update handler: does it persist to bolt? Is persisted data actually used? Report any orphaned saves.
19. **Jellyfin completeness audit** [M] — Compare every endpoint response in tvproxy pkg/jellyfin/ vs mediahub pkg/frontend/jellyfin/. Report format differences.
20. **System source profiles** [S] — Mark built-in profiles (SAT>IP, Default, HDHR, etc.) as IsSystem. Prevent deletion. Seed on startup.
21. **User roles enforcement** [S] — Standard users: channels, guide, library, recordings, favorites only. Admin: everything. Frontend + middleware checks. DLNA Basic Auth respects roles. Reference tvproxy middleware.
22. **Logos working** [S] — Verify full chain: EPG extraction → logo cache → channel assignment → display. Reference tvproxy logo.go.
23. **Favorites instant** [S] — Bulk-fetch all user favorites once on page load. No per-item IsFavorite calls. Prefix scan is already fast.

## Completed (Previous Sessions)

- Import/Export (endpoints + UI)
- HDHR per-device servers (store, manager, auto-split, SSDP)
- Multi-WireGuard failover (HealthCheck + Failover + scheduler)
- Invite system (tokens + API keys + X-API-Key middleware)
- OpenAPI spec (Swagger UI at /api/docs)
- Debug endpoints (pprof + debug_enabled)
- Error message constants (pkg/api/errors.go)
- WireGuard client config dedup (resolveWGClient helper)
- OnRefreshDone callback dedup (makeOnRefreshDone)
- Unified scheduler (robfig/cron + bolt persistence)
- Prefix-keyed stores (streams, channels, groups, EPG, recordings, favorites)
- VOD type in stream keys (ListBySourceAndType, ListByVODType)
- Post-probe stream metadata saved to store
- EPG source filtering (SourceID on programs)
- Last Refreshed bug fix (Xtream OnRefreshDone)
- Xtream Account Info error display
- SAT>IP H.265 transcode pipeline (in-process, hev1 fMP4, ffmpeg handles Annex B)
- go-astiav usage aligned with official examples
- BSF extradata extractor
- FanOut error isolation (record plugin errors non-fatal)
- Source profiles merged (SAT>IP DVB-T + DVB Satellite → SAT>IP)
- Dashboard improvements (uptime, Now/Next, bulk ops, auto-number)
- CI smoke test upgrade
- Settings validation
- DLNA now-playing EPG info
- Jellyfin TMDB enrichment + season posters
- Channel auto-numbering on startup
- IPTV playback fix (copy mode default when codec unknown, 4 test cases, post-probe stream update)

## Delivery Modes (Future)

Current: MSE (fMP4 segments, browser), HLS (MPEG-TS segments, Jellyfin/Apple TV), Stream (raw mpegts/mp4, Plex/DLNA)

Planned:
- **DASH** — MPEG-DASH delivery for adaptive bitrate. Useful for multi-quality streaming. Output plugin produces MPD manifest + segments.
- **WebRTC** — Ultra-low latency delivery for live TV in browser. Was amazing for live playback in testing. Requires signaling server (WHEP/WHIP). Output plugin produces RTP → WebRTC peer connection. Best for browser live TV, not suitable for Jellyfin/Plex.

Delivery mode per client:
- **Browser live**: WebRTC (lowest latency) or MSE (current, ~3s latency) or HLS (fallback)
- **Browser VOD**: MSE (seeking support) or HLS
- **Jellyfin**: HLS (required by Kotlin SDK/ExoPlayer)
- **Plex**: Stream (raw HTTP chunked, HDHR emulation)
- **DLNA**: Stream (raw HTTP chunked)
- **Apple TV**: HLS

## Plugin View System

Source plugins register metadata (label, color, form fields, view type). Frontend renders from metadata — no per-type `if` checks. Built-in views: list, tiles. Plugins can add new view types. Simple.

Future: a "Custom Feed" source type where users configure feeds via a CRUD UI within the plugin:
- Add **sections** (e.g. "Trending Cams", "NYC", "SpaceX Crew Missions")
- Per section: target URL(s), field extraction rules (JSONPath or CSS selectors), refresh interval
- Frontend dropdown to pick section → shows streams as list/tiles
- Demo Streams and EarthCam become example seed configs for this plugin, not standalone Go code
- The plugin handles all complexity internally — its interface with mediahub is still just streams

## New Source Plugins (Planned)

- **EarthCam** — Trending feed from earthcam.com. Live webcams of landmarks, cities, nature. Catches big events (volcanoes, storms, Times Square NYE). Source plugin fetches trending/featured cams as live streams.

## Logging

- Logs should be useful by default — no debug noise in normal operation
- Debug logging as a toggle in Developer settings (or `MEDIAHUB_LOG_LEVEL=debug`)
- Debug mode shows: what data is passed into each component, plugin options, major data flows — visibility into the pipeline
- Factory/startup logs: once at startup, not on every poll/status check
- Source refresh: log start + result, not internal steps

## Backlog (Low Priority)

- Generic store CRUD helpers
- Frontend UI component consolidation
- Stream pagination for large sources
- Virtual scroll for large poster grids

## Backlog (Done)

- ~~Handler CRUD boilerplate extraction~~ — reviewed, not worth abstracting (each handler unique)
- ~~Migrate remaining stores to keyenc~~ — all 7 stores migrated
- ~~Source plugin base type~~ — BaseSource extracted, all 5 plugins refactored
- ~~Main.go factory reorganization~~ — split into sources.go + outputs.go
- ~~SSDP advertiser consolidation~~ — reviewed, DLNA and HDHR serve different protocols, correctly separate
- ~~Magic string constants~~ — extracted to source.TypeX constants
