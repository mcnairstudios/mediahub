package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
)

func mockPipelineRunner(_ *session.Session, _ session.PipelineConfig) (*session.PipelineResult, error) {
	return &session.PipelineResult{Info: &media.ProbeResult{
		Video: &media.VideoInfo{
			Index: 0,
			Codec: "h264",
			Width: 1920, Height: 1080,
			FramerateN: 25, FramerateD: 1,
		},
		AudioTracks: []media.AudioTrack{
			{Index: 1, Codec: "aac", Channels: 2, SampleRate: 48000},
		},
	}}, nil
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

func TestStartPlayback_DeliveryPluginAttached(t *testing.T) {
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
	for _, p := range plugins {
		switch p.Mode() {
		case output.DeliveryMSE, output.DeliveryHLS, output.DeliveryStream:
			hasDelivery = true
		}
	}
	if !hasDelivery {
		t.Error("expected a delivery plugin")
	}
}

func TestStartPlayback_TranscodeFieldsPassedToPipeline(t *testing.T) {
	streams := []media.Stream{
		{ID: "stream-1", Name: "Test Stream", URL: "http://example.com/stream",
			VideoCodec: "hevc", AudioCodec: "aac", Width: 1920, Height: 1080},
	}

	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*session.PipelineResult, error) {
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

	result, err := PlayRecording(context.Background(), deps, "rec-1", "/tmp/test.mp4", "Test Recording", 0, nil)
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

	first, err := PlayRecording(context.Background(), deps, "rec-1", "/tmp/test.mp4", "Test Recording", 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !first.IsNew {
		t.Error("expected first to be new")
	}

	second, err := PlayRecording(context.Background(), deps, "rec-1", "/tmp/test.mp4", "Test Recording", 0, nil)
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

	_, err := PlayRecording(context.Background(), deps, "rec-1", "/tmp/test.mp4", "Test Recording", 0, nil)
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

func TestStartPlayback_RecordingFailureDoesNotPreventPlayback(t *testing.T) {
	ss := store.NewMemoryStreamStore()
	ss.BulkUpsert(context.Background(), []media.Stream{
		{ID: "stream-1", Name: "Test Stream", URL: "http://example.com/stream"},
	})

	reg := output.NewRegistry()
	reg.Register(output.DeliveryMSE, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryMSE}, nil
	})
	reg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return nil, fmt.Errorf("simulated recording plugin failure")
	})

	deps := PlaybackDeps{
		StreamStore:    ss,
		SettingsStore:  newMockSettingsStore(nil),
		SessionMgr:     session.NewManager("/tmp/test-sessions-rec-fail"),
		Detector:       client.NewDetector(nil),
		OutputReg:      reg,
		Strategy:       strategy.Resolve,
		PipelineRunner: mockPipelineRunner,
	}
	defer deps.SessionMgr.StopAll()

	result, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("playback should succeed even when recording plugin fails: %v", err)
	}
	if result.Plugin == nil {
		t.Fatal("expected delivery plugin to be present")
	}

	hasRecord := false
	for _, p := range result.Session.FanOut.Plugins() {
		if p.Mode() == output.DeliveryRecord {
			hasRecord = true
		}
	}
	if hasRecord {
		t.Error("expected no recording plugin when creation fails")
	}
}

func TestStartPlayback_RecordingPanicDoesNotPreventPlayback(t *testing.T) {
	ss := store.NewMemoryStreamStore()
	ss.BulkUpsert(context.Background(), []media.Stream{
		{ID: "stream-1", Name: "Test Stream", URL: "http://example.com/stream"},
	})

	reg := output.NewRegistry()
	reg.Register(output.DeliveryMSE, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryMSE}, nil
	})
	reg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		panic("simulated recording plugin panic")
	})

	deps := PlaybackDeps{
		StreamStore:    ss,
		SettingsStore:  newMockSettingsStore(nil),
		SessionMgr:     session.NewManager("/tmp/test-sessions-rec-panic"),
		Detector:       client.NewDetector(nil),
		OutputReg:      reg,
		Strategy:       strategy.Resolve,
		PipelineRunner: mockPipelineRunner,
	}
	defer deps.SessionMgr.StopAll()

	result, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("playback should succeed even when recording plugin panics: %v", err)
	}
	if result.Plugin == nil {
		t.Fatal("expected delivery plugin to be present")
	}
}

func TestPlayRecording_IsNotLive(t *testing.T) {
	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*session.PipelineResult, error) {
		capturedCfg = cfg
		return mockPipelineRunner(sess, cfg)
	}

	deps := newTestPlaybackDeps(nil)
	deps.PipelineRunner = runner
	defer deps.SessionMgr.StopAll()

	result, err := PlayRecording(context.Background(), deps, "rec-1", "/tmp/test.mp4", "My Recording", 0, nil)
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

func TestStartPlayback_PipelineFailureReturnsError(t *testing.T) {
	deps := newTestPlaybackDeps([]media.Stream{
		{ID: "stream-1", Name: "Broken Stream", URL: "http://broken.example.com/stream"},
	})
	deps.PipelineRunner = func(_ *session.Session, _ session.PipelineConfig) (*session.PipelineResult, error) {
		return nil, fmt.Errorf("pipeline: open stream: connection refused")
	}
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err == nil {
		t.Fatal("expected error when pipeline fails")
	}

	if deps.SessionMgr.ActiveCount() != 0 {
		t.Fatalf("session should be cleaned up after pipeline failure, got %d active", deps.SessionMgr.ActiveCount())
	}
}

func TestStartPlayback_PipelineFailureErrorIncludesStreamName(t *testing.T) {
	deps := newTestPlaybackDeps([]media.Stream{
		{ID: "stream-1", Name: "BBC One HD", URL: "http://iptv.example.com/bbc1"},
	})
	deps.PipelineRunner = func(_ *session.Session, _ session.PipelineConfig) (*session.PipelineResult, error) {
		return nil, fmt.Errorf("demux: open input: timeout")
	}
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err == nil {
		t.Fatal("expected error")
	}
	errMsg := err.Error()
	if !contains(errMsg, "BBC One HD") {
		t.Errorf("error should include stream name, got: %s", errMsg)
	}
	if !contains(errMsg, "iptv.example.com") {
		t.Errorf("error should include stream URL, got: %s", errMsg)
	}
}

func TestStartPlayback_AudioOnlyStream(t *testing.T) {
	runner := func(_ *session.Session, _ session.PipelineConfig) (*session.PipelineResult, error) {
		return &session.PipelineResult{Info: &media.ProbeResult{
			AudioTracks: []media.AudioTrack{
				{Index: 0, Codec: "mp3", Channels: 2, SampleRate: 44100},
			},
		}}, nil
	}

	deps := newTestPlaybackDeps([]media.Stream{
		{ID: "radio-1", Name: "Radio Station", URL: "http://example.com/radio"},
	})
	deps.PipelineRunner = runner
	defer deps.SessionMgr.StopAll()

	result, err := StartPlayback(context.Background(), deps, "radio-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("audio-only stream should not error: %v", err)
	}
	if result.ProbeInfo.Video != nil {
		t.Error("expected nil video for audio-only stream")
	}
	if len(result.ProbeInfo.AudioTracks) != 1 {
		t.Errorf("expected 1 audio track, got %d", len(result.ProbeInfo.AudioTracks))
	}
}

func TestStartPlayback_VideoOnlyStream(t *testing.T) {
	runner := func(_ *session.Session, _ session.PipelineConfig) (*session.PipelineResult, error) {
		return &session.PipelineResult{Info: &media.ProbeResult{
			Video: &media.VideoInfo{
				Index: 0, Codec: "h264", Width: 1920, Height: 1080,
			},
		}}, nil
	}

	deps := newTestPlaybackDeps([]media.Stream{
		{ID: "cam-1", Name: "Security Camera", URL: "rtsp://example.com/cam1"},
	})
	deps.PipelineRunner = runner
	defer deps.SessionMgr.StopAll()

	result, err := StartPlayback(context.Background(), deps, "cam-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("video-only stream should not error: %v", err)
	}
	if result.ProbeInfo.Video == nil {
		t.Fatal("expected video info")
	}
	if len(result.ProbeInfo.AudioTracks) != 0 {
		t.Errorf("expected 0 audio tracks, got %d", len(result.ProbeInfo.AudioTracks))
	}
}

func TestStartPlayback_JoinExistingReturnsDelivery(t *testing.T) {
	deps := newTestPlaybackDepsWithSettings(
		[]media.Stream{{ID: "stream-1", Name: "Test", URL: "http://example.com/stream"}},
		map[string]string{"delivery": "hls"},
	)
	defer deps.SessionMgr.StopAll()

	first, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.Delivery != "hls" {
		t.Fatalf("expected hls delivery, got %s", first.Delivery)
	}

	second, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second.Delivery != "hls" {
		t.Errorf("joining session should return existing delivery, got %s", second.Delivery)
	}
	if second.IsNew {
		t.Error("expected IsNew=false for joined session")
	}
}

func TestPlayRecording_PipelineFailure(t *testing.T) {
	deps := newTestPlaybackDeps(nil)
	deps.PipelineRunner = func(_ *session.Session, _ session.PipelineConfig) (*session.PipelineResult, error) {
		return nil, fmt.Errorf("file not found: /nonexistent.mp4")
	}
	defer deps.SessionMgr.StopAll()

	_, err := PlayRecording(context.Background(), deps, "rec-1", "/nonexistent.mp4", "Bad Recording", 0, nil)
	if err == nil {
		t.Fatal("expected error for pipeline failure")
	}
	if !contains(err.Error(), "Bad Recording") {
		t.Errorf("error should contain recording title, got: %s", err.Error())
	}
	if deps.SessionMgr.ActiveCount() != 0 {
		t.Error("session should be cleaned up after failure")
	}
}

func TestStartPlayback_HWAccelFromSettings(t *testing.T) {
	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*session.PipelineResult, error) {
		capturedCfg = cfg
		return mockPipelineRunner(sess, cfg)
	}

	deps := newTestPlaybackDepsWithSettings(
		[]media.Stream{{ID: "stream-1", Name: "Test", URL: "http://example.com/stream",
			VideoCodec: "hevc", AudioCodec: "ac3", Width: 1920, Height: 1080, Interlaced: true}},
		map[string]string{"default_hwaccel": "vaapi"},
	)
	deps.PipelineRunner = runner
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCfg.HWAccel != "vaapi" {
		t.Errorf("expected HWAccel vaapi from settings, got %q", capturedCfg.HWAccel)
	}
}

func TestStartPlayback_HWAccelClientOverridesSettings(t *testing.T) {
	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*session.PipelineResult, error) {
		capturedCfg = cfg
		return mockPipelineRunner(sess, cfg)
	}

	ss := store.NewMemoryStreamStore()
	ss.BulkUpsert(context.Background(), []media.Stream{
		{ID: "stream-1", Name: "Test", URL: "http://example.com/stream",
			VideoCodec: "hevc", AudioCodec: "ac3", Width: 1920, Height: 1080},
	})

	reg := output.NewRegistry()
	reg.Register(output.DeliveryMSE, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryMSE}, nil
	})
	reg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryRecord}, nil
	})

	detector := client.NewDetector([]client.Client{
		{
			ID:         "plex",
			Name:       "Plex",
			Priority:   10,
			ListenPort: 8080,
			IsEnabled:  true,
			Profile:    client.Profile{HWAccel: "nvenc", VideoCodec: "h264", AudioCodec: "aac"},
		},
	})

	deps := PlaybackDeps{
		StreamStore:    ss,
		SettingsStore:  newMockSettingsStore(map[string]string{"default_hwaccel": "vaapi"}),
		SessionMgr:     session.NewManager("/tmp/test-hwaccel-override"),
		Detector:       detector,
		OutputReg:      reg,
		Strategy:       strategy.Resolve,
		PipelineRunner: runner,
	}
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCfg.HWAccel != "nvenc" {
		t.Errorf("expected client HWAccel nvenc to override settings, got %q", capturedCfg.HWAccel)
	}
}

func TestStartPlayback_DecodeHWAccelFromSettings(t *testing.T) {
	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*session.PipelineResult, error) {
		capturedCfg = cfg
		return mockPipelineRunner(sess, cfg)
	}

	deps := newTestPlaybackDepsWithSettings(
		[]media.Stream{{ID: "stream-1", Name: "Test", URL: "http://example.com/stream",
			VideoCodec: "hevc", AudioCodec: "ac3"}},
		map[string]string{
			"default_hwaccel":        "vaapi",
			"default_decode_hwaccel": "qsv",
		},
	)
	deps.PipelineRunner = runner
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCfg.DecodeHWAccel != "qsv" {
		t.Errorf("expected DecodeHWAccel qsv, got %q", capturedCfg.DecodeHWAccel)
	}
}

func TestStartPlayback_DecodeHWAccelFallsBackToEncode(t *testing.T) {
	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*session.PipelineResult, error) {
		capturedCfg = cfg
		return mockPipelineRunner(sess, cfg)
	}

	deps := newTestPlaybackDepsWithSettings(
		[]media.Stream{{ID: "stream-1", Name: "Test", URL: "http://example.com/stream",
			VideoCodec: "hevc", AudioCodec: "ac3"}},
		map[string]string{
			"default_hwaccel": "vaapi",
		},
	)
	deps.PipelineRunner = runner
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCfg.DecodeHWAccel != "vaapi" {
		t.Errorf("expected DecodeHWAccel to fall back to vaapi, got %q", capturedCfg.DecodeHWAccel)
	}
}

func TestStartPlayback_MaxBitDepthFromSettings(t *testing.T) {
	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*session.PipelineResult, error) {
		capturedCfg = cfg
		return mockPipelineRunner(sess, cfg)
	}

	deps := newTestPlaybackDepsWithSettings(
		[]media.Stream{{ID: "stream-1", Name: "Test", URL: "http://example.com/stream",
			VideoCodec: "h264", AudioCodec: "aac", BitDepth: 10}},
		map[string]string{"default_max_bit_depth": "8"},
	)
	deps.PipelineRunner = runner
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCfg.MaxBitDepth != 8 {
		t.Errorf("expected MaxBitDepth 8, got %d", capturedCfg.MaxBitDepth)
	}
}

func TestStartPlayback_EncoderDecoderFromSettings(t *testing.T) {
	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*session.PipelineResult, error) {
		capturedCfg = cfg
		return mockPipelineRunner(sess, cfg)
	}

	deps := newTestPlaybackDepsWithSettings(
		[]media.Stream{{ID: "stream-1", Name: "Test", URL: "http://example.com/stream",
			VideoCodec: "h264", AudioCodec: "aac"}},
		map[string]string{
			"encoder_h264": "h264_vaapi",
			"decoder_h264": "h264_cuvid",
		},
	)
	deps.Strategy = func(in strategy.Input, out strategy.Output) strategy.Decision {
		return strategy.Decision{
			NeedsTranscode:      true,
			NeedsAudioTranscode: true,
			VideoCodec:          "h264",
			AudioCodec:          "aac",
		}
	}
	deps.PipelineRunner = runner
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCfg.EncoderName != "h264_vaapi" {
		t.Errorf("expected EncoderName h264_vaapi, got %q", capturedCfg.EncoderName)
	}
	if capturedCfg.DecoderName != "h264_cuvid" {
		t.Errorf("expected DecoderName h264_cuvid, got %q", capturedCfg.DecoderName)
	}
}

func TestStartPlayback_AudioLanguageFromSettings(t *testing.T) {
	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*session.PipelineResult, error) {
		capturedCfg = cfg
		return mockPipelineRunner(sess, cfg)
	}

	deps := newTestPlaybackDepsWithSettings(
		[]media.Stream{{ID: "stream-1", Name: "Test", URL: "http://example.com/stream"}},
		map[string]string{"audio_language": "eng"},
	)
	deps.PipelineRunner = runner
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCfg.AudioLanguage != "eng" {
		t.Errorf("expected AudioLanguage eng, got %q", capturedCfg.AudioLanguage)
	}
}

func TestStartPlayback_ClientOverrideID(t *testing.T) {
	var capturedCfg session.PipelineConfig
	runner := func(sess *session.Session, cfg session.PipelineConfig) (*session.PipelineResult, error) {
		capturedCfg = cfg
		return mockPipelineRunner(sess, cfg)
	}

	ss := store.NewMemoryStreamStore()
	ss.BulkUpsert(context.Background(), []media.Stream{
		{ID: "stream-1", Name: "Test", URL: "http://example.com/stream",
			VideoCodec: "hevc", AudioCodec: "ac3"},
	})

	reg := output.NewRegistry()
	reg.Register(output.DeliveryMSE, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryMSE}, nil
	})
	reg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryRecord}, nil
	})

	mockCS := &mockClientStore{
		clients: map[string]*client.Client{
			"jellyfin-client": {
				ID:        "jellyfin-client",
				Name:      "Jellyfin",
				IsEnabled: true,
				Profile:   client.Profile{VideoCodec: "h264", AudioCodec: "aac", HWAccel: "qsv"},
			},
		},
	}

	deps := PlaybackDeps{
		StreamStore:      ss,
		SettingsStore:    newMockSettingsStore(nil),
		SessionMgr:       session.NewManager("/tmp/test-client-override"),
		Detector:         client.NewDetector(nil),
		ClientStore:      mockCS,
		OutputReg:        reg,
		Strategy:         strategy.Resolve,
		PipelineRunner:   runner,
		ClientOverrideID: "jellyfin-client",
	}
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "stream-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCfg.HWAccel != "qsv" {
		t.Errorf("expected HWAccel qsv from client override, got %q", capturedCfg.HWAccel)
	}
}

type mockClientStore struct {
	clients map[string]*client.Client
}

func (m *mockClientStore) Get(_ context.Context, id string) (*client.Client, error) {
	c, ok := m.clients[id]
	if !ok {
		return nil, nil
	}
	return c, nil
}

func (m *mockClientStore) List(_ context.Context) ([]client.Client, error) {
	var result []client.Client
	for _, c := range m.clients {
		result = append(result, *c)
	}
	return result, nil
}

func (m *mockClientStore) Create(_ context.Context, c *client.Client) error {
	m.clients[c.ID] = c
	return nil
}

func (m *mockClientStore) Update(_ context.Context, c *client.Client) error {
	m.clients[c.ID] = c
	return nil
}

func (m *mockClientStore) Delete(_ context.Context, id string) error {
	delete(m.clients, id)
	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestStartPlayback_PostProbeUpdatesStream(t *testing.T) {
	ss := store.NewMemoryStreamStore()
	ss.BulkUpsert(context.Background(), []media.Stream{
		{ID: "satip-1", Name: "SAT>IP Channel", URL: "rtsp://192.168.1.100/stream"},
	})

	runner := func(_ *session.Session, _ session.PipelineConfig) (*session.PipelineResult, error) {
		return &session.PipelineResult{Info: &media.ProbeResult{
			Video: &media.VideoInfo{
				Index: 0, Codec: "h265", Width: 1920, Height: 1080, BitDepth: 8, Interlaced: true,
			},
			AudioTracks: []media.AudioTrack{
				{Index: 1, Codec: "ac3", Channels: 6, SampleRate: 48000},
			},
		}}, nil
	}

	reg := output.NewRegistry()
	reg.Register(output.DeliveryMSE, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryMSE}, nil
	})
	reg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryRecord}, nil
	})

	deps := PlaybackDeps{
		StreamStore:    ss,
		SettingsStore:  newMockSettingsStore(nil),
		SessionMgr:     session.NewManager(t.TempDir()),
		Detector:       client.NewDetector(nil),
		OutputReg:      reg,
		Strategy:       strategy.Resolve,
		PipelineRunner: runner,
	}
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "satip-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := ss.Get(context.Background(), "satip-1")
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	if updated.VideoCodec != "h265" {
		t.Errorf("expected video codec h265, got %q", updated.VideoCodec)
	}
	if updated.AudioCodec != "ac3" {
		t.Errorf("expected audio codec ac3, got %q", updated.AudioCodec)
	}
	if updated.Width != 1920 {
		t.Errorf("expected width 1920, got %d", updated.Width)
	}
	if updated.Height != 1080 {
		t.Errorf("expected height 1080, got %d", updated.Height)
	}
	if !updated.Interlaced {
		t.Error("expected interlaced to be true")
	}
	if updated.BitDepth != 8 {
		t.Errorf("expected bit depth 8, got %d", updated.BitDepth)
	}
}

func TestStartPlayback_PostProbeDoesNotOverwriteExisting(t *testing.T) {
	ss := store.NewMemoryStreamStore()
	ss.BulkUpsert(context.Background(), []media.Stream{
		{ID: "known-1", Name: "Known Stream", URL: "http://example.com/stream",
			VideoCodec: "h264", AudioCodec: "aac", Width: 1280, Height: 720},
	})

	runner := func(_ *session.Session, _ session.PipelineConfig) (*session.PipelineResult, error) {
		return &session.PipelineResult{Info: &media.ProbeResult{
			Video: &media.VideoInfo{
				Index: 0, Codec: "h265", Width: 1920, Height: 1080,
			},
			AudioTracks: []media.AudioTrack{
				{Index: 1, Codec: "ac3", Channels: 6, SampleRate: 48000},
			},
		}}, nil
	}

	reg := output.NewRegistry()
	reg.Register(output.DeliveryMSE, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryMSE}, nil
	})
	reg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryRecord}, nil
	})

	deps := PlaybackDeps{
		StreamStore:    ss,
		SettingsStore:  newMockSettingsStore(nil),
		SessionMgr:     session.NewManager(t.TempDir()),
		Detector:       client.NewDetector(nil),
		OutputReg:      reg,
		Strategy:       strategy.Resolve,
		PipelineRunner: runner,
	}
	defer deps.SessionMgr.StopAll()

	_, err := StartPlayback(context.Background(), deps, "known-1", 8080, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := ss.Get(context.Background(), "known-1")
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	if updated.VideoCodec != "h264" {
		t.Errorf("expected video codec to remain h264, got %q", updated.VideoCodec)
	}
	if updated.AudioCodec != "aac" {
		t.Errorf("expected audio codec to remain aac, got %q", updated.AudioCodec)
	}
	if updated.Width != 1280 {
		t.Errorf("expected width to remain 1280, got %d", updated.Width)
	}
}
