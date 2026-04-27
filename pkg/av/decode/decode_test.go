package decode

import (
	"testing"

	"github.com/asticode/go-astiav"
)

func TestHWAccelMap(t *testing.T) {
	m := HWAccelMap()

	expected := map[string]astiav.HardwareDeviceType{
		"vaapi":        astiav.HardwareDeviceTypeVAAPI,
		"qsv":          astiav.HardwareDeviceTypeQSV,
		"videotoolbox": astiav.HardwareDeviceTypeVideoToolbox,
		"cuda":         astiav.HardwareDeviceTypeCUDA,
		"nvenc":        astiav.HardwareDeviceTypeCUDA,
		"d3d11va":      astiav.HardwareDeviceTypeD3D11VA,
		"dxva2":        astiav.HardwareDeviceTypeDXVA2,
		"vulkan":       astiav.HardwareDeviceTypeVulkan,
	}

	if len(m) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(m))
	}

	for name, wantType := range expected {
		got, ok := m[name]
		if !ok {
			t.Errorf("missing HW accel entry %q", name)
			continue
		}
		if got != wantType {
			t.Errorf("HW accel %q: got type %d, want %d", name, got, wantType)
		}
	}
}

func TestHWAccelMapIsCopy(t *testing.T) {
	m := HWAccelMap()
	m["fake"] = astiav.HardwareDeviceTypeCUDA
	m2 := HWAccelMap()
	if _, ok := m2["fake"]; ok {
		t.Error("HWAccelMap returned reference to internal map, should return a copy")
	}
}

func TestBitDepthFromPixelFormat(t *testing.T) {
	tests := []struct {
		name     string
		pf       astiav.PixelFormat
		expected int
	}{
		{"yuv420p is 8-bit", astiav.PixelFormatYuv420P, 8},
		{"yuv420p10le is 10-bit", astiav.PixelFormatYuv420P10Le, 10},
		{"none defaults to 8", astiav.PixelFormatNone, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BitDepthFromPixelFormat(tt.pf)
			if got != tt.expected {
				t.Errorf("BitDepthFromPixelFormat(%v) = %d, want %d", tt.pf, got, tt.expected)
			}
		})
	}
}

func TestExceedsMaxBitDepth(t *testing.T) {
	tests := []struct {
		name        string
		pf          astiav.PixelFormat
		maxBitDepth int
		expected    bool
	}{
		{"10-bit exceeds 8", astiav.PixelFormatYuv420P10Le, 8, true},
		{"8-bit does not exceed 8", astiav.PixelFormatYuv420P, 8, false},
		{"8-bit does not exceed 10", astiav.PixelFormatYuv420P, 10, false},
		{"zero max means no limit", astiav.PixelFormatYuv420P10Le, 0, false},
		{"negative max means no limit", astiav.PixelFormatYuv420P10Le, -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExceedsMaxBitDepth(tt.pf, tt.maxBitDepth)
			if got != tt.expected {
				t.Errorf("ExceedsMaxBitDepth(%v, %d) = %v, want %v", tt.pf, tt.maxBitDepth, got, tt.expected)
			}
		})
	}
}

func TestIsHWPixelFormat(t *testing.T) {
	hwFormats := []astiav.PixelFormat{
		astiav.PixelFormatCuda,
		astiav.PixelFormatVaapi,
		astiav.PixelFormatQsv,
		astiav.PixelFormatVideotoolbox,
	}
	for _, pf := range hwFormats {
		if !isHWPixelFormat(pf) {
			t.Errorf("expected %v to be HW pixel format", pf)
		}
	}

	swFormats := []astiav.PixelFormat{
		astiav.PixelFormatYuv420P,
		astiav.PixelFormatYuv420P10Le,
		astiav.PixelFormatNv12,
		astiav.PixelFormatNone,
	}
	for _, pf := range swFormats {
		if isHWPixelFormat(pf) {
			t.Errorf("expected %v to NOT be HW pixel format", pf)
		}
	}
}

func TestDecodeOptsDefaults(t *testing.T) {
	opts := DecodeOpts{}
	if opts.HWAccel != "" {
		t.Error("default HWAccel should be empty")
	}
	if opts.MaxBitDepth != 0 {
		t.Error("default MaxBitDepth should be 0")
	}
	if opts.DecoderName != "" {
		t.Error("default DecoderName should be empty")
	}
}

func TestNewVideoDecoderInvalidCodecID(t *testing.T) {
	_, err := NewVideoDecoder(astiav.CodecID(999999), nil, DecodeOpts{})
	if err == nil {
		t.Error("expected error for invalid codec ID")
	}
}

func TestNewAudioDecoderInvalidCodecID(t *testing.T) {
	_, err := NewAudioDecoder(astiav.CodecID(999999), nil)
	if err == nil {
		t.Error("expected error for invalid codec ID")
	}
}

func TestNewVideoDecoderInvalidDecoderName(t *testing.T) {
	_, err := NewVideoDecoder(astiav.CodecIDH264, nil, DecodeOpts{DecoderName: "nonexistent_decoder_xyz"})
	if err == nil {
		t.Error("expected error for nonexistent decoder name")
	}
}

func TestNewVideoDecoderSW(t *testing.T) {
	dec, err := NewVideoDecoder(astiav.CodecIDH264, nil, DecodeOpts{})
	if err != nil {
		t.Fatalf("unexpected error creating SW H264 decoder: %v", err)
	}
	defer dec.Close()

	if dec.codecCtx == nil {
		t.Error("codec context should not be nil")
	}
	if dec.hwCtx != nil {
		t.Error("HW context should be nil for SW decoder")
	}
}

func TestNewAudioDecoderSW(t *testing.T) {
	dec, err := NewAudioDecoder(astiav.CodecIDAac, nil)
	if err != nil {
		t.Fatalf("unexpected error creating AAC decoder: %v", err)
	}
	defer dec.Close()

	if dec.codecCtx == nil {
		t.Error("codec context should not be nil")
	}
}

func TestFlushBuffersNilContext(t *testing.T) {
	d := &Decoder{}
	d.FlushBuffers()
}

func TestFlushNilContext(t *testing.T) {
	d := &Decoder{}
	frames, err := d.Flush()
	if err != nil {
		t.Errorf("Flush on nil context should not error, got: %v", err)
	}
	if frames != nil {
		t.Error("Flush on nil context should return nil frames")
	}
}

func TestCloseIdempotent(t *testing.T) {
	dec, err := NewVideoDecoder(astiav.CodecIDH264, nil, DecodeOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dec.Close()
	dec.Close()
}
