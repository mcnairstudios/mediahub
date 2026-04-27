# demuxloop interfaces

## Consumed interfaces

### PacketReader (defined in this package)

Narrow interface -- only requires `ReadPacket()`. The full `av.Demuxer` interface satisfies this, as does `demux.Demuxer`.

```go
type PacketReader interface {
    ReadPacket() (*av.Packet, error)
}
```

Concrete implementation: `demux.Demuxer`

### av.PacketSink (pkg/av/pipeline.go)

The loop pushes packets to the sink based on stream type.

```go
type PacketSink interface {
    PushVideo(data []byte, pts, dts int64, keyframe bool) error
    PushAudio(data []byte, pts, dts int64) error
    PushSubtitle(data []byte, pts int64, duration int64) error
    EndOfStream()
}
```

Concrete implementations: FanOut, DecodeBridge, output plugins.

## Exported types

### Config

```go
type Config struct {
    Reader PacketReader
    Sink   av.PacketSink
}
```

### Run

```go
func Run(ctx context.Context, cfg Config) error
```
