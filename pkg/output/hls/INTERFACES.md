# HLS Plugin Interfaces

## Implements
- `output.OutputPlugin` — PushVideo, PushAudio, PushSubtitle, EndOfStream, ResetForSeek, Stop, Status, Mode
- `output.ServablePlugin` — ServeHTTP, Generation, WaitReady

## Constructor
```go
func New(cfg output.PluginConfig) (*Plugin, error)
```

PluginConfig fields used:
- OutputDir, IsLive
- VideoFrameRate, AudioFrameSize (for duration fixing)
- SegmentDurationSec (default 6)
- Video/Audio codec params and timebases

## HTTP Endpoints (served via ServeHTTP)
| Path | Method | Returns |
|------|--------|---------|
| /playlist.m3u8 | GET | HLS playlist (waits up to 30s for first segment) |
| /seg{N}.ts | GET | MPEG-TS segment |

## Internal Components
- `HLSMuxer` from pkg/av/mux — libavformat native HLS output
- Segment watcher — monitors for new .ts files

## Packet Flow
```
PushVideo(data, pts, dts, keyframe)
  → conv.ToAVPacket(packet, videoTimeBase)
  → muxer.WriteVideoPacket(avPkt)  // RescaleTs + fixDuration inside muxer
  → seg{N}.ts written, playlist.m3u8 updated

PushAudio(data, pts, dts)
  → conv.ToAVPacket(packet, audioTimeBase)
  → muxer.WriteAudioPacket(avPkt)
```
