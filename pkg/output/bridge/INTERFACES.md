# DecodeBridge Interfaces

## Implements
- `av.PacketSink` — PushVideo, PushAudio, PushSubtitle, EndOfStream

## Constructor
```go
type Config struct {
    Downstream       av.PacketSink  // FanOut or single output plugin
    Info             *media.ProbeResult
    AudioIndex       int
    AudioOnly        bool           // only decode/encode audio; video passes through
    HWAccel          string
    DecodeHWAccel    string
    OutputCodec      string         // "h264", "h265", "av1"
    OutputAudioCodec string         // "aac"
    Bitrate          int
    OutputHeight     int
    MaxBitDepth      int
    Deinterlace      bool
    Framerate        int
    EncoderName      string
    DecoderName      string
    Preset           string         // encoder preset (default "ultrafast")
    VideoCodecParams any            // *astiav.CodecParameters
    AudioCodecParams any            // *astiav.CodecParameters
    Log              zerolog.Logger
}

func New(cfg Config) (*Bridge, error)
```

## Public Methods
```go
func (b *Bridge) PushVideo(data []byte, pts, dts, duration int64, keyframe bool) error
func (b *Bridge) PushAudio(data []byte, pts, dts, duration int64) error
func (b *Bridge) PushSubtitle(data []byte, pts int64, duration int64) error
func (b *Bridge) EndOfStream()
func (b *Bridge) ResetForSeek()
func (b *Bridge) Stop()
func (b *Bridge) VideoEncoderExtradata() []byte
func (b *Bridge) VideoEncoderCodecID() astiav.CodecID
func (b *Bridge) AudioEncoderExtradata() []byte
func (b *Bridge) AudioEncoderCodecID() astiav.CodecID
```

## Internal Components
- Video: decoder → [deinterlacer] → [scaler] → encoder → downstream.PushVideo
- Audio: decoder → resampler → AudioFIFO → encoder → downstream.PushAudio
- Subtitle: passthrough to downstream.PushSubtitle
- `videoFrameDurNanos` — fallback duration (1e9/fps) when encoder output has zero duration
- `scalerCfg` — deferred scaler config; scaler created on first decoded frame to read actual pixel format
- `tsToNanosSafe()` — converts encoder timestamps to nanos, preserving `NoPtsValue` as `av.NoPtsNanos`

## Seek Reset Chain
```
ResetForSeek()
  → videoDec.FlushBuffers()
  → audioDec.FlushBuffers()
  → resampler.Reset()
  → audioFifo.Reset()
  → downstream.ResetForSeek() (if PacketSink has it)
```
