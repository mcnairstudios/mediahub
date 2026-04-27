# Recording Plugin Interfaces

## Implements
- `output.OutputPlugin` — PushVideo, PushAudio, PushSubtitle, EndOfStream, ResetForSeek, Stop, Status, Mode

Does NOT implement ServablePlugin — recordings are played back as input sources.

## Constructor
```go
func New(cfg output.PluginConfig) (*Plugin, error)
```

PluginConfig fields used:
- RecordingPath — file to write to (e.g. `<recorddir>/stream/<id>/active/source.mp4`)
- Video/Audio codec params

## Public Methods (beyond OutputPlugin)
```go
func (p *Plugin) FilePath() string       // path to the recording file
func (p *Plugin) FileSize() int64        // current file size
func (p *Plugin) SetPreserved(v bool)    // mark for preservation on cleanup
func (p *Plugin) IsPreserved() bool
```

## Behaviour
- ResetForSeek: no-op — recording is continuous, seek is a playback concept
- Stop: finalizes the mp4 container (writes moov atom)
- EndOfStream: same as Stop

## Packet Flow
```
PushVideo(data, pts, dts, keyframe)
  → conv.ToAVPacket(packet, videoTimeBase)
  → streamMuxer.WritePacket(avPkt)
  → bytes appended to recording mp4

PushAudio(data, pts, dts)
  → conv.ToAVPacket(packet, audioTimeBase)
  → streamMuxer.WritePacket(avPkt)
```
