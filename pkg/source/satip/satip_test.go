package satip

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

func TestImplementsSource(t *testing.T) {
	var _ source.Source = (*Source)(nil)
}

func TestImplementsClearable(t *testing.T) {
	var _ source.Clearable = (*Source)(nil)
}

func TestImplementsDiscoverable(t *testing.T) {
	var _ source.Discoverable = (*Source)(nil)
}

func TestType(t *testing.T) {
	s := New(Config{
		ID:          "satip-1",
		Name:        "Living Room Tuner",
		Host:        "192.168.1.50",
		StreamStore: store.NewMemoryStreamStore(),
	})
	if s.Type() != "satip" {
		t.Fatalf("expected type satip, got %s", s.Type())
	}
}

func TestInfo(t *testing.T) {
	s := New(Config{
		ID:          "satip-1",
		Name:        "Living Room Tuner",
		Host:        "192.168.1.50",
		HTTPPort:    8875,
		IsEnabled:   true,
		MaxStreams:  4,
		StreamStore: store.NewMemoryStreamStore(),
	})

	info := s.Info(context.Background())
	if info.ID != "satip-1" {
		t.Fatalf("expected ID satip-1, got %s", info.ID)
	}
	if info.Name != "Living Room Tuner" {
		t.Fatalf("expected Name Living Room Tuner, got %s", info.Name)
	}
	if info.Type != "satip" {
		t.Fatalf("expected Type satip, got %s", info.Type)
	}
	if !info.IsEnabled {
		t.Fatal("expected IsEnabled true")
	}
	if info.MaxConcurrentStreams != 4 {
		t.Fatalf("expected MaxConcurrentStreams 4, got %d", info.MaxConcurrentStreams)
	}
	if info.StreamCount != 0 {
		t.Fatalf("expected StreamCount 0, got %d", info.StreamCount)
	}
	if info.LastRefreshed != nil {
		t.Fatal("expected nil LastRefreshed for new source")
	}
}

func TestInfoDefaultHTTPPort(t *testing.T) {
	s := New(Config{
		ID:          "satip-1",
		Name:        "Tuner",
		Host:        "192.168.1.50",
		StreamStore: store.NewMemoryStreamStore(),
	})

	if s.cfg.HTTPPort != 8875 {
		t.Fatalf("expected default HTTPPort 8875, got %d", s.cfg.HTTPPort)
	}
}

func TestStreamsEmpty(t *testing.T) {
	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "satip-1",
		Name:        "Tuner",
		Host:        "192.168.1.50",
		StreamStore: ss,
	})

	ids, err := s.Streams(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected 0 streams, got %d", len(ids))
	}
}

func TestStreamsFromStore(t *testing.T) {
	ss := store.NewMemoryStreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "stream-1", SourceType: "satip", SourceID: "satip-1", Name: "BBC One", URL: "rtsp://192.168.1.50/?freq=474&msys=dvbt2&pids=101,102", IsActive: true},
		{ID: "stream-2", SourceType: "satip", SourceID: "satip-1", Name: "BBC Two", URL: "rtsp://192.168.1.50/?freq=474&msys=dvbt2&pids=201,202", IsActive: true},
	}
	if err := ss.BulkUpsert(ctx, streams); err != nil {
		t.Fatalf("seeding streams: %v", err)
	}

	s := New(Config{
		ID:          "satip-1",
		Name:        "Tuner",
		Host:        "192.168.1.50",
		StreamStore: ss,
	})

	ids, err := s.Streams(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(ids))
	}
}

func TestDeleteStreams(t *testing.T) {
	ss := store.NewMemoryStreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "stream-1", SourceType: "satip", SourceID: "satip-1", Name: "BBC One", URL: "rtsp://192.168.1.50/?freq=474", IsActive: true},
		{ID: "stream-2", SourceType: "satip", SourceID: "satip-1", Name: "ITV", URL: "rtsp://192.168.1.50/?freq=490", IsActive: true},
	}
	if err := ss.BulkUpsert(ctx, streams); err != nil {
		t.Fatalf("seeding streams: %v", err)
	}

	s := New(Config{
		ID:          "satip-1",
		Name:        "Tuner",
		Host:        "192.168.1.50",
		StreamStore: ss,
	})

	if err := s.DeleteStreams(ctx); err != nil {
		t.Fatalf("delete streams failed: %v", err)
	}

	remaining, _ := ss.ListBySource(ctx, "satip", "satip-1")
	if len(remaining) != 0 {
		t.Fatalf("expected 0 streams after delete, got %d", len(remaining))
	}
}

func TestClear(t *testing.T) {
	ss := store.NewMemoryStreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "stream-1", SourceType: "satip", SourceID: "satip-1", Name: "BBC One", URL: "rtsp://192.168.1.50/?freq=474", IsActive: true},
	}
	if err := ss.BulkUpsert(ctx, streams); err != nil {
		t.Fatalf("seeding streams: %v", err)
	}

	s := New(Config{
		ID:          "satip-1",
		Name:        "Tuner",
		Host:        "192.168.1.50",
		StreamStore: ss,
	})
	s.mu.Lock()
	s.streamCount = 1
	s.lastError = "previous error"
	s.mu.Unlock()

	if err := s.Clear(ctx); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	remaining, _ := ss.ListBySource(ctx, "satip", "satip-1")
	if len(remaining) != 0 {
		t.Fatalf("expected 0 streams after clear, got %d", len(remaining))
	}

	info := s.Info(ctx)
	if info.StreamCount != 0 {
		t.Fatalf("expected StreamCount 0 after clear, got %d", info.StreamCount)
	}
	if info.LastError != "" {
		t.Fatalf("expected empty LastError after clear, got %s", info.LastError)
	}
}

func TestDeterministicStreamID(t *testing.T) {
	id1 := deterministicStreamID("satip-1", 1234)
	id2 := deterministicStreamID("satip-1", 1234)
	if id1 != id2 {
		t.Fatalf("expected deterministic IDs, got %s and %s", id1, id2)
	}
	if id1 == "" {
		t.Fatal("expected non-empty stream ID")
	}

	id3 := deterministicStreamID("satip-2", 1234)
	if id1 == id3 {
		t.Fatal("expected different IDs for different source IDs")
	}

	id4 := deterministicStreamID("satip-1", 5678)
	if id1 == id4 {
		t.Fatal("expected different IDs for different service IDs")
	}
}

func TestDeterministicStreamIDMatchesExpected(t *testing.T) {
	sourceID := "satip-1"
	serviceID := uint16(1234)
	content := fmt.Sprintf("%s:%d", sourceID, serviceID)
	h := sha256.Sum256([]byte(content))
	expected := fmt.Sprintf("%x", h[:16])

	got := deterministicStreamID(sourceID, serviceID)
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestStreamGroupClassification(t *testing.T) {
	tests := []struct {
		serviceType uint8
		want        string
	}{
		{0x01, "SD"},
		{0x02, "Radio"},
		{0x07, "Radio"},
		{0x0A, "Radio"},
		{0x11, "HD"},
		{0x19, "HD"},
		{0x1F, "HD"},
		{0x20, "HD"},
		{0xFF, "SD"},
	}

	for _, tt := range tests {
		got := streamGroup(tt.serviceType)
		if got != tt.want {
			t.Errorf("streamGroup(0x%02X) = %s, want %s", tt.serviceType, got, tt.want)
		}
	}
}

func TestDiscoverNotImplementedYet(t *testing.T) {
	s := New(Config{
		ID:          "satip-1",
		Name:        "Tuner",
		Host:        "192.168.1.50",
		StreamStore: store.NewMemoryStreamStore(),
	})

	devices, err := s.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices from stub, got %d", len(devices))
	}
}

func TestRefreshNoopForNow(t *testing.T) {
	s := New(Config{
		ID:          "satip-1",
		Name:        "Tuner",
		Host:        "192.168.1.50",
		StreamStore: store.NewMemoryStreamStore(),
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Fatalf("refresh should not error for MVP stub: %v", err)
	}
}

func TestDeleteStreamsIsolation(t *testing.T) {
	ss := store.NewMemoryStreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "satip", SourceID: "satip-1", Name: "A", IsActive: true},
		{ID: "s2", SourceType: "satip", SourceID: "satip-2", Name: "B", IsActive: true},
		{ID: "s3", SourceType: "m3u", SourceID: "m3u-1", Name: "C", IsActive: true},
	}
	if err := ss.BulkUpsert(ctx, streams); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	s := New(Config{
		ID:          "satip-1",
		Name:        "Tuner",
		Host:        "192.168.1.50",
		StreamStore: ss,
	})

	if err := s.DeleteStreams(ctx); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	remaining, _ := ss.ListBySource(ctx, "satip", "satip-1")
	if len(remaining) != 0 {
		t.Fatalf("expected 0 streams for satip-1, got %d", len(remaining))
	}

	other, _ := ss.ListBySource(ctx, "satip", "satip-2")
	if len(other) != 1 {
		t.Fatalf("expected 1 stream for satip-2, got %d", len(other))
	}

	m3uStreams, _ := ss.ListBySource(ctx, "m3u", "m3u-1")
	if len(m3uStreams) != 1 {
		t.Fatalf("expected 1 m3u stream untouched, got %d", len(m3uStreams))
	}
}
