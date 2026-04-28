# pkg/output/mse — MSE Output Plugin

## Purpose
Delivers media to browsers via Media Source Extensions. Produces fragmented MP4 (fMP4) init segments + media segments that the browser's MSE API consumes via JavaScript.

## Responsibilities
- Receive encoded video/audio packets via OutputPlugin interface
- Convert packets to go-astiav format and write to FragmentedMuxer
- Serve init segments (video + audio) and media segments via HTTP
- Track segment generation (bumps on seek for stale request detection)
- Manage a Watcher that monitors segment files for the frontend poll loop

## Audio Handling

Two paths depending on whether the bridge provides AudioExtradata:

- **Passthrough** (AudioExtradata provided): Raw audio packets go directly to the FragmentedMuxer. No decode/encode overhead.
- **Internal decode chain** (no AudioExtradata, e.g. copy mode without bridge): MSE plugin creates its own audio decode -> resample -> AudioFIFO -> encode chain. This produces the AAC extradata needed for the fMP4 init segment and handles format conversion (channel downmix, sample rate) that browsers require.

Video packets always pass through to the muxer unchanged.

## Does NOT
- Know about HLS, stream copy, or recording — it's one delivery plugin
- Manage sessions — the session manager handles lifecycle

## Key Integration Points
- **Input**: Receives packets from FanOut via PushVideo/PushAudio
- **Output**: Serves segments via ServeHTTP (implements ServablePlugin)
- **Muxer**: Uses pkg/av/mux FragmentedMuxer for fMP4 segment production
- **Conversion**: Uses pkg/av/conv to convert av.Packet-style data to go-astiav packets
- **Audio chain** (when active): Uses pkg/av/decode, pkg/av/resample, pkg/av/encode for internal audio processing
