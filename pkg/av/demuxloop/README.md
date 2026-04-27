# demuxloop

Context-aware read loop that pulls packets from a demuxer and pushes them to a PacketSink. This is the glue between demux and the output pipeline (e.g. FanOut, muxer, encoder).

## Usage

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

dm, _ := demux.NewDemuxer(url, demux.DemuxOpts{TimeoutSec: 10})
defer dm.Close()

err := demuxloop.Run(ctx, demuxloop.Config{
    Reader: dm,
    Sink:   fanOut,
})
```

## Lifecycle

The loop stops when:

- `ctx` is cancelled (viewer disconnect, seek, shutdown) - returns nil
- Reader returns `io.EOF` (VOD end) - calls `sink.EndOfStream()`, returns nil
- Reader returns a non-EOF error - returns the error
- Sink push returns an error - returns the error

Context is checked before and after each read to ensure prompt cancellation even when reads block.

## Testing

```bash
go test ./pkg/av/demuxloop/... -v
```

Tests use mock demuxer and mock sink -- no CGO or ffmpeg libs required.
