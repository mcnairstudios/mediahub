package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/session"
)

type RecordingDeps struct {
	SessionMgr     *session.Manager
	RecordingStore recording.Store
	OutputReg      *output.Registry
}

func StartRecording(ctx context.Context, deps RecordingDeps, streamID string, title string, userID string, short bool) error {
	sess := deps.SessionMgr.Get(streamID)
	if sess == nil {
		return fmt.Errorf("session %s not found", streamID)
	}

	recID := generateRecordingID()
	rec := &recording.Recording{
		ID:         recID,
		StreamID:   streamID,
		StreamName: sess.StreamName,
		Title:      title,
		UserID:     userID,
		Status:     recording.StatusRecording,
		StartedAt:  time.Now(),
	}

	if err := deps.RecordingStore.Create(ctx, rec); err != nil {
		return fmt.Errorf("create recording: %w", err)
	}

	plugin, err := deps.OutputReg.Create(output.DeliveryRecord, output.PluginConfig{
		OutputDir: sess.OutputDir,
	})
	if err != nil {
		return fmt.Errorf("create recording plugin: %w", err)
	}

	sess.FanOut.Add(plugin)
	sess.SetRecorded(true)

	return nil
}

func StopRecording(ctx context.Context, deps RecordingDeps, streamID string) error {
	sess := deps.SessionMgr.Get(streamID)
	if sess == nil {
		return fmt.Errorf("session %s not found", streamID)
	}

	if err := deps.SessionMgr.RemovePlugin(streamID, output.DeliveryRecord); err != nil {
		return fmt.Errorf("remove recording plugin: %w", err)
	}

	sess.SetRecorded(false)

	recs, err := deps.RecordingStore.ListByStatus(ctx, recording.StatusRecording)
	if err != nil {
		return fmt.Errorf("list recordings: %w", err)
	}

	for _, r := range recs {
		if r.StreamID == streamID {
			r.Status = recording.StatusCompleted
			r.StoppedAt = time.Now()
			if err := deps.RecordingStore.Update(ctx, &r); err != nil {
				return fmt.Errorf("update recording: %w", err)
			}
			break
		}
	}

	return nil
}

func ScheduleRecording(ctx context.Context, deps RecordingDeps, rec *recording.Recording) error {
	rec.Status = recording.StatusScheduled
	if err := deps.RecordingStore.Create(ctx, rec); err != nil {
		return fmt.Errorf("schedule recording: %w", err)
	}
	return nil
}

func generateRecordingID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
