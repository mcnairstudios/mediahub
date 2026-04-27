package source

import (
	"context"
	"testing"
	"time"
)

type mockSource struct {
	info    SourceInfo
	streams []string
	err     error
}

func (m *mockSource) Info(_ context.Context) SourceInfo      { return m.info }
func (m *mockSource) Refresh(_ context.Context) error        { return m.err }
func (m *mockSource) Streams(_ context.Context) ([]string, error) { return m.streams, m.err }
func (m *mockSource) DeleteStreams(_ context.Context) error   { return m.err }
func (m *mockSource) Type() SourceType                       { return m.info.Type }

type mockDiscoverable struct {
	mockSource
	devices []DiscoveredDevice
}

func (m *mockDiscoverable) Discover(_ context.Context) ([]DiscoveredDevice, error) {
	return m.devices, nil
}

type mockRetunable struct {
	mockSource
	retuneCalled bool
}

func (m *mockRetunable) Retune(_ context.Context) error {
	m.retuneCalled = true
	return nil
}

type mockVPNRoutable struct {
	mockSource
	usesVPN bool
}

func (m *mockVPNRoutable) UsesVPN() bool { return m.usesVPN }

type mockVODProvider struct {
	mockSource
	vodTypes []string
}

func (m *mockVODProvider) SupportsVOD() bool  { return true }
func (m *mockVODProvider) VODTypes() []string { return m.vodTypes }

type mockEPGProvider struct {
	mockSource
}

func (m *mockEPGProvider) ProvidesEPG() bool { return true }

type mockClearable struct {
	mockSource
	cleared bool
}

func (m *mockClearable) Clear(_ context.Context) error {
	m.cleared = true
	return nil
}

type mockConditionalRefresher struct {
	mockSource
}

func (m *mockConditionalRefresher) SupportsConditionalRefresh() bool { return true }

func TestSourceInfo(t *testing.T) {
	now := time.Now()
	info := SourceInfo{
		ID:                  "src-1",
		Type:                "m3u",
		Name:                "Test Source",
		IsEnabled:           true,
		StreamCount:         42,
		LastRefreshed:       &now,
		LastError:           "",
		SourceProfileID:     "prof-1",
		MaxConcurrentStreams: 5,
	}

	if info.ID != "src-1" {
		t.Fatalf("expected ID src-1, got %s", info.ID)
	}
	if info.Type != SourceType("m3u") {
		t.Fatalf("expected Type m3u, got %s", info.Type)
	}
	if info.Name != "Test Source" {
		t.Fatalf("expected Name Test Source, got %s", info.Name)
	}
	if !info.IsEnabled {
		t.Fatal("expected IsEnabled true")
	}
	if info.StreamCount != 42 {
		t.Fatalf("expected StreamCount 42, got %d", info.StreamCount)
	}
	if info.LastRefreshed == nil || !info.LastRefreshed.Equal(now) {
		t.Fatal("expected LastRefreshed to match")
	}
	if info.LastError != "" {
		t.Fatalf("expected empty LastError, got %s", info.LastError)
	}
	if info.SourceProfileID != "prof-1" {
		t.Fatalf("expected SourceProfileID prof-1, got %s", info.SourceProfileID)
	}
	if info.MaxConcurrentStreams != 5 {
		t.Fatalf("expected MaxConcurrentStreams 5, got %d", info.MaxConcurrentStreams)
	}
}

func TestSourceInfoLastRefreshedNil(t *testing.T) {
	info := SourceInfo{
		ID:   "src-2",
		Type: "hdhr",
		Name: "Never Refreshed",
	}

	if info.LastRefreshed != nil {
		t.Fatal("expected nil LastRefreshed for new source")
	}
}

func TestSourceInterface(t *testing.T) {
	ctx := context.Background()
	src := &mockSource{
		info: SourceInfo{
			ID:   "src-1",
			Type: "m3u",
			Name: "Mock",
		},
		streams: []string{"stream-1", "stream-2"},
	}

	var s Source = src

	info := s.Info(ctx)
	if info.ID != "src-1" {
		t.Fatalf("expected ID src-1, got %s", info.ID)
	}

	if err := s.Refresh(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	streams, err := s.Streams(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}

	if err := s.DeleteStreams(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Type() != "m3u" {
		t.Fatalf("expected type m3u, got %s", s.Type())
	}
}

func TestDiscoverableInterface(t *testing.T) {
	src := &mockDiscoverable{
		mockSource: mockSource{
			info: SourceInfo{ID: "src-1", Type: "hdhr"},
		},
		devices: []DiscoveredDevice{
			{
				Host:       "192.168.1.100",
				Identifier: "HDHR-1234",
				Name:       "HDHomeRun FLEX",
				Model:      "HDHR5-4US",
				Properties: map[string]any{"tuners": 4},
			},
		},
	}

	var s Source = src
	d, ok := s.(Discoverable)
	if !ok {
		t.Fatal("expected mockDiscoverable to implement Discoverable")
	}

	devices, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].Identifier != "HDHR-1234" {
		t.Fatalf("expected identifier HDHR-1234, got %s", devices[0].Identifier)
	}
	if devices[0].Properties["tuners"] != 4 {
		t.Fatalf("expected tuners=4, got %v", devices[0].Properties["tuners"])
	}
}

func TestRetunableInterface(t *testing.T) {
	src := &mockRetunable{
		mockSource: mockSource{
			info: SourceInfo{ID: "src-1", Type: "satip"},
		},
	}

	var s Source = src
	r, ok := s.(Retunable)
	if !ok {
		t.Fatal("expected mockRetunable to implement Retunable")
	}

	if err := r.Retune(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !src.retuneCalled {
		t.Fatal("expected Retune to be called")
	}
}

func TestVPNRoutableInterface(t *testing.T) {
	src := &mockVPNRoutable{
		mockSource: mockSource{
			info: SourceInfo{ID: "src-1", Type: "m3u"},
		},
		usesVPN: true,
	}

	var s Source = src
	v, ok := s.(VPNRoutable)
	if !ok {
		t.Fatal("expected mockVPNRoutable to implement VPNRoutable")
	}
	if !v.UsesVPN() {
		t.Fatal("expected UsesVPN to return true")
	}
}

func TestVODProviderInterface(t *testing.T) {
	src := &mockVODProvider{
		mockSource: mockSource{
			info: SourceInfo{ID: "src-1", Type: "xtream"},
		},
		vodTypes: []string{"movie", "series"},
	}

	var s Source = src
	v, ok := s.(VODProvider)
	if !ok {
		t.Fatal("expected mockVODProvider to implement VODProvider")
	}
	if !v.SupportsVOD() {
		t.Fatal("expected SupportsVOD true")
	}
	types := v.VODTypes()
	if len(types) != 2 {
		t.Fatalf("expected 2 VOD types, got %d", len(types))
	}
}

func TestEPGProviderInterface(t *testing.T) {
	src := &mockEPGProvider{
		mockSource: mockSource{
			info: SourceInfo{ID: "src-1", Type: "m3u"},
		},
	}

	var s Source = src
	e, ok := s.(EPGProvider)
	if !ok {
		t.Fatal("expected mockEPGProvider to implement EPGProvider")
	}
	if !e.ProvidesEPG() {
		t.Fatal("expected ProvidesEPG true")
	}
}

func TestClearableInterface(t *testing.T) {
	src := &mockClearable{
		mockSource: mockSource{
			info: SourceInfo{ID: "src-1", Type: "m3u"},
		},
	}

	var s Source = src
	c, ok := s.(Clearable)
	if !ok {
		t.Fatal("expected mockClearable to implement Clearable")
	}

	if err := c.Clear(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !src.cleared {
		t.Fatal("expected Clear to be called")
	}
}

func TestConditionalRefresherInterface(t *testing.T) {
	src := &mockConditionalRefresher{
		mockSource: mockSource{
			info: SourceInfo{ID: "src-1", Type: "m3u"},
		},
	}

	var s Source = src
	cr, ok := s.(ConditionalRefresher)
	if !ok {
		t.Fatal("expected mockConditionalRefresher to implement ConditionalRefresher")
	}
	if !cr.SupportsConditionalRefresh() {
		t.Fatal("expected SupportsConditionalRefresh true")
	}
}

func TestNonDiscoverableSource(t *testing.T) {
	src := &mockSource{
		info: SourceInfo{ID: "src-1", Type: "m3u"},
	}

	var s Source = src
	_, ok := s.(Discoverable)
	if ok {
		t.Fatal("plain mockSource should NOT implement Discoverable")
	}
}

func TestRefreshStatus(t *testing.T) {
	status := RefreshStatus{
		State:    "refreshing",
		Message:  "Downloading playlist",
		Total:    1000,
		Progress: 250,
	}

	if status.State != "refreshing" {
		t.Fatalf("expected state refreshing, got %s", status.State)
	}
	if status.Progress != 250 {
		t.Fatalf("expected progress 250, got %d", status.Progress)
	}
}
