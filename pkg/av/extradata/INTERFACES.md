# pkg/av/extradata -- Interfaces

## Dependencies (imports)

- `encoding/hex` -- standard library
- `fmt` -- standard library

No external dependencies. Pure Go package.

## Exported API

```go
func ToCodecData(codec string, extradata []byte) ([]byte, error)
func ToHexString(data []byte) string
func SplitNALUnits(data []byte) [][]byte
func ParseH264SPS(sps []byte) *H264SPSInfo
func ParseH265SPS(sps []byte) *H265SPSInfo
```

## Exported Types

```go
type H264SPSInfo struct {
    ProfileIDC       uint8
    LevelIDC         uint8
    ChromaFormatIDC  uint32
    BitDepthLuma     uint32
    BitDepthChroma   uint32
    FrameMBSOnlyFlag bool
    Width            uint32
    Height           uint32
}

type H265SPSInfo struct {
    ChromaFormatIDC uint32
    BitDepthLuma    uint32
    BitDepthChroma  uint32
    Width           uint32
    Height          uint32
}
```

## Consumed by

- Mux package (future) -- needs codec data for init segments (fMP4 moov, HLS)
- Probe package (future) -- needs SPS parsing for codec string extraction (e.g. "avc1.640028")
- Keyframe package (future) -- NAL unit splitting for IDR detection
