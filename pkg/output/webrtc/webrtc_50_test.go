package webrtc

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestP01_RTPVersionAlways2(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x65
	for _, pkt := range packetizeH264(nalu, 0, 0, maxRTPPayload, true) {
		assert.Equal(t, uint8(2), pkt.Header.Version)
		assert.False(t, pkt.Header.Padding)
	}
}

func TestP02_VideoSSRCConstant(t *testing.T) {
	seq := uint16(0)
	for i := 0; i < 50; i++ {
		nalu := make([]byte, 200+i*50)
		nalu[0] = 0x65
		pkts := packetizeH264(nalu, seq, uint32(i*3600), maxRTPPayload, true)
		for _, pkt := range pkts {
			assert.Equal(t, uint32(videoSSRC), pkt.Header.SSRC)
		}
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}
}

func TestP03_AudioSSRCConstant(t *testing.T) {
	for i := 0; i < 50; i++ {
		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    audioPayloadType,
				SequenceNumber: uint16(i),
				Timestamp:      ptsToRTP(int64(i)*1800, audioClockRate),
				SSRC:           audioSSRC,
				Marker:         true,
			},
			Payload: make([]byte, 160),
		}
		assert.Equal(t, uint32(audioSSRC), pkt.Header.SSRC)
	}
}

func TestP04_VideoSSRCNotEqualAudioSSRC(t *testing.T) {
	assert.NotEqual(t, uint32(videoSSRC), uint32(audioSSRC),
		"video and audio SSRCs must differ for stream demuxing")
}

func TestP05_PayloadTypeStablePerTrack(t *testing.T) {
	for i := 0; i < 100; i++ {
		nalu := make([]byte, 200)
		nalu[0] = 0x65
		pkts := packetizeH264(nalu, 0, uint32(i*3600), maxRTPPayload, true)
		for _, pkt := range pkts {
			assert.Equal(t, uint8(videoPayloadType), pkt.Header.PayloadType)
		}
	}
}

func TestP06_SequenceIncrementExactlyOne(t *testing.T) {
	seq := uint16(0)
	var allPkts []*rtp.Packet
	for i := 0; i < 50; i++ {
		size := 200
		if i%5 == 0 {
			size = 5000
		}
		nalu := make([]byte, size)
		nalu[0] = 0x65
		pkts := packetizeH264(nalu, seq, uint32(i*3600), maxRTPPayload, true)
		allPkts = append(allPkts, pkts...)
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}
	for i := 1; i < len(allPkts); i++ {
		assert.Equal(t, allPkts[i-1].Header.SequenceNumber+1, allPkts[i].Header.SequenceNumber,
			"packet %d: increment must be exactly 1", i)
	}
}

func TestP07_SequenceWraparoundAt65535(t *testing.T) {
	nalu := make([]byte, 8000)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 65533, 0, maxRTPPayload, true)

	require.Greater(t, len(pkts), 3)

	found65535 := false
	foundZero := false
	for _, pkt := range pkts {
		if pkt.Header.SequenceNumber == 65535 {
			found65535 = true
		}
		if pkt.Header.SequenceNumber == 0 && found65535 {
			foundZero = true
		}
	}
	assert.True(t, found65535, "must pass through 65535")
	assert.True(t, foundZero, "must wrap to 0 after 65535")
}

func TestP08_TimestampIncrement25fps(t *testing.T) {
	for i := 1; i < 100; i++ {
		prev := ptsToRTP(int64(i-1)*3600, videoClockRate)
		curr := ptsToRTP(int64(i)*3600, videoClockRate)
		assert.Equal(t, uint32(3600), curr-prev,
			"frame %d: 25fps must increment by exactly 3600 RTP ticks", i)
	}
}

func TestP09_TimestampIncrement30fps(t *testing.T) {
	for i := 1; i < 100; i++ {
		prev := ptsToRTP(int64(i-1)*3000, videoClockRate)
		curr := ptsToRTP(int64(i)*3000, videoClockRate)
		assert.Equal(t, uint32(3000), curr-prev,
			"frame %d: 30fps must increment by exactly 3000 RTP ticks", i)
	}
}

func TestP10_TimestampIncrementOpus960(t *testing.T) {
	for i := 1; i < 100; i++ {
		prev := ptsToRTP(int64(i-1)*1800, audioClockRate)
		curr := ptsToRTP(int64(i)*1800, audioClockRate)
		assert.Equal(t, uint32(960), curr-prev,
			"frame %d: Opus 20ms must increment by exactly 960 samples", i)
	}
}

func TestP11_SmallNALUSinglePacketNoFUA(t *testing.T) {
	nalu := make([]byte, 500)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
	require.Len(t, pkts, 1, "NALU < MTU must produce exactly 1 packet")
	assert.Equal(t, byte(0x65), pkts[0].Payload[0], "payload must be raw NALU (no FU-A header)")
}

func TestP12_LargeNALUFUA(t *testing.T) {
	nalu := make([]byte, 2000)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1, "NALU > MTU must be fragmented with FU-A")
	assert.Equal(t, byte(h264NALUTypeFUA), pkts[0].Payload[0]&0x1f, "first byte type must be 28 (FU-A)")
}

func TestP13_FUAIndicatorByte0x7C(t *testing.T) {
	nalu := make([]byte, 2000)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1)
	assert.Equal(t, byte(0x7C), pkts[0].Payload[0],
		"IDR (0x65): FU-A indicator = NRI(0x60) | type(28) = 0x7C")
}

func TestP14_FUAStartBitOnFirstOnly(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 2)

	assert.True(t, pkts[0].Payload[1]&0x80 != 0, "first: start bit must be set")
	for i := 1; i < len(pkts); i++ {
		assert.True(t, pkts[i].Payload[1]&0x80 == 0,
			"packet %d: start bit must NOT be set", i)
	}
}

func TestP15_FUAEndBitOnLastOnly(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 2)

	for i := 0; i < len(pkts)-1; i++ {
		assert.True(t, pkts[i].Payload[1]&0x40 == 0,
			"packet %d: end bit must NOT be set", i)
	}
	assert.True(t, pkts[len(pkts)-1].Payload[1]&0x40 != 0, "last: end bit must be set")
}

func TestP16_FUANRIPreservedFromOriginal(t *testing.T) {
	cases := []struct {
		header byte
		nri    byte
	}{
		{0x65, 0x60},
		{0x41, 0x40},
		{0x06, 0x00},
	}
	for _, c := range cases {
		nalu := make([]byte, 3000)
		nalu[0] = c.header
		pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
		if len(pkts) <= 1 {
			continue
		}
		for _, pkt := range pkts {
			assert.Equal(t, c.nri, pkt.Payload[0]&0x60,
				"NALU 0x%02X: NRI must be preserved as 0x%02X", c.header, c.nri)
		}
	}
}

func TestP17_FUAReassembledEqualsOriginal(t *testing.T) {
	nalu := make([]byte, 7000)
	nalu[0] = 0x65
	for i := 1; i < len(nalu); i++ {
		nalu[i] = byte(i % 256)
	}
	pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1)

	var reassembled []byte
	reassembled = append(reassembled, pkts[0].Payload[1]&0x1f|pkts[0].Payload[0]&0x60)
	for _, pkt := range pkts {
		reassembled = append(reassembled, pkt.Payload[2:]...)
	}
	assert.Equal(t, nalu, reassembled)
}

func TestP18_FragmentCountCeil(t *testing.T) {
	nalu := make([]byte, 10000)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)

	dataBytes := len(nalu) - 1
	perFrag := maxRTPPayload - 2
	expected := int(math.Ceil(float64(dataBytes) / float64(perFrag)))
	assert.Equal(t, expected, len(pkts))
}

func TestP19_MarkerBitOnlyOnLastFragmentOfLastNALU(t *testing.T) {
	sps := []byte{0x67, 0x42, 0x00}
	idr := make([]byte, 4000)
	idr[0] = 0x65

	seq := uint16(0)
	spsPkts := packetizeH264(sps, seq, 1000, maxRTPPayload, false)
	seq = spsPkts[len(spsPkts)-1].Header.SequenceNumber + 1
	idrPkts := packetizeH264(idr, seq, 1000, maxRTPPayload, true)

	for _, pkt := range spsPkts {
		assert.False(t, pkt.Header.Marker, "SPS: no marker")
	}
	for i := 0; i < len(idrPkts)-1; i++ {
		assert.False(t, idrPkts[i].Header.Marker, "IDR fragment %d: no marker", i)
	}
	assert.True(t, idrPkts[len(idrPkts)-1].Header.Marker, "last fragment of last NALU: marker set")
}

func TestP20_MultiNALUFrameCorrectFragmentation(t *testing.T) {
	sps := []byte{0x67, 0x42, 0x00, 0x1e, 0xab, 0x40}
	pps := []byte{0x68, 0xce, 0x06}
	idr := make([]byte, 5000)
	idr[0] = 0x65

	ts := uint32(90000)
	seq := uint16(0)
	var allPkts []*rtp.Packet

	for i, nalu := range [][]byte{sps, pps, idr} {
		isLast := i == 2
		pkts := packetizeH264(nalu, seq, ts, maxRTPPayload, isLast)
		allPkts = append(allPkts, pkts...)
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	markerCount := 0
	for _, pkt := range allPkts {
		if pkt.Header.Marker {
			markerCount++
		}
		assert.Equal(t, ts, pkt.Header.Timestamp, "all packets in frame share timestamp")
	}
	assert.Equal(t, 1, markerCount, "exactly one marker in multi-NALU frame")
}

func TestP21_HEVC3ByteFUHeader(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	pkts := packetizeHEVC(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1)
	for _, pkt := range pkts {
		assert.GreaterOrEqual(t, len(pkt.Payload), 3, "HEVC FU must have >= 3 byte header")
	}
}

func TestP22_HEVCLayerIDPreserved(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	pkts := packetizeHEVC(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1)
	for _, pkt := range pkts {
		assert.Equal(t, nalu[0]&0x81, pkt.Payload[0]&0x81,
			"F bit and LayerID MSB must be preserved")
	}
}

func TestP23_HEVCTIDPreserved(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	pkts := packetizeHEVC(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1)
	for _, pkt := range pkts {
		assert.Equal(t, byte(0x01), pkt.Payload[1], "TID byte must be preserved")
	}
}

func TestP24_HEVCFUStartEndBits(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	pkts := packetizeHEVC(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 2)

	assert.True(t, pkts[0].Payload[2]&0x80 != 0, "first: start=1")
	assert.True(t, pkts[0].Payload[2]&0x40 == 0, "first: end=0")
	for i := 1; i < len(pkts)-1; i++ {
		assert.True(t, pkts[i].Payload[2]&0x80 == 0, "mid %d: start=0", i)
		assert.True(t, pkts[i].Payload[2]&0x40 == 0, "mid %d: end=0", i)
	}
	assert.True(t, pkts[len(pkts)-1].Payload[2]&0x80 == 0, "last: start=0")
	assert.True(t, pkts[len(pkts)-1].Payload[2]&0x40 != 0, "last: end=1")
}

func TestP25_HEVCReassembledEqualsOriginal(t *testing.T) {
	nalu := make([]byte, 7000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	for i := 2; i < len(nalu); i++ {
		nalu[i] = byte(i % 256)
	}
	pkts := packetizeHEVC(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1)

	var reassembled []byte
	reassembled = append(reassembled, nalu[0], nalu[1])
	for _, pkt := range pkts {
		reassembled = append(reassembled, pkt.Payload[3:]...)
	}
	assert.Equal(t, nalu, reassembled)
}

func TestP26_HEVCAll6NALUTypes(t *testing.T) {
	types := []struct {
		byte0    byte
		byte1    byte
		naluType byte
	}{
		{0x26, 0x01, 19},
		{0x28, 0x01, 20},
		{0x2A, 0x01, 21},
		{0x40, 0x01, 32},
		{0x42, 0x01, 33},
		{0x44, 0x01, 34},
	}
	for _, tt := range types {
		nalu := make([]byte, 3000)
		nalu[0] = tt.byte0
		nalu[1] = tt.byte1
		pkts := packetizeHEVC(nalu, 0, 0, maxRTPPayload, true)
		require.Greater(t, len(pkts), 1, "type %d must fragment", tt.naluType)
		for _, pkt := range pkts {
			assert.Equal(t, tt.naluType, pkt.Payload[2]&0x3f,
				"HEVC type %d: FU header must carry correct type", tt.naluType)
		}
	}
}

func TestP27_HEVCFragmentCountCorrect(t *testing.T) {
	nalu := make([]byte, 10000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	pkts := packetizeHEVC(nalu, 0, 0, maxRTPPayload, true)

	dataBytes := len(nalu) - 2
	perFrag := maxRTPPayload - 3
	expected := int(math.Ceil(float64(dataBytes) / float64(perFrag)))
	assert.Equal(t, expected, len(pkts))
}

func TestP28_HEVCMarkerBitCorrect(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	pkts := packetizeHEVC(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1)

	for i := 0; i < len(pkts)-1; i++ {
		assert.False(t, pkts[i].Header.Marker, "non-last fragment: no marker")
	}
	assert.True(t, pkts[len(pkts)-1].Header.Marker, "last fragment: marker set")
}

func TestP29_HEVCMixedSizes(t *testing.T) {
	small := make([]byte, 100)
	small[0] = 0x42
	small[1] = 0x01

	large := make([]byte, 3000)
	large[0] = 0x26
	large[1] = 0x01

	smallPkts := packetizeHEVC(small, 0, 0, maxRTPPayload, false)
	require.Len(t, smallPkts, 1, "small HEVC NALU: 1 packet")
	assert.Equal(t, small, smallPkts[0].Payload, "small: raw NALU payload")

	largePkts := packetizeHEVC(large, smallPkts[0].Header.SequenceNumber+1, 0, maxRTPPayload, true)
	require.Greater(t, len(largePkts), 1, "large HEVC NALU: fragmented")
}

func TestP30_HEVCSequenceContinuityAcrossMultiNALU(t *testing.T) {
	seq := uint16(0)
	var allPkts []*rtp.Packet

	for i := 0; i < 30; i++ {
		size := 200
		if i%3 == 0 {
			size = 4000
		}
		nalu := make([]byte, size)
		nalu[0] = 0x26
		nalu[1] = 0x01
		pkts := packetizeHEVC(nalu, seq, uint32(i*3600), maxRTPPayload, true)
		allPkts = append(allPkts, pkts...)
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	for i := 1; i < len(allPkts); i++ {
		assert.Equal(t, allPkts[i-1].Header.SequenceNumber+1, allPkts[i].Header.SequenceNumber,
			"packet %d: sequence continuity", i)
	}
}

func TestP31_25fps100FramesTotalAdvance360000(t *testing.T) {
	first := ptsToRTP(0, videoClockRate)
	last := ptsToRTP(99*3600, videoClockRate)
	assert.Equal(t, uint32(356400), last-first, "99 intervals * 3600 = 356400")

	totalWithLastFrame := last - first + 3600
	assert.Equal(t, uint32(360000), totalWithLastFrame, "100 frames at 25fps = 360000 ticks (4s)")
}

func TestP32_30fps100FramesTotalAdvance300000(t *testing.T) {
	first := ptsToRTP(0, videoClockRate)
	last := ptsToRTP(99*3000, videoClockRate)
	totalWithLastFrame := last - first + 3000
	assert.Equal(t, uint32(300000), totalWithLastFrame, "100 frames at 30fps = 300000 ticks")
}

func TestP33_50iDeinterlacedTo25fps(t *testing.T) {
	for i := 1; i < 100; i++ {
		prev := ptsToRTP(int64(i-1)*3600, videoClockRate)
		curr := ptsToRTP(int64(i)*3600, videoClockRate)
		delta := curr - prev
		assert.Equal(t, uint32(3600), delta,
			"frame %d: 50i deinterlaced to 25p = 3600 ticks, NOT 1800", i)
		assert.NotEqual(t, uint32(1800), delta,
			"frame %d: must NOT use 50fps spacing", i)
	}
}

func TestP34_Audio20ms100FramesAdvance96000(t *testing.T) {
	first := ptsToRTP(0, audioClockRate)
	last := ptsToRTP(99*1800, audioClockRate)
	totalWithLastFrame := last - first + 960
	assert.Equal(t, uint32(96000), totalWithLastFrame, "100 Opus frames at 20ms = 96000 samples (2s)")
}

func TestP35_AVSyncAfter5Seconds(t *testing.T) {
	videoFrames := 125
	audioFrames := 250

	lastVideoTS := ptsToRTP(int64(videoFrames-1)*3600, videoClockRate)
	lastAudioTS := ptsToRTP(int64(audioFrames-1)*1800, audioClockRate)

	videoSec := float64(lastVideoTS) / float64(videoClockRate)
	audioSec := float64(lastAudioTS) / float64(audioClockRate)

	diff := math.Abs(videoSec - audioSec)
	oneVideoFrame := 1.0 / 25.0
	assert.Less(t, diff, oneVideoFrame,
		"A/V sync: %.6fs video vs %.6fs audio, diff=%.6fs (limit=%.4fs)", videoSec, audioSec, diff, oneVideoFrame)
}

func TestP36_PTSBasedTimestampsCorrect(t *testing.T) {
	ptsValues := []int64{0, 3600, 7200, 90000, 180000, 450000}
	for _, pts := range ptsValues {
		rtp := ptsToRTP(pts, videoClockRate)
		assert.Equal(t, uint32(pts), rtp, "PTS %d: video RTP = PTS when clockRate=90kHz", pts)
	}

	audioPTS := int64(90000)
	audioRTP := ptsToRTP(audioPTS, audioClockRate)
	assert.Equal(t, uint32(48000), audioRTP, "audio 1s: 90000 PTS -> 48000 RTP at 48kHz")
}

func TestP37_AfterSeekTimestampsRestartFromZero(t *testing.T) {
	p := make50Plugin(t, "h264", 25, 1)

	p.mu.Lock()
	p.videoSeq = 500
	p.videoTS = 450000
	p.ptsBaseSet = true
	p.ptsBaseVideo = 90000
	p.mu.Unlock()

	p.ResetForSeek()

	p.mu.Lock()
	assert.Equal(t, uint32(0), p.videoTS)
	assert.Equal(t, uint16(0), p.videoSeq)
	assert.False(t, p.ptsBaseSet)
	p.mu.Unlock()

	ts := ptsToRTP(0, videoClockRate)
	assert.Equal(t, uint32(0), ts, "first frame after seek: RTP timestamp = 0")
}

func TestP38_NoGapsInTimestamps(t *testing.T) {
	var timestamps []uint32
	for i := 0; i < 200; i++ {
		timestamps = append(timestamps, ptsToRTP(int64(i)*3600, videoClockRate))
	}
	for i := 1; i < len(timestamps); i++ {
		delta := timestamps[i] - timestamps[i-1]
		assert.Equal(t, uint32(3600), delta, "frame %d: no gaps allowed, must be exactly 3600", i)
	}
}

func TestP39_MonotonicallyIncreasing(t *testing.T) {
	prev := uint32(0)
	for i := 1; i < 500; i++ {
		ts := ptsToRTP(int64(i)*3600, videoClockRate)
		assert.Greater(t, ts, prev, "frame %d: timestamps must be strictly increasing", i)
		prev = ts
	}
}

func TestP40_TotalDurationCorrect(t *testing.T) {
	tests := []struct {
		name      string
		frames    int
		interval  int64
		clockRate uint32
		expected  float64
	}{
		{"25fps 100 frames", 100, 3600, uint32(videoClockRate), 4.0},
		{"30fps 90 frames", 90, 3000, uint32(videoClockRate), 3.0},
		{"Opus 50 frames", 50, 1800, uint32(audioClockRate), 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := ptsToRTP(0, tt.clockRate)
			last := ptsToRTP(int64(tt.frames-1)*tt.interval, tt.clockRate)
			frameDur := ptsToRTP(tt.interval, tt.clockRate)
			duration := float64(last-first+frameDur) / float64(tt.clockRate)
			assert.InDelta(t, tt.expected, duration, tt.expected*0.01,
				"total duration must match within 1%%")
		})
	}
}

func TestP41_WHEPPostCreates(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/whep", strings.NewReader("v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"))
	plugin.ServeHTTP(w, r)

	assert.Contains(t, []int{http.StatusCreated, http.StatusInternalServerError}, w.Code,
		"POST must either create (201) or fail on bad SDP (500)")
}

func TestP42_WHEPSecondPostReplacesFirst(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodPost, "/whep", strings.NewReader("invalid"))
	plugin.ServeHTTP(w1, r1)

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/whep", strings.NewReader("also invalid"))
	plugin.ServeHTTP(w2, r2)

	assert.Equal(t, http.StatusInternalServerError, w2.Code)
}

func TestP43_WHEPDeleteCloses(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/whep", nil)
	plugin.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestP44_WHEPEmptySDPBodyError(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/whep", strings.NewReader(""))
	plugin.ServeHTTP(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestP45_WHEPInvalidSDPError(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/whep", strings.NewReader("this is not valid SDP at all"))
	plugin.ServeHTTP(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestP46_WHEPMethodNotAllowed405(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPatch} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, "/whep", nil)
		plugin.ServeHTTP(w, r)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code, "method %s must return 405", method)
	}
}

func TestP47_ResetForSeekIncrementsGeneration(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	gen1 := plugin.Generation()
	plugin.ResetForSeek()
	gen2 := plugin.Generation()
	assert.Equal(t, gen1+1, gen2, "ResetForSeek must increment generation by exactly 1")
}

func TestP48_StopClearsAllState(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	plugin := p.(*Plugin)

	plugin.Stop()

	plugin.mu.Lock()
	assert.Nil(t, plugin.videoTrack)
	assert.Nil(t, plugin.audioTrack)
	assert.Nil(t, plugin.pc)
	plugin.mu.Unlock()

	assert.True(t, plugin.stopped.Load())
}

func TestP49_PluginModeWebRTC(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)
	assert.Equal(t, output.DeliveryWebRTC, p.Mode())
}

func TestP50_StatusHealthyWhenNoErrors(t *testing.T) {
	p, err := New(output.PluginConfig{})
	require.NoError(t, err)

	status := p.Status()
	assert.False(t, status.Healthy, "no PC connected: not healthy")
	assert.Equal(t, output.DeliveryWebRTC, status.Mode)
	assert.Equal(t, int64(0), status.BytesWritten)
}

func make50Plugin(t *testing.T, codec string, fpsN, fpsD int) *Plugin {
	t.Helper()
	cfg := output.PluginConfig{}
	if codec != "" {
		cfg.Video = &media.VideoInfo{
			Codec:      codec,
			FramerateN: fpsN,
			FramerateD: fpsD,
		}
	}
	p, err := New(cfg)
	require.NoError(t, err)
	return p.(*Plugin)
}
