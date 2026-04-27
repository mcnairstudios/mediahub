package hdhr

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"net"
	"net/url"

	"github.com/rs/zerolog"
)

const (
	DiscoverPort = 65001

	pktTypeDiscoverReq = 0x0002
	pktTypeDiscoverRpy = 0x0003

	tagDeviceType = 0x01
	tagDeviceID   = 0x02
	tagTunerCount = 0x10
	tagBaseURL    = 0x2A
	tagLineupURL  = 0x27
	tagDeviceAuth = 0x2B

	deviceTypeTuner    = 0x00000001
	deviceTypeWildcard = 0xFFFFFFFF

	deviceIDWildcard = 0xFFFFFFFF
)

type DiscoveryResponder struct {
	baseURL  string
	deviceID uint32
	log      zerolog.Logger
}

func NewDiscoveryResponder(baseURL string, log zerolog.Logger) *DiscoveryResponder {
	return &DiscoveryResponder{
		baseURL:  baseURL,
		deviceID: crc32.ChecksumIEEE([]byte(DefaultDeviceID)),
		log:      log.With().Str("component", "hdhr_discovery").Logger(),
	}
}

func (d *DiscoveryResponder) Run(ctx context.Context) {
	addr := &net.UDPAddr{Port: DiscoverPort}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		d.log.Error().Err(err).Int("port", DiscoverPort).Msg("failed to listen for HDHR discovery")
		return
	}
	defer conn.Close()

	d.log.Info().Int("port", DiscoverPort).Msg("HDHR discovery listener started")

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	buf := make([]byte, 2048)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				d.log.Info().Msg("HDHR discovery listener stopped")
				return
			}
			d.log.Warn().Err(err).Msg("error reading UDP packet")
			continue
		}

		if n < 8 {
			continue
		}

		pktType, tags, ok := d.parsePacket(buf[:n])
		if !ok || pktType != pktTypeDiscoverReq {
			continue
		}

		d.handleRequest(conn, remoteAddr, tags)
	}
}

func (d *DiscoveryResponder) handleRequest(conn *net.UDPConn, remoteAddr *net.UDPAddr, tags map[byte][]byte) {
	if dt, ok := tags[tagDeviceType]; ok && len(dt) == 4 {
		reqType := binary.BigEndian.Uint32(dt)
		if reqType != deviceTypeTuner && reqType != deviceTypeWildcard {
			return
		}
	}

	var requestedID uint32 = deviceIDWildcard
	if di, ok := tags[tagDeviceID]; ok && len(di) == 4 {
		requestedID = binary.BigEndian.Uint32(di)
	}

	if requestedID != deviceIDWildcard && requestedID != d.deviceID {
		return
	}

	host := d.extractHost()
	deviceBaseURL := fmt.Sprintf("http://%s", host)

	d.log.Info().Str("remote", remoteAddr.String()).Str("base_url", deviceBaseURL).Msg("sending discover reply")

	reply := d.buildReply(deviceBaseURL)
	if _, err := conn.WriteToUDP(reply, remoteAddr); err != nil {
		d.log.Warn().Err(err).Str("remote", remoteAddr.String()).Msg("failed to send discover reply")
	}
}

func (d *DiscoveryResponder) extractHost() string {
	u, err := url.Parse(d.baseURL)
	if err != nil {
		return "localhost"
	}
	host := u.Host
	if host == "" {
		return "localhost"
	}
	return host
}

func (d *DiscoveryResponder) buildReply(baseURL string) []byte {
	var payload []byte

	payload = append(payload, encodeTLV(tagDeviceType, encodeUint32(deviceTypeTuner))...)
	payload = append(payload, encodeTLV(tagDeviceID, encodeUint32(d.deviceID))...)
	payload = append(payload, encodeTLV(tagDeviceAuth, []byte(DefaultDeviceAuth))...)
	payload = append(payload, encodeTLV(tagBaseURL, []byte(baseURL))...)
	payload = append(payload, encodeTLV(tagTunerCount, []byte{byte(DefaultTunerCount)})...)

	lineupURL := fmt.Sprintf("%s/lineup.json", baseURL)
	payload = append(payload, encodeTLV(tagLineupURL, []byte(lineupURL))...)

	pkt := make([]byte, 4+len(payload)+4)
	binary.BigEndian.PutUint16(pkt[0:2], pktTypeDiscoverRpy)
	binary.BigEndian.PutUint16(pkt[2:4], uint16(len(payload)))
	copy(pkt[4:], payload)

	crc := crc32.ChecksumIEEE(pkt[:4+len(payload)])
	binary.LittleEndian.PutUint32(pkt[4+len(payload):], crc)

	return pkt
}

func (d *DiscoveryResponder) parsePacket(data []byte) (uint16, map[byte][]byte, bool) {
	if len(data) < 4 {
		return 0, nil, false
	}

	pktType := binary.BigEndian.Uint16(data[0:2])
	payloadLen := binary.BigEndian.Uint16(data[2:4])

	if len(data) < int(4+payloadLen+4) {
		return 0, nil, false
	}

	crcData := data[:4+payloadLen]
	expectedCRC := binary.LittleEndian.Uint32(data[4+payloadLen : 4+payloadLen+4])
	actualCRC := crc32.ChecksumIEEE(crcData)
	if expectedCRC != actualCRC {
		return 0, nil, false
	}

	tags := make(map[byte][]byte)
	payload := data[4 : 4+payloadLen]
	for len(payload) > 0 {
		if len(payload) < 2 {
			break
		}
		tag := payload[0]
		tagLen, consumed := readVarLen(payload[1:])
		payload = payload[1+consumed:]
		if len(payload) < tagLen {
			break
		}
		tags[tag] = payload[:tagLen]
		payload = payload[tagLen:]
	}

	return pktType, tags, true
}

func encodeTLV(tag byte, value []byte) []byte {
	result := []byte{tag}
	result = append(result, encodeVarLen(len(value))...)
	result = append(result, value...)
	return result
}

func encodeVarLen(length int) []byte {
	if length < 128 {
		return []byte{byte(length)}
	}
	return []byte{byte(length&0x7f) | 0x80, byte(length >> 7)}
}

func encodeUint32(val uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, val)
	return b
}

func readVarLen(data []byte) (int, int) {
	if len(data) == 0 {
		return 0, 0
	}
	if data[0]&0x80 == 0 {
		return int(data[0]), 1
	}
	if len(data) < 2 {
		return 0, 1
	}
	return int(data[0]&0x7f) | (int(data[1]) << 7), 2
}
