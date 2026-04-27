# MSE Plugin Interfaces

## Implements
- `output.OutputPlugin` — PushVideo, PushAudio, PushSubtitle, EndOfStream, ResetForSeek, Stop, Status, Mode
- `output.ServablePlugin` — ServeHTTP, Generation, WaitReady

## Constructor
```go
func New(cfg output.PluginConfig) (*Plugin, error)
```

## HTTP Endpoints (served via ServeHTTP)
| Path | Method | Returns |
|------|--------|---------|
| /video/init | GET | Video init segment (ftyp + moov) |
| /audio/init | GET | Audio init segment (ftyp + moov) |
| /video/segment | GET | Next video segment (moof + mdat) |
| /audio/segment | GET | Next audio segment (moof + mdat) |
| /debug | GET | JSON debug info (segment counts, codec string) |

Query params on segment requests:
- `seq` — segment sequence number
- `gen` — generation (stale detection)

## Internal Components
- `FragmentedMuxer` from pkg/av/mux — produces fMP4 segments
- Segment watcher — tracks filesystem for new segments
- Generation counter — bumps on seek/reset

## Packet Flow
```
PushVideo(data, pts, dts, keyframe)
  → conv.ToAVPacket(packet, videoTimeBase)
  → muxer.WriteVideoPacket(avPkt)
  → segment file written to outputDir/segments/

PushAudio(data, pts, dts)
  → conv.ToAVPacket(packet, audioTimeBase)
  → muxer.WriteAudioPacket(avPkt)
  → segment file written to outputDir/segments/
```
