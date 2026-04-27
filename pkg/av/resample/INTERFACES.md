# pkg/av/resample interfaces

## Dependencies

- `github.com/asticode/go-astiav` -- SoftwareResampleContext, Frame, SampleFormat, ChannelLayout

## Exported API

```go
type Resampler struct { ... }

func NewResampler(srcChannels, srcRate int, srcFmt astiav.SampleFormat,
    dstChannels, dstRate int, dstFmt astiav.SampleFormat) (*Resampler, error)

func (r *Resampler) Convert(src *astiav.Frame) (*astiav.Frame, error)
func (r *Resampler) Reset()
func (r *Resampler) Close()
```

## Used By

- Audio pipeline: downmix 5.1 to stereo for browser MSE playback
- Seek handler: Reset() clears stale samples after seek
