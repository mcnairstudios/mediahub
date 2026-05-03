package webrtc

import (
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestMode(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	assert.Equal(t, output.DeliveryWebRTC, p.Mode())
}

func TestStatus(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	status := p.Status()
	assert.Equal(t, output.DeliveryWebRTC, status.Mode)
	assert.False(t, status.Healthy)
	assert.Equal(t, int64(0), status.BytesWritten)
}

func TestStop(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	p.Stop()
	status := p.Status()
	assert.False(t, status.Healthy)
}

func TestGeneration(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	plugin := p.(*Plugin)
	assert.Equal(t, int64(1), plugin.Generation())
	plugin.ResetForSeek()
	assert.Equal(t, int64(2), plugin.Generation())
}

func TestPushVideoNoTrack(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	err = p.PushVideo([]byte{0, 0, 0, 1, 0x65, 0xFF}, 0, 0, true)
	assert.NoError(t, err)
}

func TestPushAudioNoTrack(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	err = p.PushAudio([]byte{0xFF, 0xF1}, 0, 0)
	assert.NoError(t, err)
}

func TestSplitAVCCNALUs(t *testing.T) {
	data := []byte{0, 0, 0, 3, 0x65, 0xAA, 0xBB, 0, 0, 0, 2, 0x06, 0xCC}
	nalus := splitAVCCNALUs(data)
	require.Len(t, nalus, 2)
	assert.Equal(t, []byte{0x65, 0xAA, 0xBB}, nalus[0])
	assert.Equal(t, []byte{0x06, 0xCC}, nalus[1])
}

func TestSplitAnnexBNALUs(t *testing.T) {
	data := []byte{0, 0, 0, 1, 0x65, 0xAA, 0, 0, 1, 0x06, 0xBB}
	nalus := splitAnnexBNALUs(data)
	require.Len(t, nalus, 2)
	assert.Equal(t, []byte{0x65, 0xAA}, nalus[0])
	assert.Equal(t, []byte{0x06, 0xBB}, nalus[1])
}

func TestPacketizeH264Small(t *testing.T) {
	nalu := make([]byte, 100)
	nalu[0] = 0x65
	packets := packetizeH264(nalu, 0, 0, 1400)
	require.Len(t, packets, 1)
	assert.True(t, packets[0].Header.Marker)
	assert.Equal(t, nalu, packets[0].Payload)
}

func TestPacketizeH264Large(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x65
	packets := packetizeH264(nalu, 0, 0, 1400)
	assert.Greater(t, len(packets), 1)
	assert.True(t, packets[len(packets)-1].Header.Marker)
	for i := 0; i < len(packets)-1; i++ {
		assert.False(t, packets[i].Header.Marker)
	}
}

func TestImplementsServablePlugin(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	var _ output.ServablePlugin = p.(*Plugin)
}
