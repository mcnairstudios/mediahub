package record

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ output.OutputPlugin = (*Plugin)(nil)

func testConfig(t *testing.T) output.PluginConfig {
	t.Helper()
	dir := t.TempDir()
	return output.PluginConfig{
		OutputFilePath: filepath.Join(dir, "source.mp4"),
		Video: &media.VideoInfo{
			Codec:  "h264",
			Width:  1920,
			Height: 1080,
		},
		Audio: &media.AudioTrack{
			Codec:      "aac",
			SampleRate: 48000,
			Channels:   2,
		},
	}
}

func TestMode(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, output.DeliveryRecord, p.Mode())
}

func TestConstructionCreatesFile(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	_, err = os.Stat(cfg.OutputFilePath)
	assert.NoError(t, err)
}

func TestPushVideoAudio(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA, 0xBB, 0xCC}, 0, 0, 0, true)
	assert.NoError(t, err)

	err = p.PushAudio([]byte{0xFF, 0xF1, 0x50, 0x80, 0x02, 0x00, 0xFC, 0xDE}, 0, 0, 0)
	assert.NoError(t, err)

	p.Stop()

	st := p.Status()
	assert.True(t, st.BytesWritten > 0)
}

func TestSetPreservedIsPreserved(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.False(t, p.IsPreserved())
	p.SetPreserved(true)
	assert.True(t, p.IsPreserved())
	p.SetPreserved(false)
	assert.False(t, p.IsPreserved())
}

func TestStopFinalizesMp4(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA, 0xBB, 0xCC}, 0, 0, 0, true)
	require.NoError(t, err)

	p.Stop()

	info, err := os.Stat(cfg.OutputFilePath)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0)

	st := p.Status()
	assert.False(t, st.Healthy)
}

func TestResetForSeekIsNoOp(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA, 0xBB, 0xCC}, 0, 0, 0, true)
	require.NoError(t, err)

	beforeStatus := p.Status()
	p.ResetForSeek()
	afterStatus := p.Status()

	assert.True(t, afterStatus.Healthy)
	assert.Equal(t, beforeStatus.BytesWritten, afterStatus.BytesWritten)
}

func TestStatusShowsHealthyAndBytes(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	st := p.Status()
	assert.True(t, st.Healthy)
	assert.Equal(t, output.DeliveryRecord, st.Mode)
}

func TestFilePath(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Equal(t, cfg.OutputFilePath, p.FilePath())
}

func TestDoubleStopSafe(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	p.Stop()
	p.Stop()
}

func TestEndOfStreamStops(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	p.EndOfStream()

	st := p.Status()
	assert.False(t, st.Healthy)
}

func TestConstructionVideoOnly(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputFilePath: filepath.Join(dir, "video_only.ts"),
		OutputFormat:   "mpegts",
		Video: &media.VideoInfo{
			Codec:  "h264",
			Width:  1920,
			Height: 1080,
		},
	}
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Nil(t, p.audioStream, "expected no audio stream for video-only config")
	assert.NotNil(t, p.videoStream, "expected video stream to be created")

	// PushAudio should be a no-op
	err = p.PushAudio([]byte{0xFF, 0xF1, 0x50, 0x80}, 0, 0, 0)
	assert.NoError(t, err)

	// PushVideo should work
	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA, 0xBB, 0xCC}, 0, 0, 0, true)
	assert.NoError(t, err)

	p.Stop()

	st := p.Status()
	assert.False(t, st.Healthy)
	assert.True(t, st.BytesWritten > 0)
}

func TestConstructionWithNilAudio(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputFilePath: filepath.Join(dir, "video_only.ts"),
		OutputFormat:   "mpegts",
		Video: &media.VideoInfo{
			Codec:  "h264",
			Width:  1920,
			Height: 1080,
		},
	}
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.True(t, p.Status().Healthy)
}

func TestPushVideoNilAudio(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputFilePath: filepath.Join(dir, "video_only.ts"),
		OutputFormat:   "mpegts",
		Video: &media.VideoInfo{
			Codec:  "h264",
			Width:  1920,
			Height: 1080,
		},
	}
	p, err := New(cfg)
	require.NoError(t, err)

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA, 0xBB, 0xCC}, 0, 0, 0, true)
	assert.NoError(t, err)

	p.Stop()
	st := p.Status()
	assert.True(t, st.BytesWritten > 0)
}

func TestPushAudioNoAudioStream(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputFilePath: filepath.Join(dir, "video_only.ts"),
		OutputFormat:   "mpegts",
		Video: &media.VideoInfo{
			Codec:  "h264",
			Width:  1920,
			Height: 1080,
		},
	}
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	err = p.PushAudio([]byte{0xFF, 0xF1, 0x50, 0x80, 0x02, 0x00, 0xFC, 0xDE}, 0, 0, 0)
	assert.NoError(t, err)
}

func TestPushVideoAfterStop(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	p.Stop()

	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 0, 0, 0, true)
	assert.NoError(t, err)
}

func TestPushAudioAfterStop(t *testing.T) {
	cfg := testConfig(t)
	p, err := New(cfg)
	require.NoError(t, err)

	p.Stop()

	err = p.PushAudio([]byte{0xFF, 0xF1, 0x50, 0x80}, 0, 0, 0)
	assert.NoError(t, err)
}

func TestConstructionAudioOnly(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputFilePath: filepath.Join(dir, "audio_only.ts"),
		OutputFormat:   "mpegts",
		Audio: &media.AudioTrack{
			Codec:      "aac",
			SampleRate: 48000,
			Channels:   2,
		},
	}
	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	assert.Nil(t, p.videoStream, "expected no video stream for audio-only config")
	assert.NotNil(t, p.audioStream, "expected audio stream to be created")

	// PushVideo should be a no-op when there is no video stream.
	err = p.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA}, 0, 0, 0, true)
	assert.NoError(t, err)

	// PushAudio should work.
	err = p.PushAudio([]byte{0xFF, 0xF1, 0x50, 0x80, 0x02, 0x00, 0xFC, 0xDE}, 0, 0, 0)
	assert.NoError(t, err)

	p.Stop()

	st := p.Status()
	assert.False(t, st.Healthy)
	assert.True(t, st.BytesWritten > 0)
}

func TestAudioOnlyStopWithoutPush(t *testing.T) {
	dir := t.TempDir()
	cfg := output.PluginConfig{
		OutputFilePath: filepath.Join(dir, "audio_only_empty.ts"),
		OutputFormat:   "mpegts",
		Audio: &media.AudioTrack{
			Codec:      "aac",
			SampleRate: 48000,
			Channels:   2,
		},
	}
	p, err := New(cfg)
	require.NoError(t, err)

	// Stop immediately without pushing any data — should not panic.
	p.Stop()

	st := p.Status()
	assert.False(t, st.Healthy)
}
