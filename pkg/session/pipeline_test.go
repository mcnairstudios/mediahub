package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

func TestRunPipeline_BadURL(t *testing.T) {
	m := NewManager(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sess, _, err := m.GetOrCreate(ctx, "bad-stream", "http://127.0.0.1:1/nonexistent", "Bad Stream")
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	_, err = m.RunPipeline(sess, PipelineConfig{
		StreamURL:  "http://127.0.0.1:1/nonexistent",
		StreamID:   "bad-stream",
		TimeoutSec: 1,
	})
	if err == nil {
		t.Fatal("expected error for unreachable URL")
	}
	t.Logf("got expected error: %v", err)
}

func TestRunPipeline_EmptyURL(t *testing.T) {
	m := NewManager(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sess, _, _ := m.GetOrCreate(ctx, "empty-url", "", "Empty")

	_, err := m.RunPipeline(sess, PipelineConfig{
		StreamURL: "",
		StreamID:  "empty-url",
	})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestRunPipeline_ConfigDefaults(t *testing.T) {
	cfg := PipelineConfig{
		StreamURL: "http://example.com/stream",
		StreamID:  "test",
	}

	if cfg.TimeoutSec != 0 {
		t.Fatalf("expected default TimeoutSec 0 (resolved to 10 at runtime), got %d", cfg.TimeoutSec)
	}

	if cfg.AudioLanguage != "" {
		t.Fatalf("expected empty AudioLanguage by default, got %q", cfg.AudioLanguage)
	}
}

func TestRunPipeline_PipelineConfigFields(t *testing.T) {
	cfg := PipelineConfig{
		StreamURL:        "rtsp://192.168.1.100:554/stream",
		StreamID:         "test-rtsp",
		UserAgent:        "TestAgent/1.0",
		AudioLanguage:    "eng",
		NeedsTranscode:   true,
		OutputCodec:      "h264",
		OutputAudioCodec: "aac",
		HWAccel:          "vaapi",
		DecodeHWAccel:    "vaapi",
		Bitrate:          4000000,
		OutputHeight:     720,
		MaxBitDepth:      8,
		Deinterlace:      true,
		EncoderName:      "h264_vaapi",
		DecoderName:      "h264_vaapi",
		Framerate:        25,
		FormatHint:       "rtsp",
		TimeoutSec:       15,
	}

	if cfg.StreamURL != "rtsp://192.168.1.100:554/stream" {
		t.Error("StreamURL mismatch")
	}
	if cfg.AudioLanguage != "eng" {
		t.Error("AudioLanguage mismatch")
	}
	if cfg.FormatHint != "rtsp" {
		t.Error("FormatHint mismatch")
	}
	if cfg.TimeoutSec != 15 {
		t.Error("TimeoutSec mismatch")
	}
}

func TestRunPipeline_SessionDoneSignal(t *testing.T) {
	m := NewManager(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())

	sess, _, _ := m.GetOrCreate(ctx, "done-test", "http://127.0.0.1:1/fake", "Done Test")

	fakePipelineRunner := func(s *Session, cfg PipelineConfig) (*media.ProbeResult, error) {
		go func() {
			time.Sleep(10 * time.Millisecond)
			s.MarkDone()
		}()
		return &media.ProbeResult{
			Video: &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
		}, nil
	}

	info, err := fakePipelineRunner(sess, PipelineConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Video == nil {
		t.Fatal("expected video info")
	}

	time.Sleep(50 * time.Millisecond)

	if !sess.IsFinished() {
		t.Error("expected session to be marked done")
	}

	cancel()
}

func TestRunPipeline_SessionErrorCapture(t *testing.T) {
	m := NewManager(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sess, _, _ := m.GetOrCreate(ctx, "error-test", "http://fake/stream", "Error Test")

	testErr := fmt.Errorf("simulated demux failure")
	sess.SetError(testErr)

	if sess.Err() == nil {
		t.Fatal("expected error to be set")
	}
	if sess.Err().Error() != "simulated demux failure" {
		t.Fatalf("unexpected error: %v", sess.Err())
	}
}

func TestSession_MarkDoneAndIsFinished(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sess := newSession(ctx, cancel, "s1", "http://example.com", "Test", t.TempDir())

	if sess.IsFinished() {
		t.Fatal("should not be finished initially")
	}

	sess.MarkDone()

	if !sess.IsFinished() {
		t.Fatal("should be finished after MarkDone")
	}
}

func TestSession_SetAndGetError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sess := newSession(ctx, cancel, "s1", "http://example.com", "Test", t.TempDir())

	if sess.Err() != nil {
		t.Fatal("error should be nil initially")
	}

	sess.SetError(fmt.Errorf("test error"))

	if sess.Err() == nil {
		t.Fatal("error should not be nil after SetError")
	}
	if sess.Err().Error() != "test error" {
		t.Fatalf("unexpected error message: %v", sess.Err())
	}
}
