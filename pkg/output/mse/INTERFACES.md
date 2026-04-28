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
- `audioDec` — audio decoder (nil in passthrough mode)
- `audioResample` — audio resampler for channel/rate conversion (nil in passthrough mode)
- `audioEnc` — audio encoder producing AAC for fMP4 (nil in passthrough mode)
- `audioFifo` — AudioFIFO buffering decoded frames to encoder frame size (nil in passthrough mode)
- `audioLatched` — set true on first audio decode error, latches audio off for the session

## Packet Flow
```
PushVideo(data, pts, dts, keyframe)
  → conv.ToAVPacket(packet, videoTimeBase)
  → muxer.WriteVideoPacket(avPkt)
  → segment file written to outputDir/segments/

PushAudio(data, pts, dts) — passthrough (AudioExtradata provided by bridge)
  → conv.ToAVPacket(packet, audioTimeBase)
  → muxer.WriteAudioPacket(avPkt)
  → segment file written to outputDir/segments/

PushAudio(data, pts, dts) — decode chain (no AudioExtradata, copy mode)
  → conv.ToAVPacket(packet, audioTimeBase)
  → audioDec.Decode(avPkt) → frames
  → audioResample.Convert(frame) → resampled frame
  → audioFifo.Write(resampled) → encoded packets
  → muxer.WriteAudioPacket(encPkt)
  → segment file written to outputDir/segments/
```
