package dash

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
)

func TestNewPlugin(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	if p.Mode() != output.DeliveryDASH {
		t.Errorf("Mode() = %q, want %q", p.Mode(), output.DeliveryDASH)
	}
}

func TestPluginStatus(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1280, Height: 720},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	status := p.Status()
	if status.Mode != output.DeliveryDASH {
		t.Errorf("Status().Mode = %q, want %q", status.Mode, output.DeliveryDASH)
	}
	if !status.Healthy {
		t.Error("Status().Healthy = false, want true")
	}
	if status.SegmentCount != 0 {
		t.Errorf("Status().SegmentCount = %d, want 0", status.SegmentCount)
	}
}

func TestGeneration(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	if p.Generation() != 1 {
		t.Errorf("Generation() = %d, want 1", p.Generation())
	}

	p.ResetForSeek()
	if p.Generation() != 2 {
		t.Errorf("after ResetForSeek, Generation() = %d, want 2", p.Generation())
	}
}

func TestServeManifest(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
		Audio:     &media.AudioTrack{Codec: "aac", Channels: 2, SampleRate: 48000, BitRate: 128000},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("manifest status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/dash+xml" {
		t.Errorf("Content-Type = %q, want application/dash+xml", ct)
	}
	body := rec.Body.String()
	if len(body) == 0 {
		t.Fatal("manifest body is empty")
	}
	if !contains(body, "urn:mpeg:dash:schema:mpd:2011") {
		t.Error("manifest missing MPD namespace")
	}
	if !contains(body, "video") {
		t.Error("manifest missing video representation")
	}
	if !contains(body, "audio") {
		t.Error("manifest missing audio representation")
	}
}

func TestServeInitNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	req := httptest.NewRequest("GET", "/init-video.mp4", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("init-video status = %d, want 404", rec.Code)
	}
}

func TestServeInitFound(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	segDir := filepath.Join(dir, "segments")
	os.WriteFile(filepath.Join(segDir, "init_video.mp4"), []byte("fake-init"), 0644)
	time.Sleep(200 * time.Millisecond)

	req := httptest.NewRequest("GET", "/init-video.mp4", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("init-video status = %d, want 200", rec.Code)
	}
}

func TestServeMediaSegmentBadPath(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	req := httptest.NewRequest("GET", "/video/abc.m4s", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad segment status = %d, want 400", rec.Code)
	}
}

func TestWaitReadyTimeout(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = p.WaitReady(ctx)
	if err == nil {
		t.Error("WaitReady should timeout when no init segment exists")
	}
}

func TestEndOfStream(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    false,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	p.EndOfStream()

	status := p.Status()
	if status.Healthy {
		t.Error("after EOS, Status().Healthy should be false")
	}
}

func TestServablePluginInterface(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	var _ output.ServablePlugin = p
}

func TestNewPluginAudioOnly(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Audio:     &media.AudioTrack{Codec: "aac", Channels: 2, SampleRate: 48000, BitRate: 128000},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	if p.Mode() != output.DeliveryDASH {
		t.Errorf("Mode() = %q, want %q", p.Mode(), output.DeliveryDASH)
	}
	if p.hasVideo {
		t.Error("hasVideo should be false for audio-only config")
	}
	if !p.hasAudio {
		t.Error("hasAudio should be true for audio-only config")
	}
}

func TestAudioOnlyPushVideoNoop(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Audio:     &media.AudioTrack{Codec: "aac", Channels: 2, SampleRate: 48000, BitRate: 128000},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	err = p.PushVideo([]byte{0, 0, 0, 1}, 0, 0, 3000, true)
	if err != nil {
		t.Errorf("PushVideo on audio-only should return nil, got: %v", err)
	}
}

func TestAudioOnlyManifest(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Audio:     &media.AudioTrack{Codec: "aac", Channels: 2, SampleRate: 48000, BitRate: 128000},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("manifest status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !contains(body, "urn:mpeg:dash:schema:mpd:2011") {
		t.Error("manifest missing MPD namespace")
	}
	if contains(body, "contentType=\"video\"") {
		t.Error("audio-only manifest should not contain video AdaptationSet")
	}
	if !contains(body, "contentType=\"audio\"") {
		t.Error("audio-only manifest should contain audio AdaptationSet")
	}
	if !contains(body, "mp4a.40.2") {
		t.Error("audio-only manifest should contain AAC codec string")
	}
}

func TestAudioOnlyWaitReadyTimeout(t *testing.T) {
	dir := t.TempDir()
	// Use a config that has no video and no audio — nothing to become ready.
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = p.WaitReady(ctx)
	if err == nil {
		t.Error("WaitReady should timeout when no init segment exists")
	}
}

func TestAudioOnlyWaitReadySuccess(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		IsLive:    true,
		Audio:     &media.AudioTrack{Codec: "aac", Channels: 2, SampleRate: 48000, BitRate: 128000},
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Stop()

	// Write a fake audio init segment to trigger WaitReady.
	segDir := filepath.Join(dir, "segments")
	os.WriteFile(filepath.Join(segDir, "init_audio.mp4"), []byte("fake-audio-init"), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = p.WaitReady(ctx)
	if err != nil {
		t.Errorf("WaitReady should succeed once audio init exists, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsCheck(s, substr))
}

func containsCheck(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
