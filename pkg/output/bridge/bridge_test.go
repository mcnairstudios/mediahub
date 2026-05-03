package bridge

import (
	"errors"
	"sync"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/rs/zerolog"
)

type mockSink struct {
	mu        sync.Mutex
	videos    int
	audios    int
	subtitles int
	eos       bool
	seekReset bool
}

func (m *mockSink) PushVideo(data []byte, pts, dts int64, keyframe bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.videos++
	return nil
}

func (m *mockSink) PushAudio(data []byte, pts, dts int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audios++
	return nil
}

func (m *mockSink) PushSubtitle(data []byte, pts int64, duration int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subtitles++
	return nil
}

func (m *mockSink) EndOfStream() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eos = true
}

var _ av.PacketSink = (*Bridge)(nil)

func testProbeResult() *media.ProbeResult {
	return &media.ProbeResult{
		Video: &media.VideoInfo{
			Index:      0,
			Codec:      "h264",
			Width:      1920,
			Height:     1080,
			BitDepth:   8,
			Interlaced: false,
			FramerateN: 25,
			FramerateD: 1,
			PixFmt:     "yuv420p",
		},
		AudioTracks: []media.AudioTrack{
			{
				Index:      1,
				Codec:      "aac",
				Language:   "eng",
				Channels:   2,
				SampleRate: 48000,
			},
		},
	}
}

func TestNew_MissingDownstream(t *testing.T) {
	cfg := Config{
		Info:             testProbeResult(),
		OutputCodec:      "h264",
		OutputAudioCodec: "aac",
		Log:              zerolog.Nop(),
	}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for missing downstream")
	}
	if !errors.Is(err, ErrNoDownstream) {
		t.Fatalf("expected ErrNoDownstream, got: %v", err)
	}
}

func TestNew_MissingInfo(t *testing.T) {
	sink := &mockSink{}
	cfg := Config{
		Downstream:       sink,
		OutputCodec:      "h264",
		OutputAudioCodec: "aac",
		Log:              zerolog.Nop(),
	}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for missing info")
	}
	if !errors.Is(err, ErrNoProbeResult) {
		t.Fatalf("expected ErrNoProbeResult, got: %v", err)
	}
}

func TestNew_MissingVideoInfo(t *testing.T) {
	sink := &mockSink{}
	cfg := Config{
		Downstream:       sink,
		Info:             &media.ProbeResult{},
		OutputCodec:      "h264",
		OutputAudioCodec: "aac",
		Log:              zerolog.Nop(),
	}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for missing video info")
	}
	if !errors.Is(err, ErrNoVideoInfo) {
		t.Fatalf("expected ErrNoVideoInfo, got: %v", err)
	}
}

func TestNew_ValidConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("requires CGO/ffmpeg libs")
	}

	sink := &mockSink{}
	cfg := Config{
		Downstream:       sink,
		Info:             testProbeResult(),
		AudioIndex:       1,
		OutputCodec:      "h264",
		OutputAudioCodec: "aac",
		Log:              zerolog.Nop(),
	}
	b, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Stop()

	if b.downstream != sink {
		t.Fatal("downstream not set correctly")
	}
}

func TestResetForSeek_NoPanic(t *testing.T) {
	if testing.Short() {
		t.Skip("requires CGO/ffmpeg libs")
	}

	sink := &mockSink{}
	cfg := Config{
		Downstream:       sink,
		Info:             testProbeResult(),
		AudioIndex:       1,
		OutputCodec:      "h264",
		OutputAudioCodec: "aac",
		Log:              zerolog.Nop(),
	}
	b, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Stop()

	b.ResetForSeek()
	b.ResetForSeek()
}

func TestStop_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("requires CGO/ffmpeg libs")
	}

	sink := &mockSink{}
	cfg := Config{
		Downstream:       sink,
		Info:             testProbeResult(),
		AudioIndex:       1,
		OutputCodec:      "h264",
		OutputAudioCodec: "aac",
		Log:              zerolog.Nop(),
	}
	b, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Stop()
	b.Stop()
	b.Stop()
}

func TestPushSubtitle_Passthrough(t *testing.T) {
	if testing.Short() {
		t.Skip("requires CGO/ffmpeg libs")
	}

	sink := &mockSink{}
	cfg := Config{
		Downstream:       sink,
		Info:             testProbeResult(),
		AudioIndex:       1,
		OutputCodec:      "h264",
		OutputAudioCodec: "aac",
		Log:              zerolog.Nop(),
	}
	b, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Stop()

	if err := b.PushSubtitle([]byte("test"), 1000, 500); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if sink.subtitles != 1 {
		t.Fatalf("expected 1 subtitle push, got %d", sink.subtitles)
	}
}

func TestEndOfStream_ForwardsToDownstream(t *testing.T) {
	if testing.Short() {
		t.Skip("requires CGO/ffmpeg libs")
	}

	sink := &mockSink{}
	cfg := Config{
		Downstream:       sink,
		Info:             testProbeResult(),
		AudioIndex:       1,
		OutputCodec:      "h264",
		OutputAudioCodec: "aac",
		Log:              zerolog.Nop(),
	}
	b, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.EndOfStream()

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if !sink.eos {
		t.Fatal("expected EndOfStream to be forwarded to downstream")
	}
}

func TestFramerateCalculation(t *testing.T) {
	tests := []struct {
		name        string
		framerateN  int
		framerateD  int
		interlaced  bool
		deinterlace bool
		framerate   int
		expected    int
	}{
		{"25fps progressive", 25, 1, false, false, 0, 25},
		{"50fps interlaced with deinterlace", 50, 1, true, true, 0, 25},
		{"30fps progressive", 30, 1, false, false, 0, 30},
		{"zero framerate defaults to 25", 0, 0, false, false, 0, 25},
		{"explicit framerate override", 25, 1, false, false, 30, 30},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fps := resolveFramerate(tc.framerateN, tc.framerateD, tc.interlaced, tc.deinterlace, tc.framerate)
			if fps != tc.expected {
				t.Errorf("expected framerate %d, got %d", tc.expected, fps)
			}
		})
	}
}

func TestResolveOutputDimensions(t *testing.T) {
	tests := []struct {
		name         string
		srcW, srcH   int
		outputHeight int
		expectW      int
		expectH      int
		expectScale  bool
	}{
		{"no scaling", 1920, 1080, 0, 1920, 1080, false},
		{"720p from 1080p", 1920, 1080, 720, 1280, 720, true},
		{"no upscale", 1280, 720, 1080, 1280, 720, false},
		{"even width", 1920, 1080, 480, 852, 480, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, h, needsScale := resolveOutputDimensions(tc.srcW, tc.srcH, tc.outputHeight)
			if w != tc.expectW || h != tc.expectH {
				t.Errorf("expected %dx%d, got %dx%d", tc.expectW, tc.expectH, w, h)
			}
			if needsScale != tc.expectScale {
				t.Errorf("expected needsScale=%v, got %v", tc.expectScale, needsScale)
			}
		})
	}
}
