# pkg/output — Output Plugin System

## Purpose
Defines the contract for delivery mechanisms that transport media to clients. Each output plugin receives encoded packets and delivers them in a format-specific way (MSE segments, HLS playlists, DASH manifests, WebRTC streams, raw streams, recordings).

## Delivery Plugins

| Plugin | Mode | Container | Consumers | Implements |
|--------|------|-----------|-----------|------------|
| mse | `mse` | fMP4 segments | Browsers (MSE API) | ServablePlugin |
| hls | `hls` | MPEG-TS segments + m3u8 | Jellyfin, Apple TV, hls.js | ServablePlugin |
| dash | `dash` | fMP4 segments + MPD manifest | dash.js, adaptive clients | ServablePlugin |
| webrtc | `webrtc` | RTP via WHEP | Browsers (RTCPeerConnection) | ServablePlugin |
| stream | `stream` | mp4/mpegts file | DLNA, Plex, VLC | OutputPlugin |
| record | `record` | mp4 file | Disk recording | OutputPlugin |

## Responsibilities
- Define the `OutputPlugin` interface for all delivery modes
- Define `ServablePlugin` for HTTP-served delivery (MSE, HLS, DASH, WebRTC)
- Provide the `FanOut` distributor — one decode fans out to N output plugins simultaneously
- Maintain a `Registry` of plugin factories for creating outputs by delivery mode
- Error isolation — one plugin failing does not kill others in the FanOut

## Key Design
- The FanOut supports runtime Add/Remove — a recording can start mid-stream
- Plugins are independent — changing HLS cannot break MSE, DASH, or recording
- Each plugin owns its own muxer and output lifecycle
- The frontend has a `PlayerRegistry` that mirrors the server-side plugin system — each delivery mode has a matching player plugin (MSEPlayer, HLSPlayer, DASHPlayer, WebRTCPlayer, DirectPlayer) that knows how to consume that plugin's output

## Does NOT
- Decode or encode video/audio — that's the DecodeBridge's job
- Know about source plugins or stream discovery
- Manage sessions — that's pkg/session
