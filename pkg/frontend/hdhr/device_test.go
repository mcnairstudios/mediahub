package hdhr

import (
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

func TestDeviceServerDiscover(t *testing.T) {
	store := &mockChannelStore{}
	device := &Device{
		ID:         "dev-1",
		Name:       "HDHR Device 1",
		DeviceUUID: "AABBCCDD",
		Port:       5004,
		IsEnabled:  true,
	}
	srv := NewDeviceServer(store, device, "http://192.168.1.100:5004")

	req := httptest.NewRequest(http.MethodGet, "/discover.json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp DiscoverResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	assert.Equal(t, "HDHR Device 1", resp.FriendlyName)
	assert.Equal(t, "AABBCCDD", resp.DeviceID)
	assert.Equal(t, "http://192.168.1.100:5004", resp.BaseURL)
	assert.Equal(t, "http://192.168.1.100:5004/lineup.json", resp.LineupURL)
}

func TestDeviceServerDeviceXML(t *testing.T) {
	store := &mockChannelStore{}
	device := &Device{
		ID:         "dev-1",
		Name:       "HDHR Device 1",
		DeviceUUID: "AABBCCDD",
		Port:       5004,
		IsEnabled:  true,
	}
	srv := NewDeviceServer(store, device, "http://192.168.1.100:5004")

	req := httptest.NewRequest(http.MethodGet, "/device.xml", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp DeviceXML
	require.NoError(t, xml.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "HDHR Device 1", resp.Device.FriendlyName)
	assert.Equal(t, "AABBCCDD", resp.Device.SerialNumber)
	assert.Equal(t, "uuid:AABBCCDD", resp.Device.UDN)
}

func TestDeviceServerLineupFiltersByGroup(t *testing.T) {
	channels := []channel.Channel{
		{ID: "ch1", Name: "BBC One", Number: 1, GroupID: "news", IsEnabled: true},
		{ID: "ch2", Name: "BBC Two", Number: 2, GroupID: "entertainment", IsEnabled: true},
		{ID: "ch3", Name: "Sky News", Number: 3, GroupID: "news", IsEnabled: true},
		{ID: "ch4", Name: "Disabled", Number: 4, GroupID: "news", IsEnabled: false},
	}
	store := &mockChannelStore{channels: channels}
	device := &Device{
		ID:         "dev-1",
		Name:       "News Device",
		DeviceUUID: "AABBCCDD",
		Port:       5004,
		GroupIDs:   []string{"news"},
		IsEnabled:  true,
	}
	srv := NewDeviceServer(store, device, "http://192.168.1.100:5004")

	req := httptest.NewRequest(http.MethodGet, "/lineup.json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var lineup []LineupEntry
	require.NoError(t, json.NewDecoder(w.Body).Decode(&lineup))

	require.Len(t, lineup, 2)
	assert.Equal(t, "BBC One", lineup[0].GuideName)
	assert.Equal(t, "Sky News", lineup[1].GuideName)
}

func TestDeviceServerLineupNoGroupFilter(t *testing.T) {
	channels := []channel.Channel{
		{ID: "ch1", Name: "BBC One", Number: 1, GroupID: "news", IsEnabled: true},
		{ID: "ch2", Name: "BBC Two", Number: 2, GroupID: "entertainment", IsEnabled: true},
	}
	store := &mockChannelStore{channels: channels}
	device := &Device{
		ID:         "dev-1",
		Name:       "All Channels",
		DeviceUUID: "AABBCCDD",
		Port:       5004,
		IsEnabled:  true,
	}
	srv := NewDeviceServer(store, device, "http://192.168.1.100:5004")

	req := httptest.NewRequest(http.MethodGet, "/lineup.json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var lineup []LineupEntry
	require.NoError(t, json.NewDecoder(w.Body).Decode(&lineup))
	require.Len(t, lineup, 2)
}

func TestMainServerStillWorks(t *testing.T) {
	channels := []channel.Channel{
		{ID: "ch1", Name: "BBC One", Number: 1, IsEnabled: true},
	}
	store := &mockChannelStore{channels: channels}
	cfg := &config.Config{BaseURL: "http://192.168.1.100:8080"}
	srv := NewServer(store, cfg)

	req := httptest.NewRequest(http.MethodGet, "/discover.json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp DiscoverResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	assert.Equal(t, DefaultFriendlyName, resp.FriendlyName)
	assert.Equal(t, DefaultDeviceID, resp.DeviceID)
}

func TestDeviceServerLineupURLsUseDeviceBaseURL(t *testing.T) {
	channels := []channel.Channel{
		{ID: "ch1", Name: "BBC One", Number: 1, IsEnabled: true},
	}
	store := &mockChannelStore{channels: channels}
	device := &Device{
		ID:         "dev-1",
		Name:       "Device 1",
		DeviceUUID: "11223344",
		Port:       5004,
		IsEnabled:  true,
	}
	srv := NewDeviceServer(store, device, "http://192.168.1.100:5004")

	req := httptest.NewRequest(http.MethodGet, "/lineup.json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var lineup []LineupEntry
	require.NoError(t, json.NewDecoder(w.Body).Decode(&lineup))
	require.Len(t, lineup, 1)
	assert.Equal(t, "http://192.168.1.100:5004/channel/ch1", lineup[0].URL)
}
