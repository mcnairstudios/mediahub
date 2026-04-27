# output -- Interfaces

## OutputPlugin

Core interface for all delivery plugins. Receives encoded packets from the pipeline.

```go
type OutputPlugin interface {
    Mode() DeliveryMode
    PushVideo(data []byte, pts, dts int64, keyframe bool) error
    PushAudio(data []byte, pts, dts int64) error
    PushSubtitle(data []byte, pts int64, duration int64) error
    EndOfStream()
    ResetForSeek()
    Stop()
    Status() PluginStatus
}
```

| Method | Description |
|--------|-------------|
| `Mode` | Return the delivery mode (mse, hls, stream, record) |
| `PushVideo` | Receive an encoded video packet |
| `PushAudio` | Receive an encoded audio packet |
| `PushSubtitle` | Receive a subtitle packet |
| `EndOfStream` | Signal that no more packets will arrive |
| `ResetForSeek` | Clear internal state after a seek |
| `Stop` | Shut down the plugin and release resources |
| `Status` | Return current health, byte count, segment count |

---

## ServablePlugin

Extends `OutputPlugin` with HTTP serving for browser/client-facing plugins (MSE, HLS).

```go
type ServablePlugin interface {
    OutputPlugin
    ServeHTTP(w http.ResponseWriter, r *http.Request)
    Generation() int64
    WaitReady(ctx context.Context) error
}
```

| Method | Description |
|--------|-------------|
| `ServeHTTP` | Serve segments/playlists over HTTP |
| `Generation` | Return the current generation counter (incremented on seek/reset) |
| `WaitReady` | Block until the first segment is available |

---

## FanOut

Distributes packets to multiple `OutputPlugin` instances. Not an interface but a key public struct.

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewFanOut` | `(plugins ...OutputPlugin) *FanOut` | Create with initial plugins |
| `PushVideo` | `(data []byte, pts, dts int64, keyframe bool) error` | Send video to all plugins |
| `PushAudio` | `(data []byte, pts, dts int64) error` | Send audio to all plugins |
| `PushSubtitle` | `(data []byte, pts int64, duration int64) error` | Send subtitle to all plugins |
| `EndOfStream` | `()` | Signal EOS to all plugins |
| `ResetForSeek` | `()` | Reset all plugins for seek |
| `Stop` | `()` | Stop all plugins |
| `Add` | `(p OutputPlugin)` | Add a plugin at runtime |
| `Remove` | `(mode DeliveryMode)` | Remove and stop a plugin by mode |
| `PluginCount` | `() int` | Return number of active plugins |
| `Status` | `() []PluginStatus` | Return status of all plugins |

---

## Registry

Factory registry for creating plugins by delivery mode.

| Method | Signature | Description |
|--------|-----------|-------------|
| `Register` | `(mode DeliveryMode, factory PluginFactory)` | Register a factory for a mode |
| `Create` | `(mode DeliveryMode, cfg PluginConfig) (OutputPlugin, error)` | Create a plugin from its factory |
| `Modes` | `() []DeliveryMode` | List all registered modes |

`PluginFactory` signature: `func(cfg PluginConfig) (OutputPlugin, error)`

---

## PluginConfig

Configuration passed to plugin factories when creating output plugins.

```go
type PluginConfig struct {
    OutputDir          string
    OutputFilePath     string
    OutputFormat       string
    IsLive             bool
    SegmentDurationSec int
    Video              *media.VideoInfo
    Audio              *media.AudioTrack
    VideoCodecParams   any // *astiav.CodecParameters from demuxer
    AudioCodecParams   any // *astiav.CodecParameters from demuxer
}
```

| Field | Description |
|-------|-------------|
| `OutputDir` | Directory for segment output (MSE, HLS) |
| `OutputFilePath` | Path for single-file output (stream, record) |
| `OutputFormat` | Container format (mp4, mpegts) |
| `IsLive` | Whether the source is live (infinite) or VOD (finite) |
| `SegmentDurationSec` | Target segment duration for MSE/HLS |
| `Video` | Video stream info from probe |
| `Audio` | Audio stream info from probe |
| `VideoCodecParams` | Codec parameters for video (from astiav demuxer) |
| `AudioCodecParams` | Codec parameters for audio (from astiav demuxer) |
