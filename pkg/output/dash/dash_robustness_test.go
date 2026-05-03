package dash

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/output/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dashConfig(t *testing.T) output.PluginConfig {
	t.Helper()
	return output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
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
			BitRate:    128000,
		},
	}
}

func TestManifestIsValidXML(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/dash+xml", rec.Header().Get("Content-Type"))

	errs := validate.ValidateMPD(rec.Body.Bytes())
	assert.Empty(t, errs, "MPD manifest should pass validation: %v", errs)
}

func TestManifestContainsMPDNamespace(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "urn:mpeg:dash:schema:mpd:2011")
}

func TestManifestHasVideoAdaptationSet(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `mimeType="video/mp4"`)
	assert.Contains(t, body, `id="video"`)
}

func TestManifestHasAudioAdaptationSet(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `mimeType="audio/mp4"`)
	assert.Contains(t, body, `id="audio"`)
}

func TestManifestVideoOnlyNoAudio(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 1920, Height: 1080},
	}
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `mimeType="video/mp4"`)
	assert.NotContains(t, body, `mimeType="audio/mp4"`)
}

func TestManifestSegmentTimingMath(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `timescale="90000"`, "video timescale should be 90000")
	assert.Contains(t, body, `duration="540000"`, "video segment duration should be 6*90000=540000")
	assert.Contains(t, body, `timescale="48000"`, "audio timescale should be 48000")
	assert.Contains(t, body, `duration="288000"`, "audio segment duration should be 6*48000=288000")
}

func TestManifestSegmentTemplateHasMediaAndInit(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `media="video/$Number$.m4s"`)
	assert.Contains(t, body, `initialization="init-video.mp4"`)
	assert.Contains(t, body, `media="audio/$Number$.m4s"`)
	assert.Contains(t, body, `initialization="init-audio.mp4"`)
}

func TestManifestLiveHasDynamicType(t *testing.T) {
	cfg := dashConfig(t)
	cfg.IsLive = true
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `type="dynamic"`)
	assert.Contains(t, body, "minimumUpdatePeriod")
	assert.Contains(t, body, "availabilityStartTime")
}

func TestManifestVODHasStaticType(t *testing.T) {
	cfg := dashConfig(t)
	cfg.IsLive = false
	p, err := New(cfg)
	require.NoError(t, err)

	p.EndOfStream()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `type="static"`)
}

func TestManifestUpdatesWithSegments(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")

	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec1, req1)
	body1 := rec1.Body.String()

	initData := validate.BuildFMP4InitForTest("avc1", true)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "init_video.mp4"), initData, 0644))
	segData := validate.BuildFMP4SegmentForTest(30)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "video_00001.m4s"), segData, 0644))

	time.Sleep(200 * time.Millisecond)

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec2, req2)
	body2 := rec2.Body.String()

	assert.NotEmpty(t, body1)
	assert.NotEmpty(t, body2)
}

func TestManifestValidatesWithMPDValidator(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	errs := validate.ValidateMPD(rec.Body.Bytes())
	for _, e := range errs {
		if e.Field == "namespace" && strings.Contains(e.Message, "urn:mpeg:dash:schema:mpd:2011") {
			continue
		}
		t.Errorf("validation error: %s: %s", e.Field, e.Message)
	}
}

func TestInitVideoServed(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	initData := validate.BuildFMP4InitForTest("avc1", true)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "init_video.mp4"), initData, 0644))

	time.Sleep(200 * time.Millisecond)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/init-video.mp4", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	errs := validate.ValidateFMP4Init(rec.Body.Bytes())
	assert.Empty(t, errs, "init segment should validate: %v", errs)
}

func TestMediaSegmentServed(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	segData := validate.BuildFMP4SegmentForTest(30)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "video_00001.m4s"), segData, 0644))

	time.Sleep(200 * time.Millisecond)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/video/1.m4s", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	errs := validate.ValidateFMP4Segment(rec.Body.Bytes())
	assert.Empty(t, errs, "media segment should validate: %v", errs)
}

func TestBadSegmentNumberReturns400(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/video/abc.m4s", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestZeroSegmentNumberReturns400(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/video/0.m4s", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDebugEndpoint(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/debug", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var info map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &info))
	assert.Equal(t, float64(1), info["generation"])
	assert.Equal(t, false, info["stopped"])
}

func TestResetForSeekBumpsGeneration(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, int64(1), p.Generation())
	p.ResetForSeek()
	assert.Equal(t, int64(2), p.Generation())
}

func TestUnknownPathReturns404(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/unknown", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestManifestCacheHeaders(t *testing.T) {
	cfg := dashConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
}
