# pkg/av/mux interfaces

## FragmentedMuxer

```go
func NewFragmentedMuxer(opts MuxOpts) (*FragmentedMuxer, error)
func (m *FragmentedMuxer) WriteVideoPacket(pkt *astiav.Packet) error
func (m *FragmentedMuxer) WriteAudioPacket(pkt *astiav.Packet) error
func (m *FragmentedMuxer) VideoCodecString() string
func (m *FragmentedMuxer) Reset() error
func (m *FragmentedMuxer) Close() error
```

## StreamMuxer

```go
func NewStreamMuxer(format string, w io.Writer) (*StreamMuxer, error)
func (m *StreamMuxer) AddStream(codecParams *astiav.CodecParameters) (*astiav.Stream, error)
func (m *StreamMuxer) WriteHeader() error
func (m *StreamMuxer) WritePacket(pkt *astiav.Packet) error
func (m *StreamMuxer) Close() error
```

## HLSMuxer

```go
func NewHLSMuxer(opts HLSMuxOpts) (*HLSMuxer, error)
func (m *HLSMuxer) WriteVideoPacket(pkt *astiav.Packet) error
func (m *HLSMuxer) WriteAudioPacket(pkt *astiav.Packet) error
func (m *HLSMuxer) Reset() error
func (m *HLSMuxer) Close() error
func (m *HLSMuxer) SegmentCount() int
func (m *HLSMuxer) PlaylistContent() string
```

## Configuration types

```go
type MuxOpts struct {
    OutputDir         string
    SegmentDurationMs int
    AudioFragmentMs   int
    VideoCodecID      astiav.CodecID
    VideoExtradata    []byte
    VideoWidth        int
    VideoHeight       int
    VideoTimeBase     astiav.Rational
    AudioCodecID      astiav.CodecID
    AudioExtradata    []byte
    AudioChannels     int
    AudioSampleRate   int
}

type HLSMuxOpts struct {
    OutputDir          string
    SegmentDurationSec int
    VideoCodecID       astiav.CodecID
    VideoExtradata     []byte
    VideoWidth         int
    VideoHeight        int
    VideoTimeBase      astiav.Rational
    VideoFrameRate     int
    AudioCodecID       astiav.CodecID
    AudioExtradata     []byte
    AudioChannels      int
    AudioSampleRate    int
    AudioTimeBase      astiav.Rational
    AudioFrameSize     int
}
```
