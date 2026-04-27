package filter

import (
	"testing"

	"github.com/asticode/go-astiav"
)

func TestNewDeinterlacer(t *testing.T) {
	tb := astiav.NewRational(1, 25)
	d, err := NewDeinterlacer(1920, 1080, astiav.PixelFormatYuv420P, tb)
	if err != nil {
		t.Fatalf("NewDeinterlacer: %v", err)
	}
	defer d.Close()

	if d.graph == nil {
		t.Fatal("expected non-nil graph")
	}
	if d.bufferSrc == nil {
		t.Fatal("expected non-nil bufferSrc")
	}
	if d.bufferSink == nil {
		t.Fatal("expected non-nil bufferSink")
	}
}

func TestNewDeinterlacer_VariousFormats(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		pixFmt astiav.PixelFormat
		tbNum  int
		tbDen  int
	}{
		{"1080i25", 1920, 1080, astiav.PixelFormatYuv420P, 1, 25},
		{"576i25", 720, 576, astiav.PixelFormatYuv420P, 1, 25},
		{"480i30", 720, 480, astiav.PixelFormatYuv420P, 1001, 30000},
		{"1080i30_nv12", 1920, 1080, astiav.PixelFormatNv12, 1, 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := astiav.NewRational(tt.tbNum, tt.tbDen)
			d, err := NewDeinterlacer(tt.width, tt.height, tt.pixFmt, tb)
			if err != nil {
				t.Fatalf("NewDeinterlacer(%s): %v", tt.name, err)
			}
			d.Close()
		})
	}
}

func TestDeinterlacer_CloseIdempotent(t *testing.T) {
	tb := astiav.NewRational(1, 25)
	d, err := NewDeinterlacer(1920, 1080, astiav.PixelFormatYuv420P, tb)
	if err != nil {
		t.Fatalf("NewDeinterlacer: %v", err)
	}
	d.Close()
	d.Close()

	if d.graph != nil {
		t.Fatal("expected nil graph after Close")
	}
}

func TestDeinterlacer_ProcessEAGAIN(t *testing.T) {
	tb := astiav.NewRational(1, 25)
	d, err := NewDeinterlacer(1920, 1080, astiav.PixelFormatYuv420P, tb)
	if err != nil {
		t.Fatalf("NewDeinterlacer: %v", err)
	}
	defer d.Close()

	frame := astiav.AllocFrame()
	defer frame.Free()
	frame.SetWidth(1920)
	frame.SetHeight(1080)
	frame.SetPixelFormat(astiav.PixelFormatYuv420P)
	if err := frame.AllocBuffer(0); err != nil {
		t.Fatalf("AllocBuffer: %v", err)
	}
	frame.SetPts(0)

	out, err := d.Process(frame)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if out != nil {
		out.Free()
	}
}
