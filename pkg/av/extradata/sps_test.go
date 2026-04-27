package extradata

import "testing"

func TestBitReader_ReadBits(t *testing.T) {
	r := newBitReader([]byte{0b10110001, 0b01010101})

	v, err := r.readBits(3)
	if err != nil {
		t.Fatal(err)
	}
	if v != 5 {
		t.Errorf("readBits(3) = %d, want 5", v)
	}

	v, err = r.readBits(5)
	if err != nil {
		t.Fatal(err)
	}
	if v != 17 {
		t.Errorf("readBits(5) = %d, want 17", v)
	}

	v, err = r.readBits(8)
	if err != nil {
		t.Fatal(err)
	}
	if v != 85 {
		t.Errorf("readBits(8) = %d, want 85", v)
	}

	_, err = r.readBits(1)
	if err == nil {
		t.Error("expected error reading past end")
	}
}

func TestBitReader_ReadBit(t *testing.T) {
	r := newBitReader([]byte{0b11000000})
	b, _ := r.readBit()
	if b != 1 {
		t.Errorf("bit 0 = %d, want 1", b)
	}
	b, _ = r.readBit()
	if b != 1 {
		t.Errorf("bit 1 = %d, want 1", b)
	}
	b, _ = r.readBit()
	if b != 0 {
		t.Errorf("bit 2 = %d, want 0", b)
	}
}

func TestReadUE(t *testing.T) {
	data := []byte{0xA6, 0x42, 0x98, 0xE2, 0x05, 0x00}
	r := newBitReader(data)

	expected := []uint32{0, 1, 2, 3, 4, 5, 6, 7, 9}
	for _, want := range expected {
		got, err := r.readUE()
		if err != nil {
			t.Fatalf("readUE() for value %d: %v", want, err)
		}
		if got != want {
			t.Errorf("readUE() = %d, want %d", got, want)
		}
	}
}

func TestReadSE(t *testing.T) {
	data := []byte{0xA6, 0x42, 0x80}
	r := newBitReader(data)

	type tc struct {
		want int32
	}
	cases := []tc{{0}, {1}, {-1}, {2}, {-2}}
	for _, c := range cases {
		got, err := r.readSE()
		if err != nil {
			t.Fatalf("readSE() for value %d: %v", c.want, err)
		}
		if got != c.want {
			t.Errorf("readSE() = %d, want %d", got, c.want)
		}
	}
}

func TestParseH264SPS_Progressive(t *testing.T) {
	sps := []byte{
		0x67,
		0x64,
		0x00,
		0x28,
		0xAC, 0xE5, 0x01, 0xE0, 0x08, 0x90,
	}

	info := ParseH264SPS(sps)
	if info == nil {
		t.Fatal("ParseH264SPS returned nil")
	}

	if info.ProfileIDC != 100 {
		t.Errorf("ProfileIDC = %d, want 100", info.ProfileIDC)
	}
	if info.LevelIDC != 40 {
		t.Errorf("LevelIDC = %d, want 40", info.LevelIDC)
	}
	if info.ChromaFormatIDC != 1 {
		t.Errorf("ChromaFormatIDC = %d, want 1", info.ChromaFormatIDC)
	}
	if info.BitDepthLuma != 8 {
		t.Errorf("BitDepthLuma = %d, want 8", info.BitDepthLuma)
	}
	if info.BitDepthChroma != 8 {
		t.Errorf("BitDepthChroma = %d, want 8", info.BitDepthChroma)
	}
	if !info.FrameMBSOnlyFlag {
		t.Error("FrameMBSOnlyFlag = false, want true (progressive)")
	}
	if info.Width != 120 {
		t.Errorf("Width = %d macroblocks, want 120 (1920px)", info.Width)
	}
	if info.Height != 68 {
		t.Errorf("Height = %d map units, want 68 (1088px)", info.Height)
	}
}

func TestParseH264SPS_Interlaced(t *testing.T) {
	sps := []byte{
		0x67,
		0x64,
		0x00,
		0x28,
		0xAC, 0xE5, 0x01, 0xE0, 0x08, 0x80,
	}

	info := ParseH264SPS(sps)
	if info == nil {
		t.Fatal("ParseH264SPS returned nil")
	}

	if info.FrameMBSOnlyFlag {
		t.Error("FrameMBSOnlyFlag = true, want false (interlaced)")
	}
}

func TestParseH264SPS_Baseline(t *testing.T) {
	sps := []byte{
		0x67,
		0x42,
		0xC0,
		0x1E,
		0xF4, 0x0A, 0x0F, 0x80,
	}

	info := ParseH264SPS(sps)
	if info == nil {
		t.Fatal("ParseH264SPS returned nil")
	}

	if info.ProfileIDC != 66 {
		t.Errorf("ProfileIDC = %d, want 66", info.ProfileIDC)
	}
	if info.ChromaFormatIDC != 1 {
		t.Errorf("ChromaFormatIDC = %d, want 1", info.ChromaFormatIDC)
	}
	if info.BitDepthLuma != 8 {
		t.Errorf("BitDepthLuma = %d, want 8", info.BitDepthLuma)
	}
	if !info.FrameMBSOnlyFlag {
		t.Error("FrameMBSOnlyFlag = false, want true")
	}
	if info.Width != 20 {
		t.Errorf("Width = %d, want 20 (320px)", info.Width)
	}
	if info.Height != 15 {
		t.Errorf("Height = %d, want 15 (240px)", info.Height)
	}
}

func TestParseH264SPS_TooShort(t *testing.T) {
	result := ParseH264SPS([]byte{0x67, 0x64})
	if result != nil {
		t.Error("expected nil for too-short SPS")
	}

	result = ParseH264SPS(nil)
	if result != nil {
		t.Error("expected nil for nil SPS")
	}
}

func TestParseH265SPS(t *testing.T) {
	payload := []byte{0xA0, 0x03, 0xC0, 0x80, 0x10, 0xE4, 0xD8}

	sps := make([]byte, 0, 2+1+12+len(payload))
	sps = append(sps, 0x42, 0x01)
	sps = append(sps, 0x01)
	sps = append(sps, make([]byte, 12)...)
	sps = append(sps, payload...)

	info := ParseH265SPS(sps)
	if info == nil {
		t.Fatal("ParseH265SPS returned nil")
	}

	if info.ChromaFormatIDC != 1 {
		t.Errorf("ChromaFormatIDC = %d, want 1", info.ChromaFormatIDC)
	}
	if info.BitDepthLuma != 10 {
		t.Errorf("BitDepthLuma = %d, want 10", info.BitDepthLuma)
	}
	if info.BitDepthChroma != 10 {
		t.Errorf("BitDepthChroma = %d, want 10", info.BitDepthChroma)
	}
	if info.Width != 1920 {
		t.Errorf("Width = %d, want 1920", info.Width)
	}
	if info.Height != 1080 {
		t.Errorf("Height = %d, want 1080", info.Height)
	}
}

func TestParseH265SPS_TooShort(t *testing.T) {
	result := ParseH265SPS([]byte{0x42, 0x01})
	if result != nil {
		t.Error("expected nil for too-short SPS")
	}

	result = ParseH265SPS(nil)
	if result != nil {
		t.Error("expected nil for nil SPS")
	}
}
