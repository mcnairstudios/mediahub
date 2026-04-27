# pkg/av/demux

Demuxer opens a media source (URL or tee source) and reads packets in a loop. Handles PTS normalization, seek, reconnect with retry, audio track switching, and follow mode for growing files.

## Usage

```go
dm, err := demux.NewDemuxer(url, demux.DemuxOpts{
    TimeoutSec: 10,
    AudioTrack: -1,
})
if err != nil {
    return err
}
defer dm.Close()

info := dm.StreamInfo()

for {
    pkt, err := dm.ReadPacket()
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil {
        return err
    }
    // process pkt
}
```

## Key Design Decisions

### basePTS = -1 initialization

The first packet's PTS is captured and subtracted from all subsequent packets. Using -1 (not 0) as the sentinel ensures that a source with PTS starting at 0 is handled correctly.

### No audio PTS synthesis

Source PTS passes through directly. Earlier versions synthesized audio PTS from frame counts, which caused drift and A/V desync on seek.

### Seek preserves movie time

`SeekTo` sets `basePTS = 0` so post-seek packets retain their movie-time PTS. Seeking to 60s produces packets with PTS near 60s, not rebased to 0.

### RequestSeek runs on demux goroutine

`RequestSeek` sends a seek request via channel. The demux read loop processes it, ensuring thread safety. The `onSeek` callback fires BEFORE `RequestSeek` returns, so callers can flush decoders/muxers synchronously.

### Reconnect with exponential backoff

Transient errors (connection reset, timeout, EAGAIN) trigger up to 3 reconnect attempts with 1s/2s/4s delays. Stream indices are preserved across reconnects.

## DemuxOpts

| Field | Default | Description |
|-------|---------|-------------|
| TimeoutSec | 0 | Network timeout in seconds |
| AudioTrack | -1 | Audio track index (-1 = auto-select first) |
| AudioLanguage | "" | Prefer this ISO 639 language code |
| Follow | false | Growing file mode (retry on EOF) |
| FormatHint | "" | Force input format (e.g. "mpegts") |
| SATIPHTTPMode | false | Convert rtsp:// to http://:8875 |
| UserAgent | "" | Custom HTTP User-Agent header |
| RTSPLatency | 0 | RTSP jitterbuffer in milliseconds |
| AudioPassthrough | false | Reserved field (no-op) |
| ProbeSize | auto | Probe size in bytes |
| AnalyzeDuration | auto | Analysis duration in microseconds |
| CachedStreamInfo | nil | Skip FindStreamInfo if provided |
