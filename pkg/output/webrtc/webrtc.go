package webrtc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"
)

const (
	videoClockRate = 90000
	audioClockRate = 48000

	videoPayloadType = 96
	audioPayloadType = 97

	videoSSRC = 1
	audioSSRC = 2

	maxRTPPayload = 1400

	h264NALUTypeFUA  = 28
	hevcNALUTypeFU   = 49
	hevcNALUTypeAPTr = 48
)

type Plugin struct {
	cfg output.PluginConfig
	log zerolog.Logger

	mu         sync.Mutex
	pc         *webrtc.PeerConnection
	videoTrack *webrtc.TrackLocalStaticRTP
	audioTrack *webrtc.TrackLocalStaticRTP

	videoSeq uint16
	audioSeq uint16
	videoTS  uint32
	audioTS  uint32

	videoCodec  string
	videoFPS    float64
	lastVideoPTS int64
	lastAudioPTS int64
	ptsBaseVideo int64
	ptsBaseAudio int64
	ptsBaseSet   bool

	generation   atomic.Int64
	stopped      atomic.Bool
	ready        atomic.Bool
	bytesWritten int64
}

func New(cfg output.PluginConfig) (output.OutputPlugin, error) {
	log := zerolog.New(os.Stderr).With().Str("plugin", "webrtc").Logger()

	codec := "h264"
	fps := 30.0
	if cfg.Video != nil {
		if cfg.Video.Codec != "" {
			codec = strings.ToLower(cfg.Video.Codec)
		}
		if cfg.Video.FPS() > 0 {
			fps = cfg.Video.FPS()
		}
	}

	p := &Plugin{
		cfg:        cfg,
		log:        log,
		videoCodec: codec,
		videoFPS:   fps,
	}
	p.generation.Store(1)

	log.Info().Str("video_codec", codec).Float64("fps", fps).Msg("webrtc plugin created")

	return p, nil
}

func (p *Plugin) Mode() output.DeliveryMode {
	return output.DeliveryWebRTC
}

func (p *Plugin) PushVideo(data []byte, pts, dts, _ int64, keyframe bool) error {
	if p.stopped.Load() {
		return nil
	}

	p.mu.Lock()
	track := p.videoTrack
	if track == nil {
		p.mu.Unlock()
		return nil
	}

	if !p.ptsBaseSet {
		p.ptsBaseVideo = pts
		p.ptsBaseAudio = pts
		p.ptsBaseSet = true
		p.log.Info().Int("data_len", len(data)).Bool("keyframe", keyframe).Msg("webrtc: first video to track")
	}
	p.lastVideoPTS = pts

	relativePTS := pts - p.ptsBaseVideo
	rtpTS := nanosToRTP(relativePTS, videoClockRate)


	nalus := splitNALUs(data)
	for i, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}
		isLastNALU := i == len(nalus)-1

		var packets []*rtp.Packet
		if p.videoCodec == "hevc" || p.videoCodec == "h265" {
			packets = packetizeHEVC(nalu, p.videoSeq, rtpTS, maxRTPPayload, isLastNALU)
		} else {
			packets = packetizeH264(nalu, p.videoSeq, rtpTS, maxRTPPayload, isLastNALU)
		}

		for _, pkt := range packets {
			p.videoSeq = pkt.Header.SequenceNumber + 1
			if err := track.WriteRTP(pkt); err != nil {
				p.mu.Unlock()
				return nil
			}
			p.bytesWritten += int64(len(pkt.Payload))
		}
	}

	p.mu.Unlock()
	return nil
}

func (p *Plugin) PushAudio(data []byte, pts, dts, _ int64) error {
	if p.stopped.Load() {
		return nil
	}

	p.mu.Lock()
	track := p.audioTrack
	if track == nil {
		p.mu.Unlock()
		return nil
	}

	if !p.ptsBaseSet {
		p.ptsBaseVideo = pts
		p.ptsBaseAudio = pts
		p.ptsBaseSet = true
	}
	p.lastAudioPTS = pts

	rtpTS := nanosToRTP(pts-p.ptsBaseAudio, audioClockRate)

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    audioPayloadType,
			SequenceNumber: p.audioSeq,
			Timestamp:      rtpTS,
			SSRC:           audioSSRC,
			Marker:         true,
		},
		Payload: data,
	}
	p.audioSeq++

	if err := track.WriteRTP(pkt); err != nil {
		p.mu.Unlock()
		return nil
	}
	p.bytesWritten += int64(len(data))
	p.mu.Unlock()
	return nil
}

func (p *Plugin) PushSubtitle(_ []byte, _ int64, _ int64) error {
	return nil
}

func (p *Plugin) EndOfStream() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pc != nil {
		p.pc.Close()
		p.pc = nil
	}
}

func (p *Plugin) ResetForSeek() {
	p.mu.Lock()
	p.videoSeq = 0
	p.audioSeq = 0
	p.videoTS = 0
	p.audioTS = 0
	p.ptsBaseSet = false
	p.lastVideoPTS = 0
	p.lastAudioPTS = 0
	p.mu.Unlock()
	p.generation.Add(1)
}

func (p *Plugin) Stop() {
	if p.stopped.Swap(true) {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pc != nil {
		p.pc.Close()
		p.pc = nil
	}
	p.videoTrack = nil
	p.audioTrack = nil
}

func (p *Plugin) Status() output.PluginStatus {
	p.mu.Lock()
	bw := p.bytesWritten
	connected := p.pc != nil
	p.mu.Unlock()

	return output.PluginStatus{
		Mode:         output.DeliveryWebRTC,
		BytesWritten: bw,
		Healthy:      connected && !p.stopped.Load(),
	}
}

func (p *Plugin) Generation() int64 {
	return p.generation.Load()
}

func (p *Plugin) WaitReady(ctx context.Context) error {
	if p.ready.Load() {
		return nil
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if p.ready.Load() {
				return nil
			}
		}
	}
}

func (p *Plugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPost:
		p.handleWHEPOffer(w, r)
	case http.MethodDelete:
		p.handleWHEPDelete(w)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (p *Plugin) videoMimeType() string {
	if p.videoCodec == "hevc" || p.videoCodec == "h265" {
		return "video/H265"
	}
	return webrtc.MimeTypeH264
}

func (p *Plugin) handleWHEPOffer(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read offer", http.StatusBadRequest)
		return
	}

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		http.Error(w, fmt.Sprintf("create peer connection: %v", err), http.StatusInternalServerError)
		return
	}

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: p.videoMimeType(), ClockRate: videoClockRate},
		"video", "mediahub",
	)
	if err != nil {
		pc.Close()
		http.Error(w, fmt.Sprintf("create video track: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err = pc.AddTrack(videoTrack); err != nil {
		pc.Close()
		http.Error(w, fmt.Sprintf("add video track: %v", err), http.StatusInternalServerError)
		return
	}

	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: audioClockRate, Channels: 2},
		"audio", "mediahub",
	)
	if err != nil {
		pc.Close()
		http.Error(w, fmt.Sprintf("create audio track: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err = pc.AddTrack(audioTrack); err != nil {
		pc.Close()
		http.Error(w, fmt.Sprintf("add audio track: %v", err), http.StatusInternalServerError)
		return
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  string(body),
	}

	if err = pc.SetRemoteDescription(offer); err != nil {
		pc.Close()
		http.Error(w, fmt.Sprintf("set remote description: %v", err), http.StatusInternalServerError)
		return
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		http.Error(w, fmt.Sprintf("create answer: %v", err), http.StatusInternalServerError)
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)

	if err = pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		http.Error(w, fmt.Sprintf("set local description: %v", err), http.StatusInternalServerError)
		return
	}

	select {
	case <-gatherComplete:
	case <-time.After(5 * time.Second):
	}

	p.mu.Lock()
	oldPC := p.pc
	p.pc = pc
	p.videoTrack = videoTrack
	p.audioTrack = audioTrack
	p.videoSeq = 0
	p.audioSeq = 0
	p.videoTS = 0
	p.audioTS = 0
	p.ptsBaseSet = false
	p.lastVideoPTS = 0
	p.lastAudioPTS = 0
	p.ready.Store(true)
	p.mu.Unlock()

	if oldPC != nil {
		oldPC.Close()
	}

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		p.log.Info().Str("state", state.String()).Msg("peer connection state changed")
		switch state {
		case webrtc.PeerConnectionStateFailed,
			webrtc.PeerConnectionStateClosed,
			webrtc.PeerConnectionStateDisconnected:
			p.mu.Lock()
			if p.pc == pc {
				p.pc = nil
				p.videoTrack = nil
				p.audioTrack = nil
				p.ready.Store(false)
			}
			p.mu.Unlock()
		}
	})

	w.Header().Set("Content-Type", "application/sdp")
	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(pc.LocalDescription().SDP))
}

func (p *Plugin) handleWHEPDelete(w http.ResponseWriter) {
	p.mu.Lock()
	if p.pc != nil {
		p.pc.Close()
		p.pc = nil
		p.videoTrack = nil
		p.audioTrack = nil
		p.ready.Store(false)
	}
	p.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func ptsToRTP(pts int64, clockRate uint32) uint32 {
	if pts < 0 {
		pts = 0
	}
	return uint32((pts * int64(clockRate)) / 90000)
}

func nanosToRTP(nanos int64, clockRate uint32) uint32 {
	if nanos < 0 {
		nanos = 0
	}
	return uint32((nanos / 1000) * int64(clockRate) / 1_000_000)
}

func splitNALUs(data []byte) [][]byte {
	if len(data) < 4 {
		return [][]byte{data}
	}

	if data[0] == 0 && data[1] == 0 && (data[2] == 1 || (data[2] == 0 && len(data) > 3 && data[3] == 1)) {
		return splitAnnexBNALUs(data)
	}

	return splitAVCCNALUs(data)
}

func splitAnnexBNALUs(data []byte) [][]byte {
	var result [][]byte
	i := 0
	for i < len(data) {
		start := -1
		if i+3 < len(data) && data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 1 {
			start = i + 4
		} else if i+2 < len(data) && data[i] == 0 && data[i+1] == 0 && data[i+2] == 1 {
			start = i + 3
		}
		if start < 0 {
			i++
			continue
		}
		end := len(data)
		for j := start; j < len(data)-2; j++ {
			if data[j] == 0 && data[j+1] == 0 && (data[j+2] == 1 || (j+3 < len(data) && data[j+2] == 0 && data[j+3] == 1)) {
				end = j
				break
			}
		}
		if start < end {
			result = append(result, data[start:end])
		}
		i = end
	}
	return result
}

func splitAVCCNALUs(data []byte) [][]byte {
	var result [][]byte
	i := 0
	for i+4 <= len(data) {
		naluLen := int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])
		i += 4
		if naluLen <= 0 || i+naluLen > len(data) {
			break
		}
		result = append(result, data[i:i+naluLen])
		i += naluLen
	}
	return result
}

func packetizeH264(nalu []byte, seq uint16, ts uint32, mtu int, markerOnLast bool) []*rtp.Packet {
	if len(nalu) <= mtu {
		return []*rtp.Packet{{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    videoPayloadType,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           videoSSRC,
				Marker:         markerOnLast,
			},
			Payload: nalu,
		}}
	}

	naluType := nalu[0] & 0x1f
	nri := nalu[0] & 0x60

	var packets []*rtp.Packet
	offset := 1
	first := true
	for offset < len(nalu) {
		end := offset + mtu - 2
		if end > len(nalu) {
			end = len(nalu)
		}
		last := end == len(nalu)

		var fuHeader byte
		if first {
			fuHeader = 0x80
		}
		if last {
			fuHeader |= 0x40
		}
		fuHeader |= naluType

		payload := make([]byte, 2+end-offset)
		payload[0] = h264NALUTypeFUA | nri
		payload[1] = fuHeader
		copy(payload[2:], nalu[offset:end])

		packets = append(packets, &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    videoPayloadType,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           videoSSRC,
				Marker:         last && markerOnLast,
			},
			Payload: payload,
		})

		seq++
		offset = end
		first = false
	}

	return packets
}

func packetizeHEVC(nalu []byte, seq uint16, ts uint32, mtu int, markerOnLast bool) []*rtp.Packet {
	if len(nalu) < 2 {
		return nil
	}

	if len(nalu) <= mtu {
		return []*rtp.Packet{{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    videoPayloadType,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           videoSSRC,
				Marker:         markerOnLast,
			},
			Payload: nalu,
		}}
	}

	naluType := (nalu[0] >> 1) & 0x3f
	tidByte := nalu[1]

	var packets []*rtp.Packet
	offset := 2
	first := true
	for offset < len(nalu) {
		end := offset + mtu - 3
		if end > len(nalu) {
			end = len(nalu)
		}
		last := end == len(nalu)

		var fuHeader byte
		if first {
			fuHeader = 0x80
		}
		if last {
			fuHeader |= 0x40
		}
		fuHeader |= naluType

		payload := make([]byte, 3+end-offset)
		payload[0] = (hevcNALUTypeFU << 1) | (nalu[0] & 0x81)
		payload[1] = tidByte
		payload[2] = fuHeader
		copy(payload[3:], nalu[offset:end])

		packets = append(packets, &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    videoPayloadType,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           videoSSRC,
				Marker:         last && markerOnLast,
			},
			Payload: payload,
		})

		seq++
		offset = end
		first = false
	}

	return packets
}
