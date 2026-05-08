package hls

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
	"github.com/mcnairstudios/mediahub/pkg/output/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func robustConfig(t *testing.T) output.PluginConfig {
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

func TestServePlaylistValidatesWithValidator(t *testing.T) {
	cfg := robustConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	playlist := []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
seg0.ts
#EXTINF:6.000,
seg1.ts
#EXTINF:6.000,
seg2.ts
`)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "playlist.m3u8"), playlist, 0644))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/playlist.m3u8", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/vnd.apple.mpegurl", w.Header().Get("Content-Type"))

	errs := validate.ValidateHLSPlaylist(w.Body.Bytes())
	assert.Empty(t, errs, "HLS playlist should pass validation: %v", errs)
}

func TestServePlaylistSegmentDurationsConsistent(t *testing.T) {
	cfg := robustConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	playlist := []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
seg0.ts
#EXTINF:6.000,
seg1.ts
#EXTINF:5.800,
seg2.ts
`)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "playlist.m3u8"), playlist, 0644))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/playlist.m3u8", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	errs := validate.ValidateHLSPlaylist(w.Body.Bytes())
	assert.Empty(t, errs, "playlist with consistent durations should validate")
}

func TestServePlaylistRejectsExceedingTargetDuration(t *testing.T) {
	playlist := []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
seg0.ts
#EXTINF:4.000,
seg1.ts
`)
	errs := validate.ValidateHLSPlaylist(playlist)
	assert.NotEmpty(t, errs, "segment exceeding target duration should produce validation errors")

	found := false
	for _, e := range errs {
		if e.Field == "duration" {
			found = true
		}
	}
	assert.True(t, found, "expected a 'duration' field error")
}

func TestServePlaylistSequentialSegmentNumbers(t *testing.T) {
	playlist := []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
seg0.ts
#EXTINF:6.000,
seg1.ts
#EXTINF:6.000,
seg2.ts
`)
	errs := validate.ValidateHLSPlaylist(playlist)
	assert.Empty(t, errs, "sequential segments should pass validation")
}

func TestServePlaylistNonSequentialDetected(t *testing.T) {
	playlist := []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
seg0.ts
#EXTINF:6.000,
seg5.ts
`)
	errs := validate.ValidateHLSPlaylist(playlist)
	assert.NotEmpty(t, errs, "non-sequential segments should be detected")
}

func TestServePlaylistCacheHeaders(t *testing.T) {
	cfg := robustConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	playlist := []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
seg0.ts
`)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "playlist.m3u8"), playlist, 0644))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/playlist.m3u8", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Cache-Control"), "no-cache")
}

func TestServeSegmentContentType(t *testing.T) {
	cfg := robustConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "seg0.ts"), []byte("tsdata"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "seg0.m4s"), []byte("fmp4data"), 0644))

	cases := []struct {
		path        string
		contentType string
	}{
		{"/seg0.ts", "video/mp2t"},
		{"/seg0.m4s", "video/mp4"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		p.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "path: %s", tc.path)
		assert.Equal(t, tc.contentType, w.Header().Get("Content-Type"), "path: %s", tc.path)
	}
}

func TestServeCORSOnOptions(t *testing.T) {
	cfg := robustConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	req := httptest.NewRequest(http.MethodOptions, "/playlist.m3u8", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestVODPlaylistEndlistValidation(t *testing.T) {
	playlist := []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
seg0.ts
#EXTINF:6.000,
seg1.ts
#EXTINF:4.500,
seg2.ts
#EXT-X-ENDLIST
`)
	errs := validate.ValidateHLSPlaylist(playlist)
	assert.Empty(t, errs, "VOD playlist with ENDLIST should validate cleanly")
}

func TestLivePlaylistRequiresMediaSequence(t *testing.T) {
	playlist := []byte(`#EXTM3U
#EXT-X-TARGETDURATION:6
#EXTINF:6.000,
seg0.ts
`)
	errs := validate.ValidateHLSPlaylist(playlist)
	assert.NotEmpty(t, errs, "live playlist without MEDIA-SEQUENCE should fail")

	found := false
	for _, e := range errs {
		if e.Field == "mediasequence" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestWaitReadySucceedsWhenPlaylistExists(t *testing.T) {
	cfg := robustConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "playlist.m3u8"), []byte("#EXTM3U\n#EXTINF:6.0,\nseg0.ts\n"), 0644))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = p.WaitReady(ctx)
	assert.NoError(t, err)
}

func TestStatusHealthyWhileRunning(t *testing.T) {
	cfg := robustConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	st := p.Status()
	assert.True(t, st.Healthy)
	assert.Equal(t, output.DeliveryHLS, st.Mode)
	assert.Equal(t, 0, st.SegmentCount)
	assert.Empty(t, st.Error)
}

func TestStatusUnhealthyAfterStop(t *testing.T) {
	cfg := robustConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	p.Stop()
	st := p.Status()
	assert.False(t, st.Healthy)
}
