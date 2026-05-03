package session

import (
	"context"
	"net/http"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/output"
)

type stubPlugin struct {
	mode output.DeliveryMode
}

func (s *stubPlugin) Mode() output.DeliveryMode                                    { return s.mode }
func (s *stubPlugin) PushVideo(data []byte, pts, dts int64, keyframe bool) error    { return nil }
func (s *stubPlugin) PushAudio(data []byte, pts, dts int64) error                   { return nil }
func (s *stubPlugin) PushSubtitle(data []byte, pts int64, duration int64) error     { return nil }
func (s *stubPlugin) EndOfStream()                                                  {}
func (s *stubPlugin) ResetForSeek()                                                 {}
func (s *stubPlugin) Stop()                                                         {}
func (s *stubPlugin) Status() output.PluginStatus                                   { return output.PluginStatus{Mode: s.mode, Healthy: true} }
func (s *stubPlugin) ServeHTTP(w http.ResponseWriter, r *http.Request)              {}
func (s *stubPlugin) Generation() int64                                             { return 0 }
func (s *stubPlugin) WaitReady(ctx context.Context) error                           { return nil }

func TestGetOrCreateNew(t *testing.T) {
	m := NewManager(t.TempDir())

	sess, created, err := m.GetOrCreate(context.Background(), "s1", "http://example.com/1", "Stream 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Fatal("expected created=true for new session")
	}
	if sess == nil {
		t.Fatal("expected non-nil session")
	}
	if sess.StreamID != "s1" {
		t.Fatalf("expected StreamID s1, got %s", sess.StreamID)
	}
}

func TestGetOrCreateExisting(t *testing.T) {
	m := NewManager(t.TempDir())

	sess1, _, _ := m.GetOrCreate(context.Background(), "s1", "http://example.com/1", "Stream 1")
	sess2, created, _ := m.GetOrCreate(context.Background(), "s1", "http://example.com/1", "Stream 1")

	if created {
		t.Fatal("expected created=false for existing session")
	}
	if sess1 != sess2 {
		t.Fatal("expected same session pointer")
	}
}

func TestGetUnknown(t *testing.T) {
	m := NewManager(t.TempDir())

	if m.Get("nonexistent") != nil {
		t.Fatal("expected nil for unknown streamID")
	}
}

func TestStop(t *testing.T) {
	m := NewManager(t.TempDir())

	m.GetOrCreate(context.Background(), "s1", "http://example.com/1", "Stream 1")
	m.Stop("s1")

	if m.Get("s1") != nil {
		t.Fatal("expected session removed after Stop")
	}
	if m.ActiveCount() != 0 {
		t.Fatalf("expected 0 active sessions, got %d", m.ActiveCount())
	}
}

func TestStopAll(t *testing.T) {
	m := NewManager(t.TempDir())

	m.GetOrCreate(context.Background(), "s1", "http://example.com/1", "Stream 1")
	m.GetOrCreate(context.Background(), "s2", "http://example.com/2", "Stream 2")
	m.GetOrCreate(context.Background(), "s3", "http://example.com/3", "Stream 3")

	m.StopAll()

	if m.ActiveCount() != 0 {
		t.Fatalf("expected 0 active sessions after StopAll, got %d", m.ActiveCount())
	}
}

func TestActiveCount(t *testing.T) {
	m := NewManager(t.TempDir())

	if m.ActiveCount() != 0 {
		t.Fatalf("expected 0, got %d", m.ActiveCount())
	}

	m.GetOrCreate(context.Background(), "s1", "http://example.com/1", "Stream 1")
	m.GetOrCreate(context.Background(), "s2", "http://example.com/2", "Stream 2")

	if m.ActiveCount() != 2 {
		t.Fatalf("expected 2, got %d", m.ActiveCount())
	}
}

func TestList(t *testing.T) {
	m := NewManager(t.TempDir())

	m.GetOrCreate(context.Background(), "s1", "http://example.com/1", "Stream 1")
	m.GetOrCreate(context.Background(), "s2", "http://example.com/2", "Stream 2")

	list := m.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(list))
	}

	ids := map[string]bool{}
	for _, s := range list {
		ids[s.StreamID] = true
	}
	if !ids["s1"] || !ids["s2"] {
		t.Fatal("expected both s1 and s2 in list")
	}
}

func TestAddPlugin(t *testing.T) {
	m := NewManager(t.TempDir())

	m.GetOrCreate(context.Background(), "s1", "http://example.com/1", "Stream 1")

	plugin := &stubPlugin{mode: output.DeliveryHLS}
	err := m.AddPlugin("s1", plugin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess := m.Get("s1")
	if sess.FanOut.PluginCount() != 1 {
		t.Fatalf("expected 1 plugin, got %d", sess.FanOut.PluginCount())
	}
}

func TestRemovePlugin(t *testing.T) {
	m := NewManager(t.TempDir())

	m.GetOrCreate(context.Background(), "s1", "http://example.com/1", "Stream 1")
	m.AddPlugin("s1", &stubPlugin{mode: output.DeliveryHLS})

	err := m.RemovePlugin("s1", output.DeliveryHLS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess := m.Get("s1")
	if sess.FanOut.PluginCount() != 0 {
		t.Fatalf("expected 0 plugins, got %d", sess.FanOut.PluginCount())
	}
}

func TestAddPluginNonexistent(t *testing.T) {
	m := NewManager(t.TempDir())

	err := m.AddPlugin("nonexistent", &stubPlugin{mode: output.DeliveryHLS})
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestRemovePluginNonexistent(t *testing.T) {
	m := NewManager(t.TempDir())

	err := m.RemovePlugin("nonexistent", output.DeliveryHLS)
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestStopUnknownIsNoop(t *testing.T) {
	m := NewManager(t.TempDir())
	m.Stop("nonexistent")
}
