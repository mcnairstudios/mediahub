package media

import "testing"

func TestProbeResultWithVideoAndAudio(t *testing.T) {
	pr := ProbeResult{
		Video: &VideoInfo{
			Codec:      "h264",
			Width:      1920,
			Height:     1080,
			BitDepth:   8,
			Interlaced: false,
			FramerateN: 30000,
			FramerateD: 1001,
			Profile:    "high",
			PixFmt:     "yuv420p",
		},
		AudioTracks: []AudioTrack{
			{
				Index:      1,
				Codec:      "aac",
				Language:   "eng",
				Channels:   2,
				SampleRate: 48000,
				BitRate:    128000,
			},
			{
				Index:      2,
				Codec:      "ac3",
				Language:   "spa",
				Channels:   6,
				SampleRate: 48000,
				BitRate:    384000,
			},
		},
		DurationMs: 5400000,
	}

	if pr.Video == nil {
		t.Fatal("Video is nil")
	}
	if pr.Video.Width != 1920 {
		t.Errorf("Video.Width = %d, want 1920", pr.Video.Width)
	}
	if pr.Video.FramerateN != 30000 {
		t.Errorf("Video.FramerateN = %d, want 30000", pr.Video.FramerateN)
	}
	if pr.Video.FramerateD != 1001 {
		t.Errorf("Video.FramerateD = %d, want 1001", pr.Video.FramerateD)
	}
	if len(pr.AudioTracks) != 2 {
		t.Fatalf("AudioTracks len = %d, want 2", len(pr.AudioTracks))
	}
	if pr.AudioTracks[0].Language != "eng" {
		t.Errorf("AudioTracks[0].Language = %q, want %q", pr.AudioTracks[0].Language, "eng")
	}
	if pr.AudioTracks[1].Channels != 6 {
		t.Errorf("AudioTracks[1].Channels = %d, want 6", pr.AudioTracks[1].Channels)
	}
	if pr.DurationMs != 5400000 {
		t.Errorf("DurationMs = %d, want 5400000", pr.DurationMs)
	}
}

func TestProbeResultNilVideo(t *testing.T) {
	pr := ProbeResult{
		Video: nil,
		AudioTracks: []AudioTrack{
			{
				Index:      0,
				Codec:      "mp3",
				Language:   "eng",
				Channels:   2,
				SampleRate: 44100,
				BitRate:    192000,
			},
		},
		DurationMs: 180000,
	}

	if pr.Video != nil {
		t.Error("Video should be nil for audio-only stream")
	}
	if len(pr.AudioTracks) != 1 {
		t.Fatalf("AudioTracks len = %d, want 1", len(pr.AudioTracks))
	}
}

func TestFPSFromFramerate(t *testing.T) {
	tests := []struct {
		name       string
		framerateN int
		framerateD int
		wantFPS    float64
		tolerance  float64
	}{
		{"30fps", 30, 1, 30.0, 0.001},
		{"29.97fps", 30000, 1001, 29.97, 0.01},
		{"25fps", 25, 1, 25.0, 0.001},
		{"60fps", 60, 1, 60.0, 0.001},
		{"23.976fps", 24000, 1001, 23.976, 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vi := VideoInfo{
				FramerateN: tt.framerateN,
				FramerateD: tt.framerateD,
			}
			got := vi.FPS()
			diff := got - tt.wantFPS
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tolerance {
				t.Errorf("FPS() = %f, want %f (tolerance %f)", got, tt.wantFPS, tt.tolerance)
			}
		})
	}
}

func TestFPSZeroDenominator(t *testing.T) {
	vi := VideoInfo{FramerateN: 30, FramerateD: 0}
	got := vi.FPS()
	if got != 0 {
		t.Errorf("FPS() with zero denominator = %f, want 0", got)
	}
}
