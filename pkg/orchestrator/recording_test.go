package orchestrator

import (
	"context"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

func newTestRecordingDeps() RecordingDeps {
	reg := output.NewRegistry()
	reg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return &mockPlugin{mode: output.DeliveryRecord}, nil
	})

	return RecordingDeps{
		SessionMgr:     session.NewManager("/tmp/test-sessions"),
		RecordingStore: store.NewMemoryRecordingStore(),
		OutputReg:      reg,
	}
}

func TestStartRecording_ActiveSession(t *testing.T) {
	deps := newTestRecordingDeps()
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
	deps := newTestRecordingDeps()

	err := StartRecording(context.Background(), deps, "nonexistent", "Title", "user-1", false)
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestStopRecording(t *testing.T) {
	deps := newTestRecordingDeps()
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

	sess := deps.SessionMgr.Get("stream-1")
	if sess.IsRecorded() {
		t.Error("expected session to not be recorded after stop")
	}

	recs, _ := deps.RecordingStore.ListByStatus(ctx, recording.StatusCompleted)
	if len(recs) != 1 {
		t.Fatalf("expected 1 completed recording, got %d", len(recs))
	}
}

func TestScheduleRecording(t *testing.T) {
	deps := newTestRecordingDeps()

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
