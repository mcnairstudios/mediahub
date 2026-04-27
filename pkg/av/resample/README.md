# pkg/av/resample

Audio resampling and format conversion using libswresample via go-astiav. Handles channel downmix, sample rate conversion, and sample format changes.

## Supported Channel Layouts

- 1 channel (mono)
- 2 channels (stereo)
- 6 channels (5.1)
- 8 channels (7.1)

## Usage

```go
r, err := resample.NewResampler(
    6, 48000, astiav.SampleFormatFltp,  // source: 5.1, 48kHz, float planar
    2, 48000, astiav.SampleFormatFltp,  // dest: stereo, 48kHz, float planar
)
defer r.Close()

dst, err := r.Convert(srcFrame)
// dst.Pts() == srcFrame.Pts() -- PTS is preserved
defer dst.Free()
```

## Input Change Handling

The SoftwareResampleContext auto-negotiates from source frame properties. If the input format changes mid-stream (codec switch, channel layout change), `Convert` detects `ErrInputChanged`, reallocates the context, and retries transparently.

## Seek Support

Call `Reset()` after a seek to flush stale pre-seek samples from the resampler's internal buffers. Config (dst layout, rate, format) is preserved across reset.

## CGO Required

Depends on libswresample via go-astiav. Build with `CGO_ENABLED=1`.
