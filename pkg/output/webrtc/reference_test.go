package webrtc

import (
	"math"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makePlugin(t *testing.T, codec string, fpsN, fpsD int) *Plugin {
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

func TestRTPVersion(t *testing.T) {
	nalu := []byte{0x65, 0xAA, 0xBB}
	pkts := packetizeH264(nalu, 0, 0, 1400, true)
	for _, pkt := range pkts {
		assert.Equal(t, uint8(2), pkt.Header.Version, "RTP version must always be 2")
	}
}

func TestRTPPayloadTypes(t *testing.T) {
	videoNALU := []byte{0x65, 0xAA}
	vpkts := packetizeH264(videoNALU, 0, 0, 1400, true)
	for _, pkt := range vpkts {
		assert.Equal(t, uint8(96), pkt.Header.PayloadType, "video payload type must be 96")
	}

	audioData := []byte{0x01, 0x02, 0x03}
	audioPkt := &rtp.Packet{
		Header: rtp.Header{
			Version:     2,
			PayloadType: audioPayloadType,
			SSRC:        audioSSRC,
			Marker:      true,
		},
		Payload: audioData,
	}
	assert.Equal(t, uint8(97), audioPkt.Header.PayloadType, "audio payload type must be 97")
}

func TestRTPSSRCs(t *testing.T) {
	vpkts := packetizeH264([]byte{0x65, 0xAA}, 0, 0, 1400, true)
	for _, pkt := range vpkts {
		assert.Equal(t, uint32(1), pkt.Header.SSRC, "video SSRC must be 1")
	}

	hpkts := packetizeHEVC([]byte{0x26, 0x01, 0xAA}, 0, 0, 1400, true)
	for _, pkt := range hpkts {
		assert.Equal(t, uint32(1), pkt.Header.SSRC, "HEVC video SSRC must be 1")
	}
}

func TestVideoTimestampDerivedFromPTS(t *testing.T) {
	tests := []struct {
		name     string
		pts      int64
		expected uint32
	}{
		{"0s", 0, 0},
		{"1s at 90kHz", 90000, 90000},
		{"33ms (one frame at 30fps)", 3000, 3000},
		{"10s", 900000, 900000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ptsToRTP(tt.pts, videoClockRate)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestAudioTimestampDerivedFromPTS(t *testing.T) {
	tests := []struct {
		name     string
		pts      int64
		expected uint32
	}{
		{"0s", 0, 0},
		{"1s (90000 PTS ticks)", 90000, 48000},
		{"20ms Opus frame (1800 PTS ticks)", 1800, 960},
		{"40ms (3600 PTS ticks)", 3600, 1920},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ptsToRTP(tt.pts, audioClockRate)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestH264SingleNALUBelowMTU(t *testing.T) {
	nalu := make([]byte, 500)
	nalu[0] = 0x65
	for i := 1; i < len(nalu); i++ {
		nalu[i] = byte(i)
	}

	pkts := packetizeH264(nalu, 0, 12345, 1400, true)
	require.Len(t, pkts, 1, "NALU below MTU must produce exactly 1 packet")
	assert.Equal(t, nalu, pkts[0].Payload, "single NALU payload must be the raw NALU")
	assert.True(t, pkts[0].Header.Marker, "marker must be set for last NALU of frame")
	assert.Equal(t, uint32(12345), pkts[0].Header.Timestamp)
}

func TestH264ExactlyMTUSizeNotFragmented(t *testing.T) {
	nalu := make([]byte, maxRTPPayload)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
	require.Len(t, pkts, 1, "NALU exactly at MTU must not be fragmented")
	assert.Equal(t, nalu, pkts[0].Payload)
}

func TestH264OneByteBeyondMTUFragmented(t *testing.T) {
	nalu := make([]byte, maxRTPPayload+1)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1, "NALU 1 byte over MTU must be fragmented")
}

func TestH264FUAIndicatorByte(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x65

	pkts := packetizeH264(nalu, 0, 0, 1400, true)
	require.Greater(t, len(pkts), 1)

	for _, pkt := range pkts {
		indicator := pkt.Payload[0]
		assert.Equal(t, byte(28), indicator&0x1f, "FU-A indicator type must be 28")
	}
}

func TestH264FUANRIPreserved(t *testing.T) {
	naluTypes := []struct {
		name     string
		header   byte
		nriBits  byte
	}{
		{"IDR NRI=3", 0x65, 0x60},
		{"non-IDR NRI=2", 0x41, 0x40},
		{"SPS NRI=3", 0x67, 0x60},
		{"PPS NRI=3", 0x68, 0x60},
		{"SEI NRI=0", 0x06, 0x00},
	}
	for _, tt := range naluTypes {
		t.Run(tt.name, func(t *testing.T) {
			nalu := make([]byte, 3000)
			nalu[0] = tt.header
			pkts := packetizeH264(nalu, 0, 0, 1400, true)
			if len(pkts) <= 1 {
				return
			}
			for _, pkt := range pkts {
				indicator := pkt.Payload[0]
				assert.Equal(t, tt.nriBits, indicator&0x60,
					"NRI bits must be preserved from original NALU header 0x%02X", tt.header)
			}
		})
	}
}

func TestH264FUAHeaderNALUType(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x65

	pkts := packetizeH264(nalu, 0, 0, 1400, true)
	require.Greater(t, len(pkts), 1)

	expectedType := nalu[0] & 0x1f
	for _, pkt := range pkts {
		fuHeader := pkt.Payload[1]
		assert.Equal(t, expectedType, fuHeader&0x1f,
			"FU-A header NALU type must match original")
	}
}

func TestH264FUAStartEndMiddleBits(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 0, 0, 1400, true)
	require.Greater(t, len(pkts), 2, "need at least 3 packets for start/middle/end test")

	firstFU := pkts[0].Payload[1]
	assert.True(t, firstFU&0x80 != 0, "first fragment: start bit must be set")
	assert.True(t, firstFU&0x40 == 0, "first fragment: end bit must be clear")

	for i := 1; i < len(pkts)-1; i++ {
		midFU := pkts[i].Payload[1]
		assert.True(t, midFU&0x80 == 0, "middle fragment %d: start bit must be clear", i)
		assert.True(t, midFU&0x40 == 0, "middle fragment %d: end bit must be clear", i)
	}

	lastFU := pkts[len(pkts)-1].Payload[1]
	assert.True(t, lastFU&0x80 == 0, "last fragment: start bit must be clear")
	assert.True(t, lastFU&0x40 != 0, "last fragment: end bit must be set")
}

func TestH264FUAAllFragmentsSameTimestamp(t *testing.T) {
	nalu := make([]byte, 10000)
	nalu[0] = 0x65
	ts := uint32(270000)
	pkts := packetizeH264(nalu, 0, ts, 1400, true)

	for i, pkt := range pkts {
		assert.Equal(t, ts, pkt.Header.Timestamp,
			"fragment %d: all fragments of one NALU must share the same timestamp", i)
	}
}

func TestH264FUASequenceNumbersContinuous(t *testing.T) {
	nalu := make([]byte, 10000)
	nalu[0] = 0x65
	startSeq := uint16(42)
	pkts := packetizeH264(nalu, startSeq, 0, 1400, true)

	for i, pkt := range pkts {
		assert.Equal(t, startSeq+uint16(i), pkt.Header.SequenceNumber,
			"fragment %d: sequence numbers must increment by 1", i)
	}
}

func TestH264FUAReassemblyEqualsOriginal(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x65
	for i := 1; i < len(nalu); i++ {
		nalu[i] = byte(i % 256)
	}

	pkts := packetizeH264(nalu, 0, 0, 1400, true)
	require.Greater(t, len(pkts), 1)

	var reassembled []byte
	reassembled = append(reassembled, pkts[0].Payload[1]&0x1f|pkts[0].Payload[0]&0x60)
	for _, pkt := range pkts {
		reassembled = append(reassembled, pkt.Payload[2:]...)
	}

	assert.Equal(t, nalu, reassembled, "reassembled FU-A fragments must equal original NALU")
}

func TestH264MarkerBitOnlyOnLastNALUOfFrame(t *testing.T) {
	sps := make([]byte, 20)
	sps[0] = 0x67
	pps := make([]byte, 10)
	pps[0] = 0x68
	idr := make([]byte, 100)
	idr[0] = 0x65

	spsPkts := packetizeH264(sps, 0, 1000, 1400, false)
	ppsPkts := packetizeH264(pps, spsPkts[len(spsPkts)-1].Header.SequenceNumber+1, 1000, 1400, false)
	idrPkts := packetizeH264(idr, ppsPkts[len(ppsPkts)-1].Header.SequenceNumber+1, 1000, 1400, true)

	for _, pkt := range spsPkts {
		assert.False(t, pkt.Header.Marker, "SPS packets: marker must be false")
	}
	for _, pkt := range ppsPkts {
		assert.False(t, pkt.Header.Marker, "PPS packets: marker must be false")
	}
	for i, pkt := range idrPkts {
		if i < len(idrPkts)-1 {
			assert.False(t, pkt.Header.Marker)
		} else {
			assert.True(t, pkt.Header.Marker, "last packet of last NALU: marker must be true")
		}
	}
}

func TestHEVCFUHeaderLayout(t *testing.T) {
	nalu := make([]byte, 3000)
	nalu[0] = 0x26
	nalu[1] = 0x01

	pkts := packetizeHEVC(nalu, 0, 0, 1400, true)
	require.Greater(t, len(pkts), 1)

	naluType := (nalu[0] >> 1) & 0x3f

	for _, pkt := range pkts {
		assert.Equal(t, byte((hevcNALUTypeFU<<1)|(nalu[0]&0x81)), pkt.Payload[0],
			"HEVC FU: first byte must have FU type with preserved F and LayerID MSB")
		assert.Equal(t, nalu[1], pkt.Payload[1],
			"HEVC FU: second byte must preserve TID byte")
		fuHeader := pkt.Payload[2]
		assert.Equal(t, naluType, fuHeader&0x3f,
			"HEVC FU header: NALU type must match original")
	}
}

func TestHEVCFUStartEndBits(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	pkts := packetizeHEVC(nalu, 0, 0, 1400, true)
	require.Greater(t, len(pkts), 2)

	assert.True(t, pkts[0].Payload[2]&0x80 != 0, "HEVC FU first: start bit set")
	assert.True(t, pkts[0].Payload[2]&0x40 == 0, "HEVC FU first: end bit clear")

	for i := 1; i < len(pkts)-1; i++ {
		assert.True(t, pkts[i].Payload[2]&0x80 == 0, "HEVC FU middle %d: start bit clear", i)
		assert.True(t, pkts[i].Payload[2]&0x40 == 0, "HEVC FU middle %d: end bit clear", i)
	}

	last := pkts[len(pkts)-1]
	assert.True(t, last.Payload[2]&0x80 == 0, "HEVC FU last: start bit clear")
	assert.True(t, last.Payload[2]&0x40 != 0, "HEVC FU last: end bit set")
}

func TestHEVCFUReassemblyEqualsOriginal(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	for i := 2; i < len(nalu); i++ {
		nalu[i] = byte(i % 256)
	}

	pkts := packetizeHEVC(nalu, 0, 0, 1400, true)
	require.Greater(t, len(pkts), 1)

	var reassembled []byte
	reassembled = append(reassembled, nalu[0], nalu[1])
	for _, pkt := range pkts {
		reassembled = append(reassembled, pkt.Payload[3:]...)
	}

	assert.Equal(t, nalu, reassembled, "reassembled HEVC FU fragments must equal original NALU")
}

func TestHEVCVariousNALUTypes(t *testing.T) {
	types := []struct {
		name     string
		header0  byte
		header1  byte
		naluType byte
	}{
		{"IDR_W_RADL (19)", 0x26, 0x01, 19},
		{"IDR_N_LP (20)", 0x28, 0x01, 20},
		{"CRA (21)", 0x2A, 0x01, 21},
		{"VPS (32)", 0x40, 0x01, 32},
		{"SPS (33)", 0x42, 0x01, 33},
		{"PPS (34)", 0x44, 0x01, 34},
		{"TRAIL_R (1)", 0x02, 0x01, 1},
	}
	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			nalu := make([]byte, 3000)
			nalu[0] = tt.header0
			nalu[1] = tt.header1
			pkts := packetizeHEVC(nalu, 0, 0, 1400, true)
			require.Greater(t, len(pkts), 1)

			for _, pkt := range pkts {
				fuHeader := pkt.Payload[2]
				assert.Equal(t, tt.naluType, fuHeader&0x3f,
					"HEVC FU header type for %s", tt.name)
			}
		})
	}
}

func TestAudioOpusPacketStructure(t *testing.T) {
	opusData := make([]byte, 160)
	for i := range opusData {
		opusData[i] = byte(i)
	}

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    audioPayloadType,
			SequenceNumber: 0,
			Timestamp:      0,
			SSRC:           audioSSRC,
			Marker:         true,
		},
		Payload: opusData,
	}

	assert.Equal(t, uint8(2), pkt.Header.Version)
	assert.Equal(t, uint8(97), pkt.Header.PayloadType)
	assert.Equal(t, uint32(2), pkt.Header.SSRC)
	assert.True(t, pkt.Header.Marker, "Opus audio: marker bit should be set")
	assert.Equal(t, opusData, pkt.Payload, "audio payload must be raw Opus frame")
}

func TestAudioTimestampIncrement(t *testing.T) {
	frameDurationPTS := int64(1800)

	var timestamps []uint32
	for i := 0; i < 50; i++ {
		pts := int64(i) * frameDurationPTS
		ts := ptsToRTP(pts, audioClockRate)
		timestamps = append(timestamps, ts)
	}

	for i := 1; i < len(timestamps); i++ {
		delta := timestamps[i] - timestamps[i-1]
		assert.Equal(t, uint32(960), delta,
			"audio frame %d: 20ms Opus at 48kHz = 960 sample increment", i)
	}
}

func TestPTSBaseCaptureOnFirstPacket(t *testing.T) {
	p := makePlugin(t, "h264", 30, 1)

	p.mu.Lock()
	assert.False(t, p.ptsBaseSet, "PTS base must not be set before any push")
	p.mu.Unlock()

	p.ptsBaseSet = false
	p.ptsBaseVideo = 0
	p.ptsBaseAudio = 0

	p.mu.Lock()
	if !p.ptsBaseSet {
		p.ptsBaseVideo = 50000
		p.ptsBaseAudio = 50000
		p.ptsBaseSet = true
	}
	p.mu.Unlock()

	p.mu.Lock()
	assert.True(t, p.ptsBaseSet)
	assert.Equal(t, int64(50000), p.ptsBaseVideo)
	assert.Equal(t, int64(50000), p.ptsBaseAudio)
	p.mu.Unlock()
}

func TestPTSBaseSubtractedFromSubsequentPackets(t *testing.T) {
	base := int64(100000)
	pts1 := int64(103000)
	pts2 := int64(106000)

	ts1 := ptsToRTP(pts1-base, videoClockRate)
	ts2 := ptsToRTP(pts2-base, videoClockRate)

	assert.Equal(t, uint32(3000), ts1)
	assert.Equal(t, uint32(6000), ts2)
}

func TestSeekResetsAllState(t *testing.T) {
	p := makePlugin(t, "h264", 25, 1)

	p.mu.Lock()
	p.videoSeq = 500
	p.audioSeq = 300
	p.videoTS = 450000
	p.audioTS = 240000
	p.ptsBaseSet = true
	p.ptsBaseVideo = 90000
	p.ptsBaseAudio = 90000
	p.lastVideoPTS = 180000
	p.lastAudioPTS = 180000
	p.mu.Unlock()

	genBefore := p.Generation()
	p.ResetForSeek()
	genAfter := p.Generation()

	p.mu.Lock()
	assert.Equal(t, uint16(0), p.videoSeq)
	assert.Equal(t, uint16(0), p.audioSeq)
	assert.Equal(t, uint32(0), p.videoTS)
	assert.Equal(t, uint32(0), p.audioTS)
	assert.False(t, p.ptsBaseSet, "PTS base must be cleared on seek")
	assert.Equal(t, int64(0), p.lastVideoPTS)
	assert.Equal(t, int64(0), p.lastAudioPTS)
	p.mu.Unlock()

	assert.Greater(t, genAfter, genBefore, "generation must increment on seek")
}

func TestSeekThenResumePTSBaseRecaptured(t *testing.T) {
	seekTarget := int64(270000)

	p := makePlugin(t, "h264", 30, 1)

	p.mu.Lock()
	p.ptsBaseSet = true
	p.ptsBaseVideo = 0
	p.ptsBaseAudio = 0
	p.videoSeq = 100
	p.audioSeq = 50
	p.mu.Unlock()

	p.ResetForSeek()

	p.mu.Lock()
	assert.False(t, p.ptsBaseSet)
	p.mu.Unlock()

	p.mu.Lock()
	p.ptsBaseVideo = seekTarget
	p.ptsBaseAudio = seekTarget
	p.ptsBaseSet = true
	p.mu.Unlock()

	postSeekPTS := seekTarget + 3000
	ts := ptsToRTP(postSeekPTS-seekTarget, videoClockRate)
	assert.Equal(t, uint32(3000), ts,
		"after seek, timestamps must be relative to new PTS base")
}

func TestMultipleSeeksGenerationMonotonic(t *testing.T) {
	p := makePlugin(t, "h264", 30, 1)

	var gens []int64
	gens = append(gens, p.Generation())
	for i := 0; i < 10; i++ {
		p.ResetForSeek()
		gens = append(gens, p.Generation())
	}

	for i := 1; i < len(gens); i++ {
		assert.Greater(t, gens[i], gens[i-1],
			"generation must be strictly monotonically increasing")
	}
}

func TestEndToEnd100VideoFrames(t *testing.T) {
	frameInterval := int64(3000)
	numFrames := 100

	var allPackets []*rtp.Packet
	seq := uint16(0)

	for i := 0; i < numFrames; i++ {
		pts := int64(i) * frameInterval

		naluSize := 100
		if i%30 == 0 {
			naluSize = 5000
		}
		nalu := make([]byte, naluSize)
		nalu[0] = 0x65

		ts := ptsToRTP(pts, videoClockRate)
		pkts := packetizeH264(nalu, seq, ts, maxRTPPayload, true)

		for _, pkt := range pkts {
			assert.Equal(t, uint8(2), pkt.Header.Version)
			assert.Equal(t, uint8(videoPayloadType), pkt.Header.PayloadType)
			assert.Equal(t, uint32(videoSSRC), pkt.Header.SSRC)
			allPackets = append(allPackets, pkt)
		}
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	for i := 1; i < len(allPackets); i++ {
		assert.Equal(t, allPackets[i-1].Header.SequenceNumber+1, allPackets[i].Header.SequenceNumber,
			"packet %d: no gaps in sequence numbers", i)
	}

	for i := 1; i < len(allPackets); i++ {
		assert.GreaterOrEqual(t, allPackets[i].Header.Timestamp, allPackets[i-1].Header.Timestamp,
			"packet %d: timestamps must be monotonically non-decreasing", i)
	}

	markerCount := 0
	for _, pkt := range allPackets {
		if pkt.Header.Marker {
			markerCount++
		}
	}
	assert.Equal(t, numFrames, markerCount,
		"marker bit count must equal frame count (one marker per frame)")
}

func TestEndToEnd100AudioFrames(t *testing.T) {
	frameDurationPTS := int64(1800)
	numFrames := 100

	seq := uint16(0)
	var timestamps []uint32

	for i := 0; i < numFrames; i++ {
		pts := int64(i) * frameDurationPTS
		ts := ptsToRTP(pts, audioClockRate)

		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    audioPayloadType,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           audioSSRC,
				Marker:         true,
			},
			Payload: make([]byte, 160),
		}

		assert.Equal(t, uint8(2), pkt.Header.Version)
		assert.Equal(t, uint8(audioPayloadType), pkt.Header.PayloadType)
		assert.Equal(t, uint32(audioSSRC), pkt.Header.SSRC)

		timestamps = append(timestamps, ts)
		seq++
	}

	for i := 1; i < len(timestamps); i++ {
		delta := timestamps[i] - timestamps[i-1]
		assert.Equal(t, uint32(960), delta,
			"audio frame %d: timestamp delta must be 960 (20ms at 48kHz)", i)
	}

	assert.Equal(t, uint16(numFrames), seq, "audio seq must equal frame count")
}

func TestEndToEndInterleavedVideoAudio(t *testing.T) {
	videoInterval := int64(3000)
	audioInterval := int64(1800)
	numVideoFrames := 50
	numAudioFrames := 83

	videoSeq := uint16(0)
	audioSeq := uint16(0)
	var videoTimestamps []uint32
	var audioTimestamps []uint32

	vi := 0
	ai := 0
	for vi < numVideoFrames || ai < numAudioFrames {
		videoPTS := int64(vi) * videoInterval
		audioPTS := int64(ai) * audioInterval

		pushVideo := vi < numVideoFrames && (ai >= numAudioFrames || videoPTS <= audioPTS)

		if pushVideo {
			nalu := make([]byte, 800)
			nalu[0] = 0x65
			ts := ptsToRTP(videoPTS, videoClockRate)
			pkts := packetizeH264(nalu, videoSeq, ts, maxRTPPayload, true)
			videoSeq = pkts[len(pkts)-1].Header.SequenceNumber + 1
			videoTimestamps = append(videoTimestamps, ts)
			vi++
		} else {
			ts := ptsToRTP(audioPTS, audioClockRate)
			audioTimestamps = append(audioTimestamps, ts)
			audioSeq++
			ai++
		}
	}

	for i := 1; i < len(videoTimestamps); i++ {
		assert.GreaterOrEqual(t, videoTimestamps[i], videoTimestamps[i-1],
			"video timestamp %d must be non-decreasing", i)
	}
	for i := 1; i < len(audioTimestamps); i++ {
		assert.Greater(t, audioTimestamps[i], audioTimestamps[i-1],
			"audio timestamp %d must be strictly increasing", i)
	}

	assert.Equal(t, numVideoFrames, len(videoTimestamps))
	assert.Equal(t, numAudioFrames, len(audioTimestamps))
}

func TestMultiNALUFrameSPSPPSIDR(t *testing.T) {
	sps := []byte{0x67, 0x42, 0x00, 0x1e, 0xab, 0x40, 0xf0, 0x28, 0xd0, 0x80}
	pps := []byte{0x68, 0xce, 0x06, 0xe2}
	idr := make([]byte, 4000)
	idr[0] = 0x65

	annexB := []byte{0, 0, 0, 1}
	annexB = append(annexB, sps...)
	annexB = append(annexB, 0, 0, 0, 1)
	annexB = append(annexB, pps...)
	annexB = append(annexB, 0, 0, 0, 1)
	annexB = append(annexB, idr...)

	nalus := splitNALUs(annexB)
	require.Len(t, nalus, 3, "SPS+PPS+IDR must produce 3 NALUs")

	assert.Equal(t, byte(0x67), nalus[0][0]&0x1f|0x60, "first NALU: SPS")
	assert.Equal(t, byte(0x68), nalus[1][0], "second NALU: PPS")
	assert.Equal(t, byte(0x65), nalus[2][0], "third NALU: IDR")

	ts := uint32(90000)
	seq := uint16(0)
	var allPkts []*rtp.Packet
	for i, nalu := range nalus {
		isLast := i == len(nalus)-1
		pkts := packetizeH264(nalu, seq, ts, maxRTPPayload, isLast)
		allPkts = append(allPkts, pkts...)
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	markerCount := 0
	for _, pkt := range allPkts {
		if pkt.Header.Marker {
			markerCount++
		}
	}
	assert.Equal(t, 1, markerCount, "only one marker bit in entire multi-NALU frame")
	assert.True(t, allPkts[len(allPkts)-1].Header.Marker, "marker must be on very last packet")

	for _, pkt := range allPkts {
		assert.Equal(t, ts, pkt.Header.Timestamp, "all packets in frame share one timestamp")
	}

	for i := 1; i < len(allPkts); i++ {
		assert.Equal(t, allPkts[i-1].Header.SequenceNumber+1, allPkts[i].Header.SequenceNumber,
			"sequence numbers continuous across NALUs in frame")
	}
}

func TestEmptyNALUsFiltered(t *testing.T) {
	data := []byte{0, 0, 0, 1, 0, 0, 0, 1, 0x65, 0xAA}
	nalus := splitAnnexBNALUs(data)

	nonEmpty := 0
	for _, n := range nalus {
		if len(n) > 0 {
			nonEmpty++
		}
	}
	assert.GreaterOrEqual(t, nonEmpty, 1, "at least one non-empty NALU should be present")
}

func TestSequenceNumberWraparound(t *testing.T) {
	startSeq := uint16(65534)
	nalu := make([]byte, 5000)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, startSeq, 0, 1400, true)

	require.Greater(t, len(pkts), 2)
	for i := 1; i < len(pkts); i++ {
		expected := startSeq + uint16(i)
		assert.Equal(t, expected, pkts[i].Header.SequenceNumber,
			"sequence number must wrap around naturally via uint16 overflow")
	}
}

func TestH264KnownGoodFUAFirstPacket(t *testing.T) {
	nalu := make([]byte, 2000)
	nalu[0] = 0x65
	for i := 1; i < len(nalu); i++ {
		nalu[i] = 0xAA
	}

	pkts := packetizeH264(nalu, 0, 90000, 1400, true)
	require.Greater(t, len(pkts), 1)

	first := pkts[0]
	assert.Equal(t, byte(0x7C), first.Payload[0],
		"FU indicator: NRI=0x60 (from 0x65) | type=28 = 0x7C")
	assert.Equal(t, byte(0x85), first.Payload[1],
		"FU header: start=1, end=0, type=5 (IDR) = 0x85")
	assert.Equal(t, uint32(90000), first.Header.Timestamp)
	assert.Equal(t, uint16(0), first.Header.SequenceNumber)
	assert.False(t, first.Header.Marker)
}

func TestH264KnownGoodFUALastPacket(t *testing.T) {
	nalu := make([]byte, 2000)
	nalu[0] = 0x65

	pkts := packetizeH264(nalu, 0, 90000, 1400, true)
	last := pkts[len(pkts)-1]

	assert.Equal(t, byte(0x7C), last.Payload[0], "FU indicator same as first")
	assert.Equal(t, byte(0x45), last.Payload[1],
		"FU header: start=0, end=1, type=5 (IDR) = 0x45")
	assert.True(t, last.Header.Marker)
	assert.Equal(t, uint32(90000), last.Header.Timestamp)
}

func TestH264KnownGoodSinglePacket(t *testing.T) {
	nalu := []byte{0x65, 0xB8, 0x00, 0x04, 0x00, 0x00, 0x05, 0xD4}

	pkts := packetizeH264(nalu, 42, 180000, 1400, true)
	require.Len(t, pkts, 1)

	pkt := pkts[0]
	assert.Equal(t, nalu, pkt.Payload, "single NALU: payload is raw NALU bytes")
	assert.Equal(t, uint8(2), pkt.Header.Version)
	assert.Equal(t, uint8(96), pkt.Header.PayloadType)
	assert.Equal(t, uint16(42), pkt.Header.SequenceNumber)
	assert.Equal(t, uint32(180000), pkt.Header.Timestamp)
	assert.Equal(t, uint32(1), pkt.Header.SSRC)
	assert.True(t, pkt.Header.Marker)
}

func TestHEVCKnownGoodFUFirstPacket(t *testing.T) {
	nalu := make([]byte, 2000)
	nalu[0] = 0x26
	nalu[1] = 0x01

	pkts := packetizeHEVC(nalu, 0, 90000, 1400, true)
	require.Greater(t, len(pkts), 1)

	first := pkts[0]
	expectedByte0 := byte((hevcNALUTypeFU << 1) | (nalu[0] & 0x81))
	assert.Equal(t, expectedByte0, first.Payload[0],
		"HEVC FU byte 0: type=49<<1 with F and LayerID MSB from original")
	assert.Equal(t, byte(0x01), first.Payload[1], "TID byte preserved")

	naluType := (nalu[0] >> 1) & 0x3f
	assert.Equal(t, byte(0x80)|naluType, first.Payload[2],
		"HEVC FU header: start=1, type=19 (IDR_W_RADL)")
}

func TestPTSToRTPPrecision(t *testing.T) {
	tests := []struct {
		name      string
		pts       int64
		clockRate uint32
		expected  uint32
	}{
		{"audio 1s exact", 90000, 48000, 48000},
		{"audio 0.5s", 45000, 48000, 24000},
		{"audio 100ms", 9000, 48000, 4800},
		{"video 1s", 90000, 90000, 90000},
		{"large PTS 60s video", 5400000, 90000, 5400000},
		{"large PTS 60s audio", 5400000, 48000, 2880000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ptsToRTP(tt.pts, tt.clockRate)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestPTSToRTPNegativeClamped(t *testing.T) {
	got := ptsToRTP(-5000, videoClockRate)
	assert.Equal(t, uint32(0), got, "negative PTS must be clamped to 0")
}

func TestTimestampMonotonicityAcross1000Frames(t *testing.T) {
	frameInterval := int64(3600)
	prevTS := uint32(0)

	for i := 0; i < 1000; i++ {
		pts := int64(i) * frameInterval
		ts := ptsToRTP(pts, videoClockRate)
		assert.GreaterOrEqual(t, ts, prevTS,
			"frame %d: timestamp must be monotonically non-decreasing", i)
		prevTS = ts
	}
}

func TestAVCCParsingLargeNALUs(t *testing.T) {
	naluData := make([]byte, 65536)
	for i := range naluData {
		naluData[i] = byte(i % 256)
	}

	lenBytes := []byte{
		byte(len(naluData) >> 24),
		byte(len(naluData) >> 16),
		byte(len(naluData) >> 8),
		byte(len(naluData)),
	}
	data := append(lenBytes, naluData...)

	nalus := splitAVCCNALUs(data)
	require.Len(t, nalus, 1)
	assert.Equal(t, len(naluData), len(nalus[0]))
}

func TestAnnexBParsingTrailingZeros(t *testing.T) {
	data := []byte{0, 0, 0, 1, 0x65, 0xAA, 0xBB, 0xCC}
	nalus := splitAnnexBNALUs(data)
	require.Len(t, nalus, 1)
	assert.Equal(t, []byte{0x65, 0xAA, 0xBB, 0xCC}, nalus[0])
}

func TestFragmentationCountAtMTUBoundary(t *testing.T) {
	mtu := 1400

	naluSize := mtu + 1
	nalu := make([]byte, naluSize)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 0, 0, mtu, true)

	expectedPayloadBytes := naluSize - 1
	payloadPerFragment := mtu - 2
	expectedFragments := int(math.Ceil(float64(expectedPayloadBytes) / float64(payloadPerFragment)))
	assert.Equal(t, expectedFragments, len(pkts),
		"fragment count must match ceiling(payload / (MTU - FU-A overhead))")
}

func TestHEVCFragmentationCountAtMTUBoundary(t *testing.T) {
	mtu := 1400

	naluSize := mtu + 1
	nalu := make([]byte, naluSize)
	nalu[0] = 0x26
	nalu[1] = 0x01
	pkts := packetizeHEVC(nalu, 0, 0, mtu, true)

	expectedPayloadBytes := naluSize - 2
	payloadPerFragment := mtu - 3
	expectedFragments := int(math.Ceil(float64(expectedPayloadBytes) / float64(payloadPerFragment)))
	assert.Equal(t, expectedFragments, len(pkts),
		"HEVC fragment count must match ceiling(payload / (MTU - FU overhead))")
}

func TestVideoAudioSSRCsDistinct(t *testing.T) {
	assert.NotEqual(t, videoSSRC, audioSSRC,
		"video and audio must use different SSRCs to demux streams")
}

func TestVideoAudioPayloadTypesDistinct(t *testing.T) {
	assert.NotEqual(t, videoPayloadType, audioPayloadType,
		"video and audio must use different payload types")
}

func TestVideoClockRate90kHz(t *testing.T) {
	assert.Equal(t, 90000, videoClockRate,
		"video clock rate must be 90kHz per RTP standard for video")
}

func TestAudioClockRate48kHz(t *testing.T) {
	assert.Equal(t, 48000, audioClockRate,
		"audio clock rate must be 48kHz for Opus")
}

func TestFUAOverheadExactly2Bytes(t *testing.T) {
	nalu := make([]byte, 2797)
	nalu[0] = 0x65
	pkts := packetizeH264(nalu, 0, 0, 1400, true)
	require.Len(t, pkts, 2, "2797-byte NALU: 2796 data bytes / 1398 per fragment = 2 fragments")

	assert.Equal(t, 1400, len(pkts[0].Payload),
		"first FU-A fragment payload must be exactly MTU bytes")
	firstDataLen := len(pkts[0].Payload) - 2
	assert.Equal(t, 1398, firstDataLen,
		"H.264 FU-A: 2 bytes overhead (indicator + header)")
}

func TestHEVCFUOverheadExactly3Bytes(t *testing.T) {
	nalu := make([]byte, 2800)
	nalu[0] = 0x26
	nalu[1] = 0x01
	pkts := packetizeHEVC(nalu, 0, 0, 1400, true)
	require.Greater(t, len(pkts), 1)

	assert.Equal(t, 1400, len(pkts[0].Payload),
		"first HEVC FU fragment payload must be exactly MTU bytes")
	firstDataLen := len(pkts[0].Payload) - 3
	assert.Equal(t, 1397, firstDataLen,
		"HEVC FU: 3 bytes overhead (2 header bytes + FU header)")
}

func TestSeekDoesNotAffectGeneration(t *testing.T) {
	p := makePlugin(t, "h264", 30, 1)

	gen1 := p.Generation()
	assert.Equal(t, int64(1), gen1)

	p.ResetForSeek()
	gen2 := p.Generation()
	assert.Equal(t, int64(2), gen2)

	p.ResetForSeek()
	gen3 := p.Generation()
	assert.Equal(t, int64(3), gen3)

	assert.Equal(t, int64(1), gen2-gen1, "each seek increments generation by exactly 1")
	assert.Equal(t, int64(1), gen3-gen2, "each seek increments generation by exactly 1")
}
