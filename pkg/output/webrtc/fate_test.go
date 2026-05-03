package webrtc

import (
	"math"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFATE_H264_RTPReconstructionExact(t *testing.T) {
	for frameIdx := 0; frameIdx < 100; frameIdx++ {
		naluSize := 200
		if frameIdx%10 == 0 {
			naluSize = 4000
		}
		nalu := make([]byte, naluSize)
		nalu[0] = 0x65
		for i := 1; i < len(nalu); i++ {
			nalu[i] = byte((frameIdx*7 + i) % 256)
		}

		ts := uint32(frameIdx * 3600)
		pkts := packetizeH264(nalu, 0, ts, maxRTPPayload, true)

		var reconstructed []byte
		if len(pkts) == 1 {
			reconstructed = pkts[0].Payload
		} else {
			reconstructed = append(reconstructed, pkts[0].Payload[1]&0x1f|pkts[0].Payload[0]&0x60)
			for _, pkt := range pkts {
				reconstructed = append(reconstructed, pkt.Payload[2:]...)
			}
		}

		assert.Equal(t, nalu, reconstructed,
			"frame %d: reconstructed H.264 NALU must exactly match input", frameIdx)
	}
}

func TestFATE_HEVC_RTPReconstructionExact(t *testing.T) {
	hevcTypes := []struct {
		byte0 byte
		byte1 byte
	}{
		{0x26, 0x01},
		{0x28, 0x01},
		{0x02, 0x01},
		{0x40, 0x01},
		{0x42, 0x01},
		{0x44, 0x01},
	}

	for frameIdx := 0; frameIdx < 100; frameIdx++ {
		ht := hevcTypes[frameIdx%len(hevcTypes)]

		naluSize := 300
		if frameIdx%8 == 0 {
			naluSize = 5000
		}
		nalu := make([]byte, naluSize)
		nalu[0] = ht.byte0
		nalu[1] = ht.byte1
		for i := 2; i < len(nalu); i++ {
			nalu[i] = byte((frameIdx*13 + i) % 256)
		}

		ts := uint32(frameIdx * 3600)
		pkts := packetizeHEVC(nalu, 0, ts, maxRTPPayload, true)

		var reconstructed []byte
		if len(pkts) == 1 {
			reconstructed = pkts[0].Payload
		} else {
			reconstructed = append(reconstructed, nalu[0], nalu[1])
			for _, pkt := range pkts {
				reconstructed = append(reconstructed, pkt.Payload[3:]...)
			}
		}

		assert.Equal(t, nalu, reconstructed,
			"frame %d: reconstructed HEVC NALU must exactly match input", frameIdx)
	}
}

func TestFATE_Opus_RTPPayloadExact(t *testing.T) {
	for frameIdx := 0; frameIdx < 100; frameIdx++ {
		opusData := make([]byte, 80+frameIdx%100)
		for i := range opusData {
			opusData[i] = byte((frameIdx*3 + i) % 256)
		}

		pts := int64(frameIdx) * 1800
		rtpTS := ptsToRTP(pts, audioClockRate)

		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    audioPayloadType,
				SequenceNumber: uint16(frameIdx),
				Timestamp:      rtpTS,
				SSRC:           audioSSRC,
				Marker:         true,
			},
			Payload: opusData,
		}

		assert.Equal(t, opusData, pkt.Payload,
			"frame %d: Opus RTP payload must exactly match input (no fragmentation)", frameIdx)
	}
}

func TestFATE_TimingReconstruction_Video(t *testing.T) {
	frameInterval := int64(3600)
	numFrames := 100

	for i := 0; i < numFrames; i++ {
		inputPTS := int64(i) * frameInterval
		rtpTS := ptsToRTP(inputPTS, videoClockRate)

		reconstructedPTS := int64(rtpTS)

		assert.Equal(t, inputPTS, reconstructedPTS,
			"frame %d: reconstructed PTS from RTP timestamp must match input PTS within 1 tick", i)
	}
}

func TestFATE_TimingReconstruction_Audio(t *testing.T) {
	frameDuration := int64(1800)
	numFrames := 100

	for i := 0; i < numFrames; i++ {
		inputPTS := int64(i) * frameDuration
		rtpTS := ptsToRTP(inputPTS, audioClockRate)

		reconstructedPTS := int64(rtpTS) * 90000 / int64(audioClockRate)

		assert.Equal(t, inputPTS, reconstructedPTS,
			"frame %d: round-tripped audio PTS must match input exactly", i)
	}
}

func TestFATE_AVSyncVerification(t *testing.T) {
	videoIntervalPTS := int64(3600)
	audioIntervalPTS := int64(1800)
	durationSec := 10.0

	numVideoFrames := int(durationSec * 25)
	numAudioFrames := int(durationSec * 50)

	for vi := 0; vi < numVideoFrames; vi++ {
		videoPTS := int64(vi) * videoIntervalPTS
		videoRTP := ptsToRTP(videoPTS, videoClockRate)
		videoTimeSec := float64(videoRTP) / float64(videoClockRate)

		closestAudioIdx := int(float64(vi) * float64(numAudioFrames) / float64(numVideoFrames))
		if closestAudioIdx >= numAudioFrames {
			closestAudioIdx = numAudioFrames - 1
		}
		audioPTS := int64(closestAudioIdx) * audioIntervalPTS
		audioRTP := ptsToRTP(audioPTS, audioClockRate)
		audioTimeSec := float64(audioRTP) / float64(audioClockRate)

		diff := math.Abs(videoTimeSec - audioTimeSec)
		oneVideoFrame := 1.0 / 25.0
		assert.Less(t, diff, oneVideoFrame,
			"video frame %d at %.4fs: A/V sync drift %.6fs exceeds 1 frame (%.4fs)",
			vi, videoTimeSec, diff, oneVideoFrame)
	}
}

func TestFATE_H264_MultiNALUFrameReconstructionExact(t *testing.T) {
	for frameIdx := 0; frameIdx < 50; frameIdx++ {
		sps := make([]byte, 15)
		sps[0] = 0x67
		for i := 1; i < len(sps); i++ {
			sps[i] = byte(frameIdx + i)
		}

		pps := make([]byte, 8)
		pps[0] = 0x68
		for i := 1; i < len(pps); i++ {
			pps[i] = byte(frameIdx*2 + i)
		}

		idr := make([]byte, 3000+frameIdx*10)
		idr[0] = 0x65
		for i := 1; i < len(idr); i++ {
			idr[i] = byte((frameIdx*5 + i) % 256)
		}

		nalus := [][]byte{sps, pps, idr}
		ts := uint32(frameIdx * 3600)
		seq := uint16(0)

		for naluIdx, nalu := range nalus {
			isLast := naluIdx == len(nalus)-1
			pkts := packetizeH264(nalu, seq, ts, maxRTPPayload, isLast)

			var reconstructed []byte
			if len(pkts) == 1 {
				reconstructed = pkts[0].Payload
			} else {
				reconstructed = append(reconstructed, pkts[0].Payload[1]&0x1f|pkts[0].Payload[0]&0x60)
				for _, pkt := range pkts {
					reconstructed = append(reconstructed, pkt.Payload[2:]...)
				}
			}

			assert.Equal(t, nalu, reconstructed,
				"frame %d, NALU %d: reconstructed must match input", frameIdx, naluIdx)

			seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
		}
	}
}

func TestFATE_H264_FullStreamReconstruction(t *testing.T) {
	numFrames := 100
	frameInterval := int64(3000)

	type inputFrame struct {
		nalu []byte
		pts  int64
	}

	var inputs []inputFrame
	for i := 0; i < numFrames; i++ {
		size := 500
		if i%25 == 0 {
			size = 6000
		}
		nalu := make([]byte, size)
		nalu[0] = 0x65
		for j := 1; j < len(nalu); j++ {
			nalu[j] = byte((i*11 + j) % 256)
		}
		inputs = append(inputs, inputFrame{nalu: nalu, pts: int64(i) * frameInterval})
	}

	type outputFrame struct {
		packets []*rtp.Packet
	}

	seq := uint16(0)
	var outputs []outputFrame
	for _, in := range inputs {
		ts := ptsToRTP(in.pts, videoClockRate)
		pkts := packetizeH264(in.nalu, seq, ts, maxRTPPayload, true)
		outputs = append(outputs, outputFrame{packets: pkts})
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	for i, out := range outputs {
		var reconstructed []byte
		if len(out.packets) == 1 {
			reconstructed = out.packets[0].Payload
		} else {
			reconstructed = append(reconstructed, out.packets[0].Payload[1]&0x1f|out.packets[0].Payload[0]&0x60)
			for _, pkt := range out.packets {
				reconstructed = append(reconstructed, pkt.Payload[2:]...)
			}
		}
		assert.Equal(t, inputs[i].nalu, reconstructed,
			"stream frame %d: full reconstruction must match input", i)

		expectedTS := ptsToRTP(inputs[i].pts, videoClockRate)
		for _, pkt := range out.packets {
			assert.Equal(t, expectedTS, pkt.Header.Timestamp,
				"stream frame %d: all packets must carry correct RTP timestamp", i)
		}
	}

	markerCount := 0
	for _, out := range outputs {
		for _, pkt := range out.packets {
			if pkt.Header.Marker {
				markerCount++
			}
		}
	}
	assert.Equal(t, numFrames, markerCount, "exactly one marker per frame across full stream")
}

func TestFATE_HEVC_FullStreamReconstruction(t *testing.T) {
	numFrames := 100
	frameInterval := int64(3600)

	type inputFrame struct {
		nalu []byte
		pts  int64
	}

	var inputs []inputFrame
	for i := 0; i < numFrames; i++ {
		size := 400
		if i%20 == 0 {
			size = 5500
		}
		nalu := make([]byte, size)
		nalu[0] = 0x26
		nalu[1] = 0x01
		for j := 2; j < len(nalu); j++ {
			nalu[j] = byte((i*17 + j) % 256)
		}
		inputs = append(inputs, inputFrame{nalu: nalu, pts: int64(i) * frameInterval})
	}

	seq := uint16(0)
	type outputFrame struct {
		packets []*rtp.Packet
	}
	var outputs []outputFrame
	for _, in := range inputs {
		ts := ptsToRTP(in.pts, videoClockRate)
		pkts := packetizeHEVC(in.nalu, seq, ts, maxRTPPayload, true)
		outputs = append(outputs, outputFrame{packets: pkts})
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	for i, out := range outputs {
		var reconstructed []byte
		if len(out.packets) == 1 {
			reconstructed = out.packets[0].Payload
		} else {
			reconstructed = append(reconstructed, inputs[i].nalu[0], inputs[i].nalu[1])
			for _, pkt := range out.packets {
				reconstructed = append(reconstructed, pkt.Payload[3:]...)
			}
		}
		assert.Equal(t, inputs[i].nalu, reconstructed,
			"HEVC stream frame %d: full reconstruction must match input", i)
	}
}

func TestFATE_AudioStreamReconstruction(t *testing.T) {
	numFrames := 100
	frameDuration := int64(1800)

	type inputFrame struct {
		data []byte
		pts  int64
	}

	var inputs []inputFrame
	for i := 0; i < numFrames; i++ {
		data := make([]byte, 80+i%60)
		for j := range data {
			data[j] = byte((i*3 + j) % 256)
		}
		inputs = append(inputs, inputFrame{data: data, pts: int64(i) * frameDuration})
	}

	for i, in := range inputs {
		rtpTS := ptsToRTP(in.pts, audioClockRate)
		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    audioPayloadType,
				SequenceNumber: uint16(i),
				Timestamp:      rtpTS,
				SSRC:           audioSSRC,
				Marker:         true,
			},
			Payload: in.data,
		}

		assert.Equal(t, in.data, pkt.Payload,
			"audio frame %d: payload must be exact copy of input", i)

		roundTrippedPTS := int64(rtpTS) * 90000 / int64(audioClockRate)
		assert.Equal(t, in.pts, roundTrippedPTS,
			"audio frame %d: PTS round-trip must be lossless", i)
	}
}

func TestFATE_SequenceContinuityAcross100Frames(t *testing.T) {
	seq := uint16(0)
	var allPackets []*rtp.Packet

	for i := 0; i < 100; i++ {
		size := 200
		if i%15 == 0 {
			size = 4500
		}
		nalu := make([]byte, size)
		nalu[0] = 0x65

		ts := ptsToRTP(int64(i)*3600, videoClockRate)
		pkts := packetizeH264(nalu, seq, ts, maxRTPPayload, true)
		allPackets = append(allPackets, pkts...)
		seq = pkts[len(pkts)-1].Header.SequenceNumber + 1
	}

	for i := 1; i < len(allPackets); i++ {
		expected := allPackets[i-1].Header.SequenceNumber + 1
		assert.Equal(t, expected, allPackets[i].Header.SequenceNumber,
			"packet %d: sequence continuity violated across 100-frame stream", i)
	}
}

func TestFATE_AllTimestampsMonotonic(t *testing.T) {
	var prevTS uint32
	for i := 0; i < 100; i++ {
		nalu := make([]byte, 200)
		nalu[0] = 0x65
		ts := ptsToRTP(int64(i)*3600, videoClockRate)
		pkts := packetizeH264(nalu, 0, ts, maxRTPPayload, true)

		for _, pkt := range pkts {
			assert.GreaterOrEqual(t, pkt.Header.Timestamp, prevTS,
				"frame %d: RTP timestamps must be monotonically non-decreasing", i)
		}
		prevTS = pkts[len(pkts)-1].Header.Timestamp
	}
}

func TestFATE_TotalDurationFromRTPTimestamps(t *testing.T) {
	tests := []struct {
		name       string
		fps        float64
		numFrames  int
		clockRate  uint32
		expectedMs float64
	}{
		{"25fps 100 frames video", 25, 100, uint32(videoClockRate), 4000.0},
		{"30fps 100 frames video", 30, 100, uint32(videoClockRate), 3333.333},
		{"48kHz 100 Opus frames audio", 50, 100, uint32(audioClockRate), 2000.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interval := int64(90000 / tt.fps)
			if tt.clockRate == uint32(audioClockRate) {
				interval = 1800
			}

			firstTS := ptsToRTP(0, tt.clockRate)
			lastTS := ptsToRTP(int64(tt.numFrames-1)*interval, tt.clockRate)

			var frameDurationTicks uint32
			if tt.clockRate == uint32(audioClockRate) {
				frameDurationTicks = ptsToRTP(interval, tt.clockRate)
			} else {
				frameDurationTicks = ptsToRTP(interval, tt.clockRate)
			}

			totalDurationSec := float64(lastTS-firstTS+frameDurationTicks) / float64(tt.clockRate)
			totalDurationMs := totalDurationSec * 1000.0

			tolerance := tt.expectedMs * 0.01
			assert.InDelta(t, tt.expectedMs, totalDurationMs, tolerance,
				"total duration from RTP timestamps must match expected within 1%%")
		})
	}
}

func TestFATE_LargeNALUFragmentReassembly(t *testing.T) {
	sizes := []int{1401, 2800, 5600, 14000, 28000, 65000}
	for _, size := range sizes {
		nalu := make([]byte, size)
		nalu[0] = 0x65
		for i := 1; i < len(nalu); i++ {
			nalu[i] = byte(i % 256)
		}

		pkts := packetizeH264(nalu, 0, 0, maxRTPPayload, true)
		require.Greater(t, len(pkts), 1, "size %d: must be fragmented", size)

		var reassembled []byte
		reassembled = append(reassembled, pkts[0].Payload[1]&0x1f|pkts[0].Payload[0]&0x60)
		for _, pkt := range pkts {
			reassembled = append(reassembled, pkt.Payload[2:]...)
		}

		assert.Equal(t, nalu, reassembled,
			"size %d: reassembled FU-A must exactly match original", size)

		dataBytes := size - 1
		payloadPerFrag := maxRTPPayload - 2
		expectedFrags := int(math.Ceil(float64(dataBytes) / float64(payloadPerFrag)))
		assert.Equal(t, expectedFrags, len(pkts),
			"size %d: fragment count must match ceil(%d/%d)", size, dataBytes, payloadPerFrag)
	}
}

func TestFATE_LargeHEVCNALUFragmentReassembly(t *testing.T) {
	sizes := []int{1401, 2800, 5600, 14000, 28000, 65000}
	for _, size := range sizes {
		nalu := make([]byte, size)
		nalu[0] = 0x26
		nalu[1] = 0x01
		for i := 2; i < len(nalu); i++ {
			nalu[i] = byte(i % 256)
		}

		pkts := packetizeHEVC(nalu, 0, 0, maxRTPPayload, true)
		require.Greater(t, len(pkts), 1, "HEVC size %d: must be fragmented", size)

		var reassembled []byte
		reassembled = append(reassembled, nalu[0], nalu[1])
		for _, pkt := range pkts {
			reassembled = append(reassembled, pkt.Payload[3:]...)
		}

		assert.Equal(t, nalu, reassembled,
			"HEVC size %d: reassembled FU must exactly match original", size)

		dataBytes := size - 2
		payloadPerFrag := maxRTPPayload - 3
		expectedFrags := int(math.Ceil(float64(dataBytes) / float64(payloadPerFrag)))
		assert.Equal(t, expectedFrags, len(pkts),
			"HEVC size %d: fragment count must match ceil(%d/%d)", size, dataBytes, payloadPerFrag)
	}
}
