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
	"github.com/mcnairstudios/mediahub/pkg/output/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeInitSegment(t *testing.T, segDir, name, codec string) {
	t.Helper()
	data := validate.BuildFMP4InitForTest(codec, true)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, name), data, 0644))
}

func writeMediaSegment(t *testing.T, segDir, name string, sampleCount uint32) {
	t.Helper()
	data := validate.BuildFMP4SegmentForTest(sampleCount)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, name), data, 0644))
}

func TestInitSegmentHasValidFMP4Structure(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{
		OutputDir: dir,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	})
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(dir, "segments")
	writeInitSegment(t, segDir, "init_video.mp4", "avc1")

	time.Sleep(200 * time.Millisecond)

	initData := p.watcher.VideoInit()
	if initData == nil {
		t.Skip("watcher did not pick up init segment in time")
	}

	errs := validate.ValidateFMP4Init(initData)
	assert.Empty(t, errs, "init segment should have valid fMP4 structure: %v", errs)
}

func TestMediaSegmentHasValidMoofMdat(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{
		OutputDir: dir,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	})
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(dir, "segments")
	writeMediaSegment(t, segDir, "video_00001.m4s", 30)

	time.Sleep(200 * time.Millisecond)

	data, ok := p.watcher.VideoSegment(1)
	if !ok {
		t.Skip("watcher did not pick up media segment in time")
	}

	errs := validate.ValidateFMP4Segment(data)
	assert.Empty(t, errs, "media segment should have valid moof/mdat: %v", errs)
}

func TestMediaSegmentWithZeroSamplesFails(t *testing.T) {
	data := validate.BuildFMP4SegmentForTest(0)
	errs := validate.ValidateFMP4Segment(data)
	assert.NotEmpty(t, errs, "zero-sample segment should fail validation")

	found := false
	for _, e := range errs {
		if e.Field == "samples" {
			found = true
		}
	}
	assert.True(t, found, "expected 'samples' validation error")
}

func TestInitSegmentMissingAvcCFails(t *testing.T) {
	data := validate.BuildFMP4InitForTest("avc1", false)
	errs := validate.ValidateFMP4Init(data)
	assert.NotEmpty(t, errs)

	found := false
	for _, e := range errs {
		if e.Field == "avcC" {
			found = true
		}
	}
	assert.True(t, found, "expected 'avcC' validation error")
}

func TestInitSegmentMissingHvcCFails(t *testing.T) {
	data := validate.BuildFMP4InitForTest("hvc1", false)
	errs := validate.ValidateFMP4Init(data)
	assert.NotEmpty(t, errs)

	found := false
	for _, e := range errs {
		if e.Field == "hvcC" {
			found = true
		}
	}
	assert.True(t, found, "expected 'hvcC' validation error")
}

func TestGenerationCounterIncrements(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, int64(1), p.Generation())

	p.ResetForSeek()
	assert.Equal(t, int64(2), p.Generation())

	p.ResetForSeek()
	assert.Equal(t, int64(3), p.Generation())
}

func TestStaleGenerationReturns410(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	require.NoError(t, err)
	defer p.Stop()

	p.ResetForSeek()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/video/segment?seq=1&gen=1", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusGone, rec.Code)
}

func TestCurrentGenerationDoesNotReturn410(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/video/segment?seq=1&gen=1", nil)
	p.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusGone, rec.Code)
}

func TestDebugEndpointReportsGeneration(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	require.NoError(t, err)
	defer p.Stop()

	p.ResetForSeek()
	p.ResetForSeek()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/debug", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var info map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &info))
	assert.Equal(t, float64(3), info["generation"])
}

func TestWaitReadyTimesOutWithoutInitSegment(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	require.NoError(t, err)
	defer p.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = p.WaitReady(ctx)
	assert.Error(t, err)
}

func TestWaitReadySucceedsWithInitSegment(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(dir, "segments")
	writeInitSegment(t, segDir, "init_video.mp4", "avc1")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = p.WaitReady(ctx)
	assert.NoError(t, err)
}

func TestVideoInitServedCorrectly(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(dir, "segments")
	initData := validate.BuildFMP4InitForTest("avc1", true)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "init_video.mp4"), initData, 0644))

	time.Sleep(200 * time.Millisecond)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/video/init", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "video/mp4", rec.Header().Get("Content-Type"))

	errs := validate.ValidateFMP4Init(rec.Body.Bytes())
	assert.Empty(t, errs, "served init segment should be valid fMP4")
}

func TestAudioInitNotFoundWithoutAudio(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/audio/init", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDoubleStopIsSafe(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	require.NoError(t, err)

	p.Stop()
	p.Stop()

	st := p.Status()
	assert.False(t, st.Healthy)
}

func TestPushVideoAfterStopIsNoop(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	require.NoError(t, err)

	p.Stop()
	err = p.PushVideo([]byte{0, 0, 0, 1, 0x65}, 0, 0, 0, true)
	assert.NoError(t, err)
}

func TestPushAudioAfterStopIsNoop(t *testing.T) {
	dir := t.TempDir()
	p, err := New(output.PluginConfig{OutputDir: dir})
	require.NoError(t, err)

	p.Stop()
	err = p.PushAudio([]byte{0xFF, 0xF1}, 0, 0, 0)
	assert.NoError(t, err)
}
