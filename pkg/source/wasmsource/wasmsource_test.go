package wasmsource

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

// mockPlugin implements PluginCaller for testing.
type mockPlugin struct {
	pluginType    string
	refreshResult []byte
	refreshErr    error
	interactResult []byte
	interactErr    error
}

func (m *mockPlugin) Type() string { return m.pluginType }

func (m *mockPlugin) CallRefresh(_ context.Context, _ []byte) ([]byte, error) {
	return m.refreshResult, m.refreshErr
}

func (m *mockPlugin) CallInteract(_ context.Context, _ []byte) ([]byte, error) {
	return m.interactResult, m.interactErr
}

func TestRefreshMapsStreams(t *testing.T) {
	refreshData := map[string]any{
		"streams": []map[string]any{
			{"name": "Stream 1", "url": "http://example.com/stream1.m3u8", "group": "Live"},
			{"name": "Stream 2", "url": "http://example.com/stream2.m3u8", "group": "VOD", "logo": "http://logo.png"},
		},
	}
	refreshJSON, _ := json.Marshal(refreshData)

	streamStore := store.NewMemoryStreamStore()
	plugin := &mockPlugin{
		pluginType:    "testplugin",
		refreshResult: refreshJSON,
	}

	src := New(Config{
		ID:          "src-1",
		Name:        "Test Source",
		IsEnabled:   true,
		Plugin:      plugin,
		ConfigJSON:  []byte(`{"key":"val"}`),
		StreamStore: streamStore,
	})

	if err := src.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	streams, err := streamStore.ListBySource(context.Background(), "testplugin", "src-1")
	if err != nil {
		t.Fatalf("ListBySource: %v", err)
	}
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}

	// Verify stream fields.
	byName := make(map[string]media.Stream)
	for _, s := range streams {
		byName[s.Name] = s
	}

	s1 := byName["Stream 1"]
	if s1.URL != "http://example.com/stream1.m3u8" {
		t.Errorf("Stream 1 URL: got %q", s1.URL)
	}
	if s1.Group != "Live" {
		t.Errorf("Stream 1 Group: got %q", s1.Group)
	}
	if s1.SourceType != "testplugin" {
		t.Errorf("Stream 1 SourceType: got %q", s1.SourceType)
	}
	if s1.SourceID != "src-1" {
		t.Errorf("Stream 1 SourceID: got %q", s1.SourceID)
	}
	if !s1.IsActive {
		t.Error("Stream 1 should be active")
	}

	s2 := byName["Stream 2"]
	if s2.TvgLogo != "http://logo.png" {
		t.Errorf("Stream 2 Logo: got %q", s2.TvgLogo)
	}
}

func TestRefreshDeletesStaleStreams(t *testing.T) {
	streamStore := store.NewMemoryStreamStore()
	ctx := context.Background()

	// Pre-populate a stream that won't be in the refresh result.
	streamStore.BulkUpsert(ctx, []media.Stream{
		{ID: "old-stream", SourceType: "testplugin", SourceID: "src-1", Name: "Old", IsActive: true},
	})

	refreshData := map[string]any{
		"streams": []map[string]any{
			{"name": "New Stream", "url": "http://example.com/new.m3u8"},
		},
	}
	refreshJSON, _ := json.Marshal(refreshData)

	plugin := &mockPlugin{
		pluginType:    "testplugin",
		refreshResult: refreshJSON,
	}

	src := New(Config{
		ID:          "src-1",
		Name:        "Test Source",
		IsEnabled:   true,
		Plugin:      plugin,
		StreamStore: streamStore,
	})

	if err := src.Refresh(ctx); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	streams, _ := streamStore.ListBySource(ctx, "testplugin", "src-1")
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream after refresh (stale deleted), got %d", len(streams))
	}
	if streams[0].Name != "New Stream" {
		t.Errorf("expected 'New Stream', got %q", streams[0].Name)
	}
}

func TestDeterministicStreamID(t *testing.T) {
	id1 := DeterministicStreamID("src-1", "http://example.com/a.m3u8")
	id2 := DeterministicStreamID("src-1", "http://example.com/a.m3u8")
	id3 := DeterministicStreamID("src-1", "http://example.com/b.m3u8")
	id4 := DeterministicStreamID("src-2", "http://example.com/a.m3u8")

	if id1 != id2 {
		t.Error("same inputs should produce same ID")
	}
	if id1 == id3 {
		t.Error("different URLs should produce different IDs")
	}
	if id1 == id4 {
		t.Error("different source IDs should produce different IDs")
	}
	if len(id1) != 32 {
		t.Errorf("expected 32-char hex ID, got %d chars: %s", len(id1), id1)
	}
}

func TestRefreshCallsOnRefreshDone(t *testing.T) {
	refreshJSON, _ := json.Marshal(map[string]any{
		"streams": []map[string]any{
			{"name": "S1", "url": "http://a.com/1"},
		},
	})

	var called bool
	var gotSourceID string
	var gotCount int

	src := New(Config{
		ID:          "src-1",
		Name:        "Test",
		IsEnabled:   true,
		Plugin:      &mockPlugin{pluginType: "tp", refreshResult: refreshJSON},
		StreamStore: store.NewMemoryStreamStore(),
		OnRefreshDone: func(sourceID, etag string, streamCount int) {
			called = true
			gotSourceID = sourceID
			gotCount = streamCount
		},
	})

	src.Refresh(context.Background())
	if !called {
		t.Fatal("OnRefreshDone was not called")
	}
	if gotSourceID != "src-1" {
		t.Errorf("OnRefreshDone sourceID: got %q", gotSourceID)
	}
	if gotCount != 1 {
		t.Errorf("OnRefreshDone count: got %d", gotCount)
	}
}

func TestSourceType(t *testing.T) {
	src := New(Config{
		ID:          "src-1",
		Name:        "Test",
		IsEnabled:   true,
		Plugin:      &mockPlugin{pluginType: "myplugin"},
		StreamStore: store.NewMemoryStreamStore(),
	})
	if src.Type() != "myplugin" {
		t.Errorf("Type(): got %q", src.Type())
	}
}
