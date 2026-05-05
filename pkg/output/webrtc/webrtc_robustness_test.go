package webrtc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWHEPDeleteReturns204(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	plugin := p.(*Plugin)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	plugin.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestWHEPGetReturnsMethodNotAllowed(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	plugin := p.(*Plugin)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	plugin.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestWHEPPostWithInvalidSDPReturnsError(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	defer p.Stop()

	plugin := p.(*Plugin)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not-valid-sdp"))
	plugin.ServeHTTP(rec, req)

	assert.True(t, rec.Code >= 400, "invalid SDP should produce an error response, got %d", rec.Code)
}

func TestDoubleStop(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	p.Stop()
	p.Stop()

	st := p.Status()
	assert.False(t, st.Healthy)
}

func TestEndOfStreamClosesConnection(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	p.EndOfStream()

	plugin := p.(*Plugin)
	plugin.mu.Lock()
	assert.Nil(t, plugin.pc)
	plugin.mu.Unlock()
}

func TestPushVideoAfterStopIsNoop(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	p.Stop()

	err = p.PushVideo([]byte{0, 0, 0, 1, 0x65, 0xFF}, 0, 0, 0, true)
	assert.NoError(t, err)
}

func TestPushAudioAfterStopIsNoop(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	p.Stop()

	err = p.PushAudio([]byte{0xFF, 0xF1}, 0, 0, 0)
	assert.NoError(t, err)
}

func TestPushSubtitleIsNoop(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	defer p.Stop()

	err = p.PushSubtitle([]byte("sub"), 0, 1000)
	assert.NoError(t, err)
}

func TestWaitReadyTimesOutWithoutConnection(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	defer p.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	plugin := p.(*Plugin)
	err = plugin.WaitReady(ctx)
	assert.Error(t, err, "WaitReady should timeout without a peer connection")
}

func TestWaitReadySucceedsWhenReady(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	defer p.Stop()

	plugin := p.(*Plugin)
	plugin.ready.Store(true)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = plugin.WaitReady(ctx)
	assert.NoError(t, err)
}

func TestGenerationStartsAtOne(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	defer p.Stop()

	plugin := p.(*Plugin)
	assert.Equal(t, int64(1), plugin.Generation())
}

func TestResetForSeekIncrementsGeneration(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	defer p.Stop()

	plugin := p.(*Plugin)
	plugin.ResetForSeek()
	assert.Equal(t, int64(2), plugin.Generation())
	plugin.ResetForSeek()
	assert.Equal(t, int64(3), plugin.Generation())
}

func TestStatusReportsBytesWritten(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	defer p.Stop()

	st := p.Status()
	assert.Equal(t, int64(0), st.BytesWritten)
	assert.Equal(t, output.DeliveryWebRTC, st.Mode)
}

func TestStatusNotHealthyWithoutConnection(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	defer p.Stop()

	st := p.Status()
	assert.False(t, st.Healthy, "should not be healthy without a peer connection")
}

func TestSplitNALUsSmallInput(t *testing.T) {
	data := []byte{0x65}
	nalus := splitNALUs(data)
	require.Len(t, nalus, 1)
	assert.Equal(t, data, nalus[0])
}

func TestSplitNALUsEmptyInput(t *testing.T) {
	nalus := splitNALUs([]byte{})
	assert.Len(t, nalus, 1)
}

func TestPacketizeH264SingleNALUWithTimestamp(t *testing.T) {
	nalu := make([]byte, 500)
	nalu[0] = 0x65
	packets := packetizeH264(nalu, 100, 3000, 1400, true)
	require.Len(t, packets, 1)
	assert.Equal(t, uint16(100), packets[0].Header.SequenceNumber)
	assert.Equal(t, uint32(3000), packets[0].Header.Timestamp)
	assert.True(t, packets[0].Header.Marker)
}

func TestPacketizeH264FragmentedConsecutiveSeq(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x65
	packets := packetizeH264(nalu, 0, 0, 1400, true)

	assert.Greater(t, len(packets), 1, "large NALU should be fragmented")

	for i := 0; i < len(packets)-1; i++ {
		assert.False(t, packets[i].Header.Marker, "non-final fragment should not have marker")
	}
	assert.True(t, packets[len(packets)-1].Header.Marker, "final fragment should have marker")

	for i := 1; i < len(packets); i++ {
		assert.Equal(t, packets[i-1].Header.SequenceNumber+1, packets[i].Header.SequenceNumber, "sequence numbers should be consecutive")
	}
}

func TestPacketizeH264FragmentsSameTimestamp(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x65
	ts := uint32(90000)
	packets := packetizeH264(nalu, 0, ts, 1400, true)

	for _, pkt := range packets {
		assert.Equal(t, ts, pkt.Header.Timestamp, "all fragments should share the same timestamp")
	}
}

func TestSplitAVCCThreeNALUs(t *testing.T) {
	data := []byte{
		0, 0, 0, 3, 0x67, 0xAA, 0xBB,
		0, 0, 0, 2, 0x68, 0xCC,
		0, 0, 0, 3, 0x65, 0xDD, 0xEE,
	}
	nalus := splitAVCCNALUs(data)
	require.Len(t, nalus, 3)
	assert.Equal(t, byte(0x67), nalus[0][0])
	assert.Equal(t, byte(0x68), nalus[1][0])
	assert.Equal(t, byte(0x65), nalus[2][0])
}

func TestSplitAnnexBMixedStartCodes(t *testing.T) {
	data := []byte{
		0, 0, 0, 1, 0x67, 0xAA,
		0, 0, 0, 1, 0x68, 0xBB,
		0, 0, 1, 0x65, 0xCC,
	}
	nalus := splitAnnexBNALUs(data)
	require.Len(t, nalus, 3)
	assert.Equal(t, byte(0x67), nalus[0][0])
	assert.Equal(t, byte(0x68), nalus[1][0])
	assert.Equal(t, byte(0x65), nalus[2][0])
}

func TestWHEPDeleteWithoutPCReturns204(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	defer p.Stop()

	plugin := p.(*Plugin)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	plugin.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestStopClearsTracksAndPC(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	plugin := p.(*Plugin)
	p.Stop()

	plugin.mu.Lock()
	assert.Nil(t, plugin.pc)
	assert.Nil(t, plugin.videoTrack)
	assert.Nil(t, plugin.audioTrack)
	plugin.mu.Unlock()
}
