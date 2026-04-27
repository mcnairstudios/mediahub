# pkg/av/conv — Interfaces

## Dependencies (imports)

- `github.com/asticode/go-astiav` — go-astiav CGO bindings to ffmpeg
- `github.com/mcnairstudios/mediahub/pkg/av` — `av.Packet` type
- `github.com/mcnairstudios/mediahub/pkg/media` — `media.VideoInfo`, `media.AudioTrack` types

## Exported API

```go
func ToAVPacket(p *av.Packet, timeBase astiav.Rational) (*astiav.Packet, error)
func CodecIDFromString(codec string) (astiav.CodecID, error)
func CodecParamsFromVideoProbe(v *media.VideoInfo) (*astiav.CodecParameters, error)
func CodecParamsFromAudioProbe(a *media.AudioTrack) (*astiav.CodecParameters, error)
```

## Consumed by

- Decode package (future) — needs `CodecParamsFromVideoProbe`/`CodecParamsFromAudioProbe` to create decoders
- Encode package (future) — needs `CodecIDFromString` for output codec selection
- Mux package (future) — needs `ToAVPacket` to feed packets to ffmpeg muxers

## PTS Convention

All `av.Packet` timestamps are nanoseconds. `ToAVPacket` converts to the stream's timebase using:
```
pts_tb = pts_ns * timebase_den / (1_000_000_000 * timebase_num)
```
