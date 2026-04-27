package hdhr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

func TestImplementsSource(t *testing.T) {
	var _ source.Source = (*Source)(nil)
}

func TestImplementsDiscoverable(t *testing.T) {
	var _ source.Discoverable = (*Source)(nil)
}

func TestImplementsRetunable(t *testing.T) {
	var _ source.Retunable = (*Source)(nil)
}

func TestImplementsClearable(t *testing.T) {
	var _ source.Clearable = (*Source)(nil)
}

func TestType(t *testing.T) {
	s := New(Config{
		ID:          "hdhr-1",
		Name:        "Test HDHR",
		StreamStore: store.NewMemoryStreamStore(),
	})
	if s.Type() != "hdhr" {
		t.Fatalf("expected type hdhr, got %s", s.Type())
	}
}

func TestInfo(t *testing.T) {
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test HDHR",
		IsEnabled: true,
		Devices: []Device{
			{Host: "192.168.1.100", DeviceID: "ABCD1234", TunerCount: 4},
		},
		StreamStore: store.NewMemoryStreamStore(),
	})

	info := s.Info(context.Background())
	if info.ID != "hdhr-1" {
		t.Fatalf("expected ID hdhr-1, got %s", info.ID)
	}
	if info.Name != "Test HDHR" {
		t.Fatalf("expected Name Test HDHR, got %s", info.Name)
	}
	if info.Type != "hdhr" {
		t.Fatalf("expected Type hdhr, got %s", info.Type)
	}
	if !info.IsEnabled {
		t.Fatal("expected IsEnabled true")
	}
	if info.StreamCount != 0 {
		t.Fatalf("expected StreamCount 0, got %d", info.StreamCount)
	}
	if info.MaxConcurrentStreams != 4 {
		t.Fatalf("expected MaxConcurrentStreams 4 (from tuner count), got %d", info.MaxConcurrentStreams)
	}
}

func newTestServer(discover discoverResponse, lineup []lineupEntry) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/discover.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(discover)
	})
	mux.HandleFunc("/lineup.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(lineup)
	})
	return httptest.NewServer(mux)
}

func TestRefresh(t *testing.T) {
	discover := discoverResponse{
		FriendlyName:    "HDHomeRun FLEX 4K",
		ModelNumber:     "HDHR5-4US",
		FirmwareVersion: "20230501",
		DeviceID:        "ABCD1234",
		TunerCount:      4,
	}
	lineup := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://device/auto/v2.1", HD: 1, VideoCodec: "H264", AudioCodec: "AAC"},
		{GuideNumber: "4.1", GuideName: "NBC", URL: "http://device/auto/v4.1", HD: 1, VideoCodec: "H264", AudioCodec: "AC3"},
		{GuideNumber: "7.1", GuideName: "ABC", URL: "http://device/auto/v7.1", HD: 0, VideoCodec: "MPEG2", AudioCodec: "MP2"},
	}

	ts := newTestServer(discover, lineup)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices: []Device{
			{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234", TunerCount: 4},
		},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, err := ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	if err != nil {
		t.Fatalf("listing streams: %v", err)
	}
	if len(streams) != 3 {
		t.Fatalf("expected 3 streams, got %d", len(streams))
	}

	info := s.Info(context.Background())
	if info.StreamCount != 3 {
		t.Fatalf("expected StreamCount 3, got %d", info.StreamCount)
	}
	if info.LastRefreshed == nil {
		t.Fatal("expected LastRefreshed to be set")
	}
}

func TestRefreshSetsSourceFields(t *testing.T) {
	discover := discoverResponse{DeviceID: "ABCD1234", TunerCount: 2}
	lineup := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://device/auto/v2.1", HD: 1},
	}

	ts := newTestServer(discover, lineup)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-src",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "hdhr", "hdhr-src")
	for _, stream := range streams {
		if stream.SourceType != "hdhr" {
			t.Fatalf("expected SourceType hdhr, got %s", stream.SourceType)
		}
		if stream.SourceID != "hdhr-src" {
			t.Fatalf("expected SourceID hdhr-src, got %s", stream.SourceID)
		}
	}
}

func TestRefreshDRMFiltered(t *testing.T) {
	discover := discoverResponse{DeviceID: "ABCD1234", TunerCount: 2}
	lineup := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://device/auto/v2.1", HD: 1},
		{GuideNumber: "3.1", GuideName: "DRM Channel", URL: "http://device/auto/v3.1", DRM: 1, HD: 1},
		{GuideNumber: "4.1", GuideName: "NBC", URL: "http://device/auto/v4.1", HD: 1},
	}

	ts := newTestServer(discover, lineup)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams (DRM filtered), got %d", len(streams))
	}

	for _, stream := range streams {
		if stream.Name == "DRM Channel" {
			t.Fatal("DRM channel should have been filtered")
		}
	}
}

func TestRefreshEmptyURLFiltered(t *testing.T) {
	discover := discoverResponse{DeviceID: "ABCD1234", TunerCount: 2}
	lineup := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://device/auto/v2.1", HD: 1},
		{GuideNumber: "5.1", GuideName: "No URL", URL: "", HD: 1},
		{GuideNumber: "4.1", GuideName: "NBC", URL: "http://device/auto/v4.1", HD: 1},
	}

	ts := newTestServer(discover, lineup)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams (empty URL filtered), got %d", len(streams))
	}

	for _, stream := range streams {
		if stream.Name == "No URL" {
			t.Fatal("empty URL channel should have been filtered")
		}
	}
}

func TestRefreshGroupAssignment(t *testing.T) {
	discover := discoverResponse{DeviceID: "ABCD1234", TunerCount: 2}
	lineup := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "HD Channel", URL: "http://d/v2.1", HD: 1, VideoCodec: "H264"},
		{GuideNumber: "3.1", GuideName: "SD Channel", URL: "http://d/v3.1", HD: 0, VideoCodec: "MPEG2"},
		{GuideNumber: "4.1", GuideName: "Radio", URL: "http://d/v4.1", HD: 0, VideoCodec: ""},
		{GuideNumber: "5.1", GuideName: "Radio None", URL: "http://d/v5.1", HD: 0, VideoCodec: "none"},
	}

	ts := newTestServer(discover, lineup)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	groups := make(map[string]string)
	for _, stream := range streams {
		groups[stream.Name] = stream.Group
	}

	if groups["HD Channel"] != "HD" {
		t.Fatalf("expected HD Channel group HD, got %s", groups["HD Channel"])
	}
	if groups["SD Channel"] != "SD" {
		t.Fatalf("expected SD Channel group SD, got %s", groups["SD Channel"])
	}
	if groups["Radio"] != "Radio" {
		t.Fatalf("expected Radio group Radio, got %s", groups["Radio"])
	}
	if groups["Radio None"] != "Radio" {
		t.Fatalf("expected Radio None group Radio, got %s", groups["Radio None"])
	}
}

func TestRefreshCodecFields(t *testing.T) {
	discover := discoverResponse{DeviceID: "ABCD1234", TunerCount: 2}
	lineup := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://d/v2.1", HD: 1, VideoCodec: "H264", AudioCodec: "AAC"},
		{GuideNumber: "3.1", GuideName: "NBC", URL: "http://d/v3.1", HD: 1, VideoCodec: "HEVC", AudioCodec: "AC3"},
	}

	ts := newTestServer(discover, lineup)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	streams, _ := ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	codecs := make(map[string][2]string)
	for _, stream := range streams {
		codecs[stream.Name] = [2]string{stream.VideoCodec, stream.AudioCodec}
	}

	if codecs["CBS"][0] != "h264" {
		t.Fatalf("expected CBS video h264, got %s", codecs["CBS"][0])
	}
	if codecs["CBS"][1] != "aac" {
		t.Fatalf("expected CBS audio aac, got %s", codecs["CBS"][1])
	}
	if codecs["NBC"][0] != "hevc" {
		t.Fatalf("expected NBC video hevc, got %s", codecs["NBC"][0])
	}
	if codecs["NBC"][1] != "ac3" {
		t.Fatalf("expected NBC audio ac3, got %s", codecs["NBC"][1])
	}
}

func TestStreams(t *testing.T) {
	discover := discoverResponse{DeviceID: "ABCD1234", TunerCount: 2}
	lineup := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://d/v2.1", HD: 1},
		{GuideNumber: "4.1", GuideName: "NBC", URL: "http://d/v4.1", HD: 1},
	}

	ts := newTestServer(discover, lineup)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	ids, err := s.Streams(context.Background())
	if err != nil {
		t.Fatalf("streams failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 stream IDs, got %d", len(ids))
	}
}

func TestDeleteStreams(t *testing.T) {
	discover := discoverResponse{DeviceID: "ABCD1234", TunerCount: 2}
	lineup := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://d/v2.1", HD: 1},
	}

	ts := newTestServer(discover, lineup)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	if err := s.DeleteStreams(context.Background()); err != nil {
		t.Fatalf("delete streams failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	if len(streams) != 0 {
		t.Fatalf("expected 0 streams after delete, got %d", len(streams))
	}
}

func TestClear(t *testing.T) {
	discover := discoverResponse{DeviceID: "ABCD1234", TunerCount: 2}
	lineup := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://d/v2.1", HD: 1},
		{GuideNumber: "4.1", GuideName: "NBC", URL: "http://d/v4.1", HD: 1},
	}

	ts := newTestServer(discover, lineup)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	info := s.Info(context.Background())
	if info.StreamCount != 2 {
		t.Fatalf("expected 2 streams before clear, got %d", info.StreamCount)
	}

	if err := s.Clear(context.Background()); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	if len(streams) != 0 {
		t.Fatalf("expected 0 streams after clear, got %d", len(streams))
	}

	info = s.Info(context.Background())
	if info.StreamCount != 0 {
		t.Fatalf("expected StreamCount 0 after clear, got %d", info.StreamCount)
	}
}

func TestDiscoverDevices(t *testing.T) {
	discover := discoverResponse{
		FriendlyName:    "HDHomeRun FLEX 4K",
		ModelNumber:     "HDHR5-4US",
		FirmwareVersion: "20230501",
		DeviceID:        "ABCD1234",
		TunerCount:      4,
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(discover)
	}))
	defer ts.Close()

	s := New(Config{
		ID:          "hdhr-1",
		Name:        "Test",
		StreamStore: store.NewMemoryStreamStore(),
		HTTPClient:  ts.Client(),
		discoverer: func() ([]string, error) {
			return []string{ts.Listener.Addr().String()}, nil
		},
	})

	devices, err := s.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].Identifier != "ABCD1234" {
		t.Fatalf("expected identifier ABCD1234, got %s", devices[0].Identifier)
	}
	if devices[0].Name != "HDHomeRun FLEX 4K" {
		t.Fatalf("expected name HDHomeRun FLEX 4K, got %s", devices[0].Name)
	}
	if devices[0].Model != "HDHR5-4US" {
		t.Fatalf("expected model HDHR5-4US, got %s", devices[0].Model)
	}
	if devices[0].Properties["tuner_count"] != 4 {
		t.Fatalf("expected tuner_count 4, got %v", devices[0].Properties["tuner_count"])
	}
	if devices[0].Properties["firmware_version"] != "20230501" {
		t.Fatalf("expected firmware_version 20230501, got %v", devices[0].Properties["firmware_version"])
	}
}

func TestDiscoverAlreadyAdded(t *testing.T) {
	discover := discoverResponse{
		FriendlyName: "HDHomeRun FLEX 4K",
		DeviceID:     "ABCD1234",
		TunerCount:   4,
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(discover)
	}))
	defer ts.Close()

	s := New(Config{
		ID:   "hdhr-1",
		Name: "Test",
		Devices: []Device{
			{Host: "192.168.1.100", DeviceID: "ABCD1234"},
		},
		StreamStore: store.NewMemoryStreamStore(),
		HTTPClient:  ts.Client(),
		discoverer: func() ([]string, error) {
			return []string{ts.Listener.Addr().String()}, nil
		},
	})

	devices, err := s.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if !devices[0].AlreadyAdded {
		t.Fatal("expected AlreadyAdded true for existing device")
	}
}

func TestRefreshStaleStreamsRemoved(t *testing.T) {
	discover := discoverResponse{DeviceID: "ABCD1234", TunerCount: 2}
	lineup1 := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://d/v2.1", HD: 1},
		{GuideNumber: "4.1", GuideName: "NBC", URL: "http://d/v4.1", HD: 1},
	}
	lineup2 := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://d/v2.1", HD: 1},
	}

	currentLineup := lineup1
	mux := http.NewServeMux()
	mux.HandleFunc("/discover.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(discover)
	})
	mux.HandleFunc("/lineup.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(currentLineup)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())
	streams, _ := ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}

	currentLineup = lineup2
	_ = s.Refresh(context.Background())
	streams, _ = ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream after stale removal, got %d", len(streams))
	}
}

func TestRefreshDeviceError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error for device returning 500")
	}

	info := s.Info(context.Background())
	if info.LastError == "" {
		t.Fatal("expected LastError to be set")
	}
}

func TestRefreshMultipleDevices(t *testing.T) {
	discover1 := discoverResponse{DeviceID: "DEV1", TunerCount: 2}
	lineup1 := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://d1/v2.1", HD: 1},
	}

	discover2 := discoverResponse{DeviceID: "DEV2", TunerCount: 2}
	lineup2 := []lineupEntry{
		{GuideNumber: "10.1", GuideName: "PBS", URL: "http://d2/v10.1", HD: 1},
		{GuideNumber: "11.1", GuideName: "FOX", URL: "http://d2/v11.1", HD: 1},
	}

	ts1 := newTestServer(discover1, lineup1)
	defer ts1.Close()
	ts2 := newTestServer(discover2, lineup2)
	defer ts2.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices: []Device{
			{Host: ts1.Listener.Addr().String(), DeviceID: "DEV1"},
			{Host: ts2.Listener.Addr().String(), DeviceID: "DEV2"},
		},
		StreamStore: ss,
		HTTPClient:  ts1.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	if len(streams) != 3 {
		t.Fatalf("expected 3 streams from 2 devices, got %d", len(streams))
	}
}

func TestDeterministicStreamID(t *testing.T) {
	id1 := deterministicStreamID("src-1", "2.1")
	id2 := deterministicStreamID("src-1", "2.1")
	if id1 != id2 {
		t.Fatalf("expected deterministic IDs, got %s and %s", id1, id2)
	}
	if id1 == "" {
		t.Fatal("expected non-empty stream ID")
	}

	id3 := deterministicStreamID("src-2", "2.1")
	if id1 == id3 {
		t.Fatal("expected different IDs for different source IDs")
	}
}

func TestRetune(t *testing.T) {
	scanStarted := false
	scanDone := true

	discover := discoverResponse{DeviceID: "ABCD1234", TunerCount: 2}
	lineup := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://d/v2.1", HD: 1},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/discover.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(discover)
	})
	mux.HandleFunc("/lineup.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(lineup)
	})
	mux.HandleFunc("/lineup.post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			scanStarted = true
			w.WriteHeader(http.StatusOK)
		}
	})
	mux.HandleFunc("/lineup_status.json", func(w http.ResponseWriter, r *http.Request) {
		status := struct {
			ScanInProgress int `json:"ScanInProgress"`
			Progress       int `json:"Progress"`
			Found          int `json:"Found"`
		}{ScanInProgress: 0, Progress: 100, Found: 1}
		if !scanDone {
			status.ScanInProgress = 1
			status.Progress = 50
		}
		json.NewEncoder(w).Encode(status)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Retune(context.Background()); err != nil {
		t.Fatalf("retune failed: %v", err)
	}

	if !scanStarted {
		t.Fatal("expected scan to be started on device")
	}

	streams, _ := ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream after retune, got %d", len(streams))
	}
}

func TestRefreshNoDevices(t *testing.T) {
	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "hdhr-1",
		Name:        "Test",
		IsEnabled:   true,
		Devices:     nil,
		StreamStore: ss,
	})

	err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error when no devices configured")
	}
}

func TestRefreshTvgIDSetToGuideNumber(t *testing.T) {
	discover := discoverResponse{DeviceID: "ABCD1234", TunerCount: 2}
	lineup := []lineupEntry{
		{GuideNumber: "2.1", GuideName: "CBS", URL: "http://d/v2.1", HD: 1},
	}

	ts := newTestServer(discover, lineup)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	streams, _ := ss.ListBySource(context.Background(), "hdhr", "hdhr-1")
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(streams))
	}
	if streams[0].TvgID != "2.1" {
		t.Fatalf("expected TvgID 2.1, got %s", streams[0].TvgID)
	}
}

func TestRefreshLineupURLOverride(t *testing.T) {
	lineupFetched := false

	mux := http.NewServeMux()
	mux.HandleFunc("/discover.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(discoverResponse{
			DeviceID:   "ABCD1234",
			TunerCount: 2,
			LineupURL:  fmt.Sprintf("http://%s/custom-lineup.json", r.Host),
		})
	})
	mux.HandleFunc("/custom-lineup.json", func(w http.ResponseWriter, r *http.Request) {
		lineupFetched = true
		json.NewEncoder(w).Encode([]lineupEntry{
			{GuideNumber: "2.1", GuideName: "CBS", URL: "http://d/v2.1", HD: 1},
		})
	})
	mux.HandleFunc("/lineup.json", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not fetch default lineup.json when LineupURL is set")
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:        "hdhr-1",
		Name:      "Test",
		IsEnabled: true,
		Devices:   []Device{{Host: ts.Listener.Addr().String(), DeviceID: "ABCD1234"}},
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	if !lineupFetched {
		t.Fatal("expected custom lineup URL to be fetched")
	}
}
