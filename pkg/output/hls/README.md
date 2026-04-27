# pkg/output/hls — HLS Output Plugin

## Purpose
Delivers media via HTTP Live Streaming. Produces MPEG-TS segments + m3u8 playlist using libavformat's native HLS muxer. Used by Jellyfin, Apple TV, and browsers (via hls.js).

## Responsibilities
- Receive encoded video/audio packets via OutputPlugin interface
- Convert packets and write to HLSMuxer (libavformat native)
- Serve playlist.m3u8 and seg*.ts files via HTTP
- Fix packet durations from framerate/framesize when duration=0
- RescaleTs between input and output timebases

## Does NOT
- Decode or encode
- Know about MSE, stream copy, or recording
- Manage sessions

## Key Integration Points
- **Input**: Receives packets from FanOut via PushVideo/PushAudio
- **Output**: Serves playlist + segments via ServeHTTP (implements ServablePlugin)
- **Muxer**: Uses pkg/av/mux HLSMuxer (libavformat native)
- **Conversion**: Uses pkg/av/conv for packet conversion

## Reference Implementation
Port from tvproxy's HLSCopyPipeline (pkg/session/gopipeline.go ~line 1638-1860) — muxing and serving only.
