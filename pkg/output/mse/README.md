# pkg/output/mse — MSE Output Plugin

## Purpose
Delivers media to browsers via Media Source Extensions. Produces fragmented MP4 (fMP4) init segments + media segments that the browser's MSE API consumes via JavaScript.

## Responsibilities
- Receive encoded video/audio packets via OutputPlugin interface
- Convert packets to go-astiav format and write to FragmentedMuxer
- Serve init segments (video + audio) and media segments via HTTP
- Track segment generation (bumps on seek for stale request detection)
- Manage a Watcher that monitors segment files for the frontend poll loop

## Does NOT
- Decode or encode — receives already-encoded packets
- Know about HLS, stream copy, or recording — it's one delivery plugin
- Manage sessions — the session manager handles lifecycle

## Key Integration Points
- **Input**: Receives packets from FanOut via PushVideo/PushAudio
- **Output**: Serves segments via ServeHTTP (implements ServablePlugin)
- **Muxer**: Uses pkg/av/mux FragmentedMuxer for fMP4 segment production
- **Conversion**: Uses pkg/av/conv to convert av.Packet-style data to go-astiav packets

## Reference Implementation
Port from tvproxy's MSECopyPipeline (pkg/session/gopipeline.go ~line 1400-1560) — but only the muxing and serving parts. The decode/encode chain is handled by DecodeBridge, not by this plugin.
