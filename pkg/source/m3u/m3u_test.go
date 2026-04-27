package m3u

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

const testPlaylist = `#EXTM3U
#EXTINF:-1 tvg-id="bbc1.uk" tvg-name="BBC One" tvg-logo="http://logo.example.com/bbc1.png" group-title="UK",BBC One
http://example.com/stream/bbc1
#EXTINF:-1 tvg-id="bbc2.uk" tvg-name="BBC Two" tvg-logo="http://logo.example.com/bbc2.png" group-title="UK",BBC Two
http://example.com/stream/bbc2
#EXTINF:-1 tvg-id="itv1.uk" tvg-name="ITV" group-title="UK Entertainment",ITV
http://example.com/stream/itv
`

func TestImplementsSource(t *testing.T) {
	var _ source.Source = (*Source)(nil)
}

func TestImplementsConditionalRefresher(t *testing.T) {
	var _ source.ConditionalRefresher = (*Source)(nil)
}

func TestImplementsVPNRoutable(t *testing.T) {
	var _ source.VPNRoutable = (*Source)(nil)
}

func TestImplementsClearable(t *testing.T) {
	var _ source.Clearable = (*Source)(nil)
}

func TestType(t *testing.T) {
	s := New(Config{
		ID:          "test-src",
		Name:        "Test",
		URL:         "http://example.com/test.m3u",
		StreamStore: store.NewMemoryStreamStore(),
	})
	if s.Type() != "m3u" {
		t.Fatalf("expected type m3u, got %s", s.Type())
	}
}

func TestInfo(t *testing.T) {
	s := New(Config{
		ID:          "test-src",
		Name:        "Test M3U",
		URL:         "http://example.com/test.m3u",
		IsEnabled:   true,
		MaxStreams:  500,
		StreamStore: store.NewMemoryStreamStore(),
	})

	info := s.Info(context.Background())
	if info.ID != "test-src" {
		t.Fatalf("expected ID test-src, got %s", info.ID)
	}
	if info.Name != "Test M3U" {
		t.Fatalf("expected Name Test M3U, got %s", info.Name)
	}
	if info.Type != "m3u" {
		t.Fatalf("expected Type m3u, got %s", info.Type)
	}
	if !info.IsEnabled {
		t.Fatal("expected IsEnabled true")
	}
	if info.MaxConcurrentStreams != 500 {
		t.Fatalf("expected MaxConcurrentStreams 500, got %d", info.MaxConcurrentStreams)
	}
	if info.StreamCount != 0 {
		t.Fatalf("expected StreamCount 0, got %d", info.StreamCount)
	}
}

func TestRefresh(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc123"`)
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "src-1",
		Name:        "Test",
		URL:         ts.URL,
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, err := ss.ListBySource(context.Background(), "m3u", "src-1")
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "src-abc",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "m3u", "src-abc")
	for _, stream := range streams {
		if stream.SourceType != "m3u" {
			t.Fatalf("expected SourceType m3u, got %s", stream.SourceType)
		}
		if stream.SourceID != "src-abc" {
			t.Fatalf("expected SourceID src-abc, got %s", stream.SourceID)
		}
	}
}

func TestRefreshStreamContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "src-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "m3u", "src-1")
	byName := make(map[string]struct{})
	for _, stream := range streams {
		byName[stream.Name] = struct{}{}
		if stream.URL == "" {
			t.Fatalf("expected URL to be set for %s", stream.Name)
		}
		if stream.ID == "" {
			t.Fatalf("expected ID to be set for %s", stream.Name)
		}
		if !stream.IsActive {
			t.Fatalf("expected IsActive for %s", stream.Name)
		}
	}

	for _, name := range []string{"BBC One", "BBC Two", "ITV"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("expected stream %s not found", name)
		}
	}
}

func TestStreams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "src-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	ids, err := s.Streams(context.Background())
	if err != nil {
		t.Fatalf("streams failed: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 stream IDs, got %d", len(ids))
	}
}

func TestDeleteStreams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "src-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	if err := s.DeleteStreams(context.Background()); err != nil {
		t.Fatalf("delete streams failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "m3u", "src-1")
	if len(streams) != 0 {
		t.Fatalf("expected 0 streams after delete, got %d", len(streams))
	}
}

func TestClear(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "src-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	info := s.Info(context.Background())
	if info.StreamCount != 3 {
		t.Fatalf("expected 3 streams before clear, got %d", info.StreamCount)
	}

	if err := s.Clear(context.Background()); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "m3u", "src-1")
	if len(streams) != 0 {
		t.Fatalf("expected 0 streams after clear, got %d", len(streams))
	}

	info = s.Info(context.Background())
	if info.StreamCount != 0 {
		t.Fatalf("expected StreamCount 0 after clear, got %d", info.StreamCount)
	}
}

func TestUsesVPN(t *testing.T) {
	s := New(Config{
		ID:           "src-1",
		Name:         "Test",
		URL:          "http://example.com/test.m3u",
		UseWireGuard: true,
		StreamStore:  store.NewMemoryStreamStore(),
	})
	if !s.UsesVPN() {
		t.Fatal("expected UsesVPN true")
	}

	s2 := New(Config{
		ID:          "src-2",
		Name:        "Test2",
		URL:         "http://example.com/test.m3u",
		StreamStore: store.NewMemoryStreamStore(),
	})
	if s2.UsesVPN() {
		t.Fatal("expected UsesVPN false")
	}
}

func TestSupportsConditionalRefresh(t *testing.T) {
	s := New(Config{
		ID:          "src-1",
		Name:        "Test",
		URL:         "http://example.com/test.m3u",
		StreamStore: store.NewMemoryStreamStore(),
	})
	if !s.SupportsConditionalRefresh() {
		t.Fatal("expected SupportsConditionalRefresh true")
	}
}

func TestConditionalRefreshETag(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		etag := `"playlist-v1"`
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "src-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	streams, _ := ss.ListBySource(context.Background(), "m3u", "src-1")
	if len(streams) != 3 {
		t.Fatalf("expected 3 streams after first refresh, got %d", len(streams))
	}

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("second refresh failed: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}

	streams, _ = ss.ListBySource(context.Background(), "m3u", "src-1")
	if len(streams) != 3 {
		t.Fatalf("expected 3 streams still present after conditional refresh, got %d", len(streams))
	}
}

func TestDeterministicStreamID(t *testing.T) {
	sourceID := "src-1"
	url := "http://example.com/stream/bbc1"
	id1 := streamID(sourceID, url)
	id2 := streamID(sourceID, url)
	if id1 != id2 {
		t.Fatalf("expected deterministic IDs, got %s and %s", id1, id2)
	}
	if id1 == "" {
		t.Fatal("expected non-empty stream ID")
	}

	id3 := streamID("other-src", url)
	if id1 == id3 {
		t.Fatal("expected different IDs for different source IDs")
	}
}

func TestDuplicateURLsDeduped(t *testing.T) {
	playlist := `#EXTM3U
#EXTINF:-1,Channel A
http://example.com/stream/same
#EXTINF:-1,Channel B
http://example.com/stream/same
`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, playlist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "src-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "m3u", "src-1")
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream (deduped), got %d", len(streams))
	}
}

func TestStaleStreamsRemoved(t *testing.T) {
	playlist1 := `#EXTM3U
#EXTINF:-1,Channel A
http://example.com/stream/a
#EXTINF:-1,Channel B
http://example.com/stream/b
`
	playlist2 := `#EXTM3U
#EXTINF:-1,Channel A
http://example.com/stream/a
`

	current := playlist1
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, current)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "src-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())
	streams, _ := ss.ListBySource(context.Background(), "m3u", "src-1")
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}

	current = playlist2
	_ = s.Refresh(context.Background())
	streams, _ = ss.ListBySource(context.Background(), "m3u", "src-1")
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream after stale removal, got %d", len(streams))
	}
}

func streamID(sourceID, url string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + url))
	return fmt.Sprintf("%x", h[:16])
}
