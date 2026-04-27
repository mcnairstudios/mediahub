package hls

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ output.OutputPlugin = (*Plugin)(nil)
var _ output.ServablePlugin = (*Plugin)(nil)

func testConfig(t *testing.T) output.PluginConfig {
	t.Helper()
	return output.PluginConfig{
		OutputDir:          t.TempDir(),
		SegmentDurationSec: 6,
		Video: &media.VideoInfo{
			Codec:      "h264",
			Width:      1920,
			Height:     1080,
			FramerateN: 25,
			FramerateD: 1,
		},
		Audio: &media.AudioTrack{
			Codec:      "aac",
			SampleRate: 48000,
			Channels:   2,
		},
	}
}

func TestModeReturnsDeliveryHLS(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, output.DeliveryHLS, p.Mode())
}

func TestConstructionCreatesSegmentsDir(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	info, err := os.Stat(segDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestGenerationStartsAtOne(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, int64(1), p.Generation())
}

func TestStatusShowsSegmentCount(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	st := p.Status()
	assert.Equal(t, output.DeliveryHLS, st.Mode)
	assert.True(t, st.Healthy)
	assert.Equal(t, 0, st.SegmentCount)
}

func TestResetForSeekBumpsGeneration(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, int64(1), p.Generation())
	p.ResetForSeek()
	assert.Equal(t, int64(2), p.Generation())
	p.ResetForSeek()
	assert.Equal(t, int64(3), p.Generation())
}

func TestResetForSeekClearsSegments(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	_ = os.WriteFile(filepath.Join(segDir, "seg0.ts"), []byte("fake"), 0644)

	matches, _ := filepath.Glob(filepath.Join(segDir, "seg*.ts"))
	assert.Equal(t, 1, len(matches))

	p.ResetForSeek()

	matches, _ = filepath.Glob(filepath.Join(segDir, "seg*.ts"))
	assert.Equal(t, 0, len(matches))
}

func TestDoubleStopSafe(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	p.Stop()
	p.Stop()
}

func TestPushAfterStopIsNoop(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	p.Stop()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 0, 0, true)
	assert.NoError(t, err)

	err = p.PushAudio([]byte{0xFF, 0xF1, 0x50, 0x80}, 0, 0)
	assert.NoError(t, err)
}

func TestPushSubtitleIsNoop(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	err = p.PushSubtitle([]byte("hello"), 0, 1000)
	assert.NoError(t, err)
}

func TestEndOfStreamStops(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	p.EndOfStream()

	st := p.Status()
	assert.False(t, st.Healthy)
}

func TestConstructionMissingOutputDir(t *testing.T) {
	cfg := output.PluginConfig{
		Video: &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}
	_, err := New(cfg)
	assert.Error(t, err)
}

func TestConstructionMissingVideo(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		Audio:     &media.AudioTrack{Codec: "aac", SampleRate: 48000, Channels: 2},
	}
	_, err := New(cfg)
	assert.Error(t, err)
}

func TestServePlaylistNotReady(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodGet, "/playlist.m3u8", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestServeSegmentPathTraversal(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	cases := []string{
		"/../etc/passwd",
		"/..%2Fetc%2Fpasswd",
		"/seg0.ts/../../../etc/passwd",
	}
	for _, path := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		p.ServeHTTP(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code, "path traversal not blocked: %s", path)
	}
}

func TestServeSegmentNotFound(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	req := httptest.NewRequest(http.MethodGet, "/seg999.ts", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestServeSegmentCORSHeaders(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	_ = os.WriteFile(filepath.Join(segDir, "seg0.ts"), []byte("faketsdata"), 0644)

	req := httptest.NewRequest(http.MethodGet, "/seg0.ts", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestWaitReadyCancelledContext(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = p.WaitReady(ctx)
	assert.Error(t, err)
}

func TestDefaultSegmentDuration(t *testing.T) {
	cfg := testConfig(t)
	cfg.SegmentDurationSec = 0
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, output.DeliveryHLS, p.Mode())
}

func TestConstructionNilAudioVideoOnly(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir:          t.TempDir(),
		SegmentDurationSec: 6,
		Video: &media.VideoInfo{
			Codec:      "h264",
			Width:      1920,
			Height:     1080,
			FramerateN: 25,
			FramerateD: 1,
		},
	}
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, output.DeliveryHLS, p.Mode())
	assert.True(t, p.Status().Healthy)
}

func TestPushAudioNoAudioStreamHLS(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir:          t.TempDir(),
		SegmentDurationSec: 6,
		Video: &media.VideoInfo{
			Codec:      "h264",
			Width:      1920,
			Height:     1080,
			FramerateN: 25,
			FramerateD: 1,
		},
	}
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	err = p.PushAudio([]byte{0xFF, 0xF1, 0x50, 0x80}, 0, 0)
	assert.NoError(t, err)
}

func TestPushVideoAfterStopReturnsNil(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	p.Stop()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 0, 0, true)
	assert.NoError(t, err)
}

func TestPushAudioAfterStopReturnsNil(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	p.Stop()

	err = p.PushAudio([]byte{0xFF, 0xF1, 0x50, 0x80}, 0, 0)
	assert.NoError(t, err)
}
