package hdhr

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockChannelStore struct {
	channels []channel.Channel
	err      error
}

func (m *mockChannelStore) Get(_ context.Context, id string) (*channel.Channel, error) {
	for i := range m.channels {
		if m.channels[i].ID == id {
			return &m.channels[i], nil
		}
	}
	return nil, nil
}

func (m *mockChannelStore) List(_ context.Context) ([]channel.Channel, error) {
	return m.channels, m.err
}

func (m *mockChannelStore) Create(_ context.Context, _ *channel.Channel) error { return nil }
func (m *mockChannelStore) Update(_ context.Context, _ *channel.Channel) error { return nil }
func (m *mockChannelStore) Delete(_ context.Context, _ string) error           { return nil }

func (m *mockChannelStore) AssignStreams(_ context.Context, _ string, _ []string) error { return nil }
func (m *mockChannelStore) RemoveStreamMappings(_ context.Context, _ []string) error    { return nil }

func testServer(channels []channel.Channel) *Server {
	store := &mockChannelStore{channels: channels}
	cfg := &config.Config{BaseURL: "http://192.168.1.100:8080"}
	return NewServer(store, cfg)
}

func TestDiscoverJSON(t *testing.T) {
	srv := testServer(nil)

	req := httptest.NewRequest(http.MethodGet, "/discover.json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp DiscoverResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	assert.Equal(t, DefaultFriendlyName, resp.FriendlyName)
	assert.Equal(t, "Silicondust", resp.Manufacturer)
	assert.Equal(t, DefaultModelNumber, resp.ModelNumber)
	assert.Equal(t, DefaultFirmwareName, resp.FirmwareName)
	assert.Equal(t, DefaultFirmwareVersion, resp.FirmwareVersion)
	assert.Equal(t, DefaultDeviceID, resp.DeviceID)
	assert.Equal(t, DefaultDeviceAuth, resp.DeviceAuth)
	assert.Equal(t, "http://192.168.1.100:8080", resp.BaseURL)
	assert.Equal(t, "http://192.168.1.100:8080/lineup.json", resp.LineupURL)
	assert.Equal(t, DefaultTunerCount, resp.TunerCount)
}

func TestLineupStatusJSON(t *testing.T) {
	srv := testServer(nil)

	req := httptest.NewRequest(http.MethodGet, "/lineup_status.json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp LineupStatus
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	assert.Equal(t, 0, resp.ScanInProgress)
	assert.Equal(t, 1, resp.ScanPossible)
	assert.Equal(t, "Cable", resp.Source)
	assert.Equal(t, []string{"Cable", "Antenna"}, resp.SourceList)
}

func TestLineupJSON(t *testing.T) {
	channels := []channel.Channel{
		{ID: "ch1", Name: "BBC One", Number: 1, IsEnabled: true},
		{ID: "ch2", Name: "BBC Two", Number: 2, IsEnabled: true},
		{ID: "ch3", Name: "Disabled Channel", Number: 3, IsEnabled: false},
	}
	srv := testServer(channels)

	req := httptest.NewRequest(http.MethodGet, "/lineup.json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var lineup []LineupEntry
	require.NoError(t, json.NewDecoder(w.Body).Decode(&lineup))

	require.Len(t, lineup, 2)

	assert.Equal(t, "1", lineup[0].GuideNumber)
	assert.Equal(t, "BBC One", lineup[0].GuideName)
	assert.Equal(t, "H264", lineup[0].VideoCodec)
	assert.Equal(t, "AAC", lineup[0].AudioCodec)
	assert.Equal(t, 1, lineup[0].HD)
	assert.Equal(t, "http://192.168.1.100:8080/channel/ch1", lineup[0].URL)

	assert.Equal(t, "2", lineup[1].GuideNumber)
	assert.Equal(t, "BBC Two", lineup[1].GuideName)
	assert.Equal(t, "http://192.168.1.100:8080/channel/ch2", lineup[1].URL)
}

func TestLineupJSONEmpty(t *testing.T) {
	srv := testServer(nil)

	req := httptest.NewRequest(http.MethodGet, "/lineup.json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var lineup []LineupEntry
	require.NoError(t, json.NewDecoder(w.Body).Decode(&lineup))
	assert.Empty(t, lineup)
}

func TestLineupXML(t *testing.T) {
	channels := []channel.Channel{
		{ID: "ch1", Name: "BBC One", Number: 1, IsEnabled: true},
	}
	srv := testServer(channels)

	req := httptest.NewRequest(http.MethodGet, "/lineup.xml", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/xml", w.Header().Get("Content-Type"))

	type xmlLineup struct {
		XMLName xml.Name      `xml:"Lineup"`
		Items   []LineupEntry `xml:"Program"`
	}
	var resp xmlLineup
	require.NoError(t, xml.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "BBC One", resp.Items[0].GuideName)
}

func TestDeviceXML(t *testing.T) {
	srv := testServer(nil)

	req := httptest.NewRequest(http.MethodGet, "/device.xml", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/xml", w.Header().Get("Content-Type"))

	var resp DeviceXML
	require.NoError(t, xml.NewDecoder(w.Body).Decode(&resp))

	assert.Equal(t, "urn:schemas-upnp-org:device-1-0", resp.XMLNS)
	assert.Equal(t, "http://192.168.1.100:8080", resp.URLBase)
	assert.Equal(t, DefaultFriendlyName, resp.Device.FriendlyName)
	assert.Equal(t, "Silicondust", resp.Device.Manufacturer)
	assert.Equal(t, DefaultModelNumber, resp.Device.ModelNumber)
	assert.Equal(t, DefaultDeviceID, resp.Device.SerialNumber)
	assert.Equal(t, "uuid:"+DefaultDeviceID, resp.Device.UDN)
}

func TestLineupFiltersDisabledChannels(t *testing.T) {
	channels := []channel.Channel{
		{ID: "ch1", Name: "Enabled", Number: 1, IsEnabled: true},
		{ID: "ch2", Name: "Disabled", Number: 2, IsEnabled: false},
		{ID: "ch3", Name: "Also Enabled", Number: 3, IsEnabled: true},
	}
	srv := testServer(channels)

	req := httptest.NewRequest(http.MethodGet, "/lineup.json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var lineup []LineupEntry
	require.NoError(t, json.NewDecoder(w.Body).Decode(&lineup))
	require.Len(t, lineup, 2)
	assert.Equal(t, "Enabled", lineup[0].GuideName)
	assert.Equal(t, "Also Enabled", lineup[1].GuideName)
}
