# pkg/output/stream — Stream Output Plugin

## Purpose
Delivers media as a continuous file (mp4 or mpegts) for DLNA, Plex (via HDHR emulation), VLC, and other clients that consume chunked HTTP streams. Also used as the base for recording output.

## Responsibilities
- Receive encoded video/audio packets via OutputPlugin interface
- Write to a continuous file via StreamMuxer
- Support TailFile reading (other consumers can read the file while it's being written)

## Does NOT
- Serve via HTTP directly — the handler layer reads the file via TailFile
- Decode or encode
- Know about MSE, HLS, or recording

## Key Integration Points
- **Input**: Receives packets from FanOut via PushVideo/PushAudio
- **Output**: Writes to a file on disk (mp4 or mpegts format)
- **Muxer**: Uses pkg/av/mux StreamMuxer
- **TailFile**: File can be read concurrently by HTTP handlers

## Reference Implementation
Port from tvproxy's StreamCopyPipeline (pkg/session/gopipeline.go ~line 80-175).
