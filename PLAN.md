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
- Playback refactor — frontend PlayerRegistry with 5 player plugins (MSE, HLS, DASH, WebRTC, Direct), capability detection, delivery param passing, User Choice mode
- DASH output plugin (MPD manifest + fMP4 segments, dash.go + watcher + tests)
- WebRTC output plugin (WHEP signalling, H.264/Opus RTP, pion/webrtc, tests)
- Manifest/segment validation test harness (pkg/output/validate/ — stubbed for future expansion, plugin tests cover individual formats)

## Delivery Modes (Complete)

Six delivery modes, all implemented:

| Mode | Container | Consumers |
|------|-----------|-----------|
| MSE | fMP4 segments | Browsers (MSE API) |
| HLS | MPEG-TS segments + m3u8 | Jellyfin, Apple TV, hls.js |
| DASH | fMP4 segments + MPD manifest | dash.js, adaptive clients |
| WebRTC | RTP via WHEP | Browsers (RTCPeerConnection) |
| Stream | mp4/mpegts file | DLNA, Plex, VLC |
| Record | mp4 file | Disk recording |

Frontend PlayerRegistry maps each delivery mode to a player plugin (MSEPlayer, HLSPlayer, DASHPlayer, WebRTCPlayer, DirectPlayer). Client profiles either force a delivery mode or set "user" to let the frontend show a dropdown filtered by browser capabilities.

Delivery mode per client:
- **Browser live**: WebRTC (lowest latency) or MSE (~3s latency) or HLS (fallback) or DASH (adaptive)
- **Browser VOD**: MSE (seeking support) or HLS or DASH
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

## Bugs Found & Fixed (Need Tests)

### Double PTS Rescale in fMP4 Pipeline (CRITICAL — FOUND IN INVESTIGATION)
**Symptom**: fMP4 segments report duration=101,088s for what should be ~6s. r_frame_rate=90000/1.
**Root cause**: The SW encoder (libx265) passes PTS through in the input timebase (90kHz). The test code then rescaled from encoder timebase (1/25) to muxer timebase (1/90000), multiplying PTS by 3600. The muxer ALSO rescales internally. Double rescale = PTS inflated by 3600x.
**Test program**: `/tmp/pipeline_compare.go` with `/tmp/raw_satip_60s.ts` as input.
**Reference output**: `/tmp/ref_fmp4_60/output.mp4` (correct 60.1s, produced by ffmpeg CLI).
**Our output**: `/tmp/our_fmp4_60/` — 101,088s with double rescale, 28s without (still wrong — interlaced PTS spacing issue).
**What the bridge does differently**: Converts ALL PTS to nanoseconds (avTSToNanos) before passing downstream. The muxer receives nanos and rescales to its stream timebase. This may avoid the double-rescale but needs verification with live playback.
**Next step**: Fix the PTS chain. See findings below.

### ffmpeg Subprocess Analysis (CRITICAL FINDING)
**What ffmpeg CLI does with the same content**:
```
ffmpeg -hwaccel videotoolbox -i input.ts -vf "yadif=mode=send_frame,format=nv12" -c:v hevc_videotoolbox ...
```
1. VT decode FAILS on interlaced → falls back to SW `h264 (native)` decode
2. CPU `yadif` deinterlace (same as ours)
3. Auto-inserted `auto_scale_0`: yuv420p → nv12
4. `hevc_videotoolbox` HW encode
5. **Output: 25/1 fps, 5.02s for 5s input — CORRECT**

**Our pipeline with same settings**: 25/2 fps (12.5fps), 28s for 3 segments — WRONG.

**The difference**: ffmpeg internally manages PTS through the filter graph. When yadif deinterlaces 50i→25p, ffmpeg's filter graph halves the PTS spacing. Our code runs yadif via the filter graph but the PTS may not be adjusted for the frame rate change.

**Key test**: `/tmp/ffmpeg_subprocess_test.mp4` — 5s clip produced by ffmpeg CLI, correct timing.

### fMP4 default_sample_duration=1 (ROOT CAUSE FOUND)
**Symptom**: ffprobe shows 0.003s-0.063s for segments that should be ~2s each.
**Root cause**: The fMP4 muxer's tfhd box writes `default_sample_duration=1` (one tick at 90kHz = 0.000011s) because encoder packets have `duration=0`. Chrome MSE may tolerate this (uses PTS directly) but ffprobe calculates timing from it.
**Proven by**: Parsing the moof/tfhd/trun boxes of live segments — `default_sample_duration=1`, no per-sample duration in trun.
**Fix applied**: Added `fixVideoDuration` in FragmentedMuxer that sets duration from `VideoFrameRate` when packet has duration=0. MSE plugin passes framerate.
**Status**: Fix applies for the direct muxer test path but NOT for the live MSE path — the MSE plugin's deferred muxer creation may bypass the opts. Need to verify the deferred path passes `VideoFrameRate`.
**Reference**: go-astiav transcoding example sets encoder timebase = decoder timebase, and does `pkt.RescaleTs(enc.TimeBase(), outputStream.TimeBase())` before writing. Our bridge does nanos conversion instead.
**Tests**: `TestFATE_Pipeline_FramerateCorrect` catches the 25/2 issue. Need to also test default_sample_duration in trun/tfhd boxes directly.

### PTS Integer Overflow in Bridge (CRITICAL)
**Symptom**: Video plays 2 seconds then freezes. Audio resumes after 6s, video never does.
**Root cause**: `avTSToNanos()` computed `ts * 1_000_000_000 * tbNum / tbDen`. With 90kHz timestamps on live streams, `PTS * 1_000_000_000` overflows int64 after ~102 seconds of content. First segment works (small PTS), then overflow produces garbage timestamps → muxer writes 0.003s duration segments → browser has nothing to play.
**Fix**: Overflow-safe rescale function: `(v / den) * num + (v % den) * num / den`. Applied to `conv.ToAVPacket()`, `bridge.avTSToNanos()`, and `resample.Convert()`.
**Tests needed**: 
- PTS conversion with large values (>100s at 90kHz = PTS > 9,000,000)
- PTS conversion at int64 boundary values
- Verify `avTSToNanos(9000000, 1/90000)` = 100,000,000,000 nanos (100s) not garbage
- Segment duration stays correct after 2 minutes of streaming

### Audio Input Timebase Mismatch
**Symptom**: "Failed to reconcile encoded audio times with decoded output" in Chrome.
**Root cause**: Bridge used hardcoded `audioTB = 1/48000` for converting audio PTS, but source audio may have different sample rate (e.g. AC3 at 48kHz with different packet timing). Should use actual source audio timebase.
**Fix**: Added `audioInputTB` derived from `audioTrack.SampleRate`, used for `ToAVPacket` instead of hardcoded `audioTB`.
**Tests needed**:
- Audio PTS conversion with various sample rates (44100, 48000, 96000)
- Audio sync test: verify A/V sync after 30 seconds of playback

### Resampler PTS Drift
**Symptom**: Audio gradually drifts out of sync over time.
**Root cause**: Resampler copied input frame PTS to output frame, but resampling can change sample count (e.g. 44.1kHz → 48kHz). Output PTS should be based on cumulative output sample count, not input PTS.
**Fix**: Track `nextPts` counter in resampler, increment by `NbSamples()` per output frame. Reset on `Reset()`.
**Tests needed**:
- Resample 44.1kHz → 48kHz, verify output PTS increments correctly
- Resample then seek (Reset), verify PTS restarts

### VideoToolbox HW Pipeline Findings
**What works**:
- Progressive: VT decode → scale_vt → VT encode (all GPU, 5.6x real-time)
- Interlaced: SW decode → NV12 scale → VT encode with interlaced flags (57% CPU vs 600%)
- `yadif_videotoolbox` exists but returns ENOSYS on homebrew ffmpeg 8.1

**What needed fixing**:
- VT encoder needs NV12 input (`PixelFormatNv12`) — added scaler for videotoolbox/vaapi
- VT encoder needs `InterlacedDCT` + `InterlacedMe` flags for interlaced content
- Skip CPU yadif when VT encoder handles interlaced natively
- VT decode fails on interlaced H.264 — falls back to SW automatically

**Tests needed**:
- NV12 conversion test: yuv420p → nv12 → VT encode → valid output
- Interlaced flags test: verify encoder context has correct flags set
- Fallback test: VT decode failure → SW decode → encode still works

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
- ~~Playback refactor~~ — DASH + WebRTC output plugins, frontend PlayerRegistry, capability detection, User Choice mode
- ~~Manifest/segment validation test harness~~ — pkg/output/validate/ stubbed, individual plugin tests cover formats
