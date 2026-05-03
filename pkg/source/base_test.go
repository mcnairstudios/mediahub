package source

import (
	"context"
	"net/http"
	"testing"
)

func TestNewBaseSource(t *testing.T) {
	b := NewBaseSource("id-1", "Test Source", TypeM3U, true, 5)

	if b.ID() != "id-1" {
		t.Errorf("ID = %q, want id-1", b.ID())
	}
	if b.Name() != "Test Source" {
		t.Errorf("Name = %q, want Test Source", b.Name())
	}
	if b.Type() != TypeM3U {
		t.Errorf("Type = %q, want m3u", b.Type())
	}

	info := b.Info(context.Background())
	if !info.IsEnabled {
		t.Error("expected IsEnabled=true")
	}
	if info.MaxConcurrentStreams != 5 {
		t.Errorf("MaxConcurrentStreams = %d, want 5", info.MaxConcurrentStreams)
	}
	if info.StreamCount != 0 {
		t.Errorf("StreamCount = %d, want 0", info.StreamCount)
	}
	if info.LastRefreshed != nil {
		t.Error("expected nil LastRefreshed")
	}
	if info.LastError != "" {
		t.Errorf("LastError = %q, want empty", info.LastError)
	}
}

func TestBaseSource_SetRefreshResult(t *testing.T) {
	b := NewBaseSource("id-1", "Test", TypeDemo, true, 0)

	b.SetRefreshResult(42)

	info := b.Info(context.Background())
	if info.StreamCount != 42 {
		t.Errorf("StreamCount = %d, want 42", info.StreamCount)
	}
	if info.LastRefreshed == nil {
		t.Error("expected non-nil LastRefreshed")
	}
	if info.LastError != "" {
		t.Errorf("expected empty LastError after success, got %q", info.LastError)
	}
}

func TestBaseSource_SetError(t *testing.T) {
	b := NewBaseSource("id-1", "Test", TypeDemo, true, 0)

	b.SetError("connection refused")

	info := b.Info(context.Background())
	if info.LastError != "connection refused" {
		t.Errorf("LastError = %q, want 'connection refused'", info.LastError)
	}
}

func TestBaseSource_SetRefreshClearsError(t *testing.T) {
	b := NewBaseSource("id-1", "Test", TypeDemo, true, 0)

	b.SetError("some error")
	b.SetRefreshResult(10)

	info := b.Info(context.Background())
	if info.LastError != "" {
		t.Errorf("expected empty LastError after successful refresh, got %q", info.LastError)
	}
}

func TestBaseSource_SetRefreshed(t *testing.T) {
	b := NewBaseSource("id-1", "Test", TypeDemo, true, 0)

	b.SetRefreshed()

	info := b.Info(context.Background())
	if info.LastRefreshed == nil {
		t.Error("expected non-nil LastRefreshed")
	}
	if info.LastError != "" {
		t.Errorf("expected empty LastError, got %q", info.LastError)
	}
}

func TestBaseSource_AddStreamCount(t *testing.T) {
	b := NewBaseSource("id-1", "Test", TypeDemo, true, 0)

	b.AddStreamCount(5)
	b.AddStreamCount(3)

	info := b.Info(context.Background())
	if info.StreamCount != 8 {
		t.Errorf("StreamCount = %d, want 8", info.StreamCount)
	}
}

func TestBaseSource_ClearState(t *testing.T) {
	b := NewBaseSource("id-1", "Test", TypeDemo, true, 0)

	b.SetRefreshResult(42)
	b.SetError("some error")
	b.ClearState()

	info := b.Info(context.Background())
	if info.StreamCount != 0 {
		t.Errorf("StreamCount = %d, want 0 after clear", info.StreamCount)
	}
	if info.LastError != "" {
		t.Errorf("LastError = %q, want empty after clear", info.LastError)
	}
}

func TestHTTPClientFor_DefaultClient(t *testing.T) {
	defaultClient := &http.Client{}
	got := HTTPClientFor(defaultClient, nil, false)
	if got != defaultClient {
		t.Error("expected default client when WG not requested")
	}
}

func TestHTTPClientFor_WGClient(t *testing.T) {
	defaultClient := &http.Client{}
	wgClient := &http.Client{}
	got := HTTPClientFor(defaultClient, wgClient, true)
	if got != wgClient {
		t.Error("expected WG client when useWG=true")
	}
}

func TestHTTPClientFor_WGRequestedButNilFallsBack(t *testing.T) {
	defaultClient := &http.Client{}
	got := HTTPClientFor(defaultClient, nil, true)
	if got != defaultClient {
		t.Error("expected default client when WG client is nil")
	}
}

func TestHTTPClientFor_NilDefault(t *testing.T) {
	got := HTTPClientFor(nil, nil, false)
	if got != http.DefaultClient {
		t.Error("expected http.DefaultClient when both nil")
	}
}

func TestSourceTypes(t *testing.T) {
	types := []struct {
		st   SourceType
		want string
	}{
		{TypeM3U, "m3u"},
		{TypeXtream, "xtream"},
		{TypeSATIP, "satip"},
		{TypeHDHR, "hdhr"},
		{TypeTVPStreams, "tvpstreams"},
		{TypeTrailers, "trailers"},
		{TypeDemo, "demo"},
		{TypeSpaceX, "spacex"},
	}

	for _, tt := range types {
		if string(tt.st) != tt.want {
			t.Errorf("SourceType %q != %q", tt.st, tt.want)
		}
	}
}

func TestStateConstants(t *testing.T) {
	if StateIdle != "idle" {
		t.Errorf("StateIdle = %q", StateIdle)
	}
	if StateScanning != "scanning" {
		t.Errorf("StateScanning = %q", StateScanning)
	}
	if StateDone != "done" {
		t.Errorf("StateDone = %q", StateDone)
	}
	if StateError != "error" {
		t.Errorf("StateError = %q", StateError)
	}
}
