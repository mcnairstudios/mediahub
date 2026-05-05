# DASH Plugin Interfaces

## Implements

### output.OutputPlugin
- `Mode() DeliveryMode` — returns `DeliveryDASH`
- `PushVideo(data []byte, pts, dts, duration int64, keyframe bool) error`
- `PushAudio(data []byte, pts, dts, duration int64) error`
- `PushSubtitle(data []byte, pts int64, duration int64) error`
- `EndOfStream()`
- `ResetForSeek()`
- `Stop()`
- `Status() PluginStatus`

### output.ServablePlugin
- `ServeHTTP(w http.ResponseWriter, r *http.Request)`
- `Generation() int64`
- `WaitReady(ctx context.Context) error`

## Constructor
```go
func New(cfg output.PluginConfig) (*Plugin, error)
```

Takes the standard `output.PluginConfig`. No custom options required.

## Dependencies
- `pkg/av/mux.FragmentedMuxer` — fMP4 segment production
- `pkg/av/conv` — packet format conversion
- `pkg/av/decode` — audio decoding (when no AudioExtradata)
- `pkg/av/resample` — audio resampling (when decoding)
- `pkg/av/encode` — audio encoding (when decoding)
- `pkg/av/bsf` — bitstream filter for extradata extraction
- `pkg/av/extradata` — codec data extraction from keyframes
