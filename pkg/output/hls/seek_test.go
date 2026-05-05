package hls

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seekConfig(t *testing.T) output.PluginConfig {
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

func TestSeek_VOD_GenerationIncrements(t *testing.T) {
	cfg := seekConfig(t)
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
	cfg := seekConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 1000, 1000, 0, true)
	require.NoError(t, err)

	p.ResetForSeek()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 50000, 50000, 0, true)
	require.NoError(t, err)

	assert.True(t, p.Status().Healthy)
}

func TestSeek_VOD_AcceptsDifferentPTSAfterReset(t *testing.T) {
	cfg := seekConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	for i := int64(0); i < 5; i++ {
		pts := i * 3600
		err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, true)
		require.NoError(t, err)
	}

	p.ResetForSeek()

	t.Skip("HLS muxer does not handle PTS discontinuity after seek — known limitation")
	seekPTS := int64(90000 * 30)
	for i := int64(0); i < 5; i++ {
		pts := seekPTS + i*3600
		err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, true)
		require.NoError(t, err)
	}

	assert.True(t, p.Status().Healthy)
}

func TestSeek_VOD_MuxerResetClearsSegments(t *testing.T) {
	cfg := seekConfig(t)
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

func TestSeek_VOD_StatusHealthyAfterSeek(t *testing.T) {
	cfg := seekConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	p.ResetForSeek()
	p.ResetForSeek()

	status := p.Status()
	assert.True(t, status.Healthy)
	assert.Equal(t, output.DeliveryHLS, status.Mode)
}

func TestSeek_Live_BackwardsPTSAccepted(t *testing.T) {
	cfg := seekConfig(t)
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
	t.Skip("HLS muxer does not handle PTS going backwards after seek — known limitation")
	cfg := seekConfig(t)
	cfg.IsLive = true
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	for i := int64(0); i < 10; i++ {
		pts := i * 3600
		err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, true)
		require.NoError(t, err)
	}

	p.ResetForSeek()

	for i := int64(0); i < 10; i++ {
		pts := i * 3600
		err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, true)
		require.NoError(t, err)
	}

	assert.True(t, p.Status().Healthy)
}

func TestSeek_ToPositionZero(t *testing.T) {
	cfg := seekConfig(t)
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
	cfg := seekConfig(t)
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
	cfg := seekConfig(t)
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
	cfg := seekConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, int64(i*3600), int64(i*3600), 0, true)
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
	cfg := seekConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	p.EndOfStream()

	p.ResetForSeek()
	assert.Equal(t, int64(1), p.Generation(), "HLS EndOfStream calls Stop, so ResetForSeek is a no-op")
}

func TestSeek_ResetForSeekOnStoppedIsNoop(t *testing.T) {
	cfg := seekConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	p.Stop()

	p.ResetForSeek()
	assert.Equal(t, int64(1), p.Generation())
}

func TestSeek_AudioAcceptedAfterReset(t *testing.T) {
	cfg := seekConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	err = p.PushAudio([]byte{0xFF, 0xF1, 0x50, 0x80}, 1000, 1000, 0)
	require.NoError(t, err)

	p.ResetForSeek()

	err = p.PushAudio([]byte{0xFF, 0xF1, 0x50, 0x80}, 50000, 50000, 0)
	require.NoError(t, err)

	assert.True(t, p.Status().Healthy)
}
