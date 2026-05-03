package av

import (
	"io"
	"testing"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/media"
)

func TestStreamTypeConstants(t *testing.T) {
	if Video != 0 {
		t.Fatal("Video should be 0")
	}
	if Audio != 1 {
		t.Fatal("Audio should be 1")
	}
	if Subtitle != 2 {
		t.Fatal("Subtitle should be 2")
	}
}

func TestPacketCreation(t *testing.T) {
	pkt := Packet{
		Type:     Video,
		Data:     []byte{0x00, 0x01, 0x02},
		PTS:      1_000_000_000,
		DTS:      999_000_000,
		Duration: 33_333_333,
		Keyframe: true,
	}
	if pkt.Type != Video {
		t.Fatal("expected Video type")
	}
	if len(pkt.Data) != 3 {
		t.Fatal("expected 3 bytes")
	}
	if !pkt.Keyframe {
		t.Fatal("expected keyframe")
	}
}

func TestFrameVideoFields(t *testing.T) {
	f := Frame{
		Type:     Video,
		PTS:      500_000_000,
		Width:    1920,
		Height:   1080,
		PixelFmt: "yuv420p",
	}
	if f.Width != 1920 || f.Height != 1080 {
		t.Fatal("unexpected dimensions")
	}
	if f.PixelFmt != "yuv420p" {
		t.Fatal("unexpected pixel format")
	}
}

func TestFrameAudioFields(t *testing.T) {
	f := Frame{
		Type:       Audio,
		PTS:        250_000_000,
		SampleRate: 48000,
		Channels:   2,
		Samples:    1024,
	}
	if f.SampleRate != 48000 {
		t.Fatal("unexpected sample rate")
	}
	if f.Channels != 2 {
		t.Fatal("unexpected channels")
	}
	if f.Samples != 1024 {
		t.Fatal("unexpected samples")
	}
}

func TestFrameRawOpaque(t *testing.T) {
	type nativeHandle struct{ id int }
	f := Frame{
		Type: Video,
		Raw:  &nativeHandle{id: 42},
	}
	h, ok := f.Raw.(*nativeHandle)
	if !ok {
		t.Fatal("expected *nativeHandle")
	}
	if h.id != 42 {
		t.Fatal("unexpected id")
	}
}

func TestEncoderConfigFields(t *testing.T) {
	cfg := EncoderConfig{
		Codec:       "h264",
		HWAccel:     "vaapi",
		EncoderName: "h264_vaapi",
		Bitrate:     4000,
		Width:       1920,
		Height:      1080,
		Framerate:   30,
		MaxBitDepth: 8,
		SampleRate:  48000,
		Channels:    2,
	}
	if cfg.Codec != "h264" {
		t.Fatal("unexpected codec")
	}
	if cfg.HWAccel != "vaapi" {
		t.Fatal("unexpected hwaccel")
	}
	if cfg.Bitrate != 4000 {
		t.Fatal("unexpected bitrate")
	}
}

type mockDemuxer struct{}

func (m *mockDemuxer) StreamInfo() *media.ProbeResult                        { return &media.ProbeResult{} }
func (m *mockDemuxer) ReadPacket() (*Packet, error)                          { return nil, io.EOF }
func (m *mockDemuxer) SeekTo(posMs int64) error                              { return nil }
func (m *mockDemuxer) RequestSeek(posMs int64) error                         { return nil }
func (m *mockDemuxer) SetOnSeek(fn func())                                   {}
func (m *mockDemuxer) SetAudioTrack(idx int) error                           { return nil }
func (m *mockDemuxer) Reconnect() error                                      { return nil }
func (m *mockDemuxer) VideoCodecParameters() *astiav.CodecParameters         { return nil }
func (m *mockDemuxer) AudioCodecParameters() *astiav.CodecParameters         { return nil }
func (m *mockDemuxer) Close()                                                {}

type mockDecoder struct{}

func (m *mockDecoder) Decode(pkt *Packet) ([]Frame, error) { return nil, nil }
func (m *mockDecoder) FlushBuffers()                       {}
func (m *mockDecoder) Close()                              {}

type mockEncoder struct{}

func (m *mockEncoder) Encode(frame *Frame) ([]*Packet, error) { return nil, nil }
func (m *mockEncoder) Flush() ([]*Packet, error)              { return nil, nil }
func (m *mockEncoder) Extradata() []byte                      { return nil }
func (m *mockEncoder) FrameSize() int                         { return 1024 }
func (m *mockEncoder) Close()                                 {}

type mockMuxer struct{}

func (m *mockMuxer) WriteVideoPacket(pkt *Packet) error { return nil }
func (m *mockMuxer) WriteAudioPacket(pkt *Packet) error { return nil }
func (m *mockMuxer) Close() error                       { return nil }

type mockSegmentedMuxer struct{ mockMuxer }

func (m *mockSegmentedMuxer) SegmentCount() int { return 0 }
func (m *mockSegmentedMuxer) Reset() error      { return nil }

type mockFilter struct{}

func (m *mockFilter) Process(frame *Frame) (*Frame, error) { return frame, nil }
func (m *mockFilter) Close()                               {}

type mockPacketSink struct{}

func (m *mockPacketSink) PushVideo(data []byte, pts, dts int64, keyframe bool) error { return nil }
func (m *mockPacketSink) PushAudio(data []byte, pts, dts int64) error                { return nil }
func (m *mockPacketSink) PushSubtitle(data []byte, pts int64, duration int64) error   { return nil }
func (m *mockPacketSink) EndOfStream()                                                {}

func TestInterfaceSatisfaction(t *testing.T) {
	var _ Demuxer = &mockDemuxer{}
	var _ Decoder = &mockDecoder{}
	var _ Encoder = &mockEncoder{}
	var _ Muxer = &mockMuxer{}
	var _ SegmentedMuxer = &mockSegmentedMuxer{}
	var _ Filter = &mockFilter{}
	var _ PacketSink = &mockPacketSink{}
}
