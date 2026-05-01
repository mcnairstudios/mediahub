# Mediahub vs TVProxy: Feature Comparison and Gap Analysis

This document compares mediahub (clean-architecture rewrite) against tvproxy (reference/legacy) feature-by-feature. Each section identifies what is missing in mediahub and assigns a priority.

---

## 1. Sources (M3U, Xtream, HDHR, SAT>IP, tvpstreams)

**tvproxy has:**
- M3U accounts with full CRUD, refresh (async 202), ETag-based caching, orphan stream cleanup
- Xtream Codes with dedicated `pkg/xtream/` client + cache (`xtream.NewCache`), account info endpoint
- HDHR source with device discovery (UDP broadcast), scan, retune, retune status, device clear, per-device management
- SAT>IP with DVB scan (full `pkg/tvsatipscan/` scanner), transmitter lists, system lists, scan status polling
- tvproxy-streams with mTLS enrollment, TLS status endpoint
- M3U file upload endpoint (`POST /api/sources/m3u/{id}/upload`)
- Source profile assignment per M3U account
- WireGuard routing per source (UseWireGuard flag on account)
- Orphan stream cleanup on startup (SAT>IP, HDHR)

**mediahub has:**
- All 5 source types implemented as plugins (`pkg/source/{m3u,xtream,hdhr,satip,tvpstreams}/`)
- Plugin registry pattern (`source.NewRegistry()` with factory functions)
- M3U with ETag caching, WG client injection, stream store callback
- Xtream with server/username/password, max_streams config
- HDHR with device discovery, scan, retune, retune status, device clear, add-device
- SAT>IP with DVB scan (`pkg/source/satip/scan/`), transmitter/system lists, scan status
- tvpstreams with mTLS enrollment, TLS status
- Source config stored in generic `sourceconfig.Store` (flexible key-value config map)
- Refresh orchestrator (`orchestrator.RefreshAll`) runs all sources on interval
- M3U upload endpoint

**MISSING in mediahub:**
- Dedicated Xtream cache package (tvproxy has `pkg/xtream/cache.go` for caching API responses) — **LOW**
- Per-source refresh (mediahub refreshes all sources on a single 1-minute interval; tvproxy has per-account M3U refresh interval) — **MEDIUM**
- Xtream account info endpoint (tvproxy: `GET /api/sources/xtream/{id}/info`) — **LOW**

---

## 2. Playback (MSE, HLS, Stream copy, VOD, Live)

**tvproxy has:**
- 4 pipeline types in `gopipeline.go` (2399 lines):
  - `StreamCopyPipeline` — video copy + audio copy/transcode + mux
  - `AudioTranscodePipeline` — video copy + audio transcode + mux
  - `FullTranscodePipeline` — video transcode + audio transcode + mux
  - MSE variant (fragmented MP4 muxer with watcher)
- In-place seek via `demuxer.RequestSeek()` — no stream restart
- Decoder flush (`avcodec_flush_buffers`) + resampler reset + audio counter reset on seek
- onSeek callback fires BEFORE RequestSeek returns (generation race fix)
- Watcher system (`pkg/session/watcher.go`) for filesystem segment notification
- TailFile reader (`pkg/session/tail.go`) for stream copy HTTP chunked delivery
- FileServer (`pkg/fileserver/`) for internal file serving
- Probe.pb written to session dir for watcher/frontend consumption
- Proto-based probe serialization (`pkg/proto/`)
- Auto-recovery for live streams (3 retries with exponential backoff)
- Recording intent persistence (`recording.json` per session dir) — survives restarts
- `RecoverRecordings()` on startup — resumes interrupted recordings

**mediahub has:**
- Session manager with GetOrCreate, pipeline config, FanOut consumer model
- Pipeline runs demux + optional transcode via `session.PipelineConfig`
- Output plugins: MSE (fragmented MP4 + watcher), HLS (MPEG-TS segments + m3u8), Stream (mpegts/mp4 file), Record (always-on source recording)
- Always-record: every playback session automatically adds a Record plugin alongside the delivery plugin
- Seek support via `session.SeekTo()` (delegated through orchestrator)
- Recording playback: `PlayRecording` creates session from file path, `StopRecordingPlayback`
- Completed recording browsing, deletion, streaming, seek during playback
- Bridge output plugin (`pkg/output/bridge/`) for packet routing

**MISSING in mediahub:**
- Auto-recovery for live streams (retry with backoff on pipeline failure) — **HIGH**
- Recording intent persistence + recovery on restart — **HIGH**
- TailFile reader for HTTP chunked stream delivery (mediahub writes to file, no tail-based HTTP serving) — **MEDIUM**
- FileServer for internal file serving — **MEDIUM**
- Proto-based probe serialization (mediahub uses JSON) — **LOW**
- Probe.pb watcher pattern for frontend (tvproxy writes probe.pb, watcher picks it up) — **LOW**

---

## 3. Transcoding (Codec, HW accel, Deinterlace, Scaling, Audio)

**tvproxy has:**
- Per-codec encoder settings in `settings.json` (h264/h265/av1 x software/qsv/nvenc/vaapi/videotoolbox with preset, quality, pix_fmt)
- Separate decode and encode HW accel settings (`default_hwaccel`, `default_decode_hwaccel`)
- Per-codec overrides: `encoder_h264`, `decoder_h264`, etc.
- MaxBitDepth constraint (Intel A380 forces 8-bit)
- AudioFIFO for frame size normalization between decoder/encoder
- Audio resampler (channel downmix, sample rate conversion)
- Deinterlace filter (yadif, handles EAGAIN)
- Video scaling (resolution ceiling via OutputHeight)
- Subtitle extraction to WebVTT (`pkg/lib/av/subtitle/`)
- Tee for raw byte capture (`pkg/lib/av/tee/`)
- Audio track selector (language preference, skip AD, prefer AAC)
- Keyframe tracker for segment boundaries
- Extradata extraction (H264/H265 SPS/PPS/VPS)
- Codec string extraction for MSE SourceBuffer

**mediahub has:**
- All core AV packages: probe, demux, demuxloop, decode, encode, conv, mux, filter, scale, resample, selector, keyframe, extradata
- Strategy layer resolves copy vs transcode from source + client profile
- HW accel settings (global default_hwaccel)
- Separate decode HW accel (`DecodeHWAccel` in PipelineConfig)
- MaxBitDepth, OutputHeight ceiling
- Per-codec encoder/decoder settings keys in settings store
- Audio FIFO (in encode package)
- Deinterlace filter
- Audio resampler
- MSE plugin does full audio decode/resample/encode pipeline
- Codec string extraction in `pkg/av/mux/codec_string.go`

**MISSING in mediahub:**
- Rich encoder settings per HW platform (tvproxy has preset/quality/pix_fmt tuning per codec per HW platform in settings.json; mediahub only has encoder name strings) — **MEDIUM**
- Subtitle extraction to WebVTT — **LOW**
- Tee for raw byte capture — **LOW**

---

## 4. Client Detection (Match rules, Profiles, Auto-detection)

**tvproxy has:**
- Header-based client auto-detection (`pkg/service/client.go`)
- Match rules with AND logic (User-Agent, custom headers)
- Priority system: (1) ?profile= override, (2) channel.stream_profile_id, (3) client detection, (4) default
- Client profiles linked to stream profiles (output codec, container, delivery, hwaccel)
- Auto-profile creation when client is created
- Orphan cleanup when client deleted
- 2 system + 10 seeded client profiles (Plex, VLC, Skybox, etc.)
- `_port` query param for port-based detection (Jellyfin on :8096)

**mediahub has:**
- Client detector (`pkg/client/Detector`) with port + header matching
- Client store with CRUD
- Client profiles with VideoCodec, AudioCodec, Container, HWAccel, OutputHeight, Delivery
- Seed defaults from `clients.json`
- Detection wired into playback orchestrator

**MISSING in mediahub:**
- Priority resolution chain (profile override > channel setting > client detection > default) — **MEDIUM**
- Per-channel stream profile assignment — **MEDIUM**
- Auto-profile creation/cleanup tied to client lifecycle — **LOW**
- Port-based detection passthrough (`_port` query param) — **LOW**

---

## 5. Channels (CRUD, Groups, Ordering, Stream Assignment, EPG Mapping)

**tvproxy has:**
- Full channel CRUD with channel number (UNIQUE constraint, 409 on conflict)
- Channel groups with CRUD
- Stream assignment (multiple streams per channel)
- EPG mapping (channel to EPG ID)
- Logo assignment via LogoService (EPG-derived + manual)
- Per-channel stream profile override
- Group-based filtering (per-user channel groups)

**mediahub has:**
- Channel CRUD (`pkg/channel/` store)
- Channel groups with CRUD
- Stream assignment (POST /api/channels/{id}/streams)
- EPG mapping support
- Logo URL on channel model

**MISSING in mediahub:**
- Channel number with UNIQUE constraint enforcement — **LOW**
- Per-channel stream profile override — **MEDIUM**
- Logo service (EPG logo extraction, logo management page, logo caching proxy integration with channels) — **MEDIUM**
- Per-user channel group filtering — **MEDIUM**

---

## 6. EPG (Sources, Guide Grid, Now Playing, Scheduling)

**tvproxy has:**
- EPG sources CRUD with refresh
- WireGuard-aware EPG refresh (uses WG client)
- EPG guide endpoint with time-range queries
- EPG now-playing endpoint
- EPG programs endpoint (with optional programs data)
- Bulk insert with 5000-item batches
- EPG deduplication by ChannelID
- EPG enrichment (11 extra fields: subtitle, credits, rating, etc.)
- Orphan EPG cleanup
- Guide grid UI page with program tiles, time navigation, scheduling from guide

**mediahub has:**
- EPG sources CRUD with refresh
- EPG guide endpoint (`/api/epg/guide`)
- EPG now endpoint (`/api/epg/now`)
- EPG programs endpoint (`/api/epg/programs`)
- Program store with time-range queries
- Guide grid UI page with channel rows, time navigation, program detail

**MISSING in mediahub:**
- WireGuard-aware EPG refresh — **LOW**
- EPG deduplication by ChannelID — **MEDIUM**
- Orphan EPG cleanup on source delete — **MEDIUM**
- Rich EPG enrichment fields (credits, rating, star_rating, sub_categories, episode_num_system) — **LOW**
- Bulk insert optimization (tvproxy uses 5000-item batches for ~137k programs) — **MEDIUM**

---

## 7. Recordings (Live, Scheduled, Completed, Playback, Storage)

**tvproxy has:**
- Live recording via VODService (`CreateRecordingSession`)
- Scheduled recordings with DB persistence, tick-based scheduler (30s)
- Status flow: pending > recording > completed/failed/cancelled
- Recording recovery on restart (`RecoverRecordings` scans intent files)
- Recording store (`pkg/store/`) with completed recording metadata
- EPG guide shows scheduled state on program tiles
- Completed recording playback with seek

**mediahub has:**
- Live recording via orchestrator (`StartRecording` / `StopRecording`)
- Scheduled recordings (`pkg/scheduler/`) with start/stop functions
- Recording store with CRUD
- Completed recording playback with seek
- Completed recording streaming (direct file download)
- Completed recording deletion
- Always-record: automatic source.ts recording alongside any playback

**MISSING in mediahub:**
- Recording intent persistence + recovery on restart — **HIGH**
- Recording status flow tracking (pending/recording/completed/failed/cancelled) — **MEDIUM**
- EPG guide integration (show scheduled state on program tiles) — **MEDIUM**

---

## 8. VOD/Library (Movies, Series, Collections, TMDB, Posters)

**tvproxy has:**
- Movie browsing with TMDB enrichment (poster, overview, rating, year, genres, certification)
- TV series browsing (series > seasons > episodes)
- Collections (flat directories = collection, tvp-collection M3U tag)
- TMDB client with cache (`pkg/tmdb/`)
- TMDB image cache (local disk)
- TMDB sync on startup + after M3U refresh
- Stream detail with TMDB data
- Frontend: filter pills (age/collections/decades/genres), collection modals, keyboard letter jump
- Language-based filtering
- Separate Movies and TV Series navigation pages
- TMDB sync status page

**mediahub has:**
- VOD library endpoint (`/api/vod/library?type=movie|series`)
- Stream detail endpoint (`/api/streams/{id}/detail`)
- TMDB client (`pkg/tmdb/Client`) with persistent cache (`pkg/cache/tmdb/`)
- TMDB image cache (`pkg/tmdb/ImageCache`)
- TMDB sync status endpoint
- Library page in frontend (movies + series)

**MISSING in mediahub:**
- TMDB auto-sync on startup + after refresh (tvproxy triggers `syncTMDB()` on M3U refresh done callback) — **MEDIUM**
- Collection support (flat directories, tvp-collection tag) — **LOW**
- Frontend filter pills (age/collections/decades/genres), keyboard letter jump — **LOW**
- Language-based VOD filtering — **LOW**
- Separate Movies vs TV Series pages in frontend (mediahub has combined Library page) — **LOW**

---

## 9. Jellyfin Emulation

**tvproxy has:**
- Separate server on port 8096 with full chi router
- Login/auth with token persistence (`jellyfin_state.json`)
- Auto-register unknown tokens
- Movie browsing with posters, detail, cast/crew, backdrop
- TV Series > Seasons > Episodes browsing
- UserViews (movie/tv library views)
- PlaybackInfo > HLS delivery (master.m3u8 > segments)
- Image serving (poster/backdrop/person from TMDB)
- Favorites integration (UserFavoriteItems)
- Live TV channels
- Helper functions for ID generation (group/series/season/genre/person)
- Missing endpoint test coverage

**mediahub has:**
- Separate server on configurable Jellyfin port
- Login/auth with token persistence (`state.go`)
- Auto-register unknown tokens
- Movie/series/season/episode browsing
- UserViews
- PlaybackInfo with HLS delivery
- Image serving from TMDB
- Favorites integration
- Live TV channels
- Helper functions + tests
- Missing endpoint test coverage

**MISSING in mediahub:**
- Panic recovery middleware on Jellyfin router — **LOW**
- Request logging middleware on Jellyfin router — **LOW**
- Cast/crew detail on movie items — **LOW**
- Canonical path normalization (tvproxy has `canonicalJellyfinPath`) — **LOW**

---

## 10. HDHR Emulation

**tvproxy has:**
- Per-device HDHR servers (`pkg/worker/hdhr_server.go`) — each HDHR device gets its own HTTP server on a unique port
- SSDP advertisement worker
- HDHR discovery worker (UDP broadcast)
- Device store with persistent HDHR devices
- `/discover.json`, `/lineup.json`, `/lineup.xml`, `/device.xml`, `/lineup_status.json`
- Dynamic lineup from channel store
- Configurable device ID, tuner count, model number

**mediahub has:**
- Single HDHR server (`pkg/frontend/hdhr/server.go`)
- Discovery responder (`pkg/frontend/hdhr/discovery.go`) — UDP 65001
- `/discover.json`, `/lineup.json`, `/lineup.xml`, `/device.xml`, `/lineup_status.json`
- Lineup from channel store
- Default device ID, tuner count

**MISSING in mediahub:**
- Per-device HDHR servers (multiple virtual HDHR devices each with own port) — **MEDIUM**
- SSDP advertisement for HDHR devices — **MEDIUM**
- HDHR device persistence store — **MEDIUM**
- Configurable device ID, tuner count, model number per device — **LOW**

---

## 11. DLNA

**tvproxy has:**
- Full DLNA service (`pkg/service/dlna.go`)
- SSDP advertisement worker (`pkg/worker/dlna.go`)
- ContentDirectory service with BrowseDirectChildren
- ConnectionManager service
- Per-user filtering (Basic Auth: unauthenticated=all, admin=all, non-admin=filtered)
- Group-based channel organization in DLNA tree
- albumArtURI omitted for Quest (stalls on external logo fetches)
- Toggleable via `dlna_enabled` setting
- Logo URL integration via LogoService

**mediahub has:**
- DLNA server (`pkg/frontend/dlna/server.go`)
- SSDP advertiser (`pkg/frontend/dlna/ssdp.go`)
- ContentDirectory control
- ConnectionManager control
- Channel listing via adapter interface
- Group-based browsing
- Toggleable via settings

**MISSING in mediahub:**
- Per-user filtering (Basic Auth) — **LOW**
- Quest UA workaround (omit albumArtURI) — **LOW**

---

## 12. WireGuard

**tvproxy has:**
- Legacy single-profile WireGuard service
- Multi-profile WireGuard service (`pkg/service/wireguard_multi.go`)
- WG pool with failover (`pkg/session/wgpool.go`)
- WG proxy per profile (`pkg/session/wgproxy.go`) — HTTP reverse proxy on localhost
- Profile CRUD, activate, test, status, reconnect
- Legacy config migration
- `selectBestActive()` for automatic failover
- WG pool shared across M3U refresh, EPG refresh, session manager, HLS manager
- WireGuard worker (health check every 30s)

**mediahub has:**
- Single-profile WireGuard service (`pkg/connectivity/wg/`)
- Profile CRUD, activate, test, status, reconnect
- WG tunnel management with proxy port
- Connectivity registry pattern
- Restore active profile on startup

**MISSING in mediahub:**
- Multi-profile WireGuard with failover — **MEDIUM**
- WG pool with automatic best-profile selection — **MEDIUM**
- Per-session WG proxy (localhost HTTP proxy per WG profile) — **MEDIUM**
- Health check worker (periodic connectivity monitoring) — **LOW**

---

## 13. Settings

**tvproxy has:**
- Rich `settings.json` with nested categories (ffmpeg, network, vod, workers, server, auth)
- Per-codec encoder settings with HW platform variants (preset, quality, pix_fmt)
- Tuning defaults (analyzeduration, probesize, audio_bitrate, etc.)
- Network settings (reconnect delays, timeouts)
- VOD settings (probe timeout, file retry)
- Worker intervals (SSDP, DLNA, HDHR)
- Server settings (HTTP timeouts, body limits)
- Auth settings (invite token expiry)
- Platform capabilities endpoint (HW/SW badges, per-codec dropdowns)
- Debug flag

**mediahub has:**
- Flat key-value settings store
- Default settings: hwaccel, video codec, decode hwaccel, max bit depth, per-codec encoders/decoders
- DLNA enabled, delivery mode, container, TMDB API key, audio/subtitle language
- Capabilities endpoint with HW detection

**MISSING in mediahub:**
- Rich nested settings structure (tvproxy has encoder preset/quality/pix_fmt per codec per HW platform) — **MEDIUM**
- Network tuning settings (reconnect delays, xtream API timeout, logo download timeout) — **LOW**
- VOD tuning settings (probe timeout, file retry count/delay) — **LOW**
- Worker interval configuration — **LOW**
- Server HTTP timeout settings — **LOW**
- Auth settings (invite token expiry) — **LOW**
- Debug flag toggle — **LOW**

---

## 14. Users/Auth

**tvproxy has:**
- JWT auth with access + refresh tokens
- Configurable token expiry
- Admin/standard roles
- Invite system: `CreateInvite` generates token, `AcceptInvite` sets password
- Invite token expiry (configurable)
- API key authentication (`X-API-Key` header)
- Default admin creation on first run
- User CRUD (admin only)
- Change password (any authenticated user)
- Activity tracking via auth middleware (`TouchUser`)

**mediahub has:**
- JWT auth with access + refresh tokens
- Admin/standard/jellyfin roles (3-tier)
- Google OAuth integration
- User CRUD with email field
- Default admin creation on first run
- Change password

**MISSING in mediahub:**
- Invite system (create invite link, accept invite) — **MEDIUM**
- API key authentication — **LOW**
- Activity tracking via auth middleware (session touch) — **LOW**

**mediahub has that tvproxy DOESN'T:**
- Google OAuth — mediahub has this, tvproxy does not
- Three-tier roles (admin/standard/jellyfin) — mediahub has jellyfin role
- Email field on users

---

## 15. Frontend UI

**tvproxy has (8875 lines):**
- Dashboard, Streams (per-M3U-account tabs), Channels, Channel Groups
- EPG Guide (grid), EPG Sources, Now Playing/Activity
- Movies, TV Series (separate pages with filter pills, collections, keyboard jump)
- Recordings, Scheduled Recordings from EPG guide
- Favorites
- Settings (rich), Capabilities (HW badges)
- Users, Clients, Stream Profiles, Source Profiles
- M3U Accounts, SAT>IP Sources, HDHR Sources, HDHR Devices
- Logos management page
- WireGuard
- Player (MSE with fMP4 segments, seek)
- TMDB Sync pages (Movies, TV)
- CSS embedded in JS, no separate CSS file

**mediahub has (7017 lines + 555 lines CSS):**
- Dashboard, Streams, Channels
- EPG Guide (grid with time navigation), EPG Sources
- Library (combined movies + series)
- Recordings (completed list, playback, delete)
- Favorites
- Settings, Capabilities
- Users, Clients, Source Profiles
- Sources (all types)
- WireGuard
- Player (MSE + HLS)
- Activity
- Bandwidth estimator (bwEstimate indicator)
- Separate CSS file with proper theming

**MISSING in mediahub:**
- Per-source stream tabs (tvproxy shows streams grouped by M3U account in separate nav items) — **MEDIUM**
- Channel Groups management page — **MEDIUM**
- Logos management page — **LOW**
- Separate Movies and TV Series pages with filter pills, keyboard jump, collection modals — **LOW**
- TMDB Sync status pages — **LOW**
- HDHR Devices page (separate from HDHR Sources) — **LOW**
- Scheduled recording creation from EPG guide tiles — **MEDIUM**
- Now Playing label on guide/dashboard — **LOW**

**mediahub has that tvproxy DOESN'T:**
- Separate CSS file (better maintainability)
- Bandwidth estimator in header
- Probe page (dedicated stream probe UI)

---

## 16. M3U/XMLTV Output

**tvproxy has:**
- `GET /api/output/playlist.m3u` — M3U playlist
- `GET /api/output/playlist.m3u8` — M3U8 variant
- `GET /api/output/epg.xml` — XMLTV EPG output
- Output service (`pkg/service/output.go`) with group/channel/logo integration
- Content-Disposition headers

**mediahub has:**
- `GET /api/output/playlist.m3u` — M3U playlist
- `GET /api/output/epg.xml` — XMLTV EPG output

**MISSING in mediahub:**
- M3U8 variant output endpoint — **LOW**

---

## 17. Activity Tracking

**tvproxy has:**
- In-memory activity service with sync.RWMutex
- Integrated into ProxyService (channel + raw stream proxying) and VODService
- Auth middleware `TouchUser` for session tracking (20-minute timeout)
- Admin-only activity API (`GET /api/activity`)
- Frontend "Now Playing" page with 5s auto-refresh
- Both session tracking (dashboard activity) and playback tracking

**mediahub has:**
- Activity service (`pkg/activity/`)
- Activity API endpoint (`GET /api/activity`, admin-only)
- Activity page in frontend

**MISSING in mediahub:**
- Integration with playback orchestrator (tracking who is watching what) — **MEDIUM**
- Session tracking via auth middleware (last-seen timestamps) — **LOW**
- Auto-refresh on activity page — **LOW**

---

## 18. Favorites

**tvproxy has:**
- Per-user favorites stored in `config/users/{userID}/favorites.json`
- CRUD: list, add, remove, check
- Star icons on channels and streams
- Jellyfin UserFavoriteItems integration
- Favorites page with browsing

**mediahub has:**
- Per-user favorites via FavoriteStore
- CRUD: list, add, remove, check (`/api/favorites/*`)
- Favorites page
- Jellyfin favorites integration

**MISSING in mediahub:**
- Nothing significant — feature parity **COMPLETE**

---

## 19. Import/Export (Backup, Restore, Reset)

**tvproxy has:**
- Export service (`pkg/service/export.go`) — scoped export of configuration
- Import service with imported-item tracking
- Data resetter (`pkg/service/reset.go`) — soft reset (re-seed defaults) and hard reset
- Endpoints in settings handler

**mediahub has:**
- Nothing

**MISSING in mediahub:**
- Configuration export — **MEDIUM**
- Configuration import — **MEDIUM**
- Soft reset (re-seed defaults) — **LOW**
- Hard reset (wipe all data) — **LOW**

---

## 20. Docker (Build, CI, HW Accel)

**tvproxy has:**
- Dockerfile: `linuxserver/ffmpeg:8.0.1` builder + runtime with ffmpeg 8.0.1 from source
- Full codec support: libx264, libx265, libmp3lame, libopus, libvorbis, libvpx, libdav1d, libfdk-aac, libaom
- HW accel: VAAPI + NVENC + QSV (Intel via `intel-media-va-driver-non-free` + `libvpl2`)
- NVIDIA via nv-codec-headers
- CI: GitHub Actions on self-hosted runners (amd64 + arm64), tag-triggered
- Multi-arch builds (native, no QEMU)
- GOMAXPROCS capped

**mediahub has:**
- No Dockerfile
- No CI pipeline
- No .github directory

**MISSING in mediahub:**
- Dockerfile — **HIGH**
- CI pipeline (GitHub Actions) — **HIGH**
- Multi-arch build support — **HIGH**
- HW accel driver installation in Docker — **MEDIUM**

---

## 21. API Gaps

Endpoints in tvproxy that mediahub is missing entirely:

| Endpoint | Purpose | Priority |
|----------|---------|----------|
| `GET/PUT /api/settings/export` | Config export | MEDIUM |
| `POST /api/settings/import` | Config import | MEDIUM |
| `POST /api/settings/reset` | Data reset (soft/hard) | LOW |
| `GET /api/output/playlist.m3u8` | M3U8 output | LOW |
| `GET /api/logos` | Logo management | LOW |
| `GET /api/channel-groups` (full CRUD) | Channel group management | MEDIUM |
| Invite endpoints (`POST /api/auth/invite`, `POST /api/auth/accept-invite`) | User invites | MEDIUM |
| `GET /api/docs` (OpenAPI spec) | API documentation | LOW |
| pprof on :6060 | Debug profiling | LOW |

Endpoints mediahub has that tvproxy does NOT:
| Endpoint | Purpose |
|----------|---------|
| `GET /api/auth/google` | Google OAuth |
| `GET /api/auth/google/callback` | Google OAuth callback |
| `POST /api/probe` | Dedicated probe endpoint |

---

## 22. Performance (Caching, Pagination)

**tvproxy has:**
- Bolt-backed probe cache (`store.NewBoltProbeCache`) — persists probe results
- Xtream API response cache
- ETag-based M3U refresh (skip unchanged)
- Logo caching proxy (disk cache at `/config/static/logocache/`)
- In-memory stores with gob serialization (streams, EPG)
- Bulk EPG insert (5000-item batches)
- SAT>IP cached probe (skip `FindStreamInfo` for known streams)
- Settings debug flag for conditional logging

**mediahub has:**
- TMDB persistent cache (`pkg/cache/tmdb/`)
- Logo caching (`pkg/logocache/`)
- ETag-based M3U refresh
- Bolt-backed stores (all data in single `mediahub.db`)
- VOD response cache (in-memory `vodCache sync.Map` with TTL)
- Bandwidth estimation in frontend

**MISSING in mediahub:**
- Probe result cache (persistent, cross-session) — **MEDIUM**
- Bulk EPG insert optimization — **MEDIUM**
- SAT>IP cached probe (skip FindStreamInfo for known streams) — **LOW**
- Xtream API response cache — **LOW**

---

## Priority Summary (Updated May 2026)

### DONE
- Dockerfile + CI pipeline + multi-arch Docker builds
- Auto-recovery for live streams (3 retries, exponential backoff)
- Recording intent persistence + restart recovery
- Import/export for configuration backup
- Per-channel stream profile override (stream_profile_id on channels)
- Per-user channel group filtering (ChannelGroupIDs on users, JWT propagation)
- Priority resolution chain for client detection (priority field, sorted matching)
- Channel groups management UI page
- EPG deduplication (bolt key = channelID + startTime, natural dedup)
- Bulk EPG insert optimization (5000-item batches)
- Probe result cache (persistent, bolt-backed, 24h TTL)
- SSDP advertisement for HDHR + DLNA
- Activity tracking integration with playback (in-memory, API + frontend page)
- Scheduled recording from EPG guide UI (bolt store, scheduler, status flow)
- Subtitle extraction to WebVTT (pkg/av/subtitle/)
- DLNA/Jellyfin enable/disable toggles
- Per-source refresh intervals (none/minute/hourly/daily/weekly)
- EPG per-source refresh intervals
- TMDB blob architecture (work queue, metadata worker, image worker, forward name index)
- Xtream VOD + series fetching
- SAT>IP scan with updated DVB tables (8PSK, FEC, multi-file, 871 channels)
- FFmpeg subprocess transcode (H.265+AAC via HLS for browser playback)
- Annex B → AVCC conversion for fMP4 MSE
- Encrypted channel labelling (ENC badge)
- Library per-source browsing with category drill-in for large sources

### MEDIUM (nice to have for production)
1. Multi-profile WireGuard with automatic failover — health check active tunnel periodically, if it fails try next profile in order, activate first one that works
2. WG pool with per-session proxy
3. Per-device HDHR servers
4. Invite system for user onboarding
5. ~~Rich encoder settings per HW platform~~ — DONE (default_hwaccel, default_decode_hwaccel, per-codec encoder_h264/decoder_h264/etc)
6. Recording status flow tracking (status page)
7. Logo service (EPG logo extraction, logo management page) — caching proxy exists
8. In-process H.265 encoder fix (CGO crash on ARM64, works on amd64)
9. NIT-based satellite discovery (prototype done, needs modulation auto-detect)
10. Xtream series episode enrichment from TMDB (episode names, stills)
11. TMDB collection poster/backdrop fetching

### LOW (polish)
12. Quest DLNA workaround (omit albumArtURI — Quest is a primary target)
13. Xtream account info endpoint
14. OpenAPI spec endpoint
15. pprof debug endpoint
16. Per-user DLNA filtering
17. Network/VOD/worker/server tuning settings
18. TMDB sync status pages
19. Debug flag toggle
20. API key authentication
21. Virtual scroll for large poster grids (localStorage caching done, virtual scroll not yet)

### NOT NEEDED (architectural differences make these unnecessary)
- Proto-based probe serialization — JSON is fine, probe data is small
- Probe.pb watcher pattern — mediahub returns probe info in API response directly
- TailFile reader — output plugins write to HTTP response directly
- FileServer package — standard http.ServeFile suffices
- Separate Movies/TV Series pages — combined Library with tabs is better UX
- Dedicated Xtream cache package — stream store + TMDB blob store covers this
- Port-based detection passthrough (_port param) — ListenPort matching handles this
- Canonical Jellyfin path normalization — Go mux with path variables handles this
- M3U8 output variant — M3U is sufficient, any M3U8 client reads M3U
- Auto-profile creation/cleanup on client lifecycle — clean separation of client/source profiles
- Tee for raw byte capture — debug tool, not production feature
- Sprite sheets — nice optimisation but individual files with browser caching is simpler and works for Jellyfin too
