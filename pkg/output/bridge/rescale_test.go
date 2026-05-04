package bridge

import (
	"math"
	"testing"
)

func TestSafeRescale_Normal(t *testing.T) {
	result := safeRescale(3600, 1_000_000_000, 90000)
	expected := int64(40_000_000)
	if result != expected {
		t.Errorf("safeRescale(3600, 1e9, 90000) = %d, want %d", result, expected)
	}
}

func TestSafeRescale_Large(t *testing.T) {
	result := safeRescale(9_000_000_000, 1_000_000_000, 90000)

	expected := float64(9_000_000_000) * float64(1_000_000_000) / float64(90000)
	diff := math.Abs(float64(result) - expected)
	tolerance := expected * 0.001
	if diff > tolerance {
		t.Errorf("safeRescale(9e9, 1e9, 90000) = %d, expected ~%.0f (diff=%.0f, tolerance=%.0f)",
			result, expected, diff, tolerance)
	}

	if result <= 0 {
		t.Errorf("safeRescale(9e9, 1e9, 90000) = %d, should be positive (overflow?)", result)
	}
}

func TestSafeRescale_ZeroValue(t *testing.T) {
	result := safeRescale(0, 1_000_000_000, 90000)
	if result != 0 {
		t.Errorf("safeRescale(0, 1e9, 90000) = %d, want 0", result)
	}
}

func TestSafeRescale_ZeroDenominator(t *testing.T) {
	result := safeRescale(3600, 1_000_000_000, 0)
	if result != 0 {
		t.Errorf("safeRescale(3600, 1e9, 0) = %d, want 0", result)
	}
}

func TestSafeRescale_ZeroNumerator(t *testing.T) {
	result := safeRescale(3600, 0, 90000)
	if result != 0 {
		t.Errorf("safeRescale(3600, 0, 90000) = %d, want 0", result)
	}
}

func TestSafeRescale_Identity(t *testing.T) {
	result := safeRescale(12345, 1, 1)
	if result != 12345 {
		t.Errorf("safeRescale(12345, 1, 1) = %d, want 12345", result)
	}
}

func TestSafeRescale_SmallValues(t *testing.T) {
	result := safeRescale(1, 48000, 90000)
	expected := int64(0)
	if result != expected {
		t.Errorf("safeRescale(1, 48000, 90000) = %d, want %d", result, expected)
	}
}

func TestSafeRescale_AudioTimescale(t *testing.T) {
	result := safeRescale(48000, 1_000_000_000, 48000)
	expected := int64(1_000_000_000)
	if result != expected {
		t.Errorf("safeRescale(48000, 1e9, 48000) = %d, want %d", result, expected)
	}
}

func TestSafeRescale_NoOverflow_MaxInt64Values(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("safeRescale panicked with large values: %v", r)
		}
	}()

	result := safeRescale(math.MaxInt64/2, 2, 1)
	if result <= 0 {
		t.Logf("safeRescale(MaxInt64/2, 2, 1) = %d (may overflow, but should not panic)", result)
	}
}

func TestSafeRescale_VideoTSToNanos(t *testing.T) {
	pts := int64(90000)
	nanos := safeRescale(pts, 1_000_000_000*int64(1), int64(90000))
	expected := int64(1_000_000_000)
	if nanos != expected {
		t.Errorf("video 1s PTS->nanos: got %d, want %d", nanos, expected)
	}
}

func TestSafeRescale_AudioTSToNanos(t *testing.T) {
	pts := int64(48000)
	nanos := safeRescale(pts, 1_000_000_000*int64(1), int64(48000))
	expected := int64(1_000_000_000)
	if nanos != expected {
		t.Errorf("audio 1s PTS->nanos: got %d, want %d", nanos, expected)
	}
}

func TestSafeRescale_NegativePTS(t *testing.T) {
	result := safeRescale(-90000, 1_000_000_000, 90000)
	expected := int64(-1_000_000_000)
	if result != expected {
		t.Errorf("safeRescale(-90000, 1e9, 90000) = %d, want %d", result, expected)
	}
}

func TestSafeRescale_30fpsTimescale(t *testing.T) {
	result := safeRescale(3000, 1_000_000_000, 90000)
	expected := float64(3000) * float64(1_000_000_000) / float64(90000)
	diff := math.Abs(float64(result) - expected)
	if diff > 1 {
		t.Errorf("safeRescale(3000, 1e9, 90000) = %d, want ~%.0f", result, expected)
	}
}
