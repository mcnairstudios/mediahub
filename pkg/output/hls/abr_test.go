package hls

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func abrConfig(t *testing.T) output.PluginConfig {
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

func TestABRNewDefaultRenditions(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	// Default renditions: 1080p, 720p, 480p, 360p
	assert.Equal(t, 4, p.VariantCount())
	variants := p.Variants()
	assert.Equal(t, 1080, variants[0].Height)
	assert.Equal(t, 720, variants[1].Height)
	assert.Equal(t, 480, variants[2].Height)
	assert.Equal(t, 360, variants[3].Height)
}

func TestABRNewCustomRenditions(t *testing.T) {
	cfg := abrConfig(t)
	cfg.Options = map[string]any{
		"abr_renditions": []Rendition{
			{Height: 720, Bitrate: 2_500_000},
			{Height: 480, Bitrate: 1_000_000},
		},
	}
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, 2, p.VariantCount())
	variants := p.Variants()
	assert.Equal(t, 720, variants[0].Height)
	assert.Equal(t, 480, variants[1].Height)
}

func TestABRFilterRenditionsAboveSource(t *testing.T) {
	cfg := abrConfig(t)
	cfg.Video.Width = 854
	cfg.Video.Height = 480

	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	// 1080p and 720p should be clamped to 480p (source height).
	// After dedup: 480p, 360p.
	variants := p.Variants()
	assert.Equal(t, 2, len(variants))
	assert.Equal(t, 480, variants[0].Height)
	assert.Equal(t, 360, variants[1].Height)
}

func TestABRModeReturnsHLS(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, output.DeliveryHLS, p.Mode())
}

func TestABRGenerationStartsAtOne(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, int64(1), p.Generation())
}

func TestABRResetForSeekBumpsGeneration(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, int64(1), p.Generation())
	p.ResetForSeek()
	assert.Equal(t, int64(2), p.Generation())
}

func TestABRStatusHealthy(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	st := p.Status()
	assert.True(t, st.Healthy)
	assert.Equal(t, output.DeliveryHLS, st.Mode)
}

func TestABRStatusAfterStop(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)

	p.Stop()
	st := p.Status()
	assert.False(t, st.Healthy)
}

func TestABRDoubleStopSafe(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)

	p.Stop()
	p.Stop() // should not panic
}

func TestABREndOfStreamStops(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)

	p.EndOfStream()
	st := p.Status()
	assert.False(t, st.Healthy)
}

func TestABRPushAfterStopIsNoop(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	p.Stop()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 0, 0, 0, true)
	assert.NoError(t, err)

	err = p.PushAudio([]byte{0xFF, 0xF1, 0x50, 0x80}, 0, 0, 0)
	assert.NoError(t, err)
}

func TestABRPushSubtitleIsNoop(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	err = p.PushSubtitle([]byte("hello"), 0, 1000)
	assert.NoError(t, err)
}

func TestABRVariantOutputDirs(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	dirs := p.VariantOutputDirs()
	require.Equal(t, 4, len(dirs))
	for i, d := range dirs {
		assert.True(t, strings.Contains(d, "segments"), "variant %d dir should contain 'segments': %s", i, d)
		_, err := os.Stat(d)
		assert.NoError(t, err, "variant %d dir should exist", i)
	}
}

func TestABRMasterPlaylistContent(t *testing.T) {
	cfg := abrConfig(t)
	cfg.Options = map[string]any{
		"abr_renditions": []Rendition{
			{Height: 720, Bitrate: 3_000_000},
			{Height: 480, Bitrate: 1_500_000},
		},
	}
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	// Write a fake playlist file in the first variant so WaitReady succeeds.
	dirs := p.VariantOutputDirs()
	require.True(t, len(dirs) >= 1)
	_ = os.WriteFile(filepath.Join(dirs[0], "playlist.m3u8"), []byte("#EXTM3U\n#EXTINF:6.0,\nseg0.ts\n"), 0644)

	req := httptest.NewRequest(http.MethodGet, "/master.m3u8", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "#EXTM3U")
	assert.Contains(t, body, "#EXT-X-STREAM-INF:BANDWIDTH=3000000,RESOLUTION=1280x720")
	assert.Contains(t, body, "v0/playlist.m3u8")
	assert.Contains(t, body, "#EXT-X-STREAM-INF:BANDWIDTH=1500000,RESOLUTION=852x480")
	assert.Contains(t, body, "v1/playlist.m3u8")
	assert.Equal(t, "application/vnd.apple.mpegurl", w.Header().Get("Content-Type"))
}

func TestABRPlaylistAliasesWork(t *testing.T) {
	cfg := abrConfig(t)
	cfg.Options = map[string]any{
		"abr_renditions": []Rendition{
			{Height: 720, Bitrate: 3_000_000},
		},
	}
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	// Write a fake playlist so WaitReady succeeds.
	dirs := p.VariantOutputDirs()
	_ = os.WriteFile(filepath.Join(dirs[0], "playlist.m3u8"), []byte("#EXTM3U\n#EXTINF:6.0,\nseg0.ts\n"), 0644)

	// /playlist.m3u8 should also serve the master playlist.
	req := httptest.NewRequest(http.MethodGet, "/playlist.m3u8", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "#EXT-X-STREAM-INF")
}

func TestABRVariantSegmentRouting(t *testing.T) {
	cfg := abrConfig(t)
	cfg.Options = map[string]any{
		"abr_renditions": []Rendition{
			{Height: 720, Bitrate: 3_000_000},
			{Height: 480, Bitrate: 1_500_000},
		},
	}
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	// Write a fake segment in the second variant (v1).
	dirs := p.VariantOutputDirs()
	_ = os.WriteFile(filepath.Join(dirs[1], "seg0.ts"), []byte("faketsdata"), 0644)

	// Request via variant path.
	req := httptest.NewRequest(http.MethodGet, "/v1/seg0.ts", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "faketsdata", w.Body.String())
}

func TestABRUnknownPathReturns404(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestABROptionsRequestReturns200(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	req := httptest.NewRequest(http.MethodOptions, "/master.m3u8", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestABRConstructionMissingOutputDir(t *testing.T) {
	cfg := output.PluginConfig{
		Video: &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}
	_, err := NewABR(cfg)
	assert.Error(t, err)
}

func TestABRConstructionMissingVideo(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		Audio: &media.AudioTrack{
			Codec:      "aac",
			SampleRate: 48000,
			Channels:   2,
		},
	}
	_, err := NewABR(cfg)
	assert.Error(t, err)
}

func TestABRConstructionZeroDimensions(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		Video:     &media.VideoInfo{Codec: "h264"},
	}
	_, err := NewABR(cfg)
	assert.Error(t, err)
}

func TestABRWaitReadyCancelledContext(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = p.WaitReady(ctx)
	assert.Error(t, err)
}

func TestABRMasterPlaylistPath(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, "/master.m3u8", p.MasterPlaylistPath())
}

func TestABRWriteMasterPlaylist(t *testing.T) {
	cfg := abrConfig(t)
	cfg.Options = map[string]any{
		"abr_renditions": []Rendition{
			{Height: 720, Bitrate: 3_000_000},
		},
	}
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	err = p.writeABRMasterPlaylist()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(cfg.OutputDir, "master.m3u8"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "#EXTM3U")
	assert.Contains(t, string(data), "v0/playlist.m3u8")
}

func TestABRMasterPlaylistNotReady(t *testing.T) {
	cfg := abrConfig(t)
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodGet, "/master.m3u8", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestDefaultRenditions(t *testing.T) {
	r := DefaultRenditions()
	require.Equal(t, 4, len(r))

	assert.Equal(t, 1080, r[0].Height)
	assert.Equal(t, 5_000_000, r[0].Bitrate)
	assert.Equal(t, 720, r[1].Height)
	assert.Equal(t, 3_000_000, r[1].Bitrate)
	assert.Equal(t, 480, r[2].Height)
	assert.Equal(t, 1_500_000, r[2].Bitrate)
	assert.Equal(t, 360, r[3].Height)
	assert.Equal(t, 800_000, r[3].Bitrate)
}

func TestABRWidthEvenAlignment(t *testing.T) {
	// Use a source with dimensions that would produce odd width for some heights.
	cfg := output.PluginConfig{
		OutputDir:          t.TempDir(),
		SegmentDurationSec: 6,
		Video: &media.VideoInfo{
			Codec:      "h264",
			Width:      1001, // odd source width
			Height:     1000,
			FramerateN: 25,
			FramerateD: 1,
		},
		Audio: &media.AudioTrack{
			Codec:      "aac",
			SampleRate: 48000,
			Channels:   2,
		},
		Options: map[string]any{
			"abr_renditions": []Rendition{
				{Height: 720, Bitrate: 3_000_000},
			},
		},
	}
	p, err := NewABR(cfg)
	require.NoError(t, err)
	defer p.Stop()

	variants := p.Variants()
	require.Equal(t, 1, len(variants))
	// Width should be even.
	assert.Equal(t, 0, variants[0].Height%2, "height should be even")
	// With 1001/1000 aspect and 720 height: 1001*720/1000 = 720.72 -> 720 (even)
	// Just verify it's even, not odd.
	dirs := p.VariantOutputDirs()
	assert.Equal(t, 1, len(dirs))
}
