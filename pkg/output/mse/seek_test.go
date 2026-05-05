package mse

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seekTestConfig(t *testing.T) output.PluginConfig {
	t.Helper()
	return output.PluginConfig{
		OutputDir: t.TempDir(),
		Video: &media.VideoInfo{
			Codec:  "h264",
			Width:  1920,
			Height: 1080,
		},
	}
}

func TestSeek_VOD_GenerationIncrements(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, int64(1), p.Generation())

	p.ResetForSeek()
	assert.Equal(t, int64(2), p.Generation())

	p.ResetForSeek()
	assert.Equal(t, int64(3), p.Generation())

	p.ResetForSeek()
	assert.Equal(t, int64(4), p.Generation())
}

func TestSeek_VOD_AcceptsPacketsAfterReset(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 1000, 1000, 0, true)
	require.NoError(t, err)

	p.ResetForSeek()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 50000, 50000, 0, true)
	require.NoError(t, err)

	status := p.Status()
	assert.True(t, status.Healthy)
}

func TestSeek_VOD_AcceptsPacketsWithDifferentPTS(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	for i := int64(0); i < 5; i++ {
		pts := i * 3600
		err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, i == 0)
		require.NoError(t, err)
	}

	p.ResetForSeek()

	seekPTS := int64(90000 * 30)
	for i := int64(0); i < 5; i++ {
		pts := seekPTS + i*3600
		err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, i == 0)
		require.NoError(t, err)
	}

	assert.True(t, p.Status().Healthy)
}

func TestSeek_VOD_StaleGenerationReturns410(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	p.ResetForSeek()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/video/segment?seq=1&gen=1", nil)
	p.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusGone, rec.Code)
}

func TestSeek_VOD_CurrentGenerationNotStale(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	p.ResetForSeek()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/video/segment?seq=1&gen=2", nil)
	p.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusGone, rec.Code)
}

func TestSeek_VOD_StatusHealthyAfterSeek(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	p.ResetForSeek()
	p.ResetForSeek()

	status := p.Status()
	assert.True(t, status.Healthy)
	assert.Equal(t, output.DeliveryMSE, status.Mode)
}

func TestSeek_Live_BackwardsPTSAccepted(t *testing.T) {
	cfg := seekTestConfig(t)
	cfg.IsLive = true
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 90000, 90000, 0, true)
	require.NoError(t, err)

	p.ResetForSeek()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 45000, 45000, 0, true)
	require.NoError(t, err)

	assert.True(t, p.Status().Healthy)
}

func TestSeek_Live_NoCrashOnPTSGoingBackwards(t *testing.T) {
	cfg := seekTestConfig(t)
	cfg.IsLive = true
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	for i := int64(0); i < 10; i++ {
		pts := i * 3600
		err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, i%5 == 0)
		require.NoError(t, err)
	}

	p.ResetForSeek()

	for i := int64(0); i < 10; i++ {
		pts := i * 3600
		err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, i%5 == 0)
		require.NoError(t, err)
	}

	assert.True(t, p.Status().Healthy)
}

func TestSeek_ToPositionZero(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	for i := int64(0); i < 5; i++ {
		pts := i * 3600
		err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, true)
		require.NoError(t, err)
	}

	p.ResetForSeek()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 0, 0, 0, true)
	require.NoError(t, err)

	assert.True(t, p.Status().Healthy)
}

func TestSeek_NearEndOfVOD(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 0, 0, 0, true)
	require.NoError(t, err)

	p.ResetForSeek()

	nearEnd := int64(90000 * 7200)
	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, nearEnd, nearEnd, 0, true)
	require.NoError(t, err)

	p.EndOfStream()
	assert.False(t, p.Status().Healthy)
}

func TestSeek_MultipleRapidSeeks(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	for seek := 0; seek < 20; seek++ {
		err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, int64(seek*90000), int64(seek*90000), 0, true)
		require.NoError(t, err)
		p.ResetForSeek()
	}

	assert.Equal(t, int64(21), p.Generation())
	assert.True(t, p.Status().Healthy)
}

func TestSeek_ConcurrentSeekAndPush(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, int64(i*3600), int64(i*3600), 0, i%10 == 0)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			p.ResetForSeek()
		}
	}()

	wg.Wait()

	assert.GreaterOrEqual(t, p.Generation(), int64(11))
}

func TestSeek_AfterEndOfStream(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	p.EndOfStream()

	p.ResetForSeek()
	assert.Equal(t, int64(2), p.Generation())
}

func TestSeek_WatcherResetClearsState(t *testing.T) {
	cfg := seekTestConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	gen1 := p.Generation()
	p.ResetForSeek()
	gen2 := p.Generation()

	assert.Greater(t, gen2, gen1)

	assert.Nil(t, p.watcher.VideoInit())
	assert.Nil(t, p.watcher.AudioInit())
	assert.Equal(t, 0, p.watcher.videoSegs.Count())
	assert.Equal(t, 0, p.watcher.audioSegs.Count())
}
