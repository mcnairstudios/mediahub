# Source Stream Profiles — Generic Input Configuration

## Concept

A source stream profile captures everything about how to **receive and normalize** an input stream, regardless of source type. The same profile can apply to any source — SAT>IP, HDHR, IPTV, IP camera, local file. The only difference between source types is which fields are relevant; the profile struct is universal.

This means:
- Knowledge learned from one device applies to similar devices
- New source types don't need new profile code — just select the right fields
- Users create profiles once and reuse them across sources

## What a Source Profile Controls

### Signal Processing
| Field | Purpose | Example |
|-------|---------|---------|
| `deinterlace` | Enable deinterlacing | `true` for DVB SD content |
| `deinterlace_method` | Algorithm | `"auto"`, `"bob"`, `"weave"` |

### Connection Tuning
| Field | Purpose | Example |
|-------|---------|---------|
| `rtsp_protocols` | RTSP transport | `"tcp"` (reliable) or `"udp"` (low latency) |
| `rtsp_latency` | RTSP buffer (ms) | `0` (min latency) to `2000` (stable) |
| `http_timeout_sec` | Connection timeout | `5` (LAN) to `60` (remote) |
| `http_user_agent` | UA override | Provider-specific UA strings |
| `format_hint` | Force input format (ambiguous URLs only) | `"rtsp"`, `"mpegts"`, `"hls"` |
| `probe_duration_sec` | How long to analyze stream | `10` for remote IPTV |

### Future Fields (added as needed)
| Field | Purpose | When |
|-------|---------|------|
| `audio_sync_mode` | PTS handling strategy | IP cameras with broken timestamps |
| `reconnect_attempts` | Auto-reconnect count | Unreliable sources |
| `reconnect_delay_sec` | Backoff between retries | Unreliable sources |
| `buffer_size_kb` | Input buffer | High bitrate sources |

New fields are added with `omitempty` — old profiles remain valid. No migration needed.

## What a Source Profile Does NOT Control

These belong to the **client profile** (output side):
- Encoder bitrate / quality
- Output codec (h264, h265, av1)
- Output container (mp4, mpegts, matroska)
- Delivery mode (MSE, HLS, stream)
- Output resolution ceiling
- Hardware acceleration

The separation is absolute: source profile = how to receive. Client profile = how to deliver.

## How It Works in the Pipeline

```
Source → [Source Profile applied] → Demuxer → Bridge → FanOut → [Client Profile applied] → Output
         ├─ timeout                                              ├─ codec
         ├─ user agent                                           ├─ container
         ├─ rtsp protocol                                        ├─ delivery
         └─ deinterlace                                          ├─ bitrate
                                                                 └─ resolution
```

The orchestrator's `StartPlayback`:
1. Looks up the stream's source → source config → `source_profile_id`
2. Loads the source profile
3. Applies to `PipelineConfig`: timeout, user agent, deinterlace
4. Detects the client → client profile
5. Applies to strategy `Output`: codec, container, delivery, hwaccel
6. Strategy produces decision → pipeline runs

## Seeded Profiles

Stored in `defaults/source_profiles.json`, loaded on first run:

```json
[
  {
    "name": "Default",
    "http_timeout_sec": 30
  },
  {
    "name": "DVB Terrestrial",
    "deinterlace": true,
    "deinterlace_method": "auto",
    "rtsp_protocols": "tcp",
    "http_timeout_sec": 60
  },
  {
    "name": "DVB Satellite",
    "deinterlace": true,
    "deinterlace_method": "auto",
    "rtsp_protocols": "tcp",
    "rtsp_latency": 200,
    "http_timeout_sec": 60
  },
  {
    "name": "HDHomeRun",
    "http_timeout_sec": 10
  },
  {
    "name": "Remote IPTV",
    "http_timeout_sec": 30
  },
  {
    "name": "Local Network",
    "http_timeout_sec": 5
  }
]
```

Users can:
- Edit any seeded profile
- Create new profiles for new device types
- Assign any profile to any source — the UI shows/hides irrelevant fields but the backend doesn't enforce which fields apply to which source type

## UI

### Source Profiles Page (admin)
- Table: Name, Deinterlace, RTSP, Timeout
- Create/Edit form shows ALL fields grouped by category
- Help text per field explains when to use it

### Source Forms (M3U, SAT>IP, HDHR, Xtream, tvpstreams)
- "Source Profile" dropdown populated from `/api/source-profiles`
- Same dropdown on every source type — no source-type-specific logic
- Selected profile ID stored as `source_profile_id` in source config

### Auto Refresh
- "Refresh Interval" dropdown: None, Every Minute, Hourly, Daily, Weekly
- Stored as `refresh_interval` in source config
- tvpstreams defaults to "Every Minute" (cheap ETag checks)
- Everything else defaults to "None" (manual)

## API

```
GET    /api/source-profiles         — list all
GET    /api/source-profiles/{id}    — get one
POST   /api/source-profiles         — create
PUT    /api/source-profiles/{id}    — update
DELETE /api/source-profiles/{id}    — delete
```

## Key Principles

1. **One struct, all sources** — the Profile struct is the same regardless of source type
2. **Additive, never breaking** — new fields added with omitempty, old profiles stay valid
3. **Input only** — no output/encoding concerns leak into source profiles
4. **Reusable** — a profile created for one SAT>IP device works on any similar device
5. **Overridable** — external `/config/source_profiles.json` overrides embedded defaults
6. **Seeded, not hardcoded** — defaults are JSON data, not Go code

---

# Current Work Plan

## In Progress (agents running)

1. **Auto-recovery for live streams** — 3 retries with exponential backoff (2s, 4s, 6s)
2. **Recording intent persistence** — recording.json on start, recover on restart
3. **Dockerfile** — Multi-stage build, linuxserver/ffmpeg, HW accel drivers
4. **CI pipeline** — GitHub Actions: test on push, Docker build on tags
5. **DLNA/Jellyfin enable/disable** — Settings toggles, verify Jellyfin matches tvproxy
6. **HDHR SSDP advertisement** — Device discovery for Plex/Channels DVR
7. **HW acceleration gaps** — Per-codec encoder settings, resolution-based bitrate
8. **Recording playback** — End-to-end: play, seek, serve completed recordings

## Completed This Session

- Alphabet jump sidebar on library grids
- Client profile save fix (nested profile object)
- listen_port=0 for port-agnostic client detection
- TV series grouping by group field
- TMDB series lookup by group name
- Tags system (edition, codec, resolution, audio format)
- Sync progress live updates
- VOD cache invalidation during sync
- Token TTL 24h
- Edition tag stripping for TMDB matching
- Unified refresh intervals (all sources use same mechanism)
- EPG refresh unified into source worker
- Standalone movies not grouped as collections
- Source stream profiles (format_hint, probe_duration_sec)
- Global audio/subtitle language in settings
- JSON defaults (clients.json, source_profiles.json, settings.json)
- Source count cached in config (no bolt scan)
- Slim streams API (?fields=slim)
- Bandwidth estimation
- Google OAuth SSO
- SAT>IP full scan package
- MSE playback pipeline fixes
- Audio transcode (always unless explicit copy)
- Bridge AudioOnly mode

## Next Up (prioritized)

### HIGH
9. **Import/Export** — JSON backup of channels, settings, accounts. Soft/hard reset.
10. **Per-channel profile override** — Channel forces specific client profile.
11. **Probe caching** — Store results in bolt, skip re-probe for known streams.

### MEDIUM
12. **HDHR per-device servers** — Separate lineup per physical device.
13. **EPG deduplication** — Multiple sources, deduplicate by channel ID.
14. **Multi-WireGuard failover** — Pool of tunnels, automatic failover.
15. **Invite system** — Admin generates invite tokens.
16. **Rename default_video_codec** — Recording only, not playback.

### LOW
17. **Subtitle extraction** — WebVTT from embedded subtitles.
18. **OpenAPI spec** — Swagger at /api/docs.
19. **Debug endpoints** — Pipeline state, session info.
