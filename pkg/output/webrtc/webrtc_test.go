package webrtc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestNewWithVideoInfo(t *testing.T) {
	p, err := New(output.PluginConfig{
		Video: &media.VideoInfo{
			Codec:      "hevc",
			FramerateN: 25,
			FramerateD: 1,
		},
	})
	require.NoError(t, err)
	plugin := p.(*Plugin)
	assert.Equal(t, "hevc", plugin.videoCodec)
	assert.InDelta(t, 25.0, plugin.videoFPS, 0.01)
}

func TestNewDefaultFPS(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)
	assert.InDelta(t, 30.0, plugin.videoFPS, 0.01)
	assert.Equal(t, "h264", plugin.videoCodec)
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

func TestStopIdempotent(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	p.Stop()
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

func TestPushVideoAfterStop(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	p.Stop()
	err = p.PushVideo([]byte{0, 0, 0, 1, 0x65}, 0, 0, true)
	assert.NoError(t, err)
}

func TestPushAudioAfterStop(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	p.Stop()
	err = p.PushAudio([]byte{0xFF}, 0, 0)
	assert.NoError(t, err)
}

func TestSplitAVCCNALUs(t *testing.T) {
	data := []byte{0, 0, 0, 3, 0x65, 0xAA, 0xBB, 0, 0, 0, 2, 0x06, 0xCC}
	nalus := splitAVCCNALUs(data)
	require.Len(t, nalus, 2)
	assert.Equal(t, []byte{0x65, 0xAA, 0xBB}, nalus[0])
	assert.Equal(t, []byte{0x06, 0xCC}, nalus[1])
}

func TestSplitAVCCInvalidLength(t *testing.T) {
	data := []byte{0, 0, 0, 100, 0x65}
	nalus := splitAVCCNALUs(data)
	assert.Empty(t, nalus)
}

func TestSplitAVCCZeroLength(t *testing.T) {
	data := []byte{0, 0, 0, 0}
	nalus := splitAVCCNALUs(data)
	assert.Empty(t, nalus)
}

func TestSplitAVCCSingleNALU(t *testing.T) {
	data := []byte{0, 0, 0, 5, 0x65, 0x01, 0x02, 0x03, 0x04}
	nalus := splitAVCCNALUs(data)
	require.Len(t, nalus, 1)
	assert.Equal(t, []byte{0x65, 0x01, 0x02, 0x03, 0x04}, nalus[0])
}

func TestSplitAnnexBNALUs(t *testing.T) {
	data := []byte{0, 0, 0, 1, 0x65, 0xAA, 0, 0, 1, 0x06, 0xBB}
	nalus := splitAnnexBNALUs(data)
	require.Len(t, nalus, 2)
	assert.Equal(t, []byte{0x65, 0xAA}, nalus[0])
	assert.Equal(t, []byte{0x06, 0xBB}, nalus[1])
}

func TestSplitAnnexBThreeByteStart(t *testing.T) {
	data := []byte{0, 0, 1, 0x65, 0xAA, 0xBB}
	nalus := splitAnnexBNALUs(data)
	require.Len(t, nalus, 1)
	assert.Equal(t, []byte{0x65, 0xAA, 0xBB}, nalus[0])
}

func TestSplitAnnexBFourByteStart(t *testing.T) {
	data := []byte{0, 0, 0, 1, 0x67, 0x42, 0x00, 0x1e}
	nalus := splitAnnexBNALUs(data)
	require.Len(t, nalus, 1)
	assert.Equal(t, []byte{0x67, 0x42, 0x00, 0x1e}, nalus[0])
}

func TestSplitNALUsDetectsAnnexB(t *testing.T) {
	data := []byte{0, 0, 0, 1, 0x65, 0xAA}
	nalus := splitNALUs(data)
	require.Len(t, nalus, 1)
	assert.Equal(t, byte(0x65), nalus[0][0])
}

func TestSplitNALUsDetectsAVCC(t *testing.T) {
	data := []byte{0, 0, 0, 2, 0x65, 0xAA}
	nalus := splitNALUs(data)
	require.Len(t, nalus, 1)
	assert.Equal(t, []byte{0x65, 0xAA}, nalus[0])
}

func TestSplitNALUsTooShort(t *testing.T) {
	data := []byte{0x65, 0xAA}
	nalus := splitNALUs(data)
	require.Len(t, nalus, 1)
	assert.Equal(t, data, nalus[0])
}

func TestPacketizeH264SingleNALU(t *testing.T) {
	nalu := make([]byte, 100)
	nalu[0] = 0x65
	packets := packetizeH264(nalu, 0, 1000, 1400, true)
	require.Len(t, packets, 1)
	assert.True(t, packets[0].Header.Marker)
	assert.Equal(t, nalu, packets[0].Payload)
	assert.Equal(t, uint32(1000), packets[0].Header.Timestamp)
	assert.Equal(t, uint16(0), packets[0].Header.SequenceNumber)
	assert.Equal(t, uint8(videoPayloadType), packets[0].Header.PayloadType)
	assert.Equal(t, uint32(videoSSRC), packets[0].Header.SSRC)
}

func TestPacketizeH264SingleNALUNoMarker(t *testing.T) {
	nalu := make([]byte, 100)
	nalu[0] = 0x67
	packets := packetizeH264(nalu, 0, 0, 1400, false)
	require.Len(t, packets, 1)
	assert.False(t, packets[0].Header.Marker)
}

func TestPacketizeH264FUA(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x65
	packets := packetizeH264(nalu, 10, 5000, 1400, true)

	require.Greater(t, len(packets), 1)

	first := packets[0]
	assert.Equal(t, uint16(10), first.Header.SequenceNumber)
	assert.Equal(t, uint32(5000), first.Header.Timestamp)
	assert.False(t, first.Header.Marker)

	fuIndicator := first.Payload[0]
	assert.Equal(t, byte(h264NALUTypeFUA), fuIndicator&0x1f)
	assert.Equal(t, nalu[0]&0x60, fuIndicator&0x60)

	fuHeader := first.Payload[1]
	assert.True(t, fuHeader&0x80 != 0, "start bit should be set on first FU-A")
	assert.False(t, fuHeader&0x40 != 0, "end bit should not be set on first FU-A")
	assert.Equal(t, nalu[0]&0x1f, fuHeader&0x1f)

	last := packets[len(packets)-1]
	assert.True(t, last.Header.Marker)
	lastFUHeader := last.Payload[1]
	assert.False(t, lastFUHeader&0x80 != 0, "start bit should not be set on last FU-A")
	assert.True(t, lastFUHeader&0x40 != 0, "end bit should be set on last FU-A")

	for i := 1; i < len(packets)-1; i++ {
		mid := packets[i]
		assert.False(t, mid.Header.Marker)
		midFU := mid.Payload[1]
		assert.False(t, midFU&0x80 != 0, "start bit should not be set on middle FU-A")
		assert.False(t, midFU&0x40 != 0, "end bit should not be set on middle FU-A")
	}

	for i := 0; i < len(packets); i++ {
		assert.Equal(t, uint32(5000), packets[i].Header.Timestamp)
	}

	for i := 1; i < len(packets); i++ {
		assert.Equal(t, packets[i-1].Header.SequenceNumber+1, packets[i].Header.SequenceNumber)
	}
}

func TestPacketizeH264FUANoMarker(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x65
	packets := packetizeH264(nalu, 0, 0, 1400, false)

	last := packets[len(packets)-1]
	assert.False(t, last.Header.Marker, "marker should not be set when markerOnLast is false")
}

func TestPacketizeHEVCSingleNALU(t *testing.T) {
	nalu := []byte{0x40, 0x01, 0xAA, 0xBB}
	packets := packetizeHEVC(nalu, 0, 2000, 1400, true)
	require.Len(t, packets, 1)
	assert.True(t, packets[0].Header.Marker)
	assert.Equal(t, nalu, packets[0].Payload)
	assert.Equal(t, uint32(2000), packets[0].Header.Timestamp)
}

func TestPacketizeHEVCFU(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	packets := packetizeHEVC(nalu, 5, 7000, 1400, true)

	require.Greater(t, len(packets), 1)

	first := packets[0]
	assert.Equal(t, uint16(5), first.Header.SequenceNumber)
	assert.False(t, first.Header.Marker)

	assert.Equal(t, byte((hevcNALUTypeFU<<1)|(nalu[0]&0x81)), first.Payload[0])
	assert.Equal(t, nalu[1], first.Payload[1])

	fuHeader := first.Payload[2]
	assert.True(t, fuHeader&0x80 != 0, "start bit")
	assert.False(t, fuHeader&0x40 != 0, "end bit")
	naluType := (nalu[0] >> 1) & 0x3f
	assert.Equal(t, naluType, fuHeader&0x3f)

	last := packets[len(packets)-1]
	assert.True(t, last.Header.Marker)
	lastFU := last.Payload[2]
	assert.False(t, lastFU&0x80 != 0)
	assert.True(t, lastFU&0x40 != 0)

	for i := 0; i < len(packets); i++ {
		assert.Equal(t, uint32(7000), packets[i].Header.Timestamp)
	}
}

func TestPacketizeHEVCTooShort(t *testing.T) {
	packets := packetizeHEVC([]byte{0x40}, 0, 0, 1400, true)
	assert.Nil(t, packets)
}

func TestPacketizeHEVCFUNoMarker(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	packets := packetizeHEVC(nalu, 0, 0, 1400, false)
	last := packets[len(packets)-1]
	assert.False(t, last.Header.Marker)
}

func TestPtsToRTP(t *testing.T) {
	tests := []struct {
		name      string
		pts       int64
		clockRate uint32
		expected  uint32
	}{
		{"zero", 0, 90000, 0},
		{"video 1s", 90000, 90000, 90000},
		{"video 0.5s", 45000, 90000, 45000},
		{"audio 1s at 48kHz", 90000, 48000, 48000},
		{"audio 20ms at 48kHz", 1800, 48000, 960},
		{"negative clamps to zero", -1000, 90000, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ptsToRTP(tt.pts, tt.clockRate)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestResetForSeekResetsTimestamps(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	plugin.mu.Lock()
	plugin.videoSeq = 100
	plugin.audioSeq = 50
	plugin.videoTS = 90000
	plugin.audioTS = 48000
	plugin.ptsBaseSet = true
	plugin.ptsBaseVideo = 1000
	plugin.ptsBaseAudio = 1000
	plugin.lastVideoPTS = 5000
	plugin.lastAudioPTS = 5000
	plugin.mu.Unlock()

	plugin.ResetForSeek()

	plugin.mu.Lock()
	assert.Equal(t, uint16(0), plugin.videoSeq)
	assert.Equal(t, uint16(0), plugin.audioSeq)
	assert.Equal(t, uint32(0), plugin.videoTS)
	assert.Equal(t, uint32(0), plugin.audioTS)
	assert.False(t, plugin.ptsBaseSet)
	assert.Equal(t, int64(0), plugin.lastVideoPTS)
	assert.Equal(t, int64(0), plugin.lastAudioPTS)
	plugin.mu.Unlock()

	assert.Equal(t, int64(2), plugin.Generation())
}

func TestWaitReadyTimeout(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = p.(*Plugin).WaitReady(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestWaitReadyAlreadyReady(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	plugin := p.(*Plugin)
	plugin.ready.Store(true)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = plugin.WaitReady(ctx)
	assert.NoError(t, err)
}

func TestWHEPDeleteNoConnection(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	plugin := p.(*Plugin)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/whep", nil)
	plugin.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestWHEPMethodNotAllowed(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	plugin := p.(*Plugin)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/whep", nil)
	plugin.ServeHTTP(w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestWHEPPostBadSDP(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	plugin := p.(*Plugin)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/whep", strings.NewReader("not a valid sdp"))
	plugin.ServeHTTP(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWHEPPostEmptyBody(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	plugin := p.(*Plugin)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/whep", strings.NewReader(""))
	plugin.ServeHTTP(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWHEPPostReadError(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	plugin := p.(*Plugin)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/whep", &errReader{})
	plugin.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

type errReader struct{}

func (e *errReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestVideoMimeType(t *testing.T) {
	tests := []struct {
		codec    string
		expected string
	}{
		{"h264", "video/H264"},
		{"hevc", "video/H265"},
		{"h265", "video/H265"},
		{"", "video/H264"},
	}
	for _, tt := range tests {
		p := &Plugin{videoCodec: tt.codec}
		if tt.codec == "" {
			p.videoCodec = "h264"
		}
		assert.Equal(t, tt.expected, p.videoMimeType(), "codec=%s", tt.codec)
	}
}

func TestImplementsServablePlugin(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	var _ output.ServablePlugin = p.(*Plugin)
}

func TestEndOfStreamNoPC(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	p.(*Plugin).EndOfStream()
}

func TestSplitAnnexBMultipleNALUs(t *testing.T) {
	data := []byte{
		0, 0, 0, 1, 0x67, 0x42, 0x00,
		0, 0, 0, 1, 0x68, 0xCE,
		0, 0, 0, 1, 0x65, 0x88, 0x84,
	}
	nalus := splitAnnexBNALUs(data)
	require.Len(t, nalus, 3)
	assert.Equal(t, byte(0x67), nalus[0][0])
	assert.Equal(t, byte(0x68), nalus[1][0])
	assert.Equal(t, byte(0x65), nalus[2][0])
}

func TestPacketizeH264SequenceNumbers(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x65
	packets := packetizeH264(nalu, 1000, 0, 1400, true)

	for i, pkt := range packets {
		assert.Equal(t, uint16(1000+i), pkt.Header.SequenceNumber)
	}
}

func TestPacketizeHEVCSequenceNumbers(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	packets := packetizeHEVC(nalu, 500, 0, 1400, true)

	for i, pkt := range packets {
		assert.Equal(t, uint16(500+i), pkt.Header.SequenceNumber)
	}
}

func TestSplitAVCCMultiple(t *testing.T) {
	data := []byte{
		0, 0, 0, 1, 0x67,
		0, 0, 0, 1, 0x68,
		0, 0, 0, 2, 0x65, 0xFF,
	}
	nalus := splitAVCCNALUs(data)
	require.Len(t, nalus, 3)
}
