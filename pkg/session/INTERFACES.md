# session -- Public API

No interfaces defined. `Manager` and `Session` are concrete structs.

## Manager

Manages active sessions keyed by stream ID. One session per stream.

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewManager` | `(outputDir string) *Manager` | Create a manager with the given output directory |
| `GetOrCreate` | `(ctx context.Context, streamID, streamURL, streamName string) (*Session, bool, error)` | Return existing session or create a new one; bool indicates created |
| `Get` | `(streamID string) *Session` | Look up a session by stream ID |
| `Stop` | `(streamID string)` | Stop and remove a session |
| `StopAll` | `()` | Stop all active sessions |
| `ActiveCount` | `() int` | Return number of active sessions |
| `List` | `() []*Session` | Return all active sessions |
| `AddPlugin` | `(streamID string, plugin output.OutputPlugin) error` | Add an output plugin to an existing session |
| `RemovePlugin` | `(streamID string, mode output.DeliveryMode) error` | Remove an output plugin by delivery mode |
| `RunPipeline` | `(sess *Session, cfg PipelineConfig) (*PipelineResult, error)` | Run the AV pipeline for a session |

## Session

Represents a single active stream with fan-out delivery.

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `string` | Unique session identifier |
| `StreamID` | `string` | Stream this session is playing |
| `StreamURL` | `string` | Source URL |
| `StreamName` | `string` | Display name |
| `OutputDir` | `string` | Directory for segments and metadata |
| `FanOut` | `*output.FanOut` | Packet distributor |
| `CreatedAt` | `time.Time` | When the session started |

| Method | Signature | Description |
|--------|-----------|-------------|
| `Context` | `() context.Context` | Return the session's context |
| `AddCloser` | `(c io.Closer)` | Register a closer to be called on session stop |
| `SetRecorded` | `(v bool)` | Mark session as being recorded |
| `IsRecorded` | `() bool` | Check if session is being recorded |
| `Stop` | `()` | Cancel context, stop fan-out, close done channel |
| `Done` | `() <-chan struct{}` | Channel closed when session stops |
| `SetSeekFunc` | `(fn func(posMs int64))` | Register the seek callback |
| `Seek` | `(posMs int64)` | Invoke the registered seek callback |
| `SetError` | `(err error)` | Store pipeline error |
| `Err` | `() error` | Retrieve pipeline error |
| `MarkDone` | `()` | Mark session as finished (pipeline completed normally) |
| `IsFinished` | `() bool` | Check if the pipeline has completed |

## PipelineConfig

```go
type PipelineConfig struct {
    StreamURL        string
    StreamID         string
    UserAgent        string
    AudioLanguage    string
    NeedsTranscode   bool
    OutputCodec      string
    OutputAudioCodec string
    HWAccel          string
    DecodeHWAccel    string
    Bitrate          int
    OutputHeight     int
    MaxBitDepth      int
    Deinterlace      bool
    EncoderName      string
    DecoderName      string
    Framerate        int
    FormatHint       string
    TimeoutSec       int
}
```

## PipelineResult

```go
type PipelineResult struct {
    Info             *media.ProbeResult
    VideoCodecParams any // *astiav.CodecParameters
    AudioCodecParams any // *astiav.CodecParameters
}
```
