package webrtc

import (
	"sync"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeek_VOD_GenerationIncrements(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	assert.Equal(t, int64(1), plugin.Generation())

	plugin.ResetForSeek()
	assert.Equal(t, int64(2), plugin.Generation())

	plugin.ResetForSeek()
	assert.Equal(t, int64(3), plugin.Generation())

	plugin.ResetForSeek()
	assert.Equal(t, int64(4), plugin.Generation())
}

func TestSeek_VOD_RTPSequenceNumbersReset(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	plugin.mu.Lock()
	plugin.videoSeq = 500
	plugin.audioSeq = 200
	plugin.mu.Unlock()

	plugin.ResetForSeek()

	plugin.mu.Lock()
	assert.Equal(t, uint16(0), plugin.videoSeq)
	assert.Equal(t, uint16(0), plugin.audioSeq)
	plugin.mu.Unlock()
}

func TestSeek_VOD_RTPTimestampsReset(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	plugin.mu.Lock()
	plugin.videoTS = 90000
	plugin.audioTS = 48000
	plugin.mu.Unlock()

	plugin.ResetForSeek()

	plugin.mu.Lock()
	assert.Equal(t, uint32(0), plugin.videoTS)
	assert.Equal(t, uint32(0), plugin.audioTS)
	plugin.mu.Unlock()
}

func TestSeek_VOD_PTSBaseReset(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	plugin.mu.Lock()
	plugin.ptsBaseSet = true
	plugin.ptsBaseVideo = 1000000
	plugin.ptsBaseAudio = 1000000
	plugin.lastVideoPTS = 5000000
	plugin.lastAudioPTS = 5000000
	plugin.mu.Unlock()

	plugin.ResetForSeek()

	plugin.mu.Lock()
	assert.False(t, plugin.ptsBaseSet, "ptsBaseSet should be false so next packet re-captures base")
	assert.Equal(t, int64(0), plugin.lastVideoPTS)
	assert.Equal(t, int64(0), plugin.lastAudioPTS)
	plugin.mu.Unlock()
}

func TestSeek_VOD_AcceptsVideoAfterReset(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	err = plugin.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 1000, 1000, 0, true)
	require.NoError(t, err)

	plugin.ResetForSeek()

	err = plugin.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 50000, 50000, 0, true)
	require.NoError(t, err)
}

func TestSeek_VOD_AcceptsAudioAfterReset(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	err = plugin.PushAudio([]byte{0xFF, 0xF1}, 1000, 1000, 0)
	require.NoError(t, err)

	plugin.ResetForSeek()

	err = plugin.PushAudio([]byte{0xFF, 0xF1}, 50000, 50000, 0)
	require.NoError(t, err)
}

func TestSeek_VOD_NewPTSBaseSetAfterSeek(t *testing.T) {
	p, err := New(output.PluginConfig{
		Video: &media.VideoInfo{Codec: "h264", FramerateN: 25, FramerateD: 1},
	})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	plugin.mu.Lock()
	plugin.ptsBaseSet = true
	plugin.ptsBaseVideo = 0
	plugin.ptsBaseAudio = 0
	plugin.mu.Unlock()

	plugin.ResetForSeek()

	plugin.mu.Lock()
	assert.False(t, plugin.ptsBaseSet)
	plugin.mu.Unlock()
}

func TestSeek_Live_BackwardsPTSAccepted(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	err = plugin.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 90000, 90000, 0, true)
	require.NoError(t, err)

	plugin.ResetForSeek()

	err = plugin.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 45000, 45000, 0, true)
	require.NoError(t, err)
}

func TestSeek_Live_NoCrashOnPTSGoingBackwards(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	for i := int64(0); i < 10; i++ {
		pts := i * 3600
		err = plugin.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, i%5 == 0)
		require.NoError(t, err)
	}

	plugin.ResetForSeek()

	for i := int64(0); i < 10; i++ {
		pts := i * 3600
		err = plugin.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, i%5 == 0)
		require.NoError(t, err)
	}
}

func TestSeek_ToPositionZero(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	for i := int64(0); i < 5; i++ {
		pts := i * 3600
		err = plugin.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, pts, pts, 0, true)
		require.NoError(t, err)
	}

	plugin.ResetForSeek()

	err = plugin.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, 0, 0, 0, true)
	require.NoError(t, err)
}

func TestSeek_MultipleRapidSeeks(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	for seek := 0; seek < 20; seek++ {
		err = plugin.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, int64(seek*90000), int64(seek*90000), 0, true)
		require.NoError(t, err)
		plugin.ResetForSeek()
	}

	assert.Equal(t, int64(21), plugin.Generation())
}

func TestSeek_ConcurrentSeekAndPush(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = plugin.PushVideo([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, int64(i*3600), int64(i*3600), 0, i%10 == 0)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			plugin.ResetForSeek()
		}
	}()

	wg.Wait()

	assert.GreaterOrEqual(t, plugin.Generation(), int64(11))
}

func TestSeek_NoStalePreSeekPacketsLeak(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	plugin.mu.Lock()
	plugin.ptsBaseSet = true
	plugin.ptsBaseVideo = 0
	plugin.lastVideoPTS = 90000
	plugin.videoSeq = 100
	plugin.mu.Unlock()

	plugin.ResetForSeek()

	plugin.mu.Lock()
	assert.Equal(t, uint16(0), plugin.videoSeq)
	assert.Equal(t, int64(0), plugin.lastVideoPTS)
	assert.False(t, plugin.ptsBaseSet)
	plugin.mu.Unlock()
}

func TestSeek_AfterStop(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	plugin.Stop()

	plugin.ResetForSeek()
	assert.Equal(t, int64(2), plugin.Generation())
}

func TestSeek_AfterEndOfStream(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	plugin.EndOfStream()

	plugin.ResetForSeek()
	assert.Equal(t, int64(2), plugin.Generation())
}

func TestSeek_VideoSequenceRestartsFromZero(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	plugin.mu.Lock()
	plugin.videoSeq = 12345
	plugin.audioSeq = 678
	plugin.mu.Unlock()

	plugin.ResetForSeek()

	plugin.mu.Lock()
	assert.Equal(t, uint16(0), plugin.videoSeq)
	assert.Equal(t, uint16(0), plugin.audioSeq)
	plugin.mu.Unlock()
}

func TestSeek_H265_AcceptsPacketsAfterReset(t *testing.T) {
	p, err := New(output.PluginConfig{
		Video: &media.VideoInfo{Codec: "hevc", FramerateN: 25, FramerateD: 1},
	})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	assert.Equal(t, "hevc", plugin.videoCodec)

	err = plugin.PushVideo([]byte{0x40, 0x01, 0xAA, 0xBB}, 1000, 1000, 0, true)
	require.NoError(t, err)

	plugin.ResetForSeek()

	err = plugin.PushVideo([]byte{0x40, 0x01, 0xCC, 0xDD}, 50000, 50000, 0, true)
	require.NoError(t, err)

	assert.Equal(t, int64(2), plugin.Generation())
}
