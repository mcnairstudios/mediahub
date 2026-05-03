package webrtc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"
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

	generation   atomic.Int64
	stopped      atomic.Bool
	ready        atomic.Bool
	bytesWritten int64
}

func New(cfg output.PluginConfig) (output.OutputPlugin, error) {
	log := zerolog.New(os.Stderr).With().Str("plugin", "webrtc").Logger()

	p := &Plugin{
		cfg: cfg,
		log: log,
	}
	p.generation.Store(1)

	return p, nil
}

func (p *Plugin) Mode() output.DeliveryMode {
	return output.DeliveryWebRTC
}

func (p *Plugin) PushVideo(data []byte, pts, dts int64, keyframe bool) error {
	if p.stopped.Load() {
		return nil
	}

	p.mu.Lock()
	track := p.videoTrack
	if track == nil {
		p.mu.Unlock()
		return nil
	}

	nalus := splitNALUs(data)
	for _, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}

		packets := packetizeH264(nalu, p.videoSeq, p.videoTS, 1400)
		for _, pkt := range packets {
			p.videoSeq = pkt.Header.SequenceNumber + 1
			if err := track.WriteRTP(pkt); err != nil {
				p.mu.Unlock()
				return nil
			}
			p.bytesWritten += int64(len(pkt.Payload))
		}
	}

	p.videoTS += 90000 / 30
	p.mu.Unlock()
	return nil
}

func (p *Plugin) PushAudio(data []byte, pts, dts int64) error {
	if p.stopped.Load() {
		return nil
	}

	p.mu.Lock()
	track := p.audioTrack
	if track == nil {
		p.mu.Unlock()
		return nil
	}

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    97,
			SequenceNumber: p.audioSeq,
			Timestamp:      p.audioTS,
			SSRC:           2,
			Marker:         true,
		},
		Payload: data,
	}
	p.audioSeq++
	p.audioTS += 960

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
	switch r.Method {
	case http.MethodPost:
		p.handleWHEPOffer(w, r)
	case http.MethodDelete:
		p.handleWHEPDelete(w)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000},
		"video", "mediahub-video",
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
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2},
		"audio", "mediahub-audio",
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
	if p.pc != nil {
		p.pc.Close()
	}
	p.pc = pc
	p.videoTrack = videoTrack
	p.audioTrack = audioTrack
	p.videoSeq = 0
	p.audioSeq = 0
	p.videoTS = 0
	p.audioTS = 0
	p.ready.Store(true)
	p.mu.Unlock()

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		p.log.Info().Str("state", state.String()).Msg("peer connection state changed")
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
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

func packetizeH264(nalu []byte, seq uint16, ts uint32, mtu int) []*rtp.Packet {
	if len(nalu) <= mtu {
		return []*rtp.Packet{{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    96,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           1,
				Marker:         true,
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

		var indicator byte
		if first {
			indicator = 0x80
		}
		if last {
			indicator |= 0x40
		}
		indicator |= naluType

		fuHeader := make([]byte, 2+end-offset)
		fuHeader[0] = 28 | nri
		fuHeader[1] = indicator
		copy(fuHeader[2:], nalu[offset:end])

		packets = append(packets, &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    96,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           1,
				Marker:         last,
			},
			Payload: fuHeader,
		})

		seq++
		offset = end
		first = false
	}

	return packets
}
