# pkg/output/dash — DASH Output Plugin

## Purpose
Delivers media to clients via MPEG-DASH. Produces an MPD manifest + fragmented MP4 init segments + media segments that dash.js (or any DASH client) consumes.

## Responsibilities
- Receive encoded video/audio packets via OutputPlugin interface
- Convert packets to go-astiav format and write to FragmentedMuxer
- Generate and serve MPD manifest XML (live/dynamic or static/VOD)
- Serve init segments (video + audio) and numbered media segments via HTTP
- Track segment generation (bumps on seek for stale request detection)
- Manage a Watcher that monitors segment files for the DASH client poll loop

## Audio Handling

Two paths depending on whether the bridge provides AudioExtradata:

- **Passthrough** (AudioExtradata provided): Raw audio packets go directly to the FragmentedMuxer. No decode/encode overhead.
- **Internal decode chain** (no AudioExtradata, e.g. copy mode without bridge): DASH plugin creates its own audio decode -> resample -> AudioFIFO -> encode chain. This produces the AAC extradata needed for the fMP4 init segment and handles format conversion (channel downmix, sample rate) that DASH clients require.

Video packets always pass through to the muxer unchanged.

## HTTP Endpoints
- `/manifest.mpd` — MPD manifest (dynamic for live, static for VOD)
- `/init-video.mp4` — Video initialization segment
- `/init-audio.mp4` — Audio initialization segment
- `/video/{seq}.m4s` — Video media segments (1-indexed)
- `/audio/{seq}.m4s` — Audio media segments (1-indexed)
- `/debug` — JSON debug info (generation, segment counts, codec string)

## Does NOT
- Know about MSE, HLS, stream copy, or recording — it's one delivery plugin
- Manage sessions — the session manager handles lifecycle

## Key Integration Points
- **Input**: Receives packets from FanOut via PushVideo/PushAudio
- **Output**: Serves segments and MPD via ServeHTTP (implements ServablePlugin)
- **Muxer**: Uses pkg/av/mux FragmentedMuxer for fMP4 segment production
- **Conversion**: Uses pkg/av/conv to convert av.Packet-style data to go-astiav packets
- **Audio chain** (when active): Uses pkg/av/decode, pkg/av/resample, pkg/av/encode for internal audio processing
