package probe

import (
	"fmt"
	"os"
	"testing"
)

func TestProbeTestFile(t *testing.T) {
	path := os.Getenv("AVPROBE_TEST_FILE")
	if path == "" {
		t.Skip("AVPROBE_TEST_FILE not set; skipping probe integration test")
	}

	result, err := Probe(path, 10)
	if err != nil {
		t.Fatalf("Probe(%q): %v", path, err)
	}

	if result.Video == nil {
		t.Fatal("expected video stream in probe result")
	}

	if result.Video.Width == 0 || result.Video.Height == 0 {
		t.Errorf("expected non-zero resolution, got %dx%d", result.Video.Width, result.Video.Height)
	}

	if result.Video.Codec == "" {
		t.Error("expected non-empty video codec")
	}

	t.Logf("Video: %s %dx%d %d-bit %.2ffps",
		result.Video.Codec,
		result.Video.Width, result.Video.Height,
		result.Video.BitDepth,
		result.Video.FPS(),
	)
	t.Logf("Duration: %dms", result.DurationMs)
	t.Logf("Audio tracks: %d", len(result.AudioTracks))
	for i, a := range result.AudioTracks {
		t.Logf("  [%d] %s %dch %dHz lang=%s ad=%v", i, a.Codec, a.Channels, a.SampleRate, a.Language, a.IsAD)
	}
}

func TestBitDepthFromPixelFormat(t *testing.T) {
	tests := []struct {
		name string
		re   string
		want int
	}{
		{"8bit suffix", "yuv420p", 8},
		{"10bit le", "yuv420p10le", 10},
		{"12bit be", "yuv444p12be", 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := bitDepthRe.FindStringSubmatch(tt.re)
			got := 8
			if m != nil {
				if bits, err := parseInt(m[1]); err == nil && bits > 8 && bits <= 16 {
					got = bits
				}
			}
			if got != tt.want {
				t.Errorf("bitDepth(%q) = %d, want %d", tt.re, got, tt.want)
			}
		})
	}
}

func TestDetectInterlacedEmpty(t *testing.T) {
	if detectInterlaced("h264", nil) {
		t.Error("expected false for nil extradata")
	}
	if detectInterlaced("hevc", []byte{0x01, 0x02}) {
		t.Error("expected false for non-h264 codec")
	}
}

func parseInt(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
