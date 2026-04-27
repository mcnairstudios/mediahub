package mse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
)

func TestPluginImplementsOutputPlugin(t *testing.T) {
	var _ output.OutputPlugin = (*Plugin)(nil)
}

func TestPluginImplementsServablePlugin(t *testing.T) {
	var _ output.ServablePlugin = (*Plugin)(nil)
}

func TestModeReturnsMSE(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	if p.Mode() != output.DeliveryMSE {
		t.Fatalf("expected mode %q, got %q", output.DeliveryMSE, p.Mode())
	}
}

func TestConstructionCreatesSegmentsDir(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	segDir := filepath.Join(dir, "segments")
	info, err := os.Stat(segDir)
	if err != nil {
		t.Fatalf("segments dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("segments path is not a directory")
	}
}

func TestGenerationStartsAtOne(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	if p.Generation() != 1 {
		t.Fatalf("expected generation 1, got %d", p.Generation())
	}
}

func TestGenerationBumpsOnResetForSeek(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	p.ResetForSeek()
	if p.Generation() != 2 {
		t.Fatalf("expected generation 2, got %d", p.Generation())
	}

	p.ResetForSeek()
	if p.Generation() != 3 {
		t.Fatalf("expected generation 3, got %d", p.Generation())
	}
}

func TestStatusShowsSegmentCounts(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	status := p.Status()
	if status.Mode != output.DeliveryMSE {
		t.Fatalf("expected mode %q, got %q", output.DeliveryMSE, status.Mode)
	}
	if !status.Healthy {
		t.Fatal("expected healthy status")
	}
	if status.SegmentCount != 0 {
		t.Fatalf("expected 0 segments, got %d", status.SegmentCount)
	}
}

func TestServeHTTPReturns404BeforeInitSegments(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	paths := []string{"/video/init", "/audio/init", "/video/segment?seq=1&gen=1", "/audio/segment?seq=1&gen=1"}
	for _, path := range paths {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for %s before init, got %d", path, rec.Code)
		}
	}
}

func TestServeHTTPDebugEndpoint(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/debug", nil)
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /debug, got %d", rec.Code)
	}

	var debug map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &debug); err != nil {
		t.Fatalf("debug response is not valid JSON: %v", err)
	}

	if gen, ok := debug["generation"]; !ok {
		t.Fatal("debug response missing generation field")
	} else if gen != float64(1) {
		t.Fatalf("expected generation 1, got %v", gen)
	}
}

func TestServeHTTPUnknownPathReturns404(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown path, got %d", rec.Code)
	}
}

func TestServeHTTPStaleGenerationReturns410(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	p.ResetForSeek()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/video/segment?seq=1&gen=1", nil)
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected 410 for stale generation, got %d", rec.Code)
	}
}

func TestWaitReadyTimesOutWithoutInit(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = p.WaitReady(ctx)
	if err == nil {
		t.Fatal("expected WaitReady to return error on timeout")
	}
}

func TestEndOfStreamMarksStopped(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	p.EndOfStream()

	status := p.Status()
	if status.Healthy {
		t.Fatal("expected unhealthy after EndOfStream")
	}
}

func TestConstructionNilAudioVideoOnly(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		Video: &media.VideoInfo{
			Codec:  "h264",
			Width:  1920,
			Height: 1080,
		},
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("expected nil audio to work: %v", err)
	}
	defer p.Stop()

	if p.Mode() != output.DeliveryMSE {
		t.Fatalf("expected mode %q, got %q", output.DeliveryMSE, p.Mode())
	}
}

func TestPushAudioNoAudioStreamMSE(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputDir: dir,
		Video: &media.VideoInfo{
			Codec:  "h264",
			Width:  1920,
			Height: 1080,
		},
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	if err := p.PushAudio([]byte{0xFF, 0xF1}, 0, 0); err != nil {
		t.Fatalf("PushAudio on video-only MSE should return nil, got: %v", err)
	}
}

func TestPushVideoAfterStopMSE(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}

	p.Stop()

	if err := p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 0, 0, true); err != nil {
		t.Fatalf("PushVideo after stop should return nil, got: %v", err)
	}
}

func TestPushAudioAfterStopMSE(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}

	p.Stop()

	if err := p.PushAudio([]byte{0xFF, 0xF1}, 0, 0); err != nil {
		t.Fatalf("PushAudio after stop should return nil, got: %v", err)
	}
}
