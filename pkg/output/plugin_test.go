package output

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockPlugin struct {
	mode         DeliveryMode
	videoPackets int
	audioPackets int
	subPackets   int
	eosCount     int
	seekCount    int
	stopCount    int
	healthy      bool
	err          error
}

func newMockPlugin(mode DeliveryMode) *mockPlugin {
	return &mockPlugin{mode: mode, healthy: true}
}

func (m *mockPlugin) Mode() DeliveryMode { return m.mode }

func (m *mockPlugin) PushVideo(data []byte, pts, dts int64, keyframe bool) error {
	if m.err != nil {
		return m.err
	}
	m.videoPackets++
	return nil
}

func (m *mockPlugin) PushAudio(data []byte, pts, dts int64) error {
	if m.err != nil {
		return m.err
	}
	m.audioPackets++
	return nil
}

func (m *mockPlugin) PushSubtitle(data []byte, pts int64, duration int64) error {
	if m.err != nil {
		return m.err
	}
	m.subPackets++
	return nil
}

func (m *mockPlugin) EndOfStream() { m.eosCount++ }
func (m *mockPlugin) ResetForSeek() { m.seekCount++ }
func (m *mockPlugin) Stop()         { m.stopCount++ }

func (m *mockPlugin) Status() PluginStatus {
	errStr := ""
	if m.err != nil {
		errStr = m.err.Error()
	}
	return PluginStatus{
		Mode:         m.mode,
		SegmentCount: m.videoPackets,
		BytesWritten: int64(m.videoPackets * 100),
		Healthy:      m.healthy,
		Error:        errStr,
	}
}

type mockServablePlugin struct {
	mockPlugin
	generation int64
}

func newMockServablePlugin(mode DeliveryMode) *mockServablePlugin {
	return &mockServablePlugin{
		mockPlugin: mockPlugin{mode: mode, healthy: true},
		generation: 1,
	}
}

func (m *mockServablePlugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (m *mockServablePlugin) Generation() int64 { return m.generation }

func (m *mockServablePlugin) WaitReady(ctx context.Context) error {
	return ctx.Err()
}

func TestOutputPluginInterfaceSatisfaction(t *testing.T) {
	var _ OutputPlugin = newMockPlugin(DeliveryMSE)
	var _ OutputPlugin = newMockPlugin(DeliveryHLS)
	var _ OutputPlugin = newMockPlugin(DeliveryStream)
	var _ OutputPlugin = newMockPlugin(DeliveryRecord)
}

func TestServablePluginExtendsOutputPlugin(t *testing.T) {
	var p ServablePlugin = newMockServablePlugin(DeliveryMSE)

	var _ OutputPlugin = p

	if p.Mode() != DeliveryMSE {
		t.Fatalf("expected mode %s, got %s", DeliveryMSE, p.Mode())
	}
}

func TestServablePluginServeHTTP(t *testing.T) {
	p := newMockServablePlugin(DeliveryHLS)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestServablePluginGeneration(t *testing.T) {
	p := newMockServablePlugin(DeliveryMSE)
	p.generation = 42
	if p.Generation() != 42 {
		t.Fatalf("expected generation 42, got %d", p.Generation())
	}
}

func TestPluginStatusFields(t *testing.T) {
	s := PluginStatus{
		Mode:         DeliveryHLS,
		SegmentCount: 10,
		BytesWritten: 5000,
		Healthy:      true,
		Error:        "",
	}

	if s.Mode != DeliveryHLS {
		t.Fatalf("expected mode %s, got %s", DeliveryHLS, s.Mode)
	}
	if s.SegmentCount != 10 {
		t.Fatalf("expected segment count 10, got %d", s.SegmentCount)
	}
	if s.BytesWritten != 5000 {
		t.Fatalf("expected bytes written 5000, got %d", s.BytesWritten)
	}
	if !s.Healthy {
		t.Fatal("expected healthy")
	}
	if s.Error != "" {
		t.Fatalf("expected empty error, got %q", s.Error)
	}
}

func TestDeliveryModeConstants(t *testing.T) {
	modes := []struct {
		mode DeliveryMode
		want string
	}{
		{DeliveryMSE, "mse"},
		{DeliveryHLS, "hls"},
		{DeliveryStream, "stream"},
		{DeliveryRecord, "record"},
	}

	for _, tc := range modes {
		if string(tc.mode) != tc.want {
			t.Errorf("expected %q, got %q", tc.want, string(tc.mode))
		}
	}
}
