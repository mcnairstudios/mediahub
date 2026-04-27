# pkg/av/scale

Video frame scaling and pixel format conversion using libswscale via go-astiav. Uses bilinear interpolation.

## Usage

```go
s, err := scale.NewScaler(
    1920, 1080, astiav.PixelFormatYuv420P,  // source: 1080p YUV420
    1280, 720, astiav.PixelFormatYuv420P,   // dest: 720p YUV420
)
defer s.Close()

dst, err := s.Scale(srcFrame)
// dst is a new frame owned by the caller
defer dst.Free()
```

## Pixel Format Conversion

The scaler also handles pixel format conversion without resolution change:

```go
s, err := scale.NewScaler(
    1920, 1080, astiav.PixelFormatNv12,     // source: NV12 (HW decode output)
    1920, 1080, astiav.PixelFormatYuv420P,  // dest: YUV420P (SW encoder input)
)
```

## CGO Required

Depends on libswscale via go-astiav. Build with `CGO_ENABLED=1`.
