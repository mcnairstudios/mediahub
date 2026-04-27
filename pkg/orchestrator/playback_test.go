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

func mockPipelineRunner(_ *session.Session, _ session.PipelineConfig) (*media.ProbeResult, error) {
	return &media.ProbeResult{
		Video: &media.VideoInfo{
			Index: 0,
			Codec: "h264",
			Width: 1920, Height: 1080,
			FramerateN: 25, FramerateD: 1,
		},
		AudioTracks: []media.AudioTrack{
			{Index: 1, Codec: "aac", Channels: 2, SampleRate: 48000},
		},
	}, nil
}

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

type mockSettingsStore struct {
	data map[string]string
}

func newMockSettingsStore(data map[string]string) *mockSettingsStore {
	if data == nil {
		data = make(map[string]string)
	}
	return &mockSettingsStore{data: data}
}

func (m *mockSettingsStore) Get(_ context.Context, key string) (string, error) {
	return m.data[key], nil
}
func (m *mockSettingsStore) Set(_ context.Context, key, value string) error {
	m.data[key] = value
	return nil
}
func (m *mockSettingsStore) List(_ context.Context) (map[string]string, error) {
	return m.data, nil
}

func newTestPlaybackDeps(streams []media.Stream) PlaybackDeps {
	return newTestPlaybackDepsWithSettings(streams, nil)
}

func newTestPlaybackDepsWithSettings(streams []media.Stream, settings map[string]string) PlaybackDeps {
	ss := store.NewMemoryStreamStore()
	for _, s := range streams {
		ss.BulkUpsert(context.Background(), []media.Stream{s})
	}

	reg := output.NewRegistry()
	reg.Register(output.DeliveryMSE, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryMSE}, nil
	})
	reg.Register(output.DeliveryHLS, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryHLS}, nil
	})
	reg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryRecord}, nil
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
		StreamStore:    ss,
		SettingsStore:  newMockSettingsStore(settings),
		SessionMgr:     session.NewManager("/tmp/test-sessions"),
		Detector:       detector,
		OutputReg:      reg,
		Strategy:       strategy.Resolve,
		PipelineRunner: mockPipelineRunner,
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

func TestStartPlayback_DeliveryFromSettings(t *testing.T) {
	deps := newTestPlaybackDepsWithSettings(
		[]media.Stream{{ID: "stream-1", Name: "Test Stream", URL: "http://example.com/stream"}},
		map[string]string{"delivery": "hls"},
	)
	defer deps.SessionMgr.StopAll()

	result, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Delivery != "hls" {
		t.Errorf("expected delivery hls, got %s", result.Delivery)
	}
}

func TestStartPlayback_DefaultDeliveryMSE(t *testing.T) {
	deps := newTestPlaybackDepsWithSettings(
		[]media.Stream{{ID: "stream-1", Name: "Test Stream", URL: "http://example.com/stream"}},
		nil,
	)
	defer deps.SessionMgr.StopAll()

	result, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Delivery != "mse" {
		t.Errorf("expected delivery mse, got %s", result.Delivery)
	}
}

func TestStartPlayback_RecordingAlwaysAttached(t *testing.T) {
	deps := newTestPlaybackDeps([]media.Stream{
		{ID: "stream-1", Name: "Test Stream", URL: "http://example.com/stream"},
	})
	defer deps.SessionMgr.StopAll()

	result, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plugins := result.Session.FanOut.Plugins()
	hasDelivery := false
	hasRecord := false
	for _, p := range plugins {
		switch p.Mode() {
		case output.DeliveryMSE, output.DeliveryHLS, output.DeliveryStream:
			hasDelivery = true
		case output.DeliveryRecord:
			hasRecord = true
		}
	}
	if !hasDelivery {
		t.Error("expected a delivery plugin")
	}
	if !hasRecord {
		t.Error("expected a recording plugin always attached")
	}
}

func TestStartPlayback_TranscodeFieldsPassedToPipeline(t *testing.T) {
	streams := []media.Stream{
		{ID: "stream-1", Name: "Test Stream", URL: "http://example.com/stream",
			VideoCodec: "hevc", AudioCodec: "aac", Width: 1920, Height: 1080},
	}

	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*media.ProbeResult, error) {
		capturedCfg = cfg
		return mockPipelineRunner(sess, cfg)
	}

	deps := newTestPlaybackDeps(streams)
	deps.PipelineRunner = runner
	deps.Strategy = func(in strategy.Input, out strategy.Output) strategy.Decision {
		return strategy.Decision{
			NeedsTranscode: true,
			VideoCodec:     "h264",
			AudioCodec:     "aac",
			HWAccel:        "vaapi",
			Deinterlace:    true,
		}
	}
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !capturedCfg.NeedsTranscode {
		t.Error("expected NeedsTranscode=true")
	}
	if capturedCfg.OutputCodec != "h264" {
		t.Errorf("expected OutputCodec h264, got %s", capturedCfg.OutputCodec)
	}
	if capturedCfg.HWAccel != "vaapi" {
		t.Errorf("expected HWAccel vaapi, got %s", capturedCfg.HWAccel)
	}
	if !capturedCfg.Deinterlace {
		t.Error("expected Deinterlace=true")
	}
}

func TestPlayRecording_NewSession(t *testing.T) {
	deps := newTestPlaybackDeps(nil)
	defer deps.SessionMgr.StopAll()

	result, err := PlayRecording(context.Background(), deps, "rec-1", "/tmp/test.mp4", "Test Recording")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsNew {
		t.Error("expected new session")
	}
	if result.Session == nil {
		t.Fatal("expected session")
	}
	if result.Session.StreamID != "rec:rec-1" {
		t.Errorf("expected stream ID rec:rec-1, got %s", result.Session.StreamID)
	}
	if result.Session.StreamURL != "/tmp/test.mp4" {
		t.Errorf("expected URL /tmp/test.mp4, got %s", result.Session.StreamURL)
	}
}

func TestPlayRecording_JoinExisting(t *testing.T) {
	deps := newTestPlaybackDeps(nil)
	defer deps.SessionMgr.StopAll()

	first, err := PlayRecording(context.Background(), deps, "rec-1", "/tmp/test.mp4", "Test Recording")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !first.IsNew {
		t.Error("expected first to be new")
	}

	second, err := PlayRecording(context.Background(), deps, "rec-1", "/tmp/test.mp4", "Test Recording")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second.IsNew {
		t.Error("expected second to join existing")
	}
	if second.Session.ID != first.Session.ID {
		t.Error("expected same session ID")
	}
}

func TestStopRecordingPlayback_Works(t *testing.T) {
	deps := newTestPlaybackDeps(nil)

	_, err := PlayRecording(context.Background(), deps, "rec-1", "/tmp/test.mp4", "Test Recording")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if deps.SessionMgr.ActiveCount() != 1 {
		t.Fatalf("expected 1 active session, got %d", deps.SessionMgr.ActiveCount())
	}

	StopRecordingPlayback(deps, "rec-1")

	if deps.SessionMgr.ActiveCount() != 0 {
		t.Fatalf("expected 0 active sessions, got %d", deps.SessionMgr.ActiveCount())
	}
}

func TestPlayRecording_IsNotLive(t *testing.T) {
	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*media.ProbeResult, error) {
		capturedCfg = cfg
		return mockPipelineRunner(sess, cfg)
	}

	deps := newTestPlaybackDeps(nil)
	deps.PipelineRunner = runner
	defer deps.SessionMgr.StopAll()

	result, err := PlayRecording(context.Background(), deps, "rec-1", "/tmp/test.mp4", "My Recording")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCfg.StreamURL != "/tmp/test.mp4" {
		t.Errorf("expected stream URL /tmp/test.mp4, got %s", capturedCfg.StreamURL)
	}

	plugins := result.Session.FanOut.Plugins()
	for _, p := range plugins {
		if p.Mode() == output.DeliveryRecord {
			t.Error("recording playback should not have a recording plugin attached")
		}
	}
}
