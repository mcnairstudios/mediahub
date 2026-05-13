# MediaHub

Clean-architecture media hub written in Go. Connects stream sources (M3U, Xtream, SAT>IP, HDHomeRun, tvproxy-streams, SpaceX/Space Launches, Radio Garden, Trailers, Demo) to playback sinks (Browser MSE/HLS/DASH/WebRTC, Jellyfin emulation, Plex/HDHR emulation, DLNA, VLC/direct stream) with intelligent format negotiation.

## Build & Test

```bash
# Build (requires libavformat/libavcodec/libavutil/libavfilter/libswscale/libswresample dev libs)
make build

# Run locally on :9090
make start

# Tests (69 packages)
CGO_ENABLED=1 go test ./...

# Frontend smoke test
node web/dist/smoke_test.js
```

## Project Structure

```
cmd/mediahub/main.go          — entry point, DI wiring
pkg/
  api/                        — HTTP handlers, routes, CORS
  activity/                   — active viewer tracking
  auth/                       — JWT auth, users
  av/                         — libavformat wrapper packages
    av.go                     — Packet struct (PTS/DTS/Duration in nanoseconds)
    pipeline.go               — PacketSink interface
    decode/                   — video/audio decoding (HW-aware: VT, VAAPI, etc.)
    demux/                    — open URI, read packets, reconnect, seek
    demuxloop/                — goroutine: read → push to PacketSink
    encode/                   — video/audio encoding (HW-aware), AudioFIFO
    conv/                     — codec ID/name conversion, Packet↔AVPacket
    mux/                      — FragmentedMuxer (fMP4), HLSMuxer (TS/fMP4 segments)
    filter/                   — yadif deinterlacer (mode=1, send_field)
    scale/                    — resolution scaling + pixel format conversion
    resample/                 — audio resampling (channels, rate, format)
    probe/                    — probe URI → StreamInfo
    keyframe/                 — keyframe detection from NAL units
    extradata/                — H264/H265 SPS/PPS/VPS extraction
    selector/                 — audio track selection (language, skip AD)
    bsf/                      — bitstream filter for extradata extraction
    subtitle/                 — subtitle extraction to WebVTT
  cache/                      — probe cache, TMDB cache
  channel/                    — channel store
  client/                     — client profiles, detection, seeding
  config/                     — env-based config (MEDIAHUB_ prefix)
  connectivity/               — WireGuard tunnels
  defaults/                   — JSON client/settings/source profile defaults (embedded)
  epg/                        — EPG store, XMLTV import
  favorite/                   — per-user favorites
  frontend/                   — Jellyfin emulation (port 8096), HDHR emulation
  httputil/                   — shared HTTP utilities
  logocache/                  — logo caching proxy
  m3u/                        — M3U parser
  media/                      — media types, codec normalization, probe result structs
  middleware/                  — JWT auth middleware
  mtls/                       — mTLS for tvproxy-streams
  orchestrator/               — playback + recording orchestration
  output/                     — output plugin framework
    bridge/                   — decode → [deinterlace] → [scale] → encode bridge
    fanout.go                 — distributes packets to all active plugins
    plugin.go                 — OutputPlugin, ServablePlugin interfaces
    registry.go               — plugin factory registry
    mse/                      — fMP4 segments for browser MediaSource Extensions
    hls/                      — MPEG-TS/fMP4 segments + m3u8 playlist
    dash/                     — DASH segments + MPD manifest
    webrtc/                   — RTP packetization via WHEP signalling
    stream/                   — direct chunked HTTP streaming
    record/                   — record to file (MP4/MPEG-TS)
  recording/                  — recording models
  scheduler/                  — cron-based task scheduler
  session/                    — session manager, pipeline lifecycle
  source/                     — source plugin framework (M3U, SAT>IP, HDHR, etc.)
  sourceconfig/               — source configuration
  sourceprofile/              — source input profiles (RTSP, HTTP options)
  store/                      — bolt-backed persistent stores
  strategy/                   — copy vs transcode resolution
  tmdb/                       — TMDB metadata client
  xmltv/                      — XMLTV EPG parser
  youtube/                    — YouTube URL resolver (watch URL → direct stream URL)
web/dist/app.js               — vanilla JS SPA (single file, embedded)
```

## Key Architecture

### Pipeline Flow
```
Source → Demuxer → DemuxLoop → [Bridge] → FanOut → Output Plugins
                                  ↓
                    Decode → [Deinterlace] → [Scale] → Encode
```

- **Demuxer** reads packets with PTS/DTS in nanoseconds
- **Bridge** (optional) decodes, processes, re-encodes when transcoding needed
- **FanOut** distributes identical packets to ALL active plugins simultaneously
- **Output Plugins** handle format-specific muxing and delivery

### PTS Units
- **ALL PTS/DTS/Duration in the pipeline are nanoseconds** (int64)
- Demuxer converts from stream timebase → nanos via `toNanoseconds()`
- Bridge `avTSToNanos()` converts encoder output (90kHz) → nanos
- Output plugins convert nanos → their format:
  - HLS/MSE/DASH: `conv.ToAVPacket()` converts nanos → stream timebase ticks
  - WebRTC: `nanosToRTP()` converts nanos → RTP ticks (90kHz video, 48kHz audio)
  - `ptsToRTP()` is an internal function that converts 90kHz → RTP (used by tests)
- **Encoder passes PTS through in input timebase (90kHz), NOT encoder timebase** — verified in commit e1c8dd6
- **NoPtsNanos** (`math.MinInt64`): sentinel for "PTS not set" in nanos domain; `conv.ToAVPacket` converts to `astiav.NoPtsValue`; bridge uses `tsToNanosSafe()` to preserve unset PTS from encoders
- **VT H.265 encoder outputs PTS==DTS** in encode order (B-frames not reordered); `ensureMonotonicDTS` in FragmentedMuxer and record plugin fixes DTS ordering
- **Nanos round-trip loses precision** for fractional framerates (23.976fps → int 23fps); causes ±1-3 tick rounding errors at segment boundaries — `ensureMonotonicDTS` clamps negative duration to 1

### PacketSink Interface
```go
type PacketSink interface {
    PushVideo(data []byte, pts, dts, duration int64, keyframe bool) error
    PushAudio(data []byte, pts, dts, duration int64) error
    PushSubtitle(data []byte, pts int64, duration int64) error
    EndOfStream()
}
```
- Duration is in nanoseconds; may be zero for live streams where the muxer infers duration from framerate/sample rate
- FragmentedMuxer and HLSMuxer have `fixVideoDuration`/`fixAudioDuration` as fallback when duration is zero

### Strategy
- `"copy"` or `"default"` = pass through, no transcode
- Any explicit codec (`"h264"`, `"h265"`, `"av1"`) = **always transcode**, even if source matches
  - This is correct: same codec doesn't mean same bitrate
  - Also provides error correction (corrupt input → clean re-encode)
- Unknown input codec + explicit output = still transcode (resolved after probe)

### Client Profiles
- Detector reads live from store on every request (no stale cache)
- Profile fields: delivery, video_codec, audio_codec, container, hwaccel, output_height, bitrate
- `hwaccel` on profile drives the **encoder** hardware
- `default_decode_hwaccel` setting drives the **decoder** hardware (separate from encoder)
- `default_hwaccel` setting is fallback when profile hwaccel is empty

### Hardware Acceleration
- Encode: VT (videotoolbox), VAAPI, QSV, NVENC — set via client profile `hwaccel` or `default_hwaccel` setting
- Decode: VT, VAAPI, QSV, CUDA — set via `default_decode_hwaccel` setting
- **VT decode fails on interlaced H.264** — always use SW decode for interlaced content
- VT encode needs NV12 pixel format — bridge creates scaler for YUV420P→NV12 when VT active
- SW encoder uses YUV420P — no scaler needed unless resolution change

### Audio Encoding
- Encoder uses codec's preferred sample format (`codec.SupportedSampleFormats()[0]`)
  - AAC uses `fltp` (float planar)
  - Opus (libopus) uses `flt` (float interleaved) — NOT `fltp`
- Resampler output format matches encoder's sample format
- AudioFIFO uses encoder's sample format
- All three MUST agree or you get SIGSEGV in `av_audio_fifo_write`

### WebRTC Specifics
- H.264 + Opus mandatory for browser compatibility (H.265 not supported over WebRTC)
- Video and audio tracks use same stream ID (`"mediahub"`) → single MediaStream
- `nanosToRTP()` converts pipeline nanos → RTP timestamps
- WHEP signalling: POST offer → answer, DELETE to disconnect
- Copy mode produces poor results (keyframes only, no SPS/PPS inline) — transcode recommended

### Session Lifecycle
- `waitFinished()` waits 30 seconds for demuxloop goroutine to exit before freeing resources
  - RTSP `av_read_frame` can block 10+ seconds — 2 second timeout causes SEGV
- Bridge skips first 50 video decode errors (joining mid-GOP on live streams)
- Bridge skips first 5 audio decode errors (same reason)
- Record plugin codec tags cleared (`SetCodecTag(0)`) for MP4 container compatibility

### Makefile
- `GOTRACEBACK=all` for full crash diagnostics
- Log appended (`>>`) not overwritten — crash traces preserved across restarts
- `make start` kills old instance, builds, starts with MEDIAHUB_ env vars

## Common Pitfalls

- **Scaler must copy PTS**: `dst.SetPts(src.Pts())` after `ScaleFrame` — otherwise garbage PTS propagates
- **Opus needs `flt` not `fltp`**: encoder, resampler, and AudioFIFO must all use the same sample format
- **VT decode + interlaced = broken**: SW decode produces 0 frames, VT decode fails. Use `default_decode_hwaccel=none`
- **Detector was cached**: Now reads from store live — profile changes take effect immediately
- **Record plugin codec tags**: MPEG-TS codec tags (tag [27]) don't work in MP4 — clear with `SetCodecTag(0)`
- **Strategy unknown input**: Even with unknown source codec, explicit output codec means transcode
- **Session stop race**: If `waitFinished` times out before `av_read_frame` returns, freeing the demuxer causes SEGV
- **First keyframe race (WebRTC)**: FIXED — WebRTC plugin buffers the last keyframe and replays it immediately when a new peer connects via `handleWHEPOffer`.
- **Audio-only streams**: Record, MSE, and other plugins support audio-only (no video stream). MSE `WaitReady` checks for either video or audio init segment.
- **TMDB pending index**: Bolt StreamStore maintains a `tmdb:unresolved:` prefix index for streams awaiting TMDB metadata. Methods: `TMDBPendingBatch`, `TMDBPendingCount`, `TMDBPendingRemove`, `TMDBPendingReconcile`. These are bolt-specific, not on the StreamStore interface.
- **YouTube resolver**: `pkg/youtube` resolves YouTube watch URLs to direct streamable URLs at pipeline open time. Source plugins store canonical YouTube URLs; resolution is deferred.
- **Radio Garden multi-place**: Config accepts `[]Place` (not single PlaceID). Fetches channels for all configured places, deduplicates across cities.
- **Space source uses Launch Library 2**: Fetches from thespacedevs.com (all providers), not the old SpaceX v4 API. Paginated with rate limiting.

## Service Management

```bash
# Start (builds, kills any existing instance, starts fresh)
cd /Users/gavinmcnair/claude/mediahub
make start

# Stop
make stop

# Restart (stop + start)
make restart

# Check if running
curl -s -o /dev/null -w "%{http_code}" http://localhost:9090/

# View logs (appended, survives restarts)
tail -f /tmp/mediahub.log

# Check for crashes
grep "SIGSEGV\|panic:" /tmp/mediahub.log
```

- Listens on `:9090` (API + frontend)
- Jellyfin emulation on `:8096`
- Data directory: `/Users/gavinmcnair/mediahub-data`
- Session temp files: `$TMPDIR/mediahub-sessions/`
- Log file: `/tmp/mediahub.log` (append mode, `GOTRACEBACK=all`)
- Environment prefix: `MEDIAHUB_` (NOT `TVPROXY_`)
