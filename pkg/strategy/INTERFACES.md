# strategy -- Public API

No interfaces defined. This package exports a single `Resolve` function that compares source media against client output requirements.

## Resolve

```go
func Resolve(in Input, out Output) Decision
```

Compare source stream properties against desired output and decide: copy, remux, or transcode.

### Input

| Field | Type | Description |
|-------|------|-------------|
| `VideoCodec` | `string` | Source video codec |
| `AudioCodec` | `string` | Source audio codec |
| `Width` | `int` | Source width |
| `Height` | `int` | Source height |
| `Interlaced` | `bool` | Whether source is interlaced |
| `BitDepth` | `int` | Source bit depth |

### Output

| Field | Type | Description |
|-------|------|-------------|
| `VideoCodec` | `string` | Desired output video codec ("default"/"copy" = match source) |
| `AudioCodec` | `string` | Desired output audio codec ("default"/"copy" = copy) |
| `Container` | `string` | Output container format |
| `HWAccel` | `string` | Hardware acceleration platform |
| `OutputHeight` | `int` | Maximum output height (0 = no limit) |
| `MaxBitDepth` | `int` | Maximum bit depth (0 = no limit) |

### Decision

| Field | Type | Description |
|-------|------|-------------|
| `VideoCodec` | `media.VideoCodec` | Resolved video codec (or "copy") |
| `AudioCodec` | `media.AudioCodec` | Resolved audio codec (or "copy") |
| `Container` | `media.Container` | Output container |
| `NeedsTranscode` | `bool` | Whether video transcoding is required |
| `NeedsAudioTranscode` | `bool` | Whether audio transcoding is required |
| `Deinterlace` | `bool` | Whether deinterlacing is needed |
| `HWAccel` | `string` | Hardware acceleration to use |
