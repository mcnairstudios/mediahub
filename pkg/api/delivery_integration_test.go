package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/favorite"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
)

type mockOutputPlugin struct {
	mode output.DeliveryMode
}

func (m *mockOutputPlugin) Mode() output.DeliveryMode                      { return m.mode }
func (m *mockOutputPlugin) PushVideo([]byte, int64, int64, int64, bool) error     { return nil }
func (m *mockOutputPlugin) PushAudio([]byte, int64, int64, int64) error           { return nil }
func (m *mockOutputPlugin) PushSubtitle([]byte, int64, int64) error               { return nil }
func (m *mockOutputPlugin) EndOfStream()                                   {}
func (m *mockOutputPlugin) ResetForSeek()                                  {}
func (m *mockOutputPlugin) Stop()                                          {}
func (m *mockOutputPlugin) Status() output.PluginStatus                    { return output.PluginStatus{Mode: m.mode, Healthy: true} }
func (m *mockOutputPlugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch m.mode {
	case output.DeliveryHLS:
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte("#EXTM3U\n#EXT-X-VERSION:3\n"))
	case output.DeliveryMSE:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.URL.Path == "/video/init" || r.URL.Path == "/audio/init" {
			w.Header().Set("Content-Type", "video/mp4")
		} else {
			w.Header().Set("Content-Type", "video/mp4")
		}
		w.Write([]byte{0x00})
	case output.DeliveryDASH:
		w.Header().Set("Content-Type", "application/dash+xml")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte(`<?xml version="1.0"?><MPD/>`))
	case output.DeliveryWebRTC:
		w.Header().Set("Content-Type", "application/sdp")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte("v=0\r\n"))
	}
}
func (m *mockOutputPlugin) Generation() int64               { return 1 }
func (m *mockOutputPlugin) WaitReady(_ context.Context) error { return nil }

func testPipelineRunner(_ *session.Session, _ session.PipelineConfig) (*session.PipelineResult, error) {
	return &session.PipelineResult{
		Info: &media.ProbeResult{
			Video: &media.VideoInfo{
				Index:      0,
				Codec:      "h264",
				Width:      1920,
				Height:     1080,
				FramerateN: 25,
				FramerateD: 1,
			},
			AudioTracks: []media.AudioTrack{
				{Index: 1, Codec: "aac", Channels: 2, SampleRate: 48000},
			},
		},
	}, nil
}

type deliveryTestEnv struct {
	server        *Server
	httpServer    *httptest.Server
	adminToken    string
	standardToken string
}

func newDeliveryTestEnv(t *testing.T) *deliveryTestEnv {
	t.Helper()

	userStore := auth.NewMemoryUserStore()
	authService := auth.NewJWTService(userStore, "test-delivery-secret")

	ctx := context.Background()
	_, err := authService.CreateUser(ctx, "admin", "adminpass", "", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	_, err = authService.CreateUser(ctx, "viewer", "viewerpass", "", auth.RoleStandard)
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	adminToken, err := authService.Login(ctx, "admin", "adminpass")
	if err != nil {
		t.Fatalf("admin login: %v", err)
	}
	standardToken, err := authService.Login(ctx, "viewer", "viewerpass")
	if err != nil {
		t.Fatalf("viewer login: %v", err)
	}

	streamStore := store.NewMemoryStreamStore()
	streamStore.BulkUpsert(ctx, []media.Stream{
		{ID: "stream-1", Name: "BBC One", URL: "http://example.com/bbc1", SourceType: "m3u", SourceID: "src-1", IsActive: true},
		{ID: "stream-2", Name: "BBC Two", URL: "http://example.com/bbc2", SourceType: "m3u", SourceID: "src-1", IsActive: true},
	})

	channelStore := store.NewMemoryChannelStore()
	settingsStore := store.NewMemorySettingsStore()
	settingsStore.Set(ctx, "base_url", "http://localhost:8080")

	recordingStore := store.NewMemoryRecordingStore()
	epgSourceStore := store.NewMemoryEPGSourceStore()
	sourceConfigStore := sourceconfig.NewMemoryStore()
	programStore := store.NewMemoryProgramStore()
	groupStore := store.NewMemoryGroupStore()

	reg := output.NewRegistry()
	reg.Register(output.DeliveryMSE, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockOutputPlugin{mode: output.DeliveryMSE}, nil
	})
	reg.Register(output.DeliveryHLS, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockOutputPlugin{mode: output.DeliveryHLS}, nil
	})
	reg.Register(output.DeliveryDASH, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockOutputPlugin{mode: output.DeliveryDASH}, nil
	})
	reg.Register(output.DeliveryWebRTC, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockOutputPlugin{mode: output.DeliveryWebRTC}, nil
	})
	reg.Register(output.DeliveryStream, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockOutputPlugin{mode: output.DeliveryStream}, nil
	})
	reg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockOutputPlugin{mode: output.DeliveryRecord}, nil
	})

	deps := OrchestratorDeps{
		StreamStore:       streamStore,
		ChannelStore:      channelStore,
		SettingsStore:     settingsStore,
		SourceConfigStore: sourceConfigStore,
		SessionMgr:        session.NewManager(t.TempDir()),
		Detector:          client.NewDetector(nil),
		OutputReg:         reg,
		SourceReg:         source.NewRegistry(),
		RecordingStore:    recordingStore,
		AuthService:       authService,
		EPGSourceStore:    epgSourceStore,
		ProgramStore:      programStore,
		GroupStore:        groupStore,
		FavoriteStore:     favorite.NewMemoryStore(),
		Strategy: func(in strategy.Input, out strategy.Output) strategy.Decision {
			return strategy.Resolve(in, out)
		},
		PipelineRunner: testPipelineRunner,
	}

	srv := NewServer(deps)
	ts := httptest.NewServer(srv.Handler())

	return &deliveryTestEnv{
		server:        srv,
		httpServer:    ts,
		adminToken:    adminToken,
		standardToken: standardToken,
	}
}

func (e *deliveryTestEnv) close() {
	e.server.deps.SessionMgr.StopAll()
	e.httpServer.Close()
}

func (e *deliveryTestEnv) request(method, path string, body any, token string) *http.Response {
	var bodyReader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, _ := http.NewRequest(method, e.httpServer.URL+path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	return resp
}

func startPlaybackWithDelivery(env *deliveryTestEnv, streamID, delivery string) (map[string]any, int) {
	resp := env.request("POST", "/api/play/"+streamID, map[string]any{
		"delivery": delivery,
	}, env.standardToken)

	var result map[string]any
	decodeBody(resp, &result)
	return result, resp.StatusCode
}

func TestMSEDeliveryFullPath(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result, status := startPlaybackWithDelivery(env, "stream-1", "mse")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if result["delivery"] != "mse" {
		t.Fatalf("expected delivery=mse, got %v", result["delivery"])
	}

	endpoints, ok := result["endpoints"].(map[string]any)
	if !ok {
		t.Fatal("expected endpoints map in response")
	}

	expectedKeys := []string{"video_init", "audio_init", "video_segment", "audio_segment"}
	for _, key := range expectedKeys {
		if _, ok := endpoints[key]; !ok {
			t.Errorf("missing endpoint key %q", key)
		}
	}

	videoInit := endpoints["video_init"].(string)
	resp := env.request("GET", videoInit, nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET video_init: expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "video/mp4" {
		t.Errorf("video_init Content-Type = %q, want video/mp4", ct)
	}
	resp.Body.Close()

	audioInit := endpoints["audio_init"].(string)
	resp = env.request("GET", audioInit, nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET audio_init: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	videoSeg := endpoints["video_segment"].(string)
	resp = env.request("GET", videoSeg, nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET video_segment: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	audioSeg := endpoints["audio_segment"].(string)
	resp = env.request("GET", audioSeg, nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET audio_segment: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHLSDeliveryFullPath(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result, status := startPlaybackWithDelivery(env, "stream-1", "hls")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if result["delivery"] != "hls" {
		t.Fatalf("expected delivery=hls, got %v", result["delivery"])
	}

	endpoints, ok := result["endpoints"].(map[string]any)
	if !ok {
		t.Fatal("expected endpoints map in response")
	}

	playlist, ok := endpoints["playlist"].(string)
	if !ok {
		t.Fatal("missing playlist endpoint")
	}

	resp := env.request("GET", playlist, nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET playlist: expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/vnd.apple.mpegurl" {
		t.Errorf("playlist Content-Type = %q, want application/vnd.apple.mpegurl", ct)
	}
	resp.Body.Close()
}

func TestDASHDeliveryFullPath(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result, status := startPlaybackWithDelivery(env, "stream-1", "dash")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if result["delivery"] != "dash" {
		t.Fatalf("expected delivery=dash, got %v", result["delivery"])
	}

	endpoints, ok := result["endpoints"].(map[string]any)
	if !ok {
		t.Fatal("expected endpoints map in response")
	}

	manifest, ok := endpoints["manifest"].(string)
	if !ok {
		t.Fatal("missing manifest endpoint")
	}

	resp := env.request("GET", manifest, nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET manifest: expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/dash+xml" {
		t.Errorf("manifest Content-Type = %q, want application/dash+xml", ct)
	}
	resp.Body.Close()
}

func TestWebRTCDeliveryFullPath(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result, status := startPlaybackWithDelivery(env, "stream-1", "webrtc")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if result["delivery"] != "webrtc" {
		t.Fatalf("expected delivery=webrtc, got %v", result["delivery"])
	}

	endpoints, ok := result["endpoints"].(map[string]any)
	if !ok {
		t.Fatal("expected endpoints map in response")
	}

	whep, ok := endpoints["whep"].(string)
	if !ok {
		t.Fatal("missing whep endpoint")
	}

	resp := env.request("POST", whep, "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\n", env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST whep: expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/sdp" {
		t.Errorf("whep Content-Type = %q, want application/sdp", ct)
	}
	resp.Body.Close()
}

func TestStreamDeliveryFullPath(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result, status := startPlaybackWithDelivery(env, "stream-1", "stream")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if result["delivery"] != "stream" {
		t.Fatalf("expected delivery=stream, got %v", result["delivery"])
	}

	endpoints, ok := result["endpoints"].(map[string]any)
	if !ok {
		t.Fatal("expected endpoints map in response")
	}

	_, ok = endpoints["stream"].(string)
	if !ok {
		t.Fatal("missing stream endpoint")
	}
}

func TestDeliverySwitching(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result, status := startPlaybackWithDelivery(env, "stream-1", "mse")
	if status != http.StatusOK {
		t.Fatalf("start MSE: expected 200, got %d", status)
	}
	if result["delivery"] != "mse" {
		t.Fatalf("expected delivery=mse, got %v", result["delivery"])
	}

	resp := env.request("DELETE", "/api/play/stream-1", nil, env.standardToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("stop: expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	result, status = startPlaybackWithDelivery(env, "stream-1", "hls")
	if status != http.StatusOK {
		t.Fatalf("start HLS: expected 200, got %d", status)
	}
	if result["delivery"] != "hls" {
		t.Fatalf("expected delivery=hls after switch, got %v", result["delivery"])
	}

	endpoints, ok := result["endpoints"].(map[string]any)
	if !ok {
		t.Fatal("expected endpoints map after switch")
	}
	if _, ok := endpoints["playlist"]; !ok {
		t.Error("expected playlist endpoint for HLS delivery")
	}
}

func TestDeliverySwitchWithoutStopRestartsSession(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result, status := startPlaybackWithDelivery(env, "stream-1", "mse")
	if status != http.StatusOK {
		t.Fatalf("start MSE: expected 200, got %d", status)
	}
	if result["delivery"] != "mse" {
		t.Fatalf("expected delivery=mse, got %v", result["delivery"])
	}

	result, status = startPlaybackWithDelivery(env, "stream-1", "hls")
	if status != http.StatusOK {
		t.Fatalf("switch to HLS: expected 200, got %d", status)
	}
	if result["delivery"] != "hls" {
		t.Fatalf("expected delivery=hls after switch, got %v", result["delivery"])
	}
}

func TestDefaultDeliveryFallsBackToMSE(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/stream-1", nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	decodeBody(resp, &result)

	if result["delivery"] != "mse" {
		t.Fatalf("expected default delivery=mse, got %v", result["delivery"])
	}
}

func TestPlaybackResponseContainsDecision(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result, status := startPlaybackWithDelivery(env, "stream-1", "mse")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	decision, ok := result["decision"].(map[string]any)
	if !ok {
		t.Fatal("expected decision map in response")
	}

	if _, ok := decision["video_codec"]; !ok {
		t.Error("missing video_codec in decision")
	}
	if _, ok := decision["audio_codec"]; !ok {
		t.Error("missing audio_codec in decision")
	}
	if _, ok := decision["needs_transcode"]; !ok {
		t.Error("missing needs_transcode in decision")
	}
}

func TestPlaybackResponseContainsProbeInfo(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result, status := startPlaybackWithDelivery(env, "stream-1", "mse")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	probeInfo, ok := result["probe_info"].(map[string]any)
	if !ok {
		t.Fatal("expected probe_info map in response")
	}

	video, ok := probeInfo["video"].(map[string]any)
	if !ok {
		t.Fatal("expected video in probe_info")
	}
	if video["codec"] != "h264" {
		t.Errorf("video codec = %v, want h264", video["codec"])
	}
	if video["width"] != float64(1920) {
		t.Errorf("video width = %v, want 1920", video["width"])
	}

	audio, ok := probeInfo["audio"].(map[string]any)
	if !ok {
		t.Fatal("expected audio in probe_info")
	}
	if audio["codec"] != "aac" {
		t.Errorf("audio codec = %v, want aac", audio["codec"])
	}
}

func TestPlaybackRequiresAuth(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/stream-1", map[string]any{
		"delivery": "mse",
	}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPlaybackServeRequiresAuth(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	paths := []string{
		"/api/play/stream-1/hls/playlist.m3u8",
		"/api/play/stream-1/mse/video/init",
		"/api/play/stream-1/dash/manifest.mpd",
	}

	for _, path := range paths {
		resp := env.request("GET", path, nil, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("path %s: expected 401, got %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestWebRTCEndpointRequiresAuth(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/stream-1/webrtc/whep", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestStreamEndpointRequiresAuth(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/stream-1/stream", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestStopPlaybackRequiresAuthDelivery(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/play/stream-1", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCORSOnPlaybackServe(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	startPlaybackWithDelivery(env, "stream-1", "hls")

	resp := env.request("GET", "/api/play/stream-1/hls/playlist.m3u8", nil, env.standardToken)
	cors := resp.Header.Get("Access-Control-Allow-Origin")
	if cors != "*" {
		t.Errorf("CORS header = %q, want *", cors)
	}
	resp.Body.Close()
}

func TestCORSPreflightWebRTC(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	req, _ := http.NewRequest("OPTIONS", env.httpServer.URL+"/api/play/stream-1/webrtc/whep", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	cors := resp.Header.Get("Access-Control-Allow-Origin")
	if cors != "*" {
		t.Errorf("CORS Allow-Origin = %q, want *", cors)
	}

	methods := resp.Header.Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("expected Access-Control-Allow-Methods header")
	}
}

func TestPlaybackNotFoundStream(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/nonexistent", map[string]any{
		"delivery": "mse",
	}, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	var result map[string]any
	decodeBody(resp, &result)
	if _, ok := result["error"]; !ok {
		t.Error("expected error field in response")
	}
}

func TestPlaybackStopIdempotent(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/play/nonexistent", nil, env.standardToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 for idempotent stop, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestMultipleDeliveryModesSequential(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	modes := []struct {
		delivery     string
		endpointKey  string
	}{
		{"mse", "video_init"},
		{"hls", "playlist"},
		{"dash", "manifest"},
		{"webrtc", "whep"},
		{"stream", "stream"},
	}

	for _, m := range modes {
		env.request("DELETE", "/api/play/stream-1", nil, env.standardToken).Body.Close()

		result, status := startPlaybackWithDelivery(env, "stream-1", m.delivery)
		if status != http.StatusOK {
			t.Fatalf("delivery=%s: expected 200, got %d", m.delivery, status)
		}
		if result["delivery"] != m.delivery {
			t.Fatalf("expected delivery=%s, got %v", m.delivery, result["delivery"])
		}

		endpoints, ok := result["endpoints"].(map[string]any)
		if !ok {
			t.Fatalf("delivery=%s: expected endpoints map", m.delivery)
		}
		if _, ok := endpoints[m.endpointKey]; !ok {
			t.Fatalf("delivery=%s: missing endpoint key %q", m.delivery, m.endpointKey)
		}
	}
}

func TestPlaybackWithDifferentStreams(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result1, status := startPlaybackWithDelivery(env, "stream-1", "mse")
	if status != http.StatusOK {
		t.Fatalf("stream-1: expected 200, got %d", status)
	}

	result2, status := startPlaybackWithDelivery(env, "stream-2", "hls")
	if status != http.StatusOK {
		t.Fatalf("stream-2: expected 200, got %d", status)
	}

	if result1["session_id"] == result2["session_id"] {
		t.Error("different streams should have different session IDs")
	}
	if result1["delivery"] != "mse" {
		t.Errorf("stream-1 delivery = %v, want mse", result1["delivery"])
	}
	if result2["delivery"] != "hls" {
		t.Errorf("stream-2 delivery = %v, want hls", result2["delivery"])
	}
}

func TestJoinExistingSessionRetainsDelivery(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result1, status := startPlaybackWithDelivery(env, "stream-1", "hls")
	if status != http.StatusOK {
		t.Fatalf("first start: expected 200, got %d", status)
	}
	if result1["is_new"] != true {
		t.Error("expected is_new=true for first session")
	}

	resp := env.request("POST", "/api/play/stream-1", nil, env.standardToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("join: expected 200, got %d", resp.StatusCode)
	}
	var result2 map[string]any
	decodeBody(resp, &result2)

	if result2["delivery"] != "hls" {
		t.Errorf("joined session delivery = %v, want hls", result2["delivery"])
	}
	if result2["is_new"] != false {
		t.Error("expected is_new=false for joined session")
	}
}

func TestPlaybackSessionID(t *testing.T) {
	env := newDeliveryTestEnv(t)
	defer env.close()

	result, status := startPlaybackWithDelivery(env, "stream-1", "mse")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	sessionID, ok := result["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatal("expected non-empty session_id")
	}

	streamID, ok := result["stream_id"].(string)
	if !ok || streamID != "stream-1" {
		t.Fatalf("expected stream_id=stream-1, got %v", result["stream_id"])
	}
}
