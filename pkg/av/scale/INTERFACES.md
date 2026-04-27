# pkg/av/scale interfaces

## Dependencies

- `github.com/asticode/go-astiav` -- SoftwareScaleContext, Frame, PixelFormat

## Exported API

```go
type Scaler struct { ... }

func NewScaler(srcW, srcH int, srcFmt astiav.PixelFormat,
    dstW, dstH int, dstFmt astiav.PixelFormat) (*Scaler, error)

func (s *Scaler) Scale(src *astiav.Frame) (*astiav.Frame, error)
func (s *Scaler) Close()
```

## Used By

- Video pipeline: downscale when source resolution exceeds client OutputHeight ceiling
- Pixel format conversion: NV12 (HW decode) to YUV420P (SW encode)
