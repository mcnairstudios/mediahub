package hdhr

import (
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryResponderParsePacket(t *testing.T) {
	d := NewDiscoveryResponder("http://192.168.1.100:8080", zerolog.Nop())

	pkt := buildDiscoverRequest(deviceTypeWildcard, deviceIDWildcard)
	pktType, tags, ok := d.parsePacket(pkt)

	require.True(t, ok)
	assert.Equal(t, uint16(pktTypeDiscoverReq), pktType)

	dt, exists := tags[tagDeviceType]
	require.True(t, exists)
	assert.Equal(t, uint32(deviceTypeWildcard), binary.BigEndian.Uint32(dt))
}

func TestDiscoveryResponderParsePacketInvalidCRC(t *testing.T) {
	d := NewDiscoveryResponder("http://192.168.1.100:8080", zerolog.Nop())

	pkt := buildDiscoverRequest(deviceTypeWildcard, deviceIDWildcard)
	pkt[len(pkt)-1] ^= 0xFF

	_, _, ok := d.parsePacket(pkt)
	assert.False(t, ok)
}

func TestDiscoveryResponderParsePacketTooShort(t *testing.T) {
	d := NewDiscoveryResponder("http://192.168.1.100:8080", zerolog.Nop())

	_, _, ok := d.parsePacket([]byte{0x00, 0x02})
	assert.False(t, ok)
}

func TestDiscoveryResponderBuildReply(t *testing.T) {
	d := NewDiscoveryResponder("http://192.168.1.100:8080", zerolog.Nop())

	reply := d.buildReply("http://192.168.1.100:8080")
	require.True(t, len(reply) >= 8)

	pktType := binary.BigEndian.Uint16(reply[0:2])
	assert.Equal(t, uint16(pktTypeDiscoverRpy), pktType)

	payloadLen := binary.BigEndian.Uint16(reply[2:4])
	crcData := reply[:4+payloadLen]
	expectedCRC := binary.LittleEndian.Uint32(reply[4+payloadLen : 4+payloadLen+4])
	assert.Equal(t, crc32.ChecksumIEEE(crcData), expectedCRC)
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		baseURL  string
		expected string
	}{
		{"http://192.168.1.100:8080", "192.168.1.100:8080"},
		{"http://myhost", "myhost"},
		{"", "localhost"},
	}

	for _, tt := range tests {
		d := NewDiscoveryResponder(tt.baseURL, zerolog.Nop())
		assert.Equal(t, tt.expected, d.extractHost(), "baseURL=%s", tt.baseURL)
	}
}

func TestEncodeTLV(t *testing.T) {
	result := encodeTLV(0x01, []byte{0xAA, 0xBB})
	assert.Equal(t, []byte{0x01, 0x02, 0xAA, 0xBB}, result)
}

func TestEncodeVarLen(t *testing.T) {
	assert.Equal(t, []byte{0x05}, encodeVarLen(5))
	assert.Equal(t, []byte{0x7F}, encodeVarLen(127))
	assert.Equal(t, []byte{0x80, 0x01}, encodeVarLen(128))
	assert.Equal(t, []byte{0x81, 0x01}, encodeVarLen(129))
}

func TestReadVarLen(t *testing.T) {
	length, consumed := readVarLen([]byte{0x05})
	assert.Equal(t, 5, length)
	assert.Equal(t, 1, consumed)

	length, consumed = readVarLen([]byte{0x80, 0x01})
	assert.Equal(t, 128, length)
	assert.Equal(t, 2, consumed)

	length, consumed = readVarLen([]byte{})
	assert.Equal(t, 0, length)
	assert.Equal(t, 0, consumed)
}

func buildDiscoverRequest(devType, devID uint32) []byte {
	var payload []byte
	payload = append(payload, encodeTLV(tagDeviceType, encodeUint32(devType))...)
	payload = append(payload, encodeTLV(tagDeviceID, encodeUint32(devID))...)

	pkt := make([]byte, 4+len(payload)+4)
	binary.BigEndian.PutUint16(pkt[0:2], pktTypeDiscoverReq)
	binary.BigEndian.PutUint16(pkt[2:4], uint16(len(payload)))
	copy(pkt[4:], payload)

	crc := crc32.ChecksumIEEE(pkt[:4+len(payload)])
	binary.LittleEndian.PutUint32(pkt[4+len(payload):], crc)

	return pkt
}
