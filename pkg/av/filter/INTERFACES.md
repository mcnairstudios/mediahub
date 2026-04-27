# filter interfaces

## Deinterlacer

```go
type Deinterlacer struct { /* unexported */ }

func NewDeinterlacer(width, height int, pixFmt astiav.PixelFormat, timeBase astiav.Rational) (*Deinterlacer, error)
func (d *Deinterlacer) Process(frame *astiav.Frame) (*astiav.Frame, error)
func (d *Deinterlacer) Close()
```

### NewDeinterlacer

Creates a yadif filter graph. Parameters describe the input video format. Returns error if libavfilter allocation fails.

| Param    | Type                 | Description                        |
|----------|----------------------|------------------------------------|
| width    | int                  | Frame width in pixels              |
| height   | int                  | Frame height in pixels             |
| pixFmt   | astiav.PixelFormat   | Input pixel format (e.g. yuv420p)  |
| timeBase | astiav.Rational      | Stream time base (e.g. 1/25)       |

### Process

Feeds a decoded frame through the yadif filter. Returns the deinterlaced frame, or `(nil, nil)` when yadif needs more input (EAGAIN). Caller owns the returned frame and must free it.

### Close

Frees the filter graph. Safe to call multiple times.
