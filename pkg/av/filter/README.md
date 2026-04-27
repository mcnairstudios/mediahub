# filter

Deinterlace filter using yadif via libavfilter.

Converts interlaced video frames to progressive using the yadif (Yet Another DeInterlacing Filter) algorithm. Operates in `send_frame` mode with automatic parity detection, only deinterlacing frames flagged as interlaced.

## Usage

```go
tb := astiav.NewRational(1, 25)
d, err := filter.NewDeinterlacer(1920, 1080, astiav.PixelFormatYuv420P, tb)
if err != nil {
    return err
}
defer d.Close()

out, err := d.Process(decodedFrame)
if err != nil {
    return err
}
if out == nil {
    // yadif returns nil on first frame (buffering) — continue to next frame
    continue
}
defer out.Free()
```

## EAGAIN behavior

yadif buffers the first frame to determine field order. `Process` returns `(nil, nil)` when the filter needs more input. Callers must handle this by continuing to feed frames.

## Dependencies

Requires ffmpeg libavfilter dev headers (`libavfilter-dev` or equivalent).
