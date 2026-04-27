package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/recording"
)

func seedRecording(env *testEnv, rec *recording.Recording) {
	env.server.deps.RecordingStore.Create(context.Background(), rec)
}

func TestGetRecording(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	seedRecording(env, &recording.Recording{
		ID:         "rec-1",
		StreamID:   "stream-1",
		StreamName: "BBC One",
		Title:      "Evening News",
		UserID:     "admin-id",
		Status:     recording.StatusCompleted,
		StartedAt:  time.Now().Add(-time.Hour),
		StoppedAt:  time.Now(),
	})

	resp := env.request("GET", "/api/recordings/completed/rec-1", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	decodeBody(resp, &result)

	if result["id"] != "rec-1" {
		t.Errorf("id = %q, want %q", result["id"], "rec-1")
	}
	if result["title"] != "Evening News" {
		t.Errorf("title = %q, want %q", result["title"], "Evening News")
	}
	if result["status"] != "completed" {
		t.Errorf("status = %q, want %q", result["status"], "completed")
	}
}

func TestGetRecordingNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/recordings/completed/nonexistent", nil, env.adminToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetRecordingNoAuth(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/recordings/completed/rec-1", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestDeleteRecording(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	tmpFile := filepath.Join(t.TempDir(), "test.mp4")
	os.WriteFile(tmpFile, []byte("fake mp4"), 0644)

	seedRecording(env, &recording.Recording{
		ID:       "rec-del",
		StreamID: "stream-1",
		Title:    "To Delete",
		UserID:   "admin-id",
		Status:   recording.StatusCompleted,
		FilePath: tmpFile,
	})

	resp := env.request("DELETE", "/api/recordings/completed/rec-del", nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("expected file to be deleted from disk")
	}

	resp = env.request("GET", "/api/recordings/completed/rec-del", nil, env.adminToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", resp.StatusCode)
	}
}

func TestDeleteRecordingRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	seedRecording(env, &recording.Recording{
		ID:     "rec-del2",
		Title:  "Protected",
		Status: recording.StatusCompleted,
	})

	resp := env.request("DELETE", "/api/recordings/completed/rec-del2", nil, env.standardToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestDeleteRecordingIdempotent(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/recordings/completed/nonexistent", nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestPlayRecordingNotCompleted(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	seedRecording(env, &recording.Recording{
		ID:       "rec-sched",
		StreamID: "stream-1",
		Title:    "Scheduled",
		UserID:   "admin-id",
		Status:   recording.StatusScheduled,
	})

	resp := env.request("POST", "/api/recordings/completed/rec-sched/play", nil, env.adminToken)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestPlayRecordingNoFile(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	seedRecording(env, &recording.Recording{
		ID:       "rec-nofile",
		StreamID: "stream-1",
		Title:    "No File",
		UserID:   "admin-id",
		Status:   recording.StatusCompleted,
	})

	resp := env.request("POST", "/api/recordings/completed/rec-nofile/play", nil, env.adminToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestPlayRecordingFileMissing(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	seedRecording(env, &recording.Recording{
		ID:       "rec-missing",
		StreamID: "stream-1",
		Title:    "Missing File",
		UserID:   "admin-id",
		Status:   recording.StatusCompleted,
		FilePath: "/tmp/nonexistent-file-abc123.mp4",
	})

	resp := env.request("POST", "/api/recordings/completed/rec-missing/play", nil, env.adminToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestPlayRecordingNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("POST", "/api/recordings/completed/nonexistent/play", nil, env.adminToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestStreamRecordingNotFound(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("GET", "/api/recordings/completed/nonexistent/stream", nil, env.adminToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestStreamRecordingFile(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	tmpFile := filepath.Join(t.TempDir(), "test.mp4")
	os.WriteFile(tmpFile, []byte("fake mp4 content"), 0644)

	seedRecording(env, &recording.Recording{
		ID:       "rec-stream",
		StreamID: "stream-1",
		Title:    "Streamable",
		UserID:   "admin-id",
		Status:   recording.StatusCompleted,
		FilePath: tmpFile,
	})

	resp := env.request("GET", "/api/recordings/completed/rec-stream/stream", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStopRecordingPlayback(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	resp := env.request("DELETE", "/api/recordings/completed/rec-1/play", nil, env.adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestGetRecordingWithFileSize(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	tmpFile := filepath.Join(t.TempDir(), "recording.mp4")
	os.WriteFile(tmpFile, make([]byte, 4096), 0644)

	seedRecording(env, &recording.Recording{
		ID:       "rec-size",
		StreamID: "stream-1",
		Title:    "With Size",
		UserID:   "admin-id",
		Status:   recording.StatusCompleted,
		FilePath: tmpFile,
	})

	resp := env.request("GET", "/api/recordings/completed/rec-size", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	decodeBody(resp, &result)

	if result["file_exists"] != true {
		t.Error("expected file_exists=true")
	}
	if fs, ok := result["file_size"].(float64); !ok || fs != 4096 {
		t.Errorf("expected file_size=4096, got %v", result["file_size"])
	}
}

func TestGetRecordingDuration(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()

	start := time.Now().Add(-30 * time.Minute)
	stop := time.Now()

	seedRecording(env, &recording.Recording{
		ID:        "rec-dur",
		StreamID:  "stream-1",
		Title:     "With Duration",
		UserID:    "admin-id",
		Status:    recording.StatusCompleted,
		StartedAt: start,
		StoppedAt: stop,
	})

	resp := env.request("GET", "/api/recordings/completed/rec-dur", nil, env.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	decodeBody(resp, &result)

	dur, ok := result["duration_sec"].(float64)
	if !ok {
		t.Fatalf("duration_sec not a number: %v", result["duration_sec"])
	}
	if dur < 1790 || dur > 1810 {
		t.Errorf("expected duration ~1800, got %v", dur)
	}
}
