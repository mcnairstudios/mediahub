package dlna

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractHeader(t *testing.T) {
	msg := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 3\r\n" +
		"ST: urn:schemas-upnp-org:device:MediaServer:1\r\n" +
		"\r\n"

	assert.Equal(t, "239.255.255.250:1900", extractHeader(msg, "HOST"))
	assert.Equal(t, "urn:schemas-upnp-org:device:MediaServer:1", extractHeader(msg, "ST"))
	assert.Equal(t, "3", extractHeader(msg, "MX"))
	assert.Equal(t, "", extractHeader(msg, "NONEXISTENT"))
}

func TestExtractHeaderCaseInsensitive(t *testing.T) {
	msg := "NOTIFY * HTTP/1.1\r\n" +
		"Host: 239.255.255.250:1900\r\n" +
		"st: ssdp:all\r\n" +
		"\r\n"

	assert.Equal(t, "239.255.255.250:1900", extractHeader(msg, "HOST"))
	assert.Equal(t, "ssdp:all", extractHeader(msg, "ST"))
}

func TestSSDPAdvertiserLocation(t *testing.T) {
	s := newTestServer(nil, nil, true)
	adv := NewSSDPAdvertiser(s, "http://192.168.1.100", 8080, 0, s.log)
	loc := adv.location()
	assert.Equal(t, "http://192.168.1.100:8080/dlna/device.xml", loc)
}

func TestSSDPAdvertiserLocationNoPort(t *testing.T) {
	s := newTestServer(nil, nil, true)
	adv := NewSSDPAdvertiser(s, "http://192.168.1.100", 9090, 0, s.log)
	loc := adv.location()
	assert.Equal(t, "http://192.168.1.100:9090/dlna/device.xml", loc)
}
