package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

func newTestRecordingDeps(t *testing.T) RecordingDeps {
	t.Helper()
	dir := t.TempDir()

	reg := output.NewRegistry()
	reg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryRecord}, nil
	})

	return RecordingDeps{
		SessionMgr:     session.NewManager(dir),
		RecordingStore: store.NewMemoryRecordingStore(),
		OutputReg:      reg,
		RecordDir:      dir,
	}
}

func TestStartRecording_ActiveSession(t *testing.T) {
	deps := newTestRecordingDeps(t)
	defer deps.SessionMgr.StopAll()

	ctx := context.Background()
	deps.SessionMgr.GetOrCreate(ctx, "stream-1", "http://example.com/stream", "Test")

	err := StartRecording(ctx, deps, "stream-1", "My Recording", "user-1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	recs, _ := deps.RecordingStore.ListByStatus(ctx, recording.StatusRecording)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recs))
	}
	if recs[0].StreamID != "stream-1" {
		t.Errorf("expected stream-1, got %s", recs[0].StreamID)
	}
	if recs[0].Title != "My Recording" {
		t.Errorf("expected title 'My Recording', got %s", recs[0].Title)
	}

	sess := deps.SessionMgr.Get("stream-1")
	if !sess.IsRecorded() {
		t.Error("expected session to be marked as recorded")
	}
}

func TestStartRecording_NoSession(t *testing.T) {
	deps := newTestRecordingDeps(t)

	err := StartRecording(context.Background(), deps, "nonexistent", "Title", "user-1", false)
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestStopRecording_WithFileMove(t *testing.T) {
	deps := newTestRecordingDeps(t)
	defer deps.SessionMgr.StopAll()

	ctx := context.Background()
	sess, _, _ := deps.SessionMgr.GetOrCreate(ctx, "stream-1", "http://example.com/stream", "Test")

	os.MkdirAll(sess.OutputDir, 0755)
	sourcePath := filepath.Join(sess.OutputDir, "source.ts")
	os.WriteFile(sourcePath, []byte("fake video data for testing"), 0644)

	err := StartRecording(ctx, deps, "stream-1", "My Recording", "user-1", false)
	if err != nil {
		t.Fatalf("start recording: %v", err)
	}

	err = StopRecording(ctx, deps, "stream-1")
	if err != nil {
		t.Fatalf("stop recording: %v", err)
	}

	if sess.IsRecorded() {
		t.Error("expected session to not be recorded after stop")
	}

	recs, _ := deps.RecordingStore.ListByStatus(ctx, recording.StatusCompleted)
	if len(recs) != 1 {
		t.Fatalf("expected 1 completed recording, got %d", len(recs))
	}

	destPath := recs[0].FilePath
	if !filepath.IsAbs(destPath) {
		t.Errorf("expected absolute file path, got %s", destPath)
	}
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Errorf("expected recording file at %s", destPath)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Error("expected source file to be moved (not exist at original location)")
	}

	metaPath := destPath[:len(destPath)-3] + ".json"
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Errorf("expected metadata file at %s", metaPath)
	}
}

func TestStopRecording_NoSourceFile(t *testing.T) {
	deps := newTestRecordingDeps(t)
	defer deps.SessionMgr.StopAll()

	ctx := context.Background()
	deps.SessionMgr.GetOrCreate(ctx, "stream-1", "http://example.com/stream", "Test")

	err := StartRecording(ctx, deps, "stream-1", "My Recording", "user-1", false)
	if err != nil {
		t.Fatalf("start recording: %v", err)
	}

	err = StopRecording(ctx, deps, "stream-1")
	if err != nil {
		t.Fatalf("stop recording: %v", err)
	}

	recs, _ := deps.RecordingStore.ListByStatus(ctx, recording.StatusFailed)
	if len(recs) != 1 {
		t.Fatalf("expected 1 failed recording, got %d", len(recs))
	}
}

func TestScheduleRecording(t *testing.T) {
	deps := newTestRecordingDeps(t)

	rec := &recording.Recording{
		ID:       "rec-1",
		StreamID: "stream-1",
		Title:    "Scheduled Show",
		UserID:   "user-1",
	}

	err := ScheduleRecording(context.Background(), deps, rec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scheduled, _ := deps.RecordingStore.ListScheduled(context.Background())
	if len(scheduled) != 1 {
		t.Fatalf("expected 1 scheduled, got %d", len(scheduled))
	}
	if scheduled[0].Status != recording.StatusScheduled {
		t.Errorf("expected scheduled status, got %s", scheduled[0].Status)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal Title", "Normal Title"},
		{"Special/chars<>here", "Specialcharshere"},
		{"", ""},
		{"a.b-c_d", "a.b-c_d"},
	}
	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestStartRecording_WritesIntentFile(t *testing.T) {
	deps := newTestRecordingDeps(t)
	defer deps.SessionMgr.StopAll()

	ctx := context.Background()
	sess, _, _ := deps.SessionMgr.GetOrCreate(ctx, "stream-1", "http://example.com/stream", "Test")

	err := StartRecording(ctx, deps, "stream-1", "My Recording", "user-1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	intentPath := filepath.Join(sess.OutputDir, "recording.json")
	data, err := os.ReadFile(intentPath)
	if err != nil {
		t.Fatalf("expected intent file at %s: %v", intentPath, err)
	}

	var intent recordingIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		t.Fatalf("invalid intent JSON: %v", err)
	}
	if intent.StreamID != "stream-1" {
		t.Errorf("intent StreamID = %q, want %q", intent.StreamID, "stream-1")
	}
	if intent.Title != "My Recording" {
		t.Errorf("intent Title = %q, want %q", intent.Title, "My Recording")
	}
	if intent.UserID != "user-1" {
		t.Errorf("intent UserID = %q, want %q", intent.UserID, "user-1")
	}
}

func TestStopRecording_RemovesIntentFile(t *testing.T) {
	deps := newTestRecordingDeps(t)
	defer deps.SessionMgr.StopAll()

	ctx := context.Background()
	sess, _, _ := deps.SessionMgr.GetOrCreate(ctx, "stream-1", "http://example.com/stream", "Test")

	StartRecording(ctx, deps, "stream-1", "My Recording", "user-1", false)

	intentPath := filepath.Join(sess.OutputDir, "recording.json")
	if _, err := os.Stat(intentPath); os.IsNotExist(err) {
		t.Fatal("intent file should exist after start")
	}

	StopRecording(ctx, deps, "stream-1")

	if _, err := os.Stat(intentPath); !os.IsNotExist(err) {
		t.Error("intent file should be removed after stop")
	}
}

func TestRecoverRecordings_ExpiredIntent(t *testing.T) {
	dir := t.TempDir()
	deps := RecordingDeps{
		SessionMgr:     session.NewManager(dir),
		RecordingStore: store.NewMemoryRecordingStore(),
		RecordDir:      dir,
	}

	sessionDir := filepath.Join(dir, "stream-expired")
	os.MkdirAll(sessionDir, 0755)

	intent := recordingIntent{
		StreamID: "stream-expired",
		Title:    "Old Show",
		UserID:   "user-1",
		StopAt:   time.Now().Add(-time.Hour),
	}
	data, _ := json.Marshal(intent)
	os.WriteFile(filepath.Join(sessionDir, "recording.json"), data, 0644)

	RecoverRecordings(context.Background(), deps)

	intentPath := filepath.Join(sessionDir, "recording.json")
	if _, err := os.Stat(intentPath); !os.IsNotExist(err) {
		t.Error("expired intent file should be cleaned up")
	}
}

func TestRecoverRecordings_ActiveIntent(t *testing.T) {
	dir := t.TempDir()
	deps := RecordingDeps{
		SessionMgr:     session.NewManager(dir),
		RecordingStore: store.NewMemoryRecordingStore(),
		RecordDir:      dir,
	}

	ctx := context.Background()
	deps.SessionMgr.GetOrCreate(ctx, "stream-active", "http://example.com/stream", "Active")

	sessionDir := filepath.Join(dir, "stream-active")
	os.MkdirAll(sessionDir, 0755)

	intent := recordingIntent{
		StreamID:   "stream-active",
		StreamName: "Active Stream",
		Title:      "Current Show",
		UserID:     "user-1",
		StopAt:     time.Now().Add(time.Hour),
	}
	data, _ := json.Marshal(intent)
	os.WriteFile(filepath.Join(sessionDir, "recording.json"), data, 0644)

	RecoverRecordings(ctx, deps)

	recs, _ := deps.RecordingStore.ListByStatus(ctx, recording.StatusRecording)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recovered recording, got %d", len(recs))
	}
	if recs[0].Title != "Current Show" {
		t.Errorf("recovered title = %q, want %q", recs[0].Title, "Current Show")
	}
}
