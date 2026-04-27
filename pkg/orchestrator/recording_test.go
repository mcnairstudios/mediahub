package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
