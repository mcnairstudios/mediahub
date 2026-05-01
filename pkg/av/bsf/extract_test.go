package bsf

import (
	"testing"

	"github.com/asticode/go-astiav"
)

func TestNewExtraDataExtractor_H264(t *testing.T) {
	if testing.Short() {
		t.Skip("requires CGO/ffmpeg libs")
	}

	ext, err := NewExtraDataExtractor(astiav.CodecIDH264, astiav.NewRational(1, 90000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ext.Close()
}

func TestNewExtraDataExtractor_HEVC(t *testing.T) {
	if testing.Short() {
		t.Skip("requires CGO/ffmpeg libs")
	}

	ext, err := NewExtraDataExtractor(astiav.CodecIDHevc, astiav.NewRational(1, 90000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ext.Close()
}

func TestClose_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("requires CGO/ffmpeg libs")
	}

	ext, err := NewExtraDataExtractor(astiav.CodecIDH264, astiav.NewRational(1, 90000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ext.Close()
	ext.Close()
}

func TestProcessPacket_NilExtradata(t *testing.T) {
	if testing.Short() {
		t.Skip("requires CGO/ffmpeg libs")
	}

	ext, err := NewExtraDataExtractor(astiav.CodecIDH264, astiav.NewRational(1, 90000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ext.Close()

	pkt := astiav.AllocPacket()
	if pkt == nil {
		t.Fatal("failed to allocate packet")
	}
	defer pkt.Free()

	plainData := []byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x9A, 0x00, 0x00}
	if err := pkt.FromData(plainData); err != nil {
		t.Fatalf("FromData: %v", err)
	}
	pkt.SetPts(0)
	pkt.SetDts(0)

	ed, err := ext.ProcessPacket(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ed != nil {
		t.Logf("got extradata (%d bytes) from non-parameter packet", len(ed))
	}
}

func TestProcessPacket_H264SPS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires CGO/ffmpeg libs")
	}

	ext, err := NewExtraDataExtractor(astiav.CodecIDH264, astiav.NewRational(1, 90000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ext.Close()

	sps := []byte{0x67, 0x42, 0xC0, 0x1E, 0xD9, 0x00, 0xA0, 0x47, 0xFE, 0x6C, 0x04, 0x40, 0x00, 0x00, 0x03, 0x00, 0x40}
	pps := []byte{0x68, 0xCE, 0x38, 0x80}
	idr := []byte{0x65, 0x88, 0x80, 0x40, 0x00}

	var annexB []byte
	annexB = append(annexB, 0x00, 0x00, 0x00, 0x01)
	annexB = append(annexB, sps...)
	annexB = append(annexB, 0x00, 0x00, 0x00, 0x01)
	annexB = append(annexB, pps...)
	annexB = append(annexB, 0x00, 0x00, 0x00, 0x01)
	annexB = append(annexB, idr...)

	pkt := astiav.AllocPacket()
	if pkt == nil {
		t.Fatal("failed to allocate packet")
	}
	defer pkt.Free()

	if err := pkt.FromData(annexB); err != nil {
		t.Fatalf("FromData: %v", err)
	}
	pkt.SetPts(0)
	pkt.SetDts(0)
	pkt.SetFlags(astiav.NewPacketFlags(astiav.PacketFlagKey))

	ed, err := ext.ProcessPacket(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ed) > 0 {
		t.Logf("extracted %d bytes of extradata from H.264 SPS/PPS packet", len(ed))
	}
}
