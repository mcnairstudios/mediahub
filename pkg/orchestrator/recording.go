package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/session"
)

type RecordingDeps struct {
	SessionMgr     *session.Manager
	RecordingStore recording.Store
	OutputReg      *output.Registry
	RecordDir      string
}

func StartRecording(ctx context.Context, deps RecordingDeps, streamID string, title string, userID string, short bool) error {
	sess := deps.SessionMgr.Get(streamID)
	if sess == nil {
		return fmt.Errorf("session %s not found", streamID)
	}

	recID := generateRecordingID()
	sourcePath := filepath.Join(sess.OutputDir, "source.ts")

	rec := &recording.Recording{
		ID:         recID,
		StreamID:   streamID,
		StreamName: sess.StreamName,
		Title:      title,
		UserID:     userID,
		Status:     recording.StatusRecording,
		StartedAt:  time.Now(),
		FilePath:   sourcePath,
		Container:  "mpegts",
	}

	if err := deps.RecordingStore.Create(ctx, rec); err != nil {
		return fmt.Errorf("create recording: %w", err)
	}

	sess.SetRecorded(true)

	return nil
}

func StopRecording(ctx context.Context, deps RecordingDeps, streamID string) error {
	sess := deps.SessionMgr.Get(streamID)
	if sess == nil {
		return fmt.Errorf("session %s not found", streamID)
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

			destPath, moveErr := completeRecording(deps.RecordDir, r.FilePath, r.Title)
			if moveErr != nil {
				log.Printf("recording: failed to move file for %s: %v", r.ID, moveErr)
				r.Status = recording.StatusFailed
			} else {
				r.FilePath = destPath
				if fi, err := os.Stat(destPath); err == nil {
					r.FileSize = fi.Size()
				}
			}

			if err := deps.RecordingStore.Update(ctx, &r); err != nil {
				return fmt.Errorf("update recording: %w", err)
			}
			break
		}
	}

	return nil
}

func completeRecording(recordDir, sourcePath, title string) (string, error) {
	if sourcePath == "" {
		return "", fmt.Errorf("no source file path")
	}
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return "", fmt.Errorf("source file does not exist: %s", sourcePath)
	}

	destDir := filepath.Join(recordDir, "recordings")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create recordings directory: %w", err)
	}

	safeName := sanitizeFilename(title)
	if safeName == "" {
		safeName = "recording"
	}
	destPath := filepath.Join(destDir, safeName+".ts")

	for i := 1; fileExists(destPath); i++ {
		destPath = filepath.Join(destDir, fmt.Sprintf("%s_%d.ts", safeName, i))
	}

	if err := os.Rename(sourcePath, destPath); err != nil {
		return "", fmt.Errorf("move recording: %w", err)
	}

	writeMetadata(destPath, title)

	return destPath, nil
}

func writeMetadata(recordingPath, title string) {
	metaPath := strings.TrimSuffix(recordingPath, filepath.Ext(recordingPath)) + ".json"
	content := fmt.Sprintf(`{"title":%q,"completed_at":%q,"file":%q}`,
		title, time.Now().Format(time.RFC3339), filepath.Base(recordingPath))
	os.WriteFile(metaPath, []byte(content), 0644) //nolint:errcheck
}

var unsafeChars = regexp.MustCompile(`[^\w\s\-\.]`)

func sanitizeFilename(name string) string {
	name = unsafeChars.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
