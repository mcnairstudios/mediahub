package resample

import (
	"testing"

	"github.com/asticode/go-astiav"
)

func TestChannelLayoutForCount(t *testing.T) {
	tests := []struct {
		channels int
		wantErr  bool
	}{
		{1, false},
		{2, false},
		{6, false},
		{8, false},
		{3, true},
		{0, true},
		{5, true},
	}
	for _, tt := range tests {
		_, err := channelLayoutForCount(tt.channels)
		if (err != nil) != tt.wantErr {
			t.Errorf("channelLayoutForCount(%d): err=%v, wantErr=%v", tt.channels, err, tt.wantErr)
		}
	}
}

func TestNewResamplerInvalidDstChannels(t *testing.T) {
	_, err := NewResampler(2, 48000, astiav.SampleFormatS16, 3, 48000, astiav.SampleFormatS16)
	if err == nil {
		t.Fatal("expected error for unsupported dst channel count 3")
	}
}

func TestNewResamplerValid(t *testing.T) {
	r, err := NewResampler(2, 44100, astiav.SampleFormatS16, 2, 48000, astiav.SampleFormatFltp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer r.Close()

	if r.swrCtx == nil {
		t.Fatal("expected non-nil SoftwareResampleContext")
	}
	if r.dstRate != 48000 {
		t.Fatalf("expected dstRate=48000, got %d", r.dstRate)
	}
	if r.dstFmt != astiav.SampleFormatFltp {
		t.Fatalf("expected dstFmt=fltp, got %v", r.dstFmt)
	}
}

func TestNewResamplerDownmix(t *testing.T) {
	r, err := NewResampler(6, 48000, astiav.SampleFormatFltp, 2, 48000, astiav.SampleFormatFltp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer r.Close()

	if r.swrCtx == nil {
		t.Fatal("expected non-nil SoftwareResampleContext")
	}
}

func TestResetPreservesConfig(t *testing.T) {
	r, err := NewResampler(2, 44100, astiav.SampleFormatS16, 2, 48000, astiav.SampleFormatFltp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer r.Close()

	r.Reset()

	if r.swrCtx == nil {
		t.Fatal("expected non-nil SoftwareResampleContext after Reset")
	}
	if r.dstRate != 48000 {
		t.Fatal("Reset should not change dstRate")
	}
	if r.dstFmt != astiav.SampleFormatFltp {
		t.Fatal("Reset should not change dstFmt")
	}
}

func TestCloseNilsContext(t *testing.T) {
	r, err := NewResampler(2, 48000, astiav.SampleFormatS16, 2, 48000, astiav.SampleFormatS16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r.Close()
	if r.swrCtx != nil {
		t.Fatal("expected nil SoftwareResampleContext after Close")
	}

	r.Close()
}

func TestDoubleReset(t *testing.T) {
	r, err := NewResampler(1, 22050, astiav.SampleFormatS16, 2, 48000, astiav.SampleFormatFltp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer r.Close()

	r.Reset()
	r.Reset()

	if r.swrCtx == nil {
		t.Fatal("expected non-nil SoftwareResampleContext after double Reset")
	}
}
