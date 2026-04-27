package recording

import (
	"testing"
	"time"
)

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusActive, "active"},
		{StatusRecording, "recording"},
		{StatusCompleted, "completed"},
		{StatusScheduled, "scheduled"},
		{StatusFailed, "failed"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("got %q, want %q", tt.status, tt.want)
		}
	}
}

func TestRecordingZeroValue(t *testing.T) {
	var r Recording
	if r.ID != "" {
		t.Error("zero value ID should be empty")
	}
	if r.Status != "" {
		t.Error("zero value Status should be empty")
	}
	if !r.StartedAt.IsZero() {
		t.Error("zero value StartedAt should be zero time")
	}
	if r.FileSize != 0 {
		t.Error("zero value FileSize should be 0")
	}
}

func TestRecordingFields(t *testing.T) {
	now := time.Now()
	r := Recording{
		ID:             "rec-001",
		StreamID:       "stream-1",
		StreamName:     "BBC One",
		ChannelID:      "ch-1",
		ChannelName:    "BBC One HD",
		Title:          "Evening News",
		UserID:         "user-1",
		Status:         StatusRecording,
		StartedAt:      now,
		ScheduledStart: now.Add(-time.Minute),
		ScheduledStop:  now.Add(time.Hour),
		FilePath:       "/tmp/rec-001.mp4",
		FileSize:       1024 * 1024,
		Container:      "mp4",
		VideoCodec:     "h264",
		AudioCodec:     "aac",
	}

	if r.ID != "rec-001" {
		t.Errorf("ID = %q, want %q", r.ID, "rec-001")
	}
	if r.Status != StatusRecording {
		t.Errorf("Status = %q, want %q", r.Status, StatusRecording)
	}
	if r.FileSize != 1024*1024 {
		t.Errorf("FileSize = %d, want %d", r.FileSize, 1024*1024)
	}
	if r.Container != "mp4" {
		t.Errorf("Container = %q, want %q", r.Container, "mp4")
	}
}

func TestStoreInterfaceCompiles(t *testing.T) {
	var _ Store = (Store)(nil)
}
