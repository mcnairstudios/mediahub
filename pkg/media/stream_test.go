package media

import "testing"

func TestStreamCreation(t *testing.T) {
	s := Stream{
		ID:         "stream-001",
		SourceType: "m3u",
		SourceID:   "source-abc",
		Name:       "BBC One HD",
		URL:        "http://example.com/stream/1",
		Group:      "UK Entertainment",
		TvgID:      "bbc1.uk",
		TvgName:    "BBC One",
		TvgLogo:    "http://example.com/logo/bbc1.png",
		IsActive:   true,
		VideoCodec: "h264",
		AudioCodec: "aac",
		Width:      1920,
		Height:     1080,
		BitDepth:   8,
		Interlaced: true,
		FramerateN: 25,
		FramerateD: 1,
		Duration:   0,
	}

	if s.ID != "stream-001" {
		t.Errorf("ID = %q, want %q", s.ID, "stream-001")
	}
	if s.SourceType != "m3u" {
		t.Errorf("SourceType = %q, want %q", s.SourceType, "m3u")
	}
	if s.Width != 1920 {
		t.Errorf("Width = %d, want %d", s.Width, 1920)
	}
	if s.Height != 1080 {
		t.Errorf("Height = %d, want %d", s.Height, 1080)
	}
	if !s.Interlaced {
		t.Error("Interlaced = false, want true")
	}
	if s.Duration != 0 {
		t.Errorf("Duration = %f, want 0 (live stream)", s.Duration)
	}
}

func TestStreamZeroValue(t *testing.T) {
	var s Stream

	if s.ID != "" {
		t.Errorf("zero-value ID = %q, want empty", s.ID)
	}
	if s.IsActive {
		t.Error("zero-value IsActive = true, want false")
	}
	if s.Width != 0 {
		t.Errorf("zero-value Width = %d, want 0", s.Width)
	}
	if s.Duration != 0 {
		t.Errorf("zero-value Duration = %f, want 0", s.Duration)
	}
}
