package dlna

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockChannelLister struct {
	channels []ChannelItem
	groups   []GroupItem
}

func (m *mockChannelLister) ListChannels(_ context.Context) ([]ChannelItem, error) {
	return m.channels, nil
}

func (m *mockChannelLister) GetChannel(_ context.Context, id string) (*ChannelItem, error) {
	for _, ch := range m.channels {
		if ch.ID == id {
			return &ch, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockChannelLister) ListGroups(_ context.Context) ([]GroupItem, error) {
	return m.groups, nil
}

type mockSettings struct {
	enabled bool
}

func (m *mockSettings) IsEnabled(_ context.Context) bool {
	return m.enabled
}

func newTestServer(channels []ChannelItem, groups []GroupItem, enabled bool) *Server {
	log := zerolog.Nop()
	return NewServer(
		&mockChannelLister{channels: channels, groups: groups},
		&mockSettings{enabled: enabled},
		"http://192.168.1.100",
		8080,
		log,
	)
}

func TestDeviceXMLValidXML(t *testing.T) {
	s := newTestServer(nil, nil, true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dlna/device.xml", nil)
	s.DeviceDescription(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/xml")

	var root struct {
		XMLName xml.Name `xml:"root"`
	}
	err := xml.Unmarshal(rec.Body.Bytes(), &root)
	require.NoError(t, err, "device.xml must be valid XML")
	assert.Contains(t, rec.Body.String(), "MediaHub DLNA")
	assert.Contains(t, rec.Body.String(), "MediaServer:1")
}

func TestDeviceXMLDisabled(t *testing.T) {
	s := newTestServer(nil, nil, false)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dlna/device.xml", nil)
	s.DeviceDescription(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestContentDirectoryXMLValid(t *testing.T) {
	s := newTestServer(nil, nil, true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dlna/ContentDirectory.xml", nil)
	s.ContentDirectorySCPD(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var scpd struct {
		XMLName xml.Name `xml:"scpd"`
	}
	err := xml.Unmarshal(rec.Body.Bytes(), &scpd)
	require.NoError(t, err, "ContentDirectory.xml must be valid XML")
	assert.Contains(t, rec.Body.String(), "Browse")
}

func TestConnectionManagerXMLValid(t *testing.T) {
	s := newTestServer(nil, nil, true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dlna/ConnectionManager.xml", nil)
	s.ConnectionManagerSCPD(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var scpd struct {
		XMLName xml.Name `xml:"scpd"`
	}
	err := xml.Unmarshal(rec.Body.Bytes(), &scpd)
	require.NoError(t, err, "ConnectionManager.xml must be valid XML")
	assert.Contains(t, rec.Body.String(), "GetProtocolInfo")
}

func soapBrowseBody(objectID, browseFlag string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body>
<u:Browse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
  <ObjectID>%s</ObjectID>
  <BrowseFlag>%s</BrowseFlag>
  <Filter>*</Filter>
  <StartingIndex>0</StartingIndex>
  <RequestedCount>0</RequestedCount>
  <SortCriteria></SortCriteria>
</u:Browse>
</s:Body>
</s:Envelope>`, objectID, browseFlag)
}

func doSOAPBrowse(t *testing.T, s *Server, objectID, browseFlag string) *httptest.ResponseRecorder {
	t.Helper()
	body := soapBrowseBody(objectID, browseFlag)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/dlna/control/ContentDirectory", strings.NewReader(body))
	req.Header.Set("SOAPAction", `"urn:schemas-upnp-org:service:ContentDirectory:1#Browse"`)
	s.ContentDirectoryControl(rec, req)
	return rec
}

func TestBrowseRootReturnsContainers(t *testing.T) {
	channels := []ChannelItem{
		{ID: "ch1", Name: "BBC One", GroupID: "g1"},
		{ID: "ch2", Name: "BBC Two", GroupID: "g1"},
		{ID: "ch3", Name: "ITV", GroupID: ""},
	}
	groups := []GroupItem{
		{ID: "g1", Name: "BBC"},
	}

	s := newTestServer(channels, groups, true)
	rec := doSOAPBrowse(t, s, "0", "BrowseDirectChildren")

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "BBC")
	assert.Contains(t, body, "Ungrouped")
	assert.Contains(t, body, "NumberReturned")
	assert.Contains(t, body, "TotalMatches")
}

func TestBrowseRootMetadata(t *testing.T) {
	s := newTestServer(nil, nil, true)
	rec := doSOAPBrowse(t, s, "0", "BrowseMetadata")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "MediaHub")
}

func TestBrowseGroupListsChannels(t *testing.T) {
	channels := []ChannelItem{
		{ID: "ch1", Name: "BBC One", GroupID: "g1", LogoURL: "http://example.com/bbc.png"},
		{ID: "ch2", Name: "BBC Two", GroupID: "g1"},
		{ID: "ch3", Name: "ITV", GroupID: "g2"},
	}
	groups := []GroupItem{
		{ID: "g1", Name: "BBC"},
		{ID: "g2", Name: "ITV"},
	}

	s := newTestServer(channels, groups, true)
	rec := doSOAPBrowse(t, s, "grp-g1", "BrowseDirectChildren")

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "BBC One")
	assert.Contains(t, body, "BBC Two")
	assert.NotContains(t, body, "ITV")
}

func TestBrowseUngrouped(t *testing.T) {
	channels := []ChannelItem{
		{ID: "ch1", Name: "BBC One", GroupID: "g1"},
		{ID: "ch3", Name: "Loose", GroupID: ""},
	}
	groups := []GroupItem{{ID: "g1", Name: "BBC"}}

	s := newTestServer(channels, groups, true)
	rec := doSOAPBrowse(t, s, "grp-ungrouped", "BrowseDirectChildren")

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Loose")
	assert.NotContains(t, body, "BBC One")
}

func TestBrowseChannelMetadata(t *testing.T) {
	channels := []ChannelItem{
		{ID: "ch1", Name: "BBC One", GroupID: "g1", LogoURL: "http://example.com/logo.jpg"},
	}

	s := newTestServer(channels, nil, true)
	rec := doSOAPBrowse(t, s, "ch-ch1", "BrowseMetadata")

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "BBC One")
	assert.Contains(t, body, "channel/ch1")
	assert.Contains(t, body, "JPEG_SM")
}

func TestBrowseChannelNotFound(t *testing.T) {
	s := newTestServer(nil, nil, true)
	rec := doSOAPBrowse(t, s, "ch-nonexistent", "BrowseMetadata")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "NumberReturned")
}

func TestBrowseUnknownObjectID(t *testing.T) {
	s := newTestServer(nil, nil, true)
	rec := doSOAPBrowse(t, s, "unknown-123", "BrowseDirectChildren")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "DIDL-Lite")
}

func TestGetProtocolInfo(t *testing.T) {
	s := newTestServer(nil, nil, true)

	body := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body>
<u:GetProtocolInfo xmlns:u="urn:schemas-upnp-org:service:ConnectionManager:1"/>
</s:Body>
</s:Envelope>`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/dlna/control/ConnectionManager", strings.NewReader(body))
	req.Header.Set("SOAPAction", `"urn:schemas-upnp-org:service:ConnectionManager:1#GetProtocolInfo"`)
	s.ConnectionManagerControl(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "video/mp2t")
	assert.Contains(t, rec.Body.String(), "video/mp4")
}

func TestGetSearchCapabilities(t *testing.T) {
	s := newTestServer(nil, nil, true)

	body := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body>
<u:GetSearchCapabilities xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1"/>
</s:Body>
</s:Envelope>`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/dlna/control/ContentDirectory", strings.NewReader(body))
	req.Header.Set("SOAPAction", `"urn:schemas-upnp-org:service:ContentDirectory:1#GetSearchCapabilities"`)
	s.ContentDirectoryControl(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "SearchCaps")
}

func TestBrowsePagination(t *testing.T) {
	var channels []ChannelItem
	for i := 0; i < 20; i++ {
		channels = append(channels, ChannelItem{
			ID:      fmt.Sprintf("ch%d", i),
			Name:    fmt.Sprintf("Channel %d", i),
			GroupID: "g1",
		})
	}
	groups := []GroupItem{{ID: "g1", Name: "Test"}}

	s := newTestServer(channels, groups, true)

	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body>
<u:Browse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
  <ObjectID>grp-g1</ObjectID>
  <BrowseFlag>BrowseDirectChildren</BrowseFlag>
  <Filter>*</Filter>
  <StartingIndex>5</StartingIndex>
  <RequestedCount>3</RequestedCount>
  <SortCriteria></SortCriteria>
</u:Browse>
</s:Body>
</s:Envelope>`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/dlna/control/ContentDirectory", strings.NewReader(body))
	req.Header.Set("SOAPAction", `"urn:schemas-upnp-org:service:ContentDirectory:1#Browse"`)
	s.ContentDirectoryControl(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	respBody := rec.Body.String()
	assert.Contains(t, respBody, "<NumberReturned>3</NumberReturned>")
	assert.Contains(t, respBody, "<TotalMatches>20</TotalMatches>")
}

func TestRegisterRoutes(t *testing.T) {
	s := newTestServer(nil, nil, true)
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/dlna/device.xml"},
		{"GET", "/dlna/ContentDirectory.xml"},
		{"GET", "/dlna/ConnectionManager.xml"},
		{"POST", "/dlna/control/ContentDirectory"},
		{"POST", "/dlna/control/ConnectionManager"},
	}

	for _, rt := range routes {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(rt.method, rt.path, nil)
		if rt.method == "POST" {
			req.Body = io.NopCloser(strings.NewReader(""))
			req.Header.Set("SOAPAction", `"test#Dummy"`)
		}
		mux.ServeHTTP(rec, req)
		assert.NotEqual(t, http.StatusMethodNotAllowed, rec.Code, "route %s %s should be registered", rt.method, rt.path)
	}
}

func TestUDN(t *testing.T) {
	s := newTestServer(nil, nil, true)
	udn := s.UDN()
	assert.True(t, strings.HasPrefix(udn, "uuid:"))
	assert.Len(t, udn, 41)
}

func TestXmlEscape(t *testing.T) {
	assert.Equal(t, "&amp;&lt;&gt;&quot;&apos;", xmlEscape(`&<>"'`))
}

func TestLogoProfile(t *testing.T) {
	assert.Equal(t, "PNG_SM", logoProfile("http://example.com/logo.png"))
	assert.Equal(t, "JPEG_SM", logoProfile("http://example.com/logo.jpg"))
	assert.Equal(t, "GIF_LG", logoProfile("http://example.com/logo.gif"))
	assert.Equal(t, "JPEG_SM", logoProfile("http://example.com/logo.webp"))
	assert.Equal(t, "PNG_SM", logoProfile("http://example.com/logo.png?size=sm"))
}

func TestExtractAction(t *testing.T) {
	assert.Equal(t, "Browse", extractAction(`"urn:schemas-upnp-org:service:ContentDirectory:1#Browse"`))
	assert.Equal(t, "GetProtocolInfo", extractAction("GetProtocolInfo"))
}

func TestExtractHost(t *testing.T) {
	assert.Equal(t, "192.168.1.100", extractHost("http://192.168.1.100:8080"))
	assert.Equal(t, "192.168.1.100", extractHost("http://192.168.1.100"))
	assert.Equal(t, "localhost", extractHost(""))
}
