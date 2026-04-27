# demux — Interface Contract

## Demuxer (implements av.Demuxer)

```go
type Demuxer struct { ... }

func NewDemuxer(url string, opts DemuxOpts) (*Demuxer, error)
func NewDemuxerWithTee(tee TeeSource, opts DemuxOpts) (*Demuxer, error)

func (d *Demuxer) StreamInfo() *media.ProbeResult
func (d *Demuxer) ReadPacket() (*av.Packet, error)
func (d *Demuxer) SeekTo(posMs int64) error
func (d *Demuxer) RequestSeek(posMs int64) error
func (d *Demuxer) SetOnSeek(fn func())
func (d *Demuxer) SetAudioTrack(idx int) error
func (d *Demuxer) Reconnect() error
func (d *Demuxer) VideoCodecParameters() *astiav.CodecParameters
func (d *Demuxer) AudioCodecParameters() *astiav.CodecParameters
func (d *Demuxer) Close()
```

## TeeSource

Callers that provide raw I/O (e.g. recording the source while demuxing) implement this:

```go
type TeeSource interface {
    SetupFormatContext(fc *astiav.FormatContext)
}
```

## DemuxOpts

```go
type DemuxOpts struct {
    TimeoutSec       int
    AudioTrack       int
    AudioLanguage    string
    Follow           bool
    FormatHint       string
    SATIPHTTPMode    bool
    UserAgent        string
    RTSPLatency      int
    AudioPassthrough bool
    ProbeSize        int
    AnalyzeDuration  int
    CachedStreamInfo *media.ProbeResult
}
```

## Thread Safety

- `ReadPacket` must be called from a single goroutine (the demux loop).
- `RequestSeek` is safe to call from any goroutine (channel-based).
- `SetAudioTrack` is safe to call from any goroutine (mutex-protected).
- `SetOnSeek` must be called before the demux loop starts.
- `Close` must be called after the demux loop exits.

## Error Contract

- `ReadPacket` returns `io.EOF` at end of stream.
- `ReadPacket` retries transient errors (connection reset, timeout) with exponential backoff.
- `SeekTo` / `RequestSeek` return an error if the seek fails.
- `SetAudioTrack` returns an error if the index is invalid or not an audio stream.
- `RequestSeek` returns "seek channel full" if a seek is already pending.
