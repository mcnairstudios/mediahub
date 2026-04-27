# DecodeBridge Interfaces

## Implements
- `av.PacketSink` — PushVideo, PushAudio, PushSubtitle, EndOfStream

## Constructor
```go
type Config struct {
    Downstream    av.PacketSink  // FanOut or single output plugin
    Info          *media.ProbeResult
    AudioIndex    int
    HWAccel       string
    DecodeHWAccel string
    OutputCodec   string         // "h264", "h265", "av1"
    OutputAudioCodec string      // "aac"
    Bitrate       int
    OutputHeight  int
    MaxBitDepth   int
    Deinterlace   bool
    EncoderName   string
    DecoderName   string
    VideoCodecParams any  // *astiav.CodecParameters
    AudioCodecParams any  // *astiav.CodecParameters
    Log           zerolog.Logger
}

func New(cfg Config) (*Bridge, error)
```

## Public Methods
```go
func (b *Bridge) PushVideo(data []byte, pts, dts int64, keyframe bool) error
func (b *Bridge) PushAudio(data []byte, pts, dts int64) error
func (b *Bridge) PushSubtitle(data []byte, pts int64, duration int64) error
func (b *Bridge) EndOfStream()
func (b *Bridge) ResetForSeek()
func (b *Bridge) Stop()
```

## Internal Components
- Video: decoder → [deinterlacer] → [scaler] → encoder → downstream.PushVideo
- Audio: decoder → resampler → AudioFIFO → encoder → downstream.PushAudio
- Subtitle: passthrough to downstream.PushSubtitle

## Seek Reset Chain
```
ResetForSeek()
  → videoDec.FlushBuffers()
  → audioDec.FlushBuffers()
  → resampler.Reset()
  → audioFifo.Reset()
  → downstream.ResetForSeek() (if PacketSink has it)
```
