package orchestrator

import (
	"context"
	"net/http"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
)

type mockPlugin struct {
	mode output.DeliveryMode
}

func (m *mockPlugin) Mode() output.DeliveryMode                                 { return m.mode }
func (m *mockPlugin) PushVideo([]byte, int64, int64, bool) error                { return nil }
func (m *mockPlugin) PushAudio([]byte, int64, int64) error                      { return nil }
func (m *mockPlugin) PushSubtitle([]byte, int64, int64) error                   { return nil }
func (m *mockPlugin) EndOfStream()                                              {}
func (m *mockPlugin) ResetForSeek()                                             {}
func (m *mockPlugin) Stop()                                                     {}
func (m *mockPlugin) Status() output.PluginStatus                               { return output.PluginStatus{Mode: m.mode, Healthy: true} }
func (m *mockPlugin) ServeHTTP(http.ResponseWriter, *http.Request)              {}
func (m *mockPlugin) Generation() int64                                         { return 1 }
func (m *mockPlugin) WaitReady(_ context.Context) error                         { return nil }

func newTestPlaybackDeps(streams []media.Stream) PlaybackDeps {
	ss := store.NewMemoryStreamStore()
	for _, s := range streams {
		ss.BulkUpsert(context.Background(), []media.Stream{s})
	}

	reg := output.NewRegistry()
	reg.Register(output.DeliveryMSE, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryMSE}, nil
	})

	detector := client.NewDetector([]client.Client{
		{
			ID:         "browser",
			Name:       "Browser",
			Priority:   10,
			ListenPort: 8080,
			IsEnabled:  true,
		},
	})

	return PlaybackDeps{
		StreamStore: ss,
		SessionMgr:  session.NewManager("/tmp/test-sessions"),
		Detector:    detector,
		OutputReg:   reg,
		Strategy:    strategy.Resolve,
	}
}

func TestStartPlayback_NewSession(t *testing.T) {
	deps := newTestPlaybackDeps([]media.Stream{
		{ID: "stream-1", Name: "Test Stream", URL: "http://example.com/stream"},
	})
	defer deps.SessionMgr.StopAll()

	result, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsNew {
		t.Error("expected new session")
	}
	if result.Session == nil {
		t.Fatal("expected session")
	}
	if result.Plugin == nil {
		t.Fatal("expected plugin")
	}
}

func TestStartPlayback_JoinExisting(t *testing.T) {
	deps := newTestPlaybackDeps([]media.Stream{
		{ID: "stream-1", Name: "Test Stream", URL: "http://example.com/stream"},
	})
	defer deps.SessionMgr.StopAll()

	first, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !first.IsNew {
		t.Error("expected first session to be new")
	}

	second, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second.IsNew {
		t.Error("expected second call to join existing session")
	}
	if second.Session.ID != first.Session.ID {
		t.Error("expected same session ID")
	}
}

func TestStartPlayback_UnknownStream(t *testing.T) {
	deps := newTestPlaybackDeps(nil)
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "nonexistent", 8080, map[string]string{})
	if err == nil {
		t.Fatal("expected error for unknown stream")
	}
}

func TestStopPlayback(t *testing.T) {
	deps := newTestPlaybackDeps([]media.Stream{
		{ID: "stream-1", Name: "Test Stream", URL: "http://example.com/stream"},
	})

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if deps.SessionMgr.ActiveCount() != 1 {
		t.Fatalf("expected 1 active session, got %d", deps.SessionMgr.ActiveCount())
	}

	StopPlayback(deps, "stream-1")

	if deps.SessionMgr.ActiveCount() != 0 {
		t.Fatalf("expected 0 active sessions, got %d", deps.SessionMgr.ActiveCount())
	}
}

func TestSeek(t *testing.T) {
	deps := newTestPlaybackDeps([]media.Stream{
		{ID: "stream-1", Name: "Test Stream", URL: "http://example.com/stream"},
	})
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var seeked int64
	sess := deps.SessionMgr.Get("stream-1")
	sess.SetSeekFunc(func(posMs int64) {
		seeked = posMs
	})

	if err := Seek(deps, "stream-1", 5000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seeked != 5000 {
		t.Errorf("expected seek to 5000, got %d", seeked)
	}
}

func TestSeek_NoSession(t *testing.T) {
	deps := newTestPlaybackDeps(nil)

	err := Seek(deps, "nonexistent", 5000)
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}
