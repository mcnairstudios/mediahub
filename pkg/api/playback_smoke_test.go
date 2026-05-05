package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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

const testStreamID = "smoke-stream-1"
const testStreamURL = "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8"

type smokeOutputPlugin struct {
	mode       output.DeliveryMode
	mu         sync.Mutex
	videoPkts  int
	audioPkts  int
	bytesTotal int64
	ready      chan struct{}
	readyOnce  sync.Once
}

func newSmokePlugin(mode output.DeliveryMode) *smokeOutputPlugin {
	return &smokeOutputPlugin{
		mode:  mode,
		ready: make(chan struct{}),
	}
}

func (p *smokeOutputPlugin) Mode() output.DeliveryMode { return p.mode }

func (p *smokeOutputPlugin) PushVideo(data []byte, _, _, _ int64, _ bool) error {
	p.mu.Lock()
	p.videoPkts++
	p.bytesTotal += int64(len(data))
	p.mu.Unlock()
	p.readyOnce.Do(func() { close(p.ready) })
	return nil
}

func (p *smokeOutputPlugin) PushAudio(data []byte, _, _, _ int64) error {
	p.mu.Lock()
	p.audioPkts++
	p.bytesTotal += int64(len(data))
	p.mu.Unlock()
	p.readyOnce.Do(func() { close(p.ready) })
	return nil
}

func (p *smokeOutputPlugin) PushSubtitle([]byte, int64, int64) error { return nil }
func (p *smokeOutputPlugin) EndOfStream()                            {}
func (p *smokeOutputPlugin) ResetForSeek()                           {}
func (p *smokeOutputPlugin) Stop()                                   {}
func (p *smokeOutputPlugin) Status() output.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return output.PluginStatus{
		Mode:         p.mode,
		SegmentCount: p.videoPkts,
		BytesWritten: p.bytesTotal,
		Healthy:      true,
	}
}

func (p *smokeOutputPlugin) Generation() int64 { return 1 }

func (p *smokeOutputPlugin) WaitReady(ctx context.Context) error {
	select {
	case <-p.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *smokeOutputPlugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	switch p.mode {
	case output.DeliveryHLS:
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:6\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:6.000,\nsegment0.ts\n"))
	case output.DeliveryMSE:
		path := r.URL.Path
		if strings.Contains(path, "init") {
			w.Header().Set("Content-Type", "video/mp4")
			ftyp := []byte{
				0x00, 0x00, 0x00, 0x18, // size=24
				0x66, 0x74, 0x79, 0x70, // "ftyp"
				0x69, 0x73, 0x6f, 0x35, // "iso5"
				0x00, 0x00, 0x02, 0x00, // minor version
				0x69, 0x73, 0x6f, 0x36, // "iso6"
				0x6d, 0x70, 0x34, 0x31, // "mp41"
			}
			w.Write(ftyp)
		} else {
			w.Header().Set("Content-Type", "video/mp4")
			moof := []byte{
				0x00, 0x00, 0x00, 0x10, // size=16
				0x6d, 0x6f, 0x6f, 0x66, // "moof"
				0x00, 0x00, 0x00, 0x08, // size=8 (empty mfhd)
				0x6d, 0x66, 0x68, 0x64, // "mfhd"
			}
			w.Write(moof)
		}
	case output.DeliveryDASH:
		w.Header().Set("Content-Type", "application/dash+xml")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
			`<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="dynamic" minimumUpdatePeriod="PT2S">` +
			`<Period><AdaptationSet mimeType="video/mp4"><Representation id="1" bandwidth="5000000"/>` +
			`</AdaptationSet></Period></MPD>`))
	case output.DeliveryWebRTC:
		w.Header().Set("Content-Type", "application/sdp")
		w.Write([]byte("v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\n"))
	}
}

type smokeEnv struct {
	server     *Server
	httpServer *httptest.Server
	token      string
}

func newSmokeEnv(t *testing.T) *smokeEnv {
	t.Helper()

	userStore := auth.NewMemoryUserStore()
	authService := auth.NewJWTService(userStore, "smoke-test-secret")

	ctx := context.Background()
	_, err := authService.CreateUser(ctx, "smoke", "smokepass", "", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := authService.Login(ctx, "smoke", "smokepass")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	streamStore := store.NewMemoryStreamStore()
	streamStore.BulkUpsert(ctx, []media.Stream{
		{
			ID:         testStreamID,
			Name:       "Big Buck Bunny",
			URL:        testStreamURL,
			SourceType: "m3u",
			SourceID:   "smoke-src",
			IsActive:   true,
			VideoCodec: "h264",
			AudioCodec: "aac",
			Width:      1920,
			Height:     1080,
		},
	})

	reg := output.NewRegistry()
	reg.Register(output.DeliveryMSE, func(_ output.PluginConfig) (output.OutputPlugin, error) {
		return newSmokePlugin(output.DeliveryMSE), nil
	})
	reg.Register(output.DeliveryHLS, func(_ output.PluginConfig) (output.OutputPlugin, error) {
		return newSmokePlugin(output.DeliveryHLS), nil
	})
	reg.Register(output.DeliveryDASH, func(_ output.PluginConfig) (output.OutputPlugin, error) {
		return newSmokePlugin(output.DeliveryDASH), nil
	})
	reg.Register(output.DeliveryWebRTC, func(_ output.PluginConfig) (output.OutputPlugin, error) {
		return newSmokePlugin(output.DeliveryWebRTC), nil
	})
	reg.Register(output.DeliveryStream, func(_ output.PluginConfig) (output.OutputPlugin, error) {
		return newSmokePlugin(output.DeliveryStream), nil
	})
	reg.Register(output.DeliveryRecord, func(_ output.PluginConfig) (output.OutputPlugin, error) {
		return newSmokePlugin(output.DeliveryRecord), nil
	})

	pipelineRunner := func(_ *session.Session, _ session.PipelineConfig) (*session.PipelineResult, error) {
		return &session.PipelineResult{
			Info: &media.ProbeResult{
				Video: &media.VideoInfo{
					Index:      0,
					Codec:      "h264",
					Width:      1920,
					Height:     1080,
					FramerateN: 30,
					FramerateD: 1,
					BitDepth:   8,
				},
				AudioTracks: []media.AudioTrack{
					{Index: 1, Codec: "aac", Channels: 2, SampleRate: 44100, Language: "eng"},
				},
			},
		}, nil
	}

	deps := OrchestratorDeps{
		StreamStore:       streamStore,
		ChannelStore:      store.NewMemoryChannelStore(),
		SettingsStore:     store.NewMemorySettingsStore(),
		SourceConfigStore: sourceconfig.NewMemoryStore(),
		SessionMgr:        session.NewManager(t.TempDir()),
		Detector:          client.NewDetector(nil),
		OutputReg:         reg,
		SourceReg:         source.NewRegistry(),
		RecordingStore:    store.NewMemoryRecordingStore(),
		AuthService:       authService,
		EPGSourceStore:    store.NewMemoryEPGSourceStore(),
		ProgramStore:      store.NewMemoryProgramStore(),
		GroupStore:        store.NewMemoryGroupStore(),
		FavoriteStore:     favorite.NewMemoryStore(),
		Strategy: func(in strategy.Input, out strategy.Output) strategy.Decision {
			return strategy.Resolve(in, out)
		},
		PipelineRunner: pipelineRunner,
	}

	srv := NewServer(deps)
	ts := httptest.NewServer(srv.Handler())

	return &smokeEnv{
		server:     srv,
		httpServer: ts,
		token:      token,
	}
}

func (e *smokeEnv) close() {
	e.server.deps.SessionMgr.StopAll()
	e.httpServer.Close()
}

func (e *smokeEnv) do(method, path string, body any) *http.Response {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.httpServer.URL+path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	return resp
}

func (e *smokeEnv) startPlay(delivery string) (map[string]any, int) {
	var body any
	if delivery != "" {
		body = map[string]string{"delivery": delivery}
	}
	resp := e.do("POST", "/api/play/"+testStreamID, body)
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	return result, resp.StatusCode
}

func (e *smokeEnv) stopPlay() {
	resp := e.do("DELETE", "/api/play/"+testStreamID, nil)
	resp.Body.Close()
}

func TestPlaybackSmoke_MSE(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	env := newSmokeEnv(t)
	defer env.close()

	result, status := env.startPlay("mse")
	if status != http.StatusOK {
		t.Fatalf("POST /api/play: expected 200, got %d", status)
	}
	if result["delivery"] != "mse" {
		t.Fatalf("expected delivery=mse, got %v", result["delivery"])
	}

	endpoints := result["endpoints"].(map[string]any)
	videoInit := endpoints["video_init"].(string)
	audioInit := endpoints["audio_init"].(string)
	videoSeg := endpoints["video_segment"].(string)

	resp := env.do("GET", videoInit, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET video_init: expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(body) < 8 {
		t.Fatalf("video init too small: %d bytes", len(body))
	}
	if string(body[4:8]) != "ftyp" {
		t.Fatalf("video init missing ftyp box, got %x", body[4:8])
	}

	resp = env.do("GET", audioInit, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET audio_init: expected 200, got %d", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(body) < 8 {
		t.Fatalf("audio init too small: %d bytes", len(body))
	}
	if string(body[4:8]) != "ftyp" {
		t.Fatalf("audio init missing ftyp box, got %x", body[4:8])
	}

	resp = env.do("GET", videoSeg, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET video_segment: expected 200, got %d", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(body) < 8 {
		t.Fatalf("video segment too small: %d bytes", len(body))
	}
	if string(body[4:8]) != "moof" {
		t.Fatalf("video segment missing moof box, got %x", body[4:8])
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "video/mp4" {
		t.Fatalf("Content-Type = %q, want video/mp4", ct)
	}

	env.stopPlay()
}

func TestPlaybackSmoke_HLS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	env := newSmokeEnv(t)
	defer env.close()

	result, status := env.startPlay("hls")
	if status != http.StatusOK {
		t.Fatalf("POST /api/play: expected 200, got %d", status)
	}
	if result["delivery"] != "hls" {
		t.Fatalf("expected delivery=hls, got %v", result["delivery"])
	}

	endpoints := result["endpoints"].(map[string]any)
	playlist := endpoints["playlist"].(string)

	resp := env.do("GET", playlist, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET playlist: expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	content := string(body)
	if !strings.HasPrefix(content, "#EXTM3U") {
		t.Fatalf("playlist does not start with #EXTM3U: %q", content[:min(50, len(content))])
	}
	if !strings.Contains(content, "#EXT-X-VERSION") {
		t.Fatal("playlist missing #EXT-X-VERSION")
	}
	if !strings.Contains(content, "#EXT-X-TARGETDURATION") {
		t.Fatal("playlist missing #EXT-X-TARGETDURATION")
	}
	if !strings.Contains(content, "#EXTINF") {
		t.Fatal("playlist missing #EXTINF segment entries")
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/vnd.apple.mpegurl" {
		t.Fatalf("Content-Type = %q, want application/vnd.apple.mpegurl", ct)
	}

	env.stopPlay()
}

func TestPlaybackSmoke_DASH(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	env := newSmokeEnv(t)
	defer env.close()

	result, status := env.startPlay("dash")
	if status != http.StatusOK {
		t.Fatalf("POST /api/play: expected 200, got %d", status)
	}
	if result["delivery"] != "dash" {
		t.Fatalf("expected delivery=dash, got %v", result["delivery"])
	}

	endpoints := result["endpoints"].(map[string]any)
	manifest := endpoints["manifest"].(string)

	resp := env.do("GET", manifest, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET manifest: expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	content := string(body)
	if !strings.Contains(content, "<?xml") {
		t.Fatalf("manifest missing XML declaration: %q", content[:min(80, len(content))])
	}
	if !strings.Contains(content, "<MPD") {
		t.Fatal("manifest missing <MPD> element")
	}
	if !strings.Contains(content, "urn:mpeg:dash:schema:mpd:2011") {
		t.Fatal("manifest missing DASH namespace")
	}
	if !strings.Contains(content, "<AdaptationSet") {
		t.Fatal("manifest missing <AdaptationSet>")
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/dash+xml" {
		t.Fatalf("Content-Type = %q, want application/dash+xml", ct)
	}

	env.stopPlay()
}

func TestPlaybackSmoke_WebRTC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	env := newSmokeEnv(t)
	defer env.close()

	result, status := env.startPlay("webrtc")
	if status != http.StatusOK {
		t.Fatalf("POST /api/play: expected 200, got %d", status)
	}
	if result["delivery"] != "webrtc" {
		t.Fatalf("expected delivery=webrtc, got %v", result["delivery"])
	}

	endpoints := result["endpoints"].(map[string]any)
	whep := endpoints["whep"].(string)

	sdpOffer := "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\n"
	resp := env.do("POST", whep, sdpOffer)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST whep: expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	content := string(body)
	if !strings.HasPrefix(content, "v=0") {
		t.Fatalf("WHEP response not a valid SDP: %q", content[:min(50, len(content))])
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/sdp" {
		t.Fatalf("Content-Type = %q, want application/sdp", ct)
	}

	env.stopPlay()
}

func TestPlaybackSmoke_Stream(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	env := newSmokeEnv(t)
	defer env.close()

	result, status := env.startPlay("stream")
	if status != http.StatusOK {
		t.Fatalf("POST /api/play: expected 200, got %d", status)
	}
	if result["delivery"] != "stream" {
		t.Fatalf("expected delivery=stream, got %v", result["delivery"])
	}

	endpoints := result["endpoints"].(map[string]any)
	_, ok := endpoints["stream"].(string)
	if !ok {
		t.Fatal("missing stream endpoint")
	}

	env.stopPlay()
}

func TestPlaybackSmoke_ProbeInfoPresent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	env := newSmokeEnv(t)
	defer env.close()

	result, status := env.startPlay("mse")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	probeInfo, ok := result["probe_info"].(map[string]any)
	if !ok {
		t.Fatal("missing probe_info in playback response")
	}

	video, ok := probeInfo["video"].(map[string]any)
	if !ok {
		t.Fatal("missing video in probe_info")
	}
	if video["codec"] != "h264" {
		t.Fatalf("video codec = %v, want h264", video["codec"])
	}
	if video["width"] != float64(1920) {
		t.Fatalf("video width = %v, want 1920", video["width"])
	}
	if video["height"] != float64(1080) {
		t.Fatalf("video height = %v, want 1080", video["height"])
	}

	audio, ok := probeInfo["audio"].(map[string]any)
	if !ok {
		t.Fatal("missing audio in probe_info")
	}
	if audio["codec"] != "aac" {
		t.Fatalf("audio codec = %v, want aac", audio["codec"])
	}
	if audio["channels"] != float64(2) {
		t.Fatalf("audio channels = %v, want 2", audio["channels"])
	}
	if audio["sample_rate"] != float64(44100) {
		t.Fatalf("audio sample_rate = %v, want 44100", audio["sample_rate"])
	}
	if audio["language"] != "eng" {
		t.Fatalf("audio language = %v, want eng", audio["language"])
	}

	env.stopPlay()
}

func TestPlaybackSmoke_DecisionPresent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	env := newSmokeEnv(t)
	defer env.close()

	result, status := env.startPlay("hls")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	decision, ok := result["decision"].(map[string]any)
	if !ok {
		t.Fatal("missing decision in response")
	}

	for _, key := range []string{"needs_transcode", "video_codec", "audio_codec", "container"} {
		if _, present := decision[key]; !present {
			t.Fatalf("decision missing %q", key)
		}
	}

	env.stopPlay()
}

func TestPlaybackSmoke_DeliverySwitcher(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	env := newSmokeEnv(t)
	defer env.close()

	result, status := env.startPlay("mse")
	if status != http.StatusOK {
		t.Fatalf("start MSE: expected 200, got %d", status)
	}
	if result["delivery"] != "mse" {
		t.Fatalf("expected mse, got %v", result["delivery"])
	}

	endpoints := result["endpoints"].(map[string]any)
	videoInit := endpoints["video_init"].(string)
	resp := env.do("GET", videoInit, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET video_init (MSE phase): expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(body) < 8 || string(body[4:8]) != "ftyp" {
		t.Fatal("MSE video_init not valid fMP4 before switch")
	}

	env.stopPlay()
	time.Sleep(50 * time.Millisecond)

	result, status = env.startPlay("hls")
	if status != http.StatusOK {
		t.Fatalf("start HLS: expected 200, got %d", status)
	}
	if result["delivery"] != "hls" {
		t.Fatalf("expected hls after switch, got %v", result["delivery"])
	}

	endpoints = result["endpoints"].(map[string]any)
	playlist := endpoints["playlist"].(string)
	resp = env.do("GET", playlist, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET playlist (HLS phase): expected 200, got %d", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.HasPrefix(string(body), "#EXTM3U") {
		t.Fatal("HLS playlist not valid after delivery switch")
	}

	env.stopPlay()
}

func TestPlaybackSmoke_AllModesSequential(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	env := newSmokeEnv(t)
	defer env.close()

	modes := []struct {
		delivery    string
		endpointKey string
		verifyFunc  func(t *testing.T, env *smokeEnv, endpoint string)
	}{
		{
			delivery:    "mse",
			endpointKey: "video_init",
			verifyFunc: func(t *testing.T, env *smokeEnv, endpoint string) {
				t.Helper()
				resp := env.do("GET", endpoint, nil)
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("MSE video_init: expected 200, got %d", resp.StatusCode)
				}
				body, _ := io.ReadAll(resp.Body)
				if len(body) < 8 || string(body[4:8]) != "ftyp" {
					t.Fatal("MSE video_init: invalid fMP4")
				}
			},
		},
		{
			delivery:    "hls",
			endpointKey: "playlist",
			verifyFunc: func(t *testing.T, env *smokeEnv, endpoint string) {
				t.Helper()
				resp := env.do("GET", endpoint, nil)
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("HLS playlist: expected 200, got %d", resp.StatusCode)
				}
				body, _ := io.ReadAll(resp.Body)
				if !strings.HasPrefix(string(body), "#EXTM3U") {
					t.Fatal("HLS playlist: invalid m3u8")
				}
			},
		},
		{
			delivery:    "dash",
			endpointKey: "manifest",
			verifyFunc: func(t *testing.T, env *smokeEnv, endpoint string) {
				t.Helper()
				resp := env.do("GET", endpoint, nil)
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("DASH manifest: expected 200, got %d", resp.StatusCode)
				}
				body, _ := io.ReadAll(resp.Body)
				if !strings.Contains(string(body), "<MPD") {
					t.Fatal("DASH manifest: missing <MPD>")
				}
			},
		},
		{
			delivery:    "webrtc",
			endpointKey: "whep",
			verifyFunc: func(t *testing.T, env *smokeEnv, endpoint string) {
				t.Helper()
				resp := env.do("POST", endpoint, "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\n")
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("WebRTC WHEP: expected 200, got %d", resp.StatusCode)
				}
				body, _ := io.ReadAll(resp.Body)
				if !strings.HasPrefix(string(body), "v=0") {
					t.Fatal("WebRTC WHEP: invalid SDP response")
				}
			},
		},
		{
			delivery:    "stream",
			endpointKey: "stream",
			verifyFunc:  nil,
		},
	}

	for _, m := range modes {
		t.Run(m.delivery, func(t *testing.T) {
			env.stopPlay()
			time.Sleep(50 * time.Millisecond)

			result, status := env.startPlay(m.delivery)
			if status != http.StatusOK {
				t.Fatalf("start %s: expected 200, got %d", m.delivery, status)
			}
			if result["delivery"] != m.delivery {
				t.Fatalf("expected delivery=%s, got %v", m.delivery, result["delivery"])
			}

			endpoints, ok := result["endpoints"].(map[string]any)
			if !ok {
				t.Fatalf("%s: missing endpoints", m.delivery)
			}
			endpoint, ok := endpoints[m.endpointKey].(string)
			if !ok {
				t.Fatalf("%s: missing endpoint key %q", m.delivery, m.endpointKey)
			}

			if m.verifyFunc != nil {
				m.verifyFunc(t, env, endpoint)
			}
		})
	}
}

func TestPlaybackSmoke_CleanupOnStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	env := newSmokeEnv(t)
	defer env.close()

	_, status := env.startPlay("mse")
	if status != http.StatusOK {
		t.Fatalf("start: expected 200, got %d", status)
	}

	if env.server.deps.SessionMgr.ActiveCount() != 1 {
		t.Fatalf("expected 1 active session, got %d", env.server.deps.SessionMgr.ActiveCount())
	}

	env.stopPlay()
	time.Sleep(100 * time.Millisecond)

	if env.server.deps.SessionMgr.ActiveCount() != 0 {
		t.Fatalf("expected 0 active sessions after stop, got %d", env.server.deps.SessionMgr.ActiveCount())
	}

	resp := env.do("GET", "/api/play/"+testStreamID+"/mse/video/init", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after stop, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPlaybackSmoke_CORSHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	env := newSmokeEnv(t)
	defer env.close()

	result, status := env.startPlay("hls")
	if status != http.StatusOK {
		t.Fatalf("start: expected 200, got %d", status)
	}

	endpoints := result["endpoints"].(map[string]any)
	playlist := endpoints["playlist"].(string)
	resp := env.do("GET", playlist, nil)
	cors := resp.Header.Get("Access-Control-Allow-Origin")
	resp.Body.Close()
	if cors != "*" {
		t.Fatalf("CORS header = %q, want *", cors)
	}

	env.stopPlay()
}
