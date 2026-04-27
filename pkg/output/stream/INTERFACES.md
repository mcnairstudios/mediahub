# Stream Plugin Interfaces

## Implements
- `output.OutputPlugin` — PushVideo, PushAudio, PushSubtitle, EndOfStream, ResetForSeek, Stop, Status, Mode

Does NOT implement ServablePlugin — the handler layer serves the file directly.

## Constructor
```go
func New(cfg output.PluginConfig) (*Plugin, error)
```

PluginConfig fields used:
- OutputFilePath — file to write to
- OutputFormat — "mp4" or "mpegts"
- Video/Audio codec params

## Public Methods (beyond OutputPlugin)
```go
func (p *Plugin) FilePath() string   // path to the output file
func (p *Plugin) FileSize() int64    // current file size (for progress)
```

## Packet Flow
```
PushVideo(data, pts, dts, keyframe)
  → conv.ToAVPacket(packet, videoTimeBase)
  → streamMuxer.WritePacket(avPkt)
  → bytes appended to output file

PushAudio(data, pts, dts)
  → conv.ToAVPacket(packet, audioTimeBase)
  → streamMuxer.WritePacket(avPkt)
```
