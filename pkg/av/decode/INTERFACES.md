# pkg/av/decode -- Interfaces

## Dependencies (imports)

- `github.com/asticode/go-astiav` -- go-astiav (libavcodec CGO bindings)
- `github.com/rs/zerolog` -- structured logging
- CGO: `libavcodec` (pkg-config)

## Exported API

```go
func NewVideoDecoderFromParams(cp *astiav.CodecParameters, opts DecodeOpts) (*Decoder, error)
func NewVideoDecoder(codecID astiav.CodecID, extradata []byte, opts DecodeOpts) (*Decoder, error)
func NewAudioDecoderFromParams(cp *astiav.CodecParameters) (*Decoder, error)
func NewAudioDecoder(codecID astiav.CodecID, extradata []byte) (*Decoder, error)
func HWAccelMap() map[string]astiav.HardwareDeviceType
func BitDepthFromPixelFormat(pf astiav.PixelFormat) int
func ExceedsMaxBitDepth(pf astiav.PixelFormat, maxBitDepth int) bool
```

## Exported Types

```go
type Decoder struct { /* unexported fields */ }

type DecodeOpts struct {
    HWAccel     string // "vaapi", "qsv", "videotoolbox", "cuda", "nvenc", "d3d11va", "dxva2", "vulkan", "none", ""
    MaxBitDepth int    // 0 = no limit; >0 = force SW when source exceeds this
    DecoderName string // explicit decoder name override (e.g. "h264_cuvid")
}
```

## Methods on Decoder

```go
func (d *Decoder) Decode(pkt *astiav.Packet) ([]*astiav.Frame, error)
func (d *Decoder) Flush() ([]*astiav.Frame, error)
func (d *Decoder) FlushBuffers()
func (d *Decoder) Close()
```

## Consumed by

- Pipeline orchestration (future) -- video/audio decode step between demux and encode
- Session package (future) -- seek handling calls FlushBuffers
- VOD service (future) -- transcode pipelines

## Consumes

- go-astiav CodecParameters (from demux package, future)
- go-astiav Packet (from demux/conv package, future)
