# pkg/av

AV processing interfaces and core types for mediahub.

This package defines the contracts between the AV layer (libavformat/libavcodec via go-astiav) and the rest of the system. No CGO code lives here -- only interfaces and plain Go types.

## Files

| File | Purpose |
|------|---------|
| `av.go` | Core types: `Packet`, `Frame`, `StreamType` constants |
| `demux.go` | `Demuxer` interface -- open media source, read packets, seek |
| `decode.go` | `Decoder` interface + `Frame` type -- decompress packets to raw frames |
| `encode.go` | `Encoder` interface + `EncoderConfig` -- compress frames to packets |
| `mux.go` | `Muxer` and `SegmentedMuxer` interfaces -- write packets to containers |
| `filter.go` | `Filter` interface -- frame processing (deinterlace, scale, pixfmt) |
| `pipeline.go` | `PacketSink` interface -- what the demux loop pushes packets to |

## Design

- **Interfaces only** -- concrete implementations using go-astiav live in subpackages
- **Depends only on `pkg/media/`** for `ProbeResult`
- **All PTS/DTS values are in nanoseconds**
- **`Frame.Raw` is `any`** -- implementations store native frame handles (e.g. go-astiav Frame)
- Output plugins and the orchestrator never touch go-astiav directly; they work through these abstractions

## Data Flow

```
Demuxer → PacketSink (compressed packets)
Demuxer → Decoder → Filter → Encoder → Muxer (full transcode)
Demuxer → Muxer (copy/remux)
```
