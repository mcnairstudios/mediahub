package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

type recordingIntent struct {
	StreamID   string    `json:"stream_id"`
	StreamName string    `json:"stream_name"`
	Title      string    `json:"title"`
	UserID     string    `json:"user_id"`
	StartedAt  time.Time `json:"started_at"`
	StopAt     time.Time `json:"stop_at"`
}

func StartRecording(ctx context.Context, deps RecordingDeps, streamID string, title string, userID string, short bool) error {
	sess := deps.SessionMgr.Get(streamID)
	if sess == nil {
		return fmt.Errorf("session %s not found", streamID)
	}

	recPlugin := findRecordPlugin(sess)
	if recPlugin == nil {
		return fmt.Errorf("no record plugin on session %s", streamID)
	}

	recPlugin.SetPreserved(true)

	recID := generateRecordingID()
	rec := &recording.Recording{
		ID:         recID,
		StreamID:   streamID,
		StreamName: sess.StreamName,
		Title:      title,
		UserID:     userID,
		Status:     recording.StatusRecording,
		StartedAt:  time.Now(),
		FilePath:   recPlugin.FilePath(),
		Container:  "mpegts",
	}

	if err := deps.RecordingStore.Create(ctx, rec); err != nil {
		recPlugin.SetPreserved(false)
		return fmt.Errorf("create recording: %w", err)
	}

	sess.SetRecorded(true)

	writeIntent(sess.OutputDir, recordingIntent{
		StreamID:   streamID,
		StreamName: sess.StreamName,
		Title:      title,
		UserID:     userID,
		StartedAt:  rec.StartedAt,
		StopAt:     rec.ScheduledStop,
	})

	return nil
}

func StopRecording(ctx context.Context, deps RecordingDeps, streamID string) error {
	sess := deps.SessionMgr.Get(streamID)
	if sess == nil {
		return fmt.Errorf("session %s not found", streamID)
	}

	recPlugin := findRecordPlugin(sess)

	sess.SetRecorded(false)
	removeIntent(sess.OutputDir)

	recs, err := deps.RecordingStore.ListByStatus(ctx, recording.StatusRecording)
	if err != nil {
		return fmt.Errorf("list recordings: %w", err)
	}

	for _, r := range recs {
		if r.StreamID == streamID {
			r.Status = recording.StatusCompleted
			r.StoppedAt = time.Now()

			if recPlugin != nil {
				r.FilePath = recPlugin.FilePath()
				r.FileSize = recPlugin.FileSize()
			}

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

			if recPlugin != nil {
				recPlugin.SetPreserved(false)
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

type recordingPlugin interface {
	FilePath() string
	FileSize() int64
	SetPreserved(bool)
	IsPreserved() bool
}

func findRecordPlugin(sess *session.Session) recordingPlugin {
	for _, p := range sess.FanOut.Plugins() {
		if p.Mode() == output.DeliveryRecord {
			if rp, ok := p.(recordingPlugin); ok {
				return rp
			}
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

func writeIntent(sessionDir string, intent recordingIntent) {
	data, err := json.Marshal(intent)
	if err != nil {
		return
	}
	os.MkdirAll(sessionDir, 0755) //nolint:errcheck
	os.WriteFile(filepath.Join(sessionDir, "recording.json"), data, 0644) //nolint:errcheck
}

func removeIntent(sessionDir string) {
	os.Remove(filepath.Join(sessionDir, "recording.json")) //nolint:errcheck
}

func RecoverRecordings(ctx context.Context, deps RecordingDeps) {
	entries, err := os.ReadDir(deps.RecordDir)
	if err != nil {
		log.Printf("recording recovery: failed to read record dir: %v", err)
		return
	}

	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		intentPath := filepath.Join(deps.RecordDir, entry.Name(), "recording.json")
		data, err := os.ReadFile(intentPath)
		if err != nil {
			continue
		}

		var intent recordingIntent
		if err := json.Unmarshal(data, &intent); err != nil {
			log.Printf("recording recovery: invalid intent in %s: %v", intentPath, err)
			os.Remove(intentPath) //nolint:errcheck
			continue
		}

		if !intent.StopAt.IsZero() && intent.StopAt.Before(now) {
			log.Printf("recording recovery: expired intent for %s (%s), cleaning up", intent.StreamID, intent.Title)
			os.Remove(intentPath) //nolint:errcheck
			continue
		}

		log.Printf("recording recovery: restoring %s (%s), stop_at=%s", intent.StreamID, intent.Title, intent.StopAt.Format(time.RFC3339))
		if err := StartRecording(ctx, deps, intent.StreamID, intent.Title, intent.UserID, false); err != nil {
			log.Printf("recording recovery: failed to restart %s: %v", intent.StreamID, err)
			os.Remove(intentPath) //nolint:errcheck
		}
	}
}
