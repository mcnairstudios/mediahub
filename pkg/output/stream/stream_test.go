package stream

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
)

var _ output.OutputPlugin = (*Plugin)(nil)

func TestModeReturnsDeliveryStream(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")
	defer p.Stop()

	if p.Mode() != output.DeliveryStream {
		t.Fatalf("expected mode %s, got %s", output.DeliveryStream, p.Mode())
	}
}

func TestConstructionWithValidConfig(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")
	defer p.Stop()

	if p.FilePath() == "" {
		t.Fatal("expected non-empty file path")
	}
	if _, err := os.Stat(p.FilePath()); err != nil {
		t.Fatalf("output file should exist: %v", err)
	}
}

func TestConstructionMissingFilePath(t *testing.T) {
	cfg := output.PluginConfig{
		OutputFormat: "mpegts",
		Video:        testVideo(),
	}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for missing file path")
	}
}

func TestConstructionMissingFormat(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputFilePath: filepath.Join(dir, "out.ts"),
		Video:          testVideo(),
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("expected default format, got error: %v", err)
	}
	defer p.Stop()
}

func TestPushVideoWritesToFile(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")

	for i := 0; i < 10; i++ {
		data := makeNALU(4096)
		pts := int64(i) * 33_333_333
		if err := p.PushVideo(data, pts, pts, i == 0); err != nil {
			t.Fatalf("PushVideo[%d]: %v", i, err)
		}
	}

	p.Stop()

	info, err := os.Stat(p.FilePath())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected file to have data after PushVideo + Stop")
	}
	if p.FileSize() == 0 {
		t.Fatal("expected non-zero FileSize after PushVideo + Stop")
	}
}

func TestPushAudioWritesToFile(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")

	for i := 0; i < 10; i++ {
		pts := int64(i) * 33_333_333
		if err := p.PushVideo(makeNALU(4096), pts, pts, i == 0); err != nil {
			t.Fatalf("PushVideo[%d]: %v", i, err)
		}
		audioPTS := int64(i) * 21_333_333
		if err := p.PushAudio(make([]byte, 1024), audioPTS, audioPTS); err != nil {
			t.Fatalf("PushAudio[%d]: %v", i, err)
		}
	}

	p.Stop()

	info, err := os.Stat(p.FilePath())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected file to have data after PushAudio + Stop")
	}
}

func TestStopFinalizesFile(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")

	for i := 0; i < 5; i++ {
		pts := int64(i) * 33_333_333
		_ = p.PushVideo(makeNALU(4096), pts, pts, i == 0)
	}

	filePath := p.FilePath()
	p.Stop()

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("file should still exist after Stop: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("file should not be empty after Stop")
	}
}

func TestStatusReportsHealthyAndBytes(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")

	status := p.Status()
	if !status.Healthy {
		t.Fatal("expected healthy status before stop")
	}
	if status.Mode != output.DeliveryStream {
		t.Fatalf("expected mode %s, got %s", output.DeliveryStream, status.Mode)
	}

	for i := 0; i < 10; i++ {
		pts := int64(i) * 33_333_333
		_ = p.PushVideo(makeNALU(4096), pts, pts, i == 0)
	}

	p.Stop()

	status = p.Status()
	if status.BytesWritten == 0 {
		t.Fatal("expected non-zero BytesWritten after push + stop")
	}
}

func TestDoubleStopIsSafe(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")
	p.Stop()
	p.Stop()
}

func TestPushAfterStopIsNoop(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")
	p.Stop()

	if err := p.PushVideo([]byte{0x00}, 0, 0, true); err != nil {
		t.Fatalf("PushVideo after stop should not error, got: %v", err)
	}
	if err := p.PushAudio([]byte{0x00}, 0, 0); err != nil {
		t.Fatalf("PushAudio after stop should not error, got: %v", err)
	}
}

func TestPushSubtitleIsNoop(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")
	defer p.Stop()

	if err := p.PushSubtitle([]byte("hello"), 0, 1000); err != nil {
		t.Fatalf("PushSubtitle should not error, got: %v", err)
	}
}

func TestEndOfStreamStopsPlugin(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")
	p.EndOfStream()

	status := p.Status()
	if status.Healthy {
		t.Fatal("expected unhealthy after EndOfStream (plugin stopped)")
	}
}

func TestResetForSeekIsNoop(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")
	defer p.Stop()
	p.ResetForSeek()
}

func TestConstructionVideoOnly(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputFilePath: filepath.Join(dir, "video_only.ts"),
		OutputFormat:   "mpegts",
		Video:          testVideo(),
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New with nil audio should work: %v", err)
	}
	defer p.Stop()

	if p.Mode() != output.DeliveryStream {
		t.Fatalf("expected mode %s, got %s", output.DeliveryStream, p.Mode())
	}
}

func TestPushAudioNoAudioStream(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputFilePath: filepath.Join(dir, "video_only.ts"),
		OutputFormat:   "mpegts",
		Video:          testVideo(),
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Stop()

	if err := p.PushAudio([]byte{0xFF, 0xF1}, 0, 0); err != nil {
		t.Fatalf("PushAudio with no audio stream should return nil, got: %v", err)
	}
}

func TestPushVideoAfterStopReturnsNil(t *testing.T) {
	p := mustNewPlugin(t, "mpegts")
	p.Stop()

	if err := p.PushVideo(makeNALU(128), 0, 0, true); err != nil {
		t.Fatalf("PushVideo after stop should return nil, got: %v", err)
	}
}

func TestFilePathReturnsOutputPath(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test_output.ts")
	cfg := output.PluginConfig{
		OutputFilePath: filePath,
		OutputFormat:   "mpegts",
		Video:          testVideo(),
		Audio:          testAudio(),
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Stop()

	if p.FilePath() != filePath {
		t.Fatalf("expected %q, got %q", filePath, p.FilePath())
	}
}

func mustNewPlugin(t *testing.T, format string) *Plugin {
	t.Helper()
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputFilePath: filepath.Join(dir, "output.ts"),
		OutputFormat:   format,
		Video:          testVideo(),
		Audio:          testAudio(),
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func testVideo() *media.VideoInfo {
	return &media.VideoInfo{
		Index: 0,
		Codec: "h264",
		Width: 1920, Height: 1080,
	}
}

func testAudio() *media.AudioTrack {
	return &media.AudioTrack{
		Index:      1,
		Codec:      "aac",
		Channels:   2,
		SampleRate: 48000,
	}
}

func makeNALU(size int) []byte {
	data := make([]byte, size)
	data[0] = 0x00
	data[1] = 0x00
	data[2] = 0x00
	data[3] = 0x01
	data[4] = 0x65
	return data
}
