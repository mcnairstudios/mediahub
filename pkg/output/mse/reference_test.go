package mse

import (
	"embed"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/output/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/ref_h264_init.bin
//go:embed testdata/ref_hevc_init.bin
//go:embed testdata/ref_av_init.bin
var refFS embed.FS

func readRefData(t *testing.T, name string) []byte {
	t.Helper()
	data, err := refFS.ReadFile("testdata/" + name)
	require.NoError(t, err, "reading embedded reference %s", name)
	return data
}

type refBox struct {
	boxType string
	payload []byte
}

func refParseBoxes(data []byte) []*refBox {
	var boxes []*refBox
	offset := 0
	for offset+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		fourcc := string(data[offset+4 : offset+8])
		headerSize := 8
		if size == 1 && offset+16 <= len(data) {
			extSize := binary.BigEndian.Uint64(data[offset+8 : offset+16])
			if extSize > uint64(len(data)-offset) {
				size = len(data) - offset
			} else {
				size = int(extSize)
			}
			headerSize = 16
		}
		if size < headerSize {
			break
		}
		if size > len(data)-offset {
			size = len(data) - offset
		}
		boxes = append(boxes, &refBox{
			boxType: fourcc,
			payload: data[offset+headerSize : offset+size],
		})
		offset += size
	}
	return boxes
}

func refFindBox(boxes []*refBox, bt string) *refBox {
	for _, b := range boxes {
		if b.boxType == bt {
			return b
		}
	}
	return nil
}

func refHasBox(boxes []*refBox, bt string) bool {
	return refFindBox(boxes, bt) != nil
}

func refBoxTypes(boxes []*refBox) []string {
	var types []string
	for _, b := range boxes {
		types = append(types, b.boxType)
	}
	return types
}

func refCountBoxes(boxes []*refBox, bt string) int {
	n := 0
	for _, b := range boxes {
		if b.boxType == bt {
			n++
		}
	}
	return n
}

func refFindNestedBox(data []byte, path ...string) []byte {
	current := data
	for _, name := range path {
		boxes := refParseBoxes(current)
		b := refFindBox(boxes, name)
		if b == nil {
			return nil
		}
		current = b.payload
	}
	return current
}

func TestRefH264Init_ValidatorAccepts(t *testing.T) {
	data := readRefData(t, "ref_h264_init.bin")
	errs := validate.ValidateFMP4Init(data)
	assert.Empty(t, errs, "validator should accept ffmpeg H.264 init segment: %v", errs)
}

func TestRefHEVCInit_ValidatorAccepts(t *testing.T) {
	data := readRefData(t, "ref_hevc_init.bin")
	errs := validate.ValidateFMP4Init(data)
	assert.Empty(t, errs, "validator should accept ffmpeg HEVC init segment: %v", errs)
}

func TestRefAVInit_ValidatorAccepts(t *testing.T) {
	data := readRefData(t, "ref_av_init.bin")
	errs := validate.ValidateFMP4Init(data)
	assert.Empty(t, errs, "validator should accept ffmpeg A/V init segment: %v", errs)
}

func TestRefH264Init_FtypBox(t *testing.T) {
	data := readRefData(t, "ref_h264_init.bin")
	boxes := refParseBoxes(data)
	require.True(t, refHasBox(boxes, "ftyp"), "missing ftyp box")

	ftyp := refFindBox(boxes, "ftyp")
	require.GreaterOrEqual(t, len(ftyp.payload), 4)
	majorBrand := string(ftyp.payload[:4])
	validBrands := []string{"isom", "iso5", "iso6", "mp41", "mp42", "avc1", "dash"}
	assert.Contains(t, validBrands, majorBrand, "unexpected major brand: %s", majorBrand)
}

func TestRefH264Init_MoovStructure(t *testing.T) {
	data := readRefData(t, "ref_h264_init.bin")
	boxes := refParseBoxes(data)
	require.True(t, refHasBox(boxes, "moov"), "missing moov")

	moov := refFindBox(boxes, "moov")
	moovChildren := refParseBoxes(moov.payload)

	required := []string{"mvhd", "trak"}
	for _, req := range required {
		assert.True(t, refHasBox(moovChildren, req), "moov missing %s", req)
	}
}

func TestRefH264Init_TrakChain(t *testing.T) {
	data := readRefData(t, "ref_h264_init.bin")
	boxes := refParseBoxes(data)
	moov := refFindBox(boxes, "moov")
	require.NotNil(t, moov)

	trak := refFindBox(refParseBoxes(moov.payload), "trak")
	require.NotNil(t, trak)
	trakChildren := refParseBoxes(trak.payload)
	assert.True(t, refHasBox(trakChildren, "tkhd"), "trak missing tkhd")
	assert.True(t, refHasBox(trakChildren, "mdia"), "trak missing mdia")

	mdia := refFindBox(trakChildren, "mdia")
	mdiaChildren := refParseBoxes(mdia.payload)
	assert.True(t, refHasBox(mdiaChildren, "mdhd"), "mdia missing mdhd")
	assert.True(t, refHasBox(mdiaChildren, "hdlr"), "mdia missing hdlr")
	assert.True(t, refHasBox(mdiaChildren, "minf"), "mdia missing minf")

	minf := refFindBox(mdiaChildren, "minf")
	minfChildren := refParseBoxes(minf.payload)
	assert.True(t, refHasBox(minfChildren, "stbl"), "minf missing stbl")

	stbl := refFindBox(minfChildren, "stbl")
	stblChildren := refParseBoxes(stbl.payload)
	assert.True(t, refHasBox(stblChildren, "stsd"), "stbl missing stsd")
}

func TestRefH264Init_AvcCPresent(t *testing.T) {
	data := readRefData(t, "ref_h264_init.bin")
	boxes := refParseBoxes(data)
	moov := refFindBox(boxes, "moov")
	trak := refFindBox(refParseBoxes(moov.payload), "trak")
	mdia := refFindBox(refParseBoxes(trak.payload), "mdia")
	minf := refFindBox(refParseBoxes(mdia.payload), "minf")
	stbl := refFindBox(refParseBoxes(minf.payload), "stbl")
	stsd := refFindBox(refParseBoxes(stbl.payload), "stsd")

	require.GreaterOrEqual(t, len(stsd.payload), 8)
	entries := stsd.payload[8:]
	codecBoxes := refParseBoxes(entries)

	avc1 := refFindBox(codecBoxes, "avc1")
	require.NotNil(t, avc1, "stsd missing avc1 entry")
	require.GreaterOrEqual(t, len(avc1.payload), 78+8)

	innerBoxes := refParseBoxes(avc1.payload[78:])
	assert.True(t, refHasBox(innerBoxes, "avcC"), "avc1 missing avcC box")

	avcC := refFindBox(innerBoxes, "avcC")
	require.GreaterOrEqual(t, len(avcC.payload), 4)
	assert.Equal(t, byte(1), avcC.payload[0], "avcC config version should be 1")
	profile := avcC.payload[1]
	assert.Equal(t, byte(0x42), profile, "expected baseline profile (0x42)")
}

func TestRefHEVCInit_HvcCPresent(t *testing.T) {
	data := readRefData(t, "ref_hevc_init.bin")
	boxes := refParseBoxes(data)
	moov := refFindBox(boxes, "moov")
	trak := refFindBox(refParseBoxes(moov.payload), "trak")
	mdia := refFindBox(refParseBoxes(trak.payload), "mdia")
	minf := refFindBox(refParseBoxes(mdia.payload), "minf")
	stbl := refFindBox(refParseBoxes(minf.payload), "stbl")
	stsd := refFindBox(refParseBoxes(stbl.payload), "stsd")

	require.GreaterOrEqual(t, len(stsd.payload), 8)
	entries := stsd.payload[8:]
	codecBoxes := refParseBoxes(entries)

	hvc1 := refFindBox(codecBoxes, "hvc1")
	require.NotNil(t, hvc1, "stsd missing hvc1 entry")
	require.GreaterOrEqual(t, len(hvc1.payload), 78+8)

	innerBoxes := refParseBoxes(hvc1.payload[78:])
	assert.True(t, refHasBox(innerBoxes, "hvcC"), "hvc1 missing hvcC box")

	hvcC := refFindBox(innerBoxes, "hvcC")
	require.GreaterOrEqual(t, len(hvcC.payload), 13)
	assert.Equal(t, byte(1), hvcC.payload[0], "hvcC config version should be 1")
	profileIDC := hvcC.payload[1] & 0x1F
	assert.Equal(t, byte(1), profileIDC, "expected Main profile (1)")
}

func TestRefAVInit_HasVideoAndAudioTracks(t *testing.T) {
	data := readRefData(t, "ref_av_init.bin")
	boxes := refParseBoxes(data)
	moov := refFindBox(boxes, "moov")
	require.NotNil(t, moov)

	moovChildren := refParseBoxes(moov.payload)
	trakCount := refCountBoxes(moovChildren, "trak")
	assert.Equal(t, 2, trakCount, "A/V init should have 2 trak boxes (video + audio)")
}

func TestRefAVInit_AudioEsdsPresent(t *testing.T) {
	data := readRefData(t, "ref_av_init.bin")
	boxes := refParseBoxes(data)
	moov := refFindBox(boxes, "moov")
	moovChildren := refParseBoxes(moov.payload)

	foundMp4a := false
	for _, child := range moovChildren {
		if child.boxType != "trak" {
			continue
		}
		trakChildren := refParseBoxes(child.payload)
		mdia := refFindBox(trakChildren, "mdia")
		if mdia == nil {
			continue
		}
		minf := refFindBox(refParseBoxes(mdia.payload), "minf")
		if minf == nil {
			continue
		}
		stbl := refFindBox(refParseBoxes(minf.payload), "stbl")
		if stbl == nil {
			continue
		}
		stsd := refFindBox(refParseBoxes(stbl.payload), "stsd")
		if stsd == nil || len(stsd.payload) < 8 {
			continue
		}
		codecBoxes := refParseBoxes(stsd.payload[8:])
		mp4a := refFindBox(codecBoxes, "mp4a")
		if mp4a != nil && len(mp4a.payload) >= 28 {
			foundMp4a = true
			innerBoxes := refParseBoxes(mp4a.payload[28:])
			assert.True(t, refHasBox(innerBoxes, "esds"), "mp4a missing esds box")
		}
	}
	assert.True(t, foundMp4a, "A/V init should contain mp4a codec entry")
}

func TestRefH264Init_CodecString(t *testing.T) {
	data := readRefData(t, "ref_h264_init.bin")
	boxes := refParseBoxes(data)
	moov := refFindBox(boxes, "moov")
	trak := refFindBox(refParseBoxes(moov.payload), "trak")
	mdia := refFindBox(refParseBoxes(trak.payload), "mdia")
	minf := refFindBox(refParseBoxes(mdia.payload), "minf")
	stbl := refFindBox(refParseBoxes(minf.payload), "stbl")
	stsd := refFindBox(refParseBoxes(stbl.payload), "stsd")
	entries := stsd.payload[8:]
	avc1 := refFindBox(refParseBoxes(entries), "avc1")
	avcC := refFindBox(refParseBoxes(avc1.payload[78:]), "avcC")

	profile := avcC.payload[1]
	compat := avcC.payload[2]
	level := avcC.payload[3]
	codecStr := strings.ToUpper(
		"avc1." + hexByte(profile) + hexByte(compat) + hexByte(level),
	)

	assert.True(t, strings.HasPrefix(codecStr, "AVC1.42"), "expected baseline profile codec string, got %s", codecStr)
	assert.Equal(t, "AVC1.42C01E", codecStr, "expected avc1.42C01E for baseline level 3.0")
}

func TestRefHEVCInit_CodecString(t *testing.T) {
	data := readRefData(t, "ref_hevc_init.bin")
	boxes := refParseBoxes(data)
	moov := refFindBox(boxes, "moov")
	trak := refFindBox(refParseBoxes(moov.payload), "trak")
	mdia := refFindBox(refParseBoxes(trak.payload), "mdia")
	minf := refFindBox(refParseBoxes(mdia.payload), "minf")
	stbl := refFindBox(refParseBoxes(minf.payload), "stbl")
	stsd := refFindBox(refParseBoxes(stbl.payload), "stsd")
	entries := stsd.payload[8:]
	hvc1 := refFindBox(refParseBoxes(entries), "hvc1")
	hvcC := refFindBox(refParseBoxes(hvc1.payload[78:]), "hvcC")

	profileIDC := hvcC.payload[1] & 0x1F
	tierFlag := (hvcC.payload[1] >> 5) & 0x01
	levelIDC := hvcC.payload[12]

	assert.Equal(t, byte(1), profileIDC, "expected Main profile")
	assert.Equal(t, byte(0), tierFlag, "expected Main tier")
	assert.Greater(t, levelIDC, byte(0), "level should be > 0")

	tier := "L"
	if tierFlag == 1 {
		tier = "H"
	}
	assert.True(t, strings.HasPrefix("hvc1.1."+tier, "hvc1.1.L"), "expected hvc1.1.L prefix")
}

func TestRefSegment_MoofMdatStructure(t *testing.T) {
	seg := validate.BuildFMP4SegmentForTest(25)
	boxes := refParseBoxes(seg)
	require.True(t, refHasBox(boxes, "moof"), "segment missing moof")
	require.True(t, refHasBox(boxes, "mdat"), "segment missing mdat")

	moof := refFindBox(boxes, "moof")
	moofChildren := refParseBoxes(moof.payload)
	require.True(t, refHasBox(moofChildren, "traf"), "moof missing traf")

	traf := refFindBox(moofChildren, "traf")
	trafChildren := refParseBoxes(traf.payload)
	assert.True(t, refHasBox(trafChildren, "tfhd"), "traf missing tfhd")
	assert.True(t, refHasBox(trafChildren, "tfdt"), "traf missing tfdt")
	assert.True(t, refHasBox(trafChildren, "trun"), "traf missing trun")

	trun := refFindBox(trafChildren, "trun")
	require.GreaterOrEqual(t, len(trun.payload), 8)
	sampleCount := binary.BigEndian.Uint32(trun.payload[4:8])
	assert.Equal(t, uint32(25), sampleCount)
}

func TestRefSegment_TfdtBaseDecodeTime(t *testing.T) {
	seg := validate.BuildFMP4SegmentForTest(30)
	boxes := refParseBoxes(seg)
	moof := refFindBox(boxes, "moof")
	traf := refFindBox(refParseBoxes(moof.payload), "traf")
	tfdt := refFindBox(refParseBoxes(traf.payload), "tfdt")
	require.NotNil(t, tfdt)
	require.GreaterOrEqual(t, len(tfdt.payload), 12)

	version := tfdt.payload[0]
	assert.Equal(t, byte(1), version, "tfdt should use version 1 (64-bit)")

	bdt := binary.BigEndian.Uint64(tfdt.payload[4:12])
	assert.Equal(t, uint64(90000), bdt)
}

func TestRefSegment_ZeroSamplesFails(t *testing.T) {
	seg := validate.BuildFMP4SegmentForTest(0)
	errs := validate.ValidateFMP4Segment(seg)
	assert.NotEmpty(t, errs, "zero-sample segment should fail validation")
	found := false
	for _, e := range errs {
		if e.Field == "samples" {
			found = true
		}
	}
	assert.True(t, found, "expected 'samples' error")
}

func TestRef_GeneratedFMP4_InitStructureMatchesEmbedded(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=3:size=320x240:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-g", "25",
		"-an",
		"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof",
		outPath,
	)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)

	genBoxes := refParseBoxes(data)
	embeddedBoxes := refParseBoxes(readRefData(t, "ref_h264_init.bin"))

	genTypes := refBoxTypes(genBoxes)
	embTypes := refBoxTypes(embeddedBoxes)

	assert.Contains(t, genTypes, "ftyp")
	assert.Contains(t, genTypes, "moov")
	assert.Contains(t, embTypes, "ftyp")
	assert.Contains(t, embTypes, "moov")

	genFtyp := refFindBox(genBoxes, "ftyp")
	embFtyp := refFindBox(embeddedBoxes, "ftyp")
	require.GreaterOrEqual(t, len(genFtyp.payload), 4)
	require.GreaterOrEqual(t, len(embFtyp.payload), 4)
	genBrand := string(genFtyp.payload[:4])
	embBrand := string(embFtyp.payload[:4])
	assert.Equal(t, embBrand, genBrand, "ftyp major brand should match")

	for _, required := range []string{"mvhd", "trak"} {
		genMoov := refFindBox(genBoxes, "moov")
		genMoovChildren := refParseBoxes(genMoov.payload)
		assert.True(t, refHasBox(genMoovChildren, required), "generated moov missing %s", required)
	}
}

func TestRef_GeneratedFMP4_SegmentTimingMonotonic(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=320x240:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-g", "25",
		"-an",
		"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof",
		outPath,
	)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)

	boxes := refParseBoxes(data)

	var seqNums []uint32
	var baseDecodeTimes []uint64
	var sampleCounts []uint32

	for _, b := range boxes {
		if b.boxType != "moof" {
			continue
		}
		moofChildren := refParseBoxes(b.payload)
		mfhd := refFindBox(moofChildren, "mfhd")
		if mfhd != nil && len(mfhd.payload) >= 8 {
			seqNums = append(seqNums, binary.BigEndian.Uint32(mfhd.payload[4:8]))
		}

		traf := refFindBox(moofChildren, "traf")
		if traf == nil {
			continue
		}
		trafChildren := refParseBoxes(traf.payload)

		tfdt := refFindBox(trafChildren, "tfdt")
		if tfdt != nil && len(tfdt.payload) >= 12 {
			version := tfdt.payload[0]
			if version == 1 {
				baseDecodeTimes = append(baseDecodeTimes, binary.BigEndian.Uint64(tfdt.payload[4:12]))
			} else if len(tfdt.payload) >= 8 {
				baseDecodeTimes = append(baseDecodeTimes, uint64(binary.BigEndian.Uint32(tfdt.payload[4:8])))
			}
		}

		trun := refFindBox(trafChildren, "trun")
		if trun != nil && len(trun.payload) >= 8 {
			sampleCounts = append(sampleCounts, binary.BigEndian.Uint32(trun.payload[4:8]))
		}
	}

	require.GreaterOrEqual(t, len(seqNums), 2, "should have multiple fragments")
	t.Logf("fragment count: %d", len(seqNums))
	t.Logf("sequence numbers: %v", seqNums)
	t.Logf("base decode times: %v", baseDecodeTimes)
	t.Logf("sample counts: %v", sampleCounts)

	for i := 1; i < len(seqNums); i++ {
		assert.Equal(t, seqNums[i-1]+1, seqNums[i],
			"mfhd sequence numbers should increment: %d -> %d", seqNums[i-1], seqNums[i])
	}

	for i := 1; i < len(baseDecodeTimes); i++ {
		assert.Greater(t, baseDecodeTimes[i], baseDecodeTimes[i-1],
			"base_decode_time should increase monotonically: %d -> %d", baseDecodeTimes[i-1], baseDecodeTimes[i])
	}

	for _, sc := range sampleCounts {
		assert.Greater(t, sc, uint32(0), "sample count should be > 0")
	}
}

func TestRef_GeneratedFMP4_FragmentDurationWithinTolerance(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=10:size=320x240:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-g", "25",
		"-an",
		"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof",
		outPath,
	)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)

	boxes := refParseBoxes(data)

	var timescale uint32
	moov := refFindBox(boxes, "moov")
	if moov != nil {
		moovChildren := refParseBoxes(moov.payload)
		trak := refFindBox(moovChildren, "trak")
		if trak != nil {
			mdiaData := refFindNestedBox(trak.payload, "mdia")
			if mdiaData != nil {
				mdiaChildren := refParseBoxes(mdiaData)
				mdhd := refFindBox(mdiaChildren, "mdhd")
				if mdhd != nil && len(mdhd.payload) >= 20 {
					version := mdhd.payload[0]
					if version == 0 && len(mdhd.payload) >= 16 {
						timescale = binary.BigEndian.Uint32(mdhd.payload[12:16])
					} else if version == 1 && len(mdhd.payload) >= 24 {
						timescale = binary.BigEndian.Uint32(mdhd.payload[20:24])
					}
				}
			}
		}
	}

	if timescale == 0 {
		timescale = 12800
		t.Logf("using default timescale %d", timescale)
	} else {
		t.Logf("detected timescale: %d", timescale)
	}

	var baseDecodeTimes []uint64
	for _, b := range boxes {
		if b.boxType != "moof" {
			continue
		}
		moofChildren := refParseBoxes(b.payload)
		traf := refFindBox(moofChildren, "traf")
		if traf == nil {
			continue
		}
		tfdt := refFindBox(refParseBoxes(traf.payload), "tfdt")
		if tfdt == nil {
			continue
		}
		version := tfdt.payload[0]
		if version == 1 && len(tfdt.payload) >= 12 {
			baseDecodeTimes = append(baseDecodeTimes, binary.BigEndian.Uint64(tfdt.payload[4:12]))
		} else if len(tfdt.payload) >= 8 {
			baseDecodeTimes = append(baseDecodeTimes, uint64(binary.BigEndian.Uint32(tfdt.payload[4:8])))
		}
	}

	require.GreaterOrEqual(t, len(baseDecodeTimes), 2)

	expectedDurSec := 1.0
	for i := 1; i < len(baseDecodeTimes); i++ {
		delta := baseDecodeTimes[i] - baseDecodeTimes[i-1]
		durSec := float64(delta) / float64(timescale)
		t.Logf("fragment %d->%d: delta=%d (%.3fs)", i-1, i, delta, durSec)

		deviation := math.Abs(durSec-expectedDurSec) / expectedDurSec
		assert.Less(t, deviation, 0.25,
			"fragment duration %.3fs deviates >25%% from expected %.1fs", durSec, expectedDurSec)
	}
}

func TestRef_GeneratedHEVC_InitBoxStructure(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "hevc.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=3:size=320x240:rate=25",
		"-c:v", "libx265", "-preset", "ultrafast",
		"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-tag:v", "hvc1",
		outPath,
	)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)

	initEnd := 0
	offset := 0
	for offset+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		fourcc := string(data[offset+4 : offset+8])
		if size < 8 {
			break
		}
		if fourcc == "moof" {
			break
		}
		initEnd = offset + size
		offset += size
	}

	initData := data[:initEnd]
	errs := validate.ValidateFMP4Init(initData)
	assert.Empty(t, errs, "HEVC init should pass validation: %v", errs)

	boxes := refParseBoxes(initData)
	moov := refFindBox(boxes, "moov")
	require.NotNil(t, moov)
	trak := refFindBox(refParseBoxes(moov.payload), "trak")
	require.NotNil(t, trak)
	mdia := refFindBox(refParseBoxes(trak.payload), "mdia")
	minf := refFindBox(refParseBoxes(mdia.payload), "minf")
	stbl := refFindBox(refParseBoxes(minf.payload), "stbl")
	stsd := refFindBox(refParseBoxes(stbl.payload), "stsd")
	codecBoxes := refParseBoxes(stsd.payload[8:])

	hvc1 := refFindBox(codecBoxes, "hvc1")
	require.NotNil(t, hvc1, "HEVC stsd should have hvc1 entry")
	require.GreaterOrEqual(t, len(hvc1.payload), 78+8)

	innerBoxes := refParseBoxes(hvc1.payload[78:])
	assert.True(t, refHasBox(innerBoxes, "hvcC"), "hvc1 should have hvcC config box")
}

func TestRef_GeneratedAV_HasBothTracksAndFragments(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "av.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=320x240:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-g", "25",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof",
		outPath,
	)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)

	boxes := refParseBoxes(data)
	moov := refFindBox(boxes, "moov")
	require.NotNil(t, moov)

	moovChildren := refParseBoxes(moov.payload)
	trakCount := refCountBoxes(moovChildren, "trak")
	assert.Equal(t, 2, trakCount, "A/V should have 2 tracks")

	moofCount := refCountBoxes(boxes, "moof")
	mdatCount := refCountBoxes(boxes, "mdat")
	assert.GreaterOrEqual(t, moofCount, 2, "should have at least 2 moof fragments")
	assert.Equal(t, moofCount, mdatCount, "moof and mdat counts should match")
	t.Logf("A/V: %d moof + %d mdat", moofCount, mdatCount)
}

func TestRef_InitNoMoofOrMdat(t *testing.T) {
	for _, name := range []string{"ref_h264_init.bin", "ref_hevc_init.bin", "ref_av_init.bin"} {
		t.Run(name, func(t *testing.T) {
			data := readRefData(t, name)
			boxes := refParseBoxes(data)
			assert.False(t, refHasBox(boxes, "moof"), "init segment should not contain moof")
			assert.False(t, refHasBox(boxes, "mdat"), "init segment should not contain mdat")
		})
	}
}

func TestRef_MediaSegmentNoFtypOrMoov(t *testing.T) {
	seg := validate.BuildFMP4SegmentForTest(10)
	boxes := refParseBoxes(seg)
	assert.False(t, refHasBox(boxes, "ftyp"), "media segment should not contain ftyp")
	assert.False(t, refHasBox(boxes, "moov"), "media segment should not contain moov")
}

func TestRef_OurMuxerInitMatchesReference(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found")
	}

	dir := t.TempDir()
	segDir := filepath.Join(dir, "segments")
	require.NoError(t, os.MkdirAll(segDir, 0755))

	refData := readRefData(t, "ref_h264_init.bin")
	refBoxes := refParseBoxes(refData)

	writeInitSegment(t, segDir, "init_video.mp4", "avc1")
	ourInitData, err := os.ReadFile(filepath.Join(segDir, "init_video.mp4"))
	require.NoError(t, err)
	ourBoxes := refParseBoxes(ourInitData)

	refTypes := refBoxTypes(refBoxes)
	ourTypes := refBoxTypes(ourBoxes)
	t.Logf("reference top-level: %v", refTypes)
	t.Logf("ours top-level: %v", ourTypes)

	assert.Contains(t, ourTypes, "ftyp", "our init must have ftyp")
	assert.Contains(t, ourTypes, "moov", "our init must have moov")

	refMoov := refFindBox(refBoxes, "moov")
	ourMoov := refFindBox(ourBoxes, "moov")
	refMoovTypes := refBoxTypes(refParseBoxes(refMoov.payload))
	ourMoovTypes := refBoxTypes(refParseBoxes(ourMoov.payload))
	t.Logf("reference moov children: %v", refMoovTypes)
	t.Logf("our moov children: %v", ourMoovTypes)

	for _, required := range []string{"trak"} {
		if refHasBox(refParseBoxes(refMoov.payload), required) {
			assert.Contains(t, ourMoovTypes, required, "our moov should have %s (reference has it)", required)
		}
	}
}

func hexByte(b byte) string {
	const hex = "0123456789ABCDEF"
	return string([]byte{hex[b>>4], hex[b&0x0f]})
}

