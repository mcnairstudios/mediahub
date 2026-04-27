package extradata

import (
	"bytes"
	"testing"
)

func TestH264AnnexBToAvcC(t *testing.T) {
	annexB := []byte{
		0x00, 0x00, 0x00, 0x01,
		0x67, 0x64, 0x00, 0x28, 0xAC, 0xD9, 0x40,
		0x00, 0x00, 0x00, 0x01,
		0x68, 0xEE, 0x38, 0x80,
	}

	result, err := ToCodecData("h264", annexB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result[0] != 0x01 {
		t.Errorf("configurationVersion: got 0x%02x, want 0x01", result[0])
	}
	if result[1] != 0x64 {
		t.Errorf("AVCProfileIndication: got 0x%02x, want 0x64", result[1])
	}
	if result[2] != 0x00 {
		t.Errorf("profile_compatibility: got 0x%02x, want 0x00", result[2])
	}
	if result[3] != 0x28 {
		t.Errorf("AVCLevelIndication: got 0x%02x, want 0x28", result[3])
	}
	if result[4] != 0xFF {
		t.Errorf("lengthSizeMinusOne byte: got 0x%02x, want 0xFF", result[4])
	}
	if result[5] != 0xE1 {
		t.Errorf("numSPS byte: got 0x%02x, want 0xE1", result[5])
	}

	spsLen := int(result[6])<<8 | int(result[7])
	if spsLen != 7 {
		t.Errorf("SPS length: got %d, want 7", spsLen)
	}

	spsData := result[8 : 8+spsLen]
	expectedSPS := []byte{0x67, 0x64, 0x00, 0x28, 0xAC, 0xD9, 0x40}
	if !bytes.Equal(spsData, expectedSPS) {
		t.Errorf("SPS data: got %x, want %x", spsData, expectedSPS)
	}

	ppsCountIdx := 8 + spsLen
	if result[ppsCountIdx] != 0x01 {
		t.Errorf("numPPS: got 0x%02x, want 0x01", result[ppsCountIdx])
	}

	ppsLenIdx := ppsCountIdx + 1
	ppsLen := int(result[ppsLenIdx])<<8 | int(result[ppsLenIdx+1])
	if ppsLen != 4 {
		t.Errorf("PPS length: got %d, want 4", ppsLen)
	}

	ppsData := result[ppsLenIdx+2 : ppsLenIdx+2+ppsLen]
	expectedPPS := []byte{0x68, 0xEE, 0x38, 0x80}
	if !bytes.Equal(ppsData, expectedPPS) {
		t.Errorf("PPS data: got %x, want %x", ppsData, expectedPPS)
	}
}

func TestH264AlreadyAvcC(t *testing.T) {
	avcC := []byte{
		0x01, 0x64, 0x00, 0x28, 0xFF, 0xE1,
		0x00, 0x07, 0x67, 0x64, 0x00, 0x28, 0xAC, 0xD9, 0x40,
		0x01, 0x00, 0x04, 0x68, 0xEE, 0x38, 0x80,
	}

	result, err := ToCodecData("h264", avcC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(result, avcC) {
		t.Errorf("avcC passthrough failed: got %x, want %x", result, avcC)
	}
}

func TestH264ThreeByteStartCode(t *testing.T) {
	annexB := []byte{
		0x00, 0x00, 0x01,
		0x67, 0x42, 0x00, 0x1E, 0xAB,
		0x00, 0x00, 0x01,
		0x68, 0xCE, 0x38, 0x80,
	}

	result, err := ToCodecData("h264", annexB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0] != 0x01 {
		t.Errorf("configurationVersion: got 0x%02x, want 0x01", result[0])
	}
	if result[1] != 0x42 {
		t.Errorf("profile: got 0x%02x, want 0x42 (Baseline)", result[1])
	}
	if result[3] != 0x1E {
		t.Errorf("level: got 0x%02x, want 0x1E (3.0)", result[3])
	}
}

func TestH265AnnexBToHvcC(t *testing.T) {
	fakeSPS := []byte{
		0x42, 0x01,
		0x01,
		0x01,
		0x60, 0x00, 0x00, 0x00,
		0x90, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x5D,
	}

	fakeVPS := []byte{0x40, 0x01, 0x0C, 0x01, 0xFF, 0xFF}
	fakePPS := []byte{0x44, 0x01, 0xC1, 0x72, 0xB4}

	annexB := []byte{}
	annexB = append(annexB, 0x00, 0x00, 0x00, 0x01)
	annexB = append(annexB, fakeVPS...)
	annexB = append(annexB, 0x00, 0x00, 0x00, 0x01)
	annexB = append(annexB, fakeSPS...)
	annexB = append(annexB, 0x00, 0x00, 0x00, 0x01)
	annexB = append(annexB, fakePPS...)

	result, err := ToCodecData("hevc", annexB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result[0] != 0x01 {
		t.Errorf("configurationVersion: got 0x%02x, want 0x01", result[0])
	}

	profileIDC := result[1] & 0x1F
	if profileIDC != 0x01 {
		t.Errorf("general_profile_idc: got %d, want 1", profileIDC)
	}

	if result[12] != 0x5D {
		t.Errorf("general_level_idc: got 0x%02x, want 0x5D", result[12])
	}

	if result[22] != 3 {
		t.Errorf("numOfArrays: got %d, want 3", result[22])
	}

	expectedLen := 23 +
		3 + 2 + len(fakeVPS) +
		3 + 2 + len(fakeSPS) +
		3 + 2 + len(fakePPS)
	if len(result) != expectedLen {
		t.Errorf("output length: got %d, want %d", len(result), expectedLen)
	}
}

func TestH265AlreadyHvcC(t *testing.T) {
	hvcC := []byte{0x01, 0x01, 0x60, 0x00, 0x00, 0x00, 0x90, 0x00}

	result, err := ToCodecData("hevc", hvcC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(result, hvcC) {
		t.Errorf("hvcC passthrough failed: got %x, want %x", result, hvcC)
	}
}

func TestEmptyExtradata(t *testing.T) {
	result, err := ToCodecData("h264", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for empty extradata, got %x", result)
	}

	result, err = ToCodecData("h264", []byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for empty extradata, got %x", result)
	}
}

func TestUnknownCodec(t *testing.T) {
	result, err := ToCodecData("vp8", []byte{0x00, 0x01, 0x02})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for unknown codec, got %x", result)
	}
}

func TestToHexString(t *testing.T) {
	data := []byte{0x01, 0x64, 0x00, 0x28, 0xFF, 0xE1}
	expected := "01640028ffe1"
	got := ToHexString(data)
	if got != expected {
		t.Errorf("ToHexString: got %q, want %q", got, expected)
	}
}

func TestToHexStringEmpty(t *testing.T) {
	got := ToHexString(nil)
	if got != "" {
		t.Errorf("ToHexString(nil): got %q, want empty", got)
	}
}

func TestSplitNALUnits(t *testing.T) {
	data := []byte{
		0x00, 0x00, 0x00, 0x01, 0xAA, 0xBB,
		0x00, 0x00, 0x01, 0xCC, 0xDD, 0xEE,
	}

	nalus := SplitNALUnits(data)
	if len(nalus) != 2 {
		t.Fatalf("expected 2 NALUs, got %d", len(nalus))
	}
	if !bytes.Equal(nalus[0], []byte{0xAA, 0xBB}) {
		t.Errorf("NALU 0: got %x, want aabb", nalus[0])
	}
	if !bytes.Equal(nalus[1], []byte{0xCC, 0xDD, 0xEE}) {
		t.Errorf("NALU 1: got %x, want ccddee", nalus[1])
	}
}

func TestH264NoSPS(t *testing.T) {
	annexB := []byte{
		0x00, 0x00, 0x00, 0x01,
		0x68, 0xEE, 0x38, 0x80,
	}
	_, err := ToCodecData("h264", annexB)
	if err == nil {
		t.Fatal("expected error for missing SPS")
	}
}

func TestH264NoPPS(t *testing.T) {
	annexB := []byte{
		0x00, 0x00, 0x00, 0x01,
		0x67, 0x64, 0x00, 0x28, 0xAC, 0xD9, 0x40,
	}
	_, err := ToCodecData("h264", annexB)
	if err == nil {
		t.Fatal("expected error for missing PPS")
	}
}

func TestH265AlsoAcceptsH265String(t *testing.T) {
	fakeSPS := []byte{
		0x42, 0x01, 0x01, 0x01,
		0x60, 0x00, 0x00, 0x00,
		0x90, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x5D,
	}
	fakePPS := []byte{0x44, 0x01, 0xC1}

	annexB := []byte{}
	annexB = append(annexB, 0x00, 0x00, 0x00, 0x01)
	annexB = append(annexB, fakeSPS...)
	annexB = append(annexB, 0x00, 0x00, 0x00, 0x01)
	annexB = append(annexB, fakePPS...)

	result, err := ToCodecData("h265", annexB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result[0] != 0x01 {
		t.Errorf("configurationVersion: got 0x%02x, want 0x01", result[0])
	}
}
