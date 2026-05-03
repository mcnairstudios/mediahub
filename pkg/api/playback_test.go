package api

import (
	"net/http"
	"testing"
)

func TestStartPlaybackUnknownStream(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/nonexistent-stream", nil, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	var result map[string]any
	decodeBody(resp, &result)
	if _, ok := result["error"]; !ok {
		t.Fatal("expected error field in response")
	}
}

func TestStartPlaybackRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/stream-1", nil, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestStartPlaybackWithDeliveryOverride(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/stream-1", map[string]any{
		"delivery": "mse",
	}, env.standardToken)

	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("should not get 401 with valid token")
	}
	if resp.StatusCode == http.StatusBadRequest {
		t.Fatal("should not get 400 - delivery override should be accepted")
	}
}

func TestStartPlaybackWithHLSDelivery(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/stream-1", map[string]any{
		"delivery": "hls",
	}, env.standardToken)

	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("should not get 401 with valid token")
	}
	if resp.StatusCode == http.StatusBadRequest {
		t.Fatal("should not get 400 - delivery override should be accepted")
	}
}

func TestStopPlaybackNoSessionReturns204(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/play/nonexistent-stream", nil, env.standardToken)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestStopPlaybackNoToken(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/play/some-stream", nil, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestHLSPlaylistNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/stream-1/hls/playlist.m3u8", nil, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for HLS playlist with no session, got %d", resp.StatusCode)
	}
}

func TestHLSSegmentNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/stream-1/hls/segment0.ts", nil, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for HLS segment with no session, got %d", resp.StatusCode)
	}
}

func TestHLSServeRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/stream-1/hls/playlist.m3u8", nil, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestMSEVideoInitNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/stream-1/mse/video/init", nil, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for MSE video init with no session, got %d", resp.StatusCode)
	}
}

func TestMSEAudioInitNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/stream-1/mse/audio/init", nil, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for MSE audio init with no session, got %d", resp.StatusCode)
	}
}

func TestMSEVideoSegmentNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/stream-1/mse/video/segment", nil, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for MSE video segment with no session, got %d", resp.StatusCode)
	}
}

func TestMSEAudioSegmentNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/stream-1/mse/audio/segment", nil, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for MSE audio segment with no session, got %d", resp.StatusCode)
	}
}

func TestMSEServeRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/stream-1/mse/video/init", nil, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSeekNoSessionPlayback(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/nonexistent-stream/seek", map[string]any{
		"position_ms": 5000,
	}, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	var result map[string]any
	decodeBody(resp, &result)
	if _, ok := result["error"]; !ok {
		t.Fatal("expected error field in response")
	}
}

func TestSeekRequiresAuthPlayback(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/stream-1/seek", map[string]any{
		"position_ms": 5000,
	}, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSeekInvalidBody(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	req, _ := http.NewRequest("POST", env.httpServer.URL+"/api/play/stream-1/seek", nil)
	req.Header.Set("Authorization", "Bearer "+env.standardToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 400 or 404, got %d", resp.StatusCode)
	}
}

func TestStartRecordingNoSessionPlayback(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/nonexistent-stream/record", nil, env.standardToken)

	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected error status for recording with no session, got %d", resp.StatusCode)
	}
}

func TestStartRecordingRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/stream-1/record", nil, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestStopRecordingNoSessionPlayback(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/play/nonexistent-stream/record", nil, env.standardToken)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		t.Fatalf("expected error status for stopping recording with no session, got %d", resp.StatusCode)
	}
}

func TestStopRecordingRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/play/stream-1/record", nil, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSubtitlesNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/stream-1/subtitles", nil, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for subtitles with no session, got %d", resp.StatusCode)
	}
}

func TestSubtitlesRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/stream-1/subtitles", nil, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestPlaybackServeEmptyStreamID(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/nonexistent/hls/playlist.m3u8", nil, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestPlaybackServeNonexistentMSEPath(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/play/nonexistent/mse/video/init", nil, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestPlayURLRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/url", map[string]any{
		"url": "http://example.com/stream.ts",
	}, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestPlayURLMissingURL(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/play/url", map[string]any{}, env.adminToken)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestStopPlayURLRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/play/url", map[string]any{
		"url": "http://example.com/stream.ts",
	}, env.standardToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestStopPlayURLMissingURL(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/play/url", map[string]any{}, env.adminToken)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestStopPlayURLNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/play/url", map[string]any{
		"url": "http://example.com/nonexistent.ts",
	}, env.adminToken)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestMultipleHLSPathsNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	paths := []string{
		"/api/play/stream-1/hls/playlist.m3u8",
		"/api/play/stream-1/hls/segment0.ts",
		"/api/play/stream-1/hls/segment1.ts",
		"/api/play/stream-1/hls/init.mp4",
	}

	for _, path := range paths {
		resp := env.request("GET", path, nil, env.standardToken)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("path %s: expected 404, got %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestMultipleMSEPathsNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	paths := []string{
		"/api/play/stream-1/mse/video/init",
		"/api/play/stream-1/mse/audio/init",
		"/api/play/stream-1/mse/video/segment",
		"/api/play/stream-1/mse/audio/segment",
	}

	for _, path := range paths {
		resp := env.request("GET", path, nil, env.standardToken)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("path %s: expected 404, got %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestPlaybackRecordingServeNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	paths := []string{
		"/api/recordings/completed/rec-1/play/hls/playlist.m3u8",
		"/api/recordings/completed/rec-1/play/mse/video/init",
	}

	for _, path := range paths {
		resp := env.request("GET", path, nil, env.standardToken)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("path %s: expected 404, got %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestSeekRecordingNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/recordings/completed/nonexistent/seek", map[string]any{
		"position_ms": 10000,
	}, env.standardToken)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestStopRecordingPlaybackNoSession(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/recordings/completed/nonexistent/play", nil, env.standardToken)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}
