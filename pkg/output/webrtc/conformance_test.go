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

func makeTestPlugin(t *testing.T, codec string, fpsN, fpsD int) *Plugin {
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

func TestConformance_VideoRTPTimestampSpacing25fps(t *testing.T) {
	frameIntervalPTS := int64(3600)
	numFrames := 150

	var timestamps []uint32
	for i := 0; i < numFrames; i++ {
		pts := int64(i) * frameIntervalPTS
		ts := ptsToRTP(pts, videoClockRate)
		timestamps = append(timestamps, ts)
	}

	for i := 1; i < numFrames; i++ {
		delta := timestamps[i] - timestamps[i-1]
		assert.Equal(t, uint32(3600), delta,
			"frame %d: 25fps video RTP timestamp must increment by exactly 3600 (90000/25), got %d", i, delta)
	}
}

func TestConformance_VideoRTPTimestampSpacing30fps(t *testing.T) {
	frameIntervalPTS := int64(3000)
	numFrames := 150

	var timestamps []uint32
	for i := 0; i < numFrames; i++ {
		pts := int64(i) * frameIntervalPTS
		ts := ptsToRTP(pts, videoClockRate)
		timestamps = append(timestamps, ts)
	}

	for i := 1; i < numFrames; i++ {
		delta := timestamps[i] - timestamps[i-1]
		assert.Equal(t, uint32(3000), delta,
			"frame %d: 30fps video RTP timestamp must increment by exactly 3000 (90000/30), got %d", i, delta)
	}
}

func TestConformance_AudioRTPTimestampSpacing48kHz20ms(t *testing.T) {
	frameDurationPTS := int64(1800)
	numFrames := 150

	var timestamps []uint32
	for i := 0; i < numFrames; i++ {
		pts := int64(i) * frameDurationPTS
		ts := ptsToRTP(pts, audioClockRate)
		timestamps = append(timestamps, ts)
	}

	for i := 1; i < numFrames; i++ {
		delta := timestamps[i] - timestamps[i-1]
		assert.Equal(t, uint32(960), delta,
			"audio frame %d: 48kHz Opus at 20ms must increment by exactly 960, got %d", i, delta)
	}
}

func TestConformance_AVSync5Seconds(t *testing.T) {
	videoFrameIntervalPTS := int64(3600)
	audioFrameDurationPTS := int64(1800)
	durationSec := 5.0

	totalVideoFrames := int(durationSec * 25)
	totalAudioFrames := int(durationSec * 50)

	lastVideoPTS := int64(totalVideoFrames-1) * videoFrameIntervalPTS
	lastAudioPTS := int64(totalAudioFrames-1) * audioFrameDurationPTS

	videoTimeSeconds := float64(ptsToRTP(lastVideoPTS, videoClockRate)) / float64(videoClockRate)
	audioTimeSeconds := float64(ptsToRTP(lastAudioPTS, audioClockRate)) / float64(audioClockRate)

	diff := math.Abs(videoTimeSeconds - audioTimeSeconds)
	oneVideoFrame := 1.0 / 25.0
	assert.Less(t, diff, oneVideoFrame,
		"A/V sync: video time (%.6fs) and audio time (%.6fs) must be within 1 video frame (%.4fs), diff=%.6fs",
		videoTimeSeconds, audioTimeSeconds, oneVideoFrame, diff)
}

func TestConformance_TotalDuration5Seconds125Frames(t *testing.T) {
	numFrames := 125
	frameIntervalPTS := int64(3600)

	lastPTS := int64(numFrames-1) * frameIntervalPTS
	lastRTPTS := ptsToRTP(lastPTS, videoClockRate)

	totalAdvance := lastRTPTS - ptsToRTP(0, videoClockRate)
	expectedAdvance := uint32((numFrames - 1) * 3600)
	assert.Equal(t, expectedAdvance, totalAdvance,
		"125 frames at 25fps: RTP timestamp advance must be %d (124 * 3600), got %d", expectedAdvance, totalAdvance)

	durationSeconds := float64(totalAdvance+3600) / float64(videoClockRate)
	assert.InDelta(t, 5.0, durationSeconds, 0.001,
		"125 frames at 25fps = exactly 5.0 seconds (450000 ticks at 90kHz)")
}

func TestConformance_RTPHeaderVersion(t *testing.T) {
	nalu := make([]byte, 500)
	nalu[0] = 0x65
	for _, pkt := range packetizeH264(nalu, 0, 0, maxRTPPayload, true) {
		assert.Equal(t, uint8(2), pkt.Header.Version,
			"RTP version must always be 2 per RFC 3550")
	}

	hevcNALU := make([]byte, 500)
	hevcNALU[0] = 0x26
	hevcNALU[1] = 0x01
	for _, pkt := range packetizeHEVC(hevcNALU, 0, 0, maxRTPPayload, true) {
		assert.Equal(t, uint8(2), pkt.Header.Version,
			"RTP version must always be 2 for HEVC packets")
	}
}

func TestConformance_SSRCStability(t *testing.T) {
	seq := uint16(0)
	for i := 0; i < 50; i++ {
		nalu := make([]byte, 200)
		nalu[0] = 0x65
		pkts := packetizeH264(nalu, seq, uint32(i*3600), maxRTPPayload, true)
		for _, pkt := range pkts {
			assert.Equal(t, uint32(videoSSRC), pkt.Header.SSRC,
				"video SSRC must remain stable across all packets")
		}
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}
}

func TestConformance_PayloadTypes(t *testing.T) {
	assert.Equal(t, uint8(96), uint8(videoPayloadType), "video payload type must be 96 (dynamic range)")
	assert.Equal(t, uint8(97), uint8(audioPayloadType), "audio payload type must be 97 (dynamic range)")
	assert.NotEqual(t, videoPayloadType, audioPayloadType, "video and audio payload types must differ")
}

func TestConformance_SequenceContinuityNoGaps(t *testing.T) {
	seq := uint16(0)
	var allPkts []*rtp.Packet

	for i := 0; i < 200; i++ {
		naluSize := 100
		if i%10 == 0 {
			naluSize = 5000
		}
		nalu := make([]byte, naluSize)
		nalu[0] = 0x65

		ts := uint32(i * 3600)
		pkts := packetizeH264(nalu, seq, ts, maxRTPPayload, true)
		allPkts = append(allPkts, pkts...)
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	for i := 1; i < len(allPkts); i++ {
		expected := allPkts[i-1].Header.SequenceNumber + 1
		assert.Equal(t, expected, allPkts[i].Header.SequenceNumber,
			"packet %d: no gaps allowed in sequence numbers (expected %d, got %d)",
			i, expected, allPkts[i].Header.SequenceNumber)
	}
}

func TestConformance_SequenceWraparound(t *testing.T) {
	startSeq := uint16(65530)
	seq := startSeq
	var allPkts []*rtp.Packet

	for i := 0; i < 20; i++ {
		nalu := make([]byte, 200)
		nalu[0] = 0x65
		pkts := packetizeH264(nalu, seq, uint32(i*3600), maxRTPPayload, true)
		allPkts = append(allPkts, pkts...)
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	for i := 1; i < len(allPkts); i++ {
		expected := allPkts[i-1].Header.SequenceNumber + 1
		assert.Equal(t, expected, allPkts[i].Header.SequenceNumber,
			"packet %d: uint16 wraparound must be natural", i)
	}

	assert.Less(t, allPkts[len(allPkts)-1].Header.SequenceNumber, startSeq,
		"sequence numbers should have wrapped past 65535")
}

func TestConformance_MarkerBitOnlyOnLastPacketOfFrame(t *testing.T) {
	sps := make([]byte, 20)
	sps[0] = 0x67
	pps := make([]byte, 10)
	pps[0] = 0x68
	idr := make([]byte, 5000)
	idr[0] = 0x65

	nalus := [][]byte{sps, pps, idr}
	seq := uint16(0)
	ts := uint32(90000)
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
	assert.Equal(t, 1, markerCount, "exactly one marker bit per frame (on last packet of last NALU)")
	assert.True(t, allPkts[len(allPkts)-1].Header.Marker, "marker must be on the very last packet")

	for i := 0; i < len(allPkts)-1; i++ {
		assert.False(t, allPkts[i].Header.Marker,
			"packet %d: marker must NOT be set on non-final packets", i)
	}
}

func TestConformance_H264FUAFragmentation5000Byte(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x65
	for i := 1; i < len(nalu); i++ {
		nalu[i] = byte(i % 256)
	}

	pkts := packetizeH264(nalu, 0, 90000, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1, "5000-byte NALU must be fragmented at MTU=1400")

	first := pkts[0]
	indicator := first.Payload[0]
	assert.Equal(t, byte(28), indicator&0x1f, "FU-A indicator type must be 28")
	assert.Equal(t, nalu[0]&0x60, indicator&0x60, "NRI bits must be preserved from original NALU")

	fuHeader := first.Payload[1]
	assert.True(t, fuHeader&0x80 != 0, "first fragment: Start=1")
	assert.True(t, fuHeader&0x40 == 0, "first fragment: End=0")
	assert.Equal(t, nalu[0]&0x1f, fuHeader&0x1f, "FU header NALU type matches original")

	for i := 1; i < len(pkts)-1; i++ {
		midFU := pkts[i].Payload[1]
		assert.True(t, midFU&0x80 == 0, "middle fragment %d: Start=0", i)
		assert.True(t, midFU&0x40 == 0, "middle fragment %d: End=0", i)
	}

	last := pkts[len(pkts)-1]
	lastFU := last.Payload[1]
	assert.True(t, lastFU&0x80 == 0, "last fragment: Start=0")
	assert.True(t, lastFU&0x40 != 0, "last fragment: End=1")
	assert.True(t, last.Header.Marker, "last fragment: Marker=1")

	var reassembled []byte
	reassembled = append(reassembled, pkts[0].Payload[1]&0x1f|pkts[0].Payload[0]&0x60)
	for _, pkt := range pkts {
		reassembled = append(reassembled, pkt.Payload[2:]...)
	}
	assert.Equal(t, nalu, reassembled, "reassembled FU-A fragments must exactly equal original NALU")
}

func TestConformance_H264FUAKnownIndicatorByte(t *testing.T) {
	nalu := make([]byte, 2000)
	nalu[0] = 0x65

	pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1)

	assert.Equal(t, byte(0x7C), pkts[0].Payload[0],
		"IDR (0x65): indicator must be 0x7C (NRI=0x60 | type=28)")
	assert.Equal(t, byte(0x85), pkts[0].Payload[1],
		"first FU header: Start=1 | type=5 = 0x85")
	assert.Equal(t, byte(0x45), pkts[len(pkts)-1].Payload[1],
		"last FU header: End=1 | type=5 = 0x45")
}

func TestConformance_HEVCFU3ByteHeader(t *testing.T) {
	nalu := make([]byte, 5000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	for i := 2; i < len(nalu); i++ {
		nalu[i] = byte(i % 256)
	}

	pkts := packetizeHEVC(nalu, 0, 90000, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1, "5000-byte HEVC NALU must be fragmented")

	naluType := (nalu[0] >> 1) & 0x3f
	expectedByte0 := byte((hevcNALUTypeFU << 1) | (nalu[0] & 0x81))

	for _, pkt := range pkts {
		assert.Len(t, pkt.Payload, len(pkt.Payload))
		assert.True(t, len(pkt.Payload) >= 3, "HEVC FU must have at least 3-byte header")
		assert.Equal(t, expectedByte0, pkt.Payload[0], "HEVC FU byte 0: type=49<<1 with preserved bits")
		assert.Equal(t, nalu[1], pkt.Payload[1], "HEVC FU byte 1: TID preserved")
		assert.Equal(t, naluType, pkt.Payload[2]&0x3f, "HEVC FU byte 2: NALU type in lower 6 bits")
	}

	first := pkts[0]
	assert.True(t, first.Payload[2]&0x80 != 0, "first HEVC FU: Start=1")
	assert.True(t, first.Payload[2]&0x40 == 0, "first HEVC FU: End=0")

	last := pkts[len(pkts)-1]
	assert.True(t, last.Payload[2]&0x80 == 0, "last HEVC FU: Start=0")
	assert.True(t, last.Payload[2]&0x40 != 0, "last HEVC FU: End=1")

	var reassembled []byte
	reassembled = append(reassembled, nalu[0], nalu[1])
	for _, pkt := range pkts {
		reassembled = append(reassembled, pkt.Payload[3:]...)
	}
	assert.Equal(t, nalu, reassembled, "reassembled HEVC FU fragments must exactly equal original NALU")
}

func TestConformance_HEVCLayerIDAndTIDPreserved(t *testing.T) {
	types := []struct {
		name    string
		byte0   byte
		byte1   byte
	}{
		{"IDR_W_RADL layerID=0 TID=1", 0x26, 0x01},
		{"TRAIL_R layerID=0 TID=1", 0x02, 0x01},
		{"CRA layerID=0 TID=1", 0x2A, 0x01},
		{"VPS layerID=0 TID=1", 0x40, 0x01},
		{"SPS layerID=0 TID=1", 0x42, 0x01},
		{"PPS layerID=0 TID=1", 0x44, 0x01},
	}

	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			nalu := make([]byte, 3000)
			nalu[0] = tt.byte0
			nalu[1] = tt.byte1
			pkts := packetizeHEVC(nalu, 0, 0, maxRTPPayload, true)
			require.Greater(t, len(pkts), 1)

			for _, pkt := range pkts {
				assert.Equal(t, tt.byte1, pkt.Payload[1],
					"TID byte must be preserved for %s", tt.name)
				assert.Equal(t, tt.byte0&0x81, pkt.Payload[0]&0x81,
					"F bit and LayerID MSB must be preserved for %s", tt.name)
			}
		})
	}
}

func TestConformance_ResetForSeekResetsEverything(t *testing.T) {
	p := makeTestPlugin(t, "h264", 25, 1)

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

	p.mu.Lock()
	assert.Equal(t, uint16(0), p.videoSeq, "videoSeq must reset to 0")
	assert.Equal(t, uint16(0), p.audioSeq, "audioSeq must reset to 0")
	assert.Equal(t, uint32(0), p.videoTS, "videoTS must reset to 0")
	assert.Equal(t, uint32(0), p.audioTS, "audioTS must reset to 0")
	assert.False(t, p.ptsBaseSet, "ptsBaseSet must be false so next packet recaptures")
	assert.Equal(t, int64(0), p.lastVideoPTS, "lastVideoPTS must reset to 0")
	assert.Equal(t, int64(0), p.lastAudioPTS, "lastAudioPTS must reset to 0")
	p.mu.Unlock()

	assert.Equal(t, genBefore+1, p.Generation(), "generation must increment by exactly 1")
}

func TestConformance_PostSeekTimestampContinuity(t *testing.T) {
	seekTargetPTS := int64(2700000)
	frameInterval := int64(3600)

	var timestamps []uint32
	for i := 0; i < 50; i++ {
		pts := seekTargetPTS + int64(i)*frameInterval
		ts := ptsToRTP(pts-seekTargetPTS, videoClockRate)
		timestamps = append(timestamps, ts)
	}

	assert.Equal(t, uint32(0), timestamps[0],
		"first frame after seek must have RTP timestamp 0 (relative to new PTS base)")

	for i := 1; i < len(timestamps); i++ {
		delta := timestamps[i] - timestamps[i-1]
		assert.Equal(t, uint32(3600), delta,
			"post-seek frame %d: RTP timestamp spacing must be exactly 3600 at 25fps", i)
	}
}

func TestConformance_NoStalePacketsAfterSeek(t *testing.T) {
	preSeekFrames := 10
	postSeekFrames := 10
	frameInterval := int64(3600)

	preSeqStart := uint16(0)
	seq := preSeqStart
	prePktCount := 0
	for i := 0; i < preSeekFrames; i++ {
		nalu := make([]byte, 200)
		nalu[0] = 0x65
		pkts := packetizeH264(nalu, seq, uint32(i*3600), maxRTPPayload, true)
		prePktCount += len(pkts)
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	seq = 0
	postPktCount := 0
	for i := 0; i < postSeekFrames; i++ {
		nalu := make([]byte, 200)
		nalu[0] = 0x65
		pts := int64(i) * frameInterval
		pkts := packetizeH264(nalu, seq, ptsToRTP(pts, videoClockRate), maxRTPPayload, true)
		postPktCount += len(pkts)
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	assert.Equal(t, preSeekFrames, prePktCount,
		"pre-seek: small NALUs produce 1 packet each = %d packets", preSeekFrames)
	assert.Equal(t, postSeekFrames, postPktCount,
		"post-seek: small NALUs produce 1 packet each = %d packets", postSeekFrames)
	assert.Equal(t, preSeekFrames+postSeekFrames, prePktCount+postPktCount,
		"total packets = pre + post, no duplicates from stale state")
}

func TestConformance_EndToEnd125Video250Audio5Seconds(t *testing.T) {
	numVideoFrames := 125
	numAudioFrames := 250
	videoIntervalPTS := int64(3600)
	audioIntervalPTS := int64(1800)

	videoSeq := uint16(0)
	var videoPackets []*rtp.Packet
	for i := 0; i < numVideoFrames; i++ {
		naluSize := 200
		if i%25 == 0 {
			naluSize = 5000
		}
		nalu := make([]byte, naluSize)
		nalu[0] = 0x65
		for j := 1; j < len(nalu); j++ {
			nalu[j] = byte(j % 256)
		}

		pts := int64(i) * videoIntervalPTS
		ts := ptsToRTP(pts, videoClockRate)
		pkts := packetizeH264(nalu, videoSeq, ts, maxRTPPayload, true)
		videoPackets = append(videoPackets, pkts...)
		videoSeq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	audioSeq := uint16(0)
	var audioTimestamps []uint32
	for i := 0; i < numAudioFrames; i++ {
		pts := int64(i) * audioIntervalPTS
		ts := ptsToRTP(pts, audioClockRate)
		audioTimestamps = append(audioTimestamps, ts)
		audioSeq++
	}

	videoAdvance := videoPackets[len(videoPackets)-1].Header.Timestamp - videoPackets[0].Header.Timestamp
	expectedVideoAdvance := uint32((numVideoFrames - 1) * 3600)
	assert.Equal(t, expectedVideoAdvance, videoAdvance,
		"video: timestamp advance over 125 frames must be %d (124*3600)", expectedVideoAdvance)

	videoSeconds := float64(videoAdvance+3600) / float64(videoClockRate)
	assert.InDelta(t, 5.0, videoSeconds, 0.001,
		"video: 125 frames at 25fps = 5.0 seconds")

	audioAdvance := audioTimestamps[len(audioTimestamps)-1] - audioTimestamps[0]
	expectedAudioAdvance := uint32((numAudioFrames - 1) * 960)
	assert.Equal(t, expectedAudioAdvance, audioAdvance,
		"audio: timestamp advance over 250 frames must be %d (249*960)", expectedAudioAdvance)

	audioSeconds := float64(audioAdvance+960) / float64(audioClockRate)
	assert.InDelta(t, 5.0, audioSeconds, 0.021,
		"audio: 250 frames at 20ms = 5.0 seconds")

	for i := 1; i < len(videoPackets); i++ {
		expected := videoPackets[i-1].Header.SequenceNumber + 1
		assert.Equal(t, expected, videoPackets[i].Header.SequenceNumber,
			"video packet %d: sequence continuity violated", i)
	}

	for i := 1; i < len(audioTimestamps); i++ {
		delta := audioTimestamps[i] - audioTimestamps[i-1]
		assert.Equal(t, uint32(960), delta,
			"audio frame %d: timestamp delta must be 960", i)
	}

	videoMarkers := 0
	for _, pkt := range videoPackets {
		if pkt.Header.Marker {
			videoMarkers++
		}
	}
	assert.Equal(t, numVideoFrames, videoMarkers,
		"marker count must equal video frame count (one marker per frame)")

	for _, pkt := range videoPackets {
		assert.Equal(t, uint8(2), pkt.Header.Version)
		assert.Equal(t, uint8(videoPayloadType), pkt.Header.PayloadType)
		assert.Equal(t, uint32(videoSSRC), pkt.Header.SSRC)
	}
}

func TestConformance_InterlacedSource50i_25pDeinterlaced(t *testing.T) {
	numFrames := 150
	frameIntervalPTS := int64(3600)

	var timestamps []uint32
	for i := 0; i < numFrames; i++ {
		pts := int64(i) * frameIntervalPTS
		ts := ptsToRTP(pts, videoClockRate)
		timestamps = append(timestamps, ts)
	}

	for i := 1; i < numFrames; i++ {
		delta := timestamps[i] - timestamps[i-1]
		assert.Equal(t, uint32(3600), delta,
			"frame %d: 50i deinterlaced to 25p must use 3600 RTP tick spacing (25fps), NOT 1800 (50fps). "+
				"Got %d. Using 50fps spacing would play at 2x speed.", i, delta)
	}

	wrongSpacingCount := 0
	for i := 1; i < numFrames; i++ {
		delta := timestamps[i] - timestamps[i-1]
		if delta == 1800 {
			wrongSpacingCount++
		}
	}
	assert.Equal(t, 0, wrongSpacingCount,
		"no frames should use 1800 tick spacing (50fps); that causes the 50%% speed bug")
}

func TestConformance_H264FragmentationPayloadSizes(t *testing.T) {
	nalu := make([]byte, 10000)
	nalu[0] = 0x65
	for i := 1; i < len(nalu); i++ {
		nalu[i] = byte(i % 256)
	}

	pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1)

	for i := 0; i < len(pkts)-1; i++ {
		assert.LessOrEqual(t, len(pkts[i].Payload), maxRTPPayload,
			"fragment %d: payload must not exceed MTU (%d bytes)", i, maxRTPPayload)
	}

	assert.Equal(t, maxRTPPayload, len(pkts[0].Payload),
		"first fragment should fill MTU exactly")

	assert.LessOrEqual(t, len(pkts[len(pkts)-1].Payload), maxRTPPayload,
		"last fragment must not exceed MTU")

	dataBytes := len(nalu) - 1
	payloadPerFragment := maxRTPPayload - 2
	expectedFragments := int(math.Ceil(float64(dataBytes) / float64(payloadPerFragment)))
	assert.Equal(t, expectedFragments, len(pkts),
		"H.264 FU-A fragment count: ceil(%d / %d) = %d", dataBytes, payloadPerFragment, expectedFragments)
}

func TestConformance_HEVCFragmentationPayloadSizes(t *testing.T) {
	nalu := make([]byte, 10000)
	nalu[0] = 0x26
	nalu[1] = 0x01
	for i := 2; i < len(nalu); i++ {
		nalu[i] = byte(i % 256)
	}

	pkts := packetizeHEVC(nalu, 0, 0, maxRTPPayload, true)
	require.Greater(t, len(pkts), 1)

	for i := 0; i < len(pkts)-1; i++ {
		assert.LessOrEqual(t, len(pkts[i].Payload), maxRTPPayload,
			"HEVC fragment %d: payload must not exceed MTU", i)
	}

	assert.Equal(t, maxRTPPayload, len(pkts[0].Payload),
		"first HEVC fragment should fill MTU exactly")

	dataBytes := len(nalu) - 2
	payloadPerFragment := maxRTPPayload - 3
	expectedFragments := int(math.Ceil(float64(dataBytes) / float64(payloadPerFragment)))
	assert.Equal(t, expectedFragments, len(pkts),
		"HEVC FU fragment count: ceil(%d / %d) = %d", dataBytes, payloadPerFragment, expectedFragments)
}

func TestConformance_PTSToRTPConversion(t *testing.T) {
	tests := []struct {
		name      string
		pts       int64
		clockRate uint32
		expected  uint32
	}{
		{"video 0s", 0, videoClockRate, 0},
		{"video 1s", 90000, videoClockRate, 90000},
		{"video 40ms (1 frame at 25fps)", 3600, videoClockRate, 3600},
		{"video 33.33ms (1 frame at 30fps)", 3000, videoClockRate, 3000},
		{"video 10s", 900000, videoClockRate, 900000},
		{"video 60s", 5400000, videoClockRate, 5400000},
		{"audio 0s", 0, audioClockRate, 0},
		{"audio 1s", 90000, audioClockRate, 48000},
		{"audio 20ms", 1800, audioClockRate, 960},
		{"audio 10s", 900000, audioClockRate, 480000},
		{"audio 60s", 5400000, audioClockRate, 2880000},
		{"negative clamps to 0", -5000, videoClockRate, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ptsToRTP(tt.pts, tt.clockRate)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConformance_ClockRates(t *testing.T) {
	assert.Equal(t, uint32(90000), uint32(videoClockRate),
		"video clock rate must be 90kHz per RTP standard")
	assert.Equal(t, uint32(48000), uint32(audioClockRate),
		"audio clock rate must be 48kHz for Opus")
}

func TestConformance_MultiSeekGenerationMonotonic(t *testing.T) {
	p := makeTestPlugin(t, "h264", 25, 1)

	var gens []int64
	gens = append(gens, p.Generation())

	for i := 0; i < 20; i++ {
		p.ResetForSeek()
		gens = append(gens, p.Generation())
	}

	for i := 1; i < len(gens); i++ {
		assert.Equal(t, gens[i-1]+1, gens[i],
			"seek %d: generation must increment by exactly 1 each time", i)
	}
}

func TestConformance_TimestampMonotonicity1000Frames(t *testing.T) {
	var prevVideoTS uint32
	for i := 0; i < 1000; i++ {
		pts := int64(i) * 3600
		ts := ptsToRTP(pts, videoClockRate)
		assert.GreaterOrEqual(t, ts, prevVideoTS,
			"video frame %d: timestamp must be monotonically non-decreasing", i)
		if i > 0 {
			assert.Greater(t, ts, prevVideoTS,
				"video frame %d: with constant interval, timestamps must strictly increase", i)
		}
		prevVideoTS = ts
	}

	var prevAudioTS uint32
	for i := 0; i < 1000; i++ {
		pts := int64(i) * 1800
		ts := ptsToRTP(pts, audioClockRate)
		assert.GreaterOrEqual(t, ts, prevAudioTS,
			"audio frame %d: timestamp must be monotonically non-decreasing", i)
		if i > 0 {
			assert.Greater(t, ts, prevAudioTS,
				"audio frame %d: with constant interval, timestamps must strictly increase", i)
		}
		prevAudioTS = ts
	}
}

func TestConformance_AnnexBAndAVCCBothWork(t *testing.T) {
	annexB := []byte{0, 0, 0, 1, 0x67, 0x42, 0, 0, 0, 1, 0x68, 0xCE, 0, 0, 0, 1, 0x65, 0xFF}
	nalus := splitNALUs(annexB)
	require.Len(t, nalus, 3, "Annex B: SPS+PPS+IDR must produce 3 NALUs")
	assert.Equal(t, byte(0x67), nalus[0][0])
	assert.Equal(t, byte(0x68), nalus[1][0])
	assert.Equal(t, byte(0x65), nalus[2][0])

	avcc := []byte{
		0, 0, 0, 2, 0x67, 0x42,
		0, 0, 0, 1, 0x68,
		0, 0, 0, 1, 0x65,
	}
	nalus2 := splitNALUs(avcc)
	require.Len(t, nalus2, 3, "AVCC: 3 NALUs with length-prefixed format")
	assert.Equal(t, byte(0x67), nalus2[0][0])
	assert.Equal(t, byte(0x68), nalus2[1][0])
	assert.Equal(t, byte(0x65), nalus2[2][0])
}

func TestConformance_LargeFrameCount500Frames(t *testing.T) {
	numFrames := 500
	frameInterval := int64(3600)
	seq := uint16(0)
	var allPkts []*rtp.Packet
	markerCount := 0

	for i := 0; i < numFrames; i++ {
		naluSize := 200
		if i%50 == 0 {
			naluSize = 8000
		}
		nalu := make([]byte, naluSize)
		nalu[0] = 0x65

		ts := ptsToRTP(int64(i)*frameInterval, videoClockRate)
		pkts := packetizeH264(nalu, seq, ts, maxRTPPayload, true)
		for _, pkt := range pkts {
			if pkt.Header.Marker {
				markerCount++
			}
		}
		allPkts = append(allPkts, pkts...)
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	assert.Equal(t, numFrames, markerCount, "500 frames must produce exactly 500 marker bits")

	for i := 1; i < len(allPkts); i++ {
		expected := allPkts[i-1].Header.SequenceNumber + 1
		assert.Equal(t, expected, allPkts[i].Header.SequenceNumber,
			"packet %d/%d: sequence continuity over 500 frames", i, len(allPkts))
	}

	lastVideoTS := allPkts[len(allPkts)-1].Header.Timestamp
	expectedLast := ptsToRTP(int64(numFrames-1)*frameInterval, videoClockRate)
	assert.Equal(t, expectedLast, lastVideoTS,
		"last frame timestamp must match expected PTS-derived value")
}
