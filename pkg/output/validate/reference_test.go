package validate

import (
	"embed"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/ref_playlist.m3u8
//go:embed testdata/ref_init.mp4
//go:embed testdata/ref_manifest.mpd
//go:embed testdata/ref_dash_video_init.m4s
//go:embed testdata/ref_dash_audio_init.m4s
var refData embed.FS

func readRef(t *testing.T, name string) []byte {
	t.Helper()
	data, err := refData.ReadFile("testdata/" + name)
	require.NoError(t, err, "reading embedded reference file %s", name)
	return data
}

func hasFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

func hasFFprobe() bool {
	_, err := exec.LookPath("ffprobe")
	return err == nil
}

func generateHLSReference(t *testing.T, dir string) string {
	t.Helper()
	playlistPath := filepath.Join(dir, "playlist.m3u8")
	segPattern := filepath.Join(dir, "seg%d.ts")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=10:size=320x240:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=10:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-g", "50", "-keyint_min", "50",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "hls", "-hls_time", "2", "-hls_list_size", "0",
		"-hls_segment_filename", segPattern,
		playlistPath,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "ffmpeg HLS generation failed: %s", out)
	return dir
}

func generateFMP4Reference(t *testing.T, dir string) string {
	t.Helper()
	outPath := filepath.Join(dir, "output.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=10:size=320x240:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=10:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-frag_duration", "2000000",
		outPath,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "ffmpeg fMP4 generation failed: %s", out)
	return dir
}

func TestReference_HLSPlaylist_AcceptedByValidator(t *testing.T) {
	data := readRef(t, "ref_playlist.m3u8")
	errs := ValidateHLSPlaylist(data)
	assert.Empty(t, errs, "validator rejected ffmpeg reference HLS playlist: %v", errs)
}

func TestReference_HLSPlaylist_Structure(t *testing.T) {
	data := readRef(t, "ref_playlist.m3u8")
	text := string(data)
	lines := strings.Split(strings.TrimSpace(text), "\n")

	assert.Equal(t, "#EXTM3U", strings.TrimSpace(lines[0]), "must start with #EXTM3U")

	hasTargetDuration := false
	hasMediaSequence := false
	hasEndList := false
	var segDurations []float64
	var segNames []string

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") {
			hasTargetDuration = true
		}
		if strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:") {
			hasMediaSequence = true
		}
		if line == "#EXT-X-ENDLIST" {
			hasEndList = true
		}
		if strings.HasPrefix(line, "#EXTINF:") {
			val := strings.TrimPrefix(line, "#EXTINF:")
			if idx := strings.Index(val, ","); idx >= 0 {
				val = val[:idx]
			}
			dur, err := strconv.ParseFloat(val, 64)
			if err == nil {
				segDurations = append(segDurations, dur)
			}
			if i+1 < len(lines) {
				name := strings.TrimSpace(lines[i+1])
				if !strings.HasPrefix(name, "#") {
					segNames = append(segNames, name)
				}
			}
		}
	}

	assert.True(t, hasTargetDuration, "missing #EXT-X-TARGETDURATION")
	assert.True(t, hasMediaSequence, "missing #EXT-X-MEDIA-SEQUENCE")
	assert.True(t, hasEndList, "VOD playlist should have #EXT-X-ENDLIST")
	assert.GreaterOrEqual(t, len(segDurations), 1, "should have at least 1 segment")
	assert.Equal(t, len(segDurations), len(segNames), "each EXTINF should have a segment URI")

	for i := 1; i < len(segNames); i++ {
		assert.NotEqual(t, segNames[i], segNames[i-1], "segment names should be unique")
	}
}

func TestReference_FMP4Init_AcceptedByValidator(t *testing.T) {
	data := readRef(t, "ref_init.mp4")
	errs := ValidateFMP4Init(data)
	assert.Empty(t, errs, "validator rejected ffmpeg reference fMP4 init: %v", errs)
}

func TestReference_FMP4Init_BoxStructure(t *testing.T) {
	data := readRef(t, "ref_init.mp4")
	boxes := parseBoxes(data)

	require.True(t, hasBox(boxes, "ftyp"), "missing ftyp box")
	require.True(t, hasBox(boxes, "moov"), "missing moov box")

	ftyp := findBox(boxes, "ftyp")
	require.NotNil(t, ftyp)
	require.GreaterOrEqual(t, len(ftyp.payload), 4, "ftyp payload too short")
	majorBrand := string(ftyp.payload[:4])
	assert.Contains(t, []string{"isom", "iso5", "iso6", "mp41", "mp42", "avc1", "dash"}, majorBrand,
		"unexpected ftyp major brand: %s", majorBrand)

	moov := findBox(boxes, "moov")
	require.NotNil(t, moov)
	moovChildren := parseBoxes(moov.payload)
	require.True(t, hasBox(moovChildren, "trak"), "moov missing trak")
	require.True(t, hasBox(moovChildren, "mvhd"), "moov missing mvhd")
}

func TestReference_FMP4Init_CodecEntries(t *testing.T) {
	data := readRef(t, "ref_init.mp4")
	boxes := parseBoxes(data)
	moov := findBox(boxes, "moov")
	require.NotNil(t, moov)

	moovChildren := parseBoxes(moov.payload)
	traks := findAllBoxes(moovChildren, "trak")
	require.GreaterOrEqual(t, len(traks), 1, "should have at least one trak")

	foundVideo := false
	foundAudio := false

	for _, trak := range traks {
		stsd := findNestedBox(trak, "mdia", "minf", "stbl", "stsd")
		if stsd == nil {
			continue
		}
		stsdPayload := stsd.payload
		if len(stsdPayload) >= 8 {
			stsdPayload = stsdPayload[8:]
		}
		codecBoxes := parseBoxes(stsdPayload)
		for _, cb := range codecBoxes {
			switch cb.boxType {
			case "avc1", "avc3", "hvc1", "hev1":
				foundVideo = true
			case "mp4a", "Opus":
				foundAudio = true
			}
		}
	}

	assert.True(t, foundVideo || foundAudio, "should find at least one codec entry (video or audio)")
}

func TestReference_DASHInit_VideoAcceptedByValidator(t *testing.T) {
	data := readRef(t, "ref_dash_video_init.m4s")
	errs := ValidateFMP4Init(data)
	assert.Empty(t, errs, "validator rejected ffmpeg reference DASH video init: %v", errs)
}

func TestReference_DASHInit_AudioAcceptedByValidator(t *testing.T) {
	data := readRef(t, "ref_dash_audio_init.m4s")
	errs := ValidateFMP4Init(data)
	assert.Empty(t, errs, "validator rejected ffmpeg reference DASH audio init: %v", errs)
}

func TestReference_DASHInit_VideoBoxStructure(t *testing.T) {
	data := readRef(t, "ref_dash_video_init.m4s")
	boxes := parseBoxes(data)

	require.True(t, hasBox(boxes, "ftyp"), "DASH video init missing ftyp")
	require.True(t, hasBox(boxes, "moov"), "DASH video init missing moov")

	moov := findBox(boxes, "moov")
	moovChildren := parseBoxes(moov.payload)
	trak := findBox(moovChildren, "trak")
	require.NotNil(t, trak, "DASH video init missing trak")

	stsd := findNestedBox(trak, "mdia", "minf", "stbl", "stsd")
	require.NotNil(t, stsd, "DASH video init missing stsd")

	stsdPayload := stsd.payload
	if len(stsdPayload) >= 8 {
		stsdPayload = stsdPayload[8:]
	}
	codecBoxes := parseBoxes(stsdPayload)
	found := false
	for _, cb := range codecBoxes {
		if cb.boxType == "avc1" || cb.boxType == "avc3" || cb.boxType == "hvc1" || cb.boxType == "hev1" {
			found = true
			break
		}
	}
	assert.True(t, found, "DASH video init should contain a video codec box (avc1/hvc1)")
}

func TestReference_DASHInit_AudioBoxStructure(t *testing.T) {
	data := readRef(t, "ref_dash_audio_init.m4s")
	boxes := parseBoxes(data)

	require.True(t, hasBox(boxes, "ftyp"), "DASH audio init missing ftyp")
	require.True(t, hasBox(boxes, "moov"), "DASH audio init missing moov")

	moov := findBox(boxes, "moov")
	moovChildren := parseBoxes(moov.payload)
	trak := findBox(moovChildren, "trak")
	require.NotNil(t, trak, "DASH audio init missing trak")

	stsd := findNestedBox(trak, "mdia", "minf", "stbl", "stsd")
	require.NotNil(t, stsd, "DASH audio init missing stsd")

	stsdPayload := stsd.payload
	if len(stsdPayload) >= 8 {
		stsdPayload = stsdPayload[8:]
	}
	codecBoxes := parseBoxes(stsdPayload)
	found := false
	for _, cb := range codecBoxes {
		if cb.boxType == "mp4a" || cb.boxType == "Opus" {
			found = true
			break
		}
	}
	assert.True(t, found, "DASH audio init should contain an audio codec box (mp4a/Opus)")
}

func TestReference_MPD_AcceptedByValidator(t *testing.T) {
	data := readRef(t, "ref_manifest.mpd")
	errs := ValidateMPD(data)
	assert.Empty(t, errs, "validator rejected ffmpeg reference MPD: %v", errs)
}

func TestReference_MPD_Structure(t *testing.T) {
	data := readRef(t, "ref_manifest.mpd")
	text := string(data)

	assert.Contains(t, text, "urn:mpeg:dash:schema:mpd:2011", "MPD should have DASH namespace")
	assert.Contains(t, text, "<Period", "MPD should have Period element")
	assert.Contains(t, text, "<AdaptationSet", "MPD should have AdaptationSet element")
	assert.Contains(t, text, "<Representation", "MPD should have Representation element")
	assert.Contains(t, text, "<SegmentTemplate", "MPD should have SegmentTemplate element")
	assert.Contains(t, text, "timescale=", "SegmentTemplate should have timescale")
	assert.Contains(t, text, `codecs="avc1`, "MPD should reference H.264 codec")
	assert.Contains(t, text, `codecs="mp4a`, "MPD should reference AAC codec")
}

func TestReference_GeneratedHLS_SegmentDurations(t *testing.T) {
	if !hasFFmpeg() || !hasFFprobe() {
		t.Skip("ffmpeg/ffprobe not available")
	}

	dir := t.TempDir()
	generateHLSReference(t, dir)

	playlistData, err := os.ReadFile(filepath.Join(dir, "playlist.m3u8"))
	require.NoError(t, err)

	errs := ValidateHLSPlaylist(playlistData)
	assert.Empty(t, errs, "validator rejected generated HLS playlist: %v", errs)

	lines := strings.Split(strings.TrimSpace(string(playlistData)), "\n")
	var extinfDurations []float64
	var segFiles []string

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#EXTINF:") {
			val := strings.TrimPrefix(line, "#EXTINF:")
			if idx := strings.Index(val, ","); idx >= 0 {
				val = val[:idx]
			}
			dur, err := strconv.ParseFloat(val, 64)
			require.NoError(t, err)
			extinfDurations = append(extinfDurations, dur)

			if i+1 < len(lines) {
				name := strings.TrimSpace(lines[i+1])
				if !strings.HasPrefix(name, "#") {
					segFiles = append(segFiles, name)
				}
			}
		}
	}

	require.GreaterOrEqual(t, len(segFiles), 3, "should have at least 3 segments for 10s content at 2s target")

	for i, segFile := range segFiles {
		segPath := filepath.Join(dir, segFile)
		cmd := exec.Command("ffprobe", "-v", "quiet",
			"-show_entries", "format=duration",
			"-of", "csv=p=0",
			segPath,
		)
		out, err := cmd.Output()
		require.NoError(t, err, "ffprobe failed on %s", segFile)

		probeDur, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		require.NoError(t, err, "parsing ffprobe duration for %s", segFile)

		assert.InDelta(t, extinfDurations[i], probeDur, 0.1,
			"segment %s: EXTINF duration %.3f vs probed duration %.3f",
			segFile, extinfDurations[i], probeDur)
	}

	totalExtinf := 0.0
	for _, d := range extinfDurations {
		totalExtinf += d
	}
	assert.InDelta(t, 10.0, totalExtinf, 0.5, "total EXTINF duration should be ~10s, got %.3f", totalExtinf)

	targetDur := 2.0
	for _, d := range extinfDurations {
		assert.LessOrEqual(t, d, targetDur*1.1+0.1, "segment duration %.3f exceeds 110%% of target", d)
	}
}

func TestReference_GeneratedHLS_TSSegmentStructure(t *testing.T) {
	if !hasFFmpeg() {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	generateHLSReference(t, dir)

	seg0, err := os.ReadFile(filepath.Join(dir, "seg0.ts"))
	require.NoError(t, err)

	errs := ValidateTSSegment(seg0)
	assert.Empty(t, errs, "validator rejected reference TS segment: %v", errs)

	assert.Equal(t, 0, len(seg0)%188, "TS segment should be multiple of 188 bytes")
	assert.Equal(t, byte(0x47), seg0[0], "first byte should be TS sync byte")
}

func TestReference_GeneratedHLS_SegmentConsistency(t *testing.T) {
	if !hasFFmpeg() || !hasFFprobe() {
		t.Skip("ffmpeg/ffprobe not available")
	}

	dir := t.TempDir()
	generateHLSReference(t, dir)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var segSizes []int64
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".ts") {
			info, err := entry.Info()
			require.NoError(t, err)
			segSizes = append(segSizes, info.Size())
		}
	}

	require.GreaterOrEqual(t, len(segSizes), 3)

	var totalSize int64
	for _, s := range segSizes {
		totalSize += s
		assert.Greater(t, s, int64(1000), "TS segment should be > 1KB")
	}

	avgSize := float64(totalSize) / float64(len(segSizes))
	for _, s := range segSizes {
		ratio := float64(s) / avgSize
		assert.Greater(t, ratio, 0.2, "segment size %d is too small relative to avg %.0f", s, avgSize)
		assert.Less(t, ratio, 5.0, "segment size %d is too large relative to avg %.0f", s, avgSize)
	}
}

func TestReference_GeneratedFMP4_InitAndSegments(t *testing.T) {
	if !hasFFmpeg() {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	generateFMP4Reference(t, dir)

	data, err := os.ReadFile(filepath.Join(dir, "output.mp4"))
	require.NoError(t, err)

	boxes := parseBoxes(data)
	require.True(t, hasBox(boxes, "ftyp"), "fMP4 missing ftyp")
	require.True(t, hasBox(boxes, "moov"), "fMP4 missing moov")
	require.True(t, hasBox(boxes, "moof"), "fMP4 missing moof (no fragments?)")
	require.True(t, hasBox(boxes, "mdat"), "fMP4 missing mdat")

	ftyp := findBox(boxes, "ftyp")
	moov := findBox(boxes, "moov")
	initEnd := 8 + len(ftyp.payload) + 8 + len(moov.payload)
	initData := data[:initEnd]

	errs := ValidateFMP4Init(initData)
	assert.Empty(t, errs, "validator rejected fMP4 init portion: %v", errs)

	moofCount := 0
	for _, b := range boxes {
		if b.boxType == "moof" {
			moofCount++
		}
	}
	assert.GreaterOrEqual(t, moofCount, 1, "should have at least 1 moof fragment")

	var firstMoofStart, firstMdatEnd int
	pos := 0
	foundMoof := false
	for _, b := range boxes {
		boxSize := 8 + len(b.payload)
		if b.boxType == "moof" && !foundMoof {
			firstMoofStart = pos
			foundMoof = true
		}
		if b.boxType == "mdat" && foundMoof {
			firstMdatEnd = pos + boxSize
			break
		}
		pos += boxSize
	}

	if firstMdatEnd > firstMoofStart {
		segData := data[firstMoofStart:firstMdatEnd]
		errs := ValidateFMP4Segment(segData)
		assert.Empty(t, errs, "validator rejected first fMP4 segment: %v", errs)
	}
}

func TestReference_GeneratedFMP4_FragmentTiming(t *testing.T) {
	if !hasFFmpeg() || !hasFFprobe() {
		t.Skip("ffmpeg/ffprobe not available")
	}

	dir := t.TempDir()
	generateFMP4Reference(t, dir)

	outPath := filepath.Join(dir, "output.mp4")
	cmd := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		outPath,
	)
	out, err := cmd.Output()
	require.NoError(t, err)

	dur, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	require.NoError(t, err)
	assert.InDelta(t, 10.0, dur, 0.5, "fMP4 duration should be ~10s, got %.3f", dur)

	cmd = exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "stream=codec_name,codec_type",
		"-of", "csv=p=0",
		outPath,
	)
	out, err = cmd.Output()
	require.NoError(t, err)

	streamInfo := string(out)
	assert.Contains(t, streamInfo, "h264", "should contain H.264 video stream")
	assert.Contains(t, streamInfo, "aac", "should contain AAC audio stream")
}

func TestReference_TSSegment_SyncBytes(t *testing.T) {
	packetCount := 20
	data := make([]byte, packetCount*188)
	for i := 0; i < packetCount; i++ {
		data[i*188] = 0x47
		if i == 0 {
			data[i*188+1] = 0x40
			data[i*188+2] = 0x00
		} else {
			data[i*188+1] = 0x01
			data[i*188+2] = 0x00
		}
	}

	errs := ValidateTSSegment(data)
	assert.Empty(t, errs)
}

func TestReference_TSSegment_InvalidSync(t *testing.T) {
	data := make([]byte, 188*3)
	data[0] = 0xFF
	data[188] = 0xFF
	data[376] = 0xFF

	errs := ValidateTSSegment(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "sync")
}

func TestReference_TSSegment_Empty(t *testing.T) {
	errs := ValidateTSSegment(nil)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "data")
}

func TestReference_TSSegment_BadAlignment(t *testing.T) {
	data := make([]byte, 100)
	data[0] = 0x47

	errs := ValidateTSSegment(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "alignment")
}

func findAllBoxes(boxes []*mp4Box, boxType string) []*mp4Box {
	var result []*mp4Box
	for _, b := range boxes {
		if b.boxType == boxType {
			result = append(result, b)
		}
	}
	return result
}

func findNestedBox(parent *mp4Box, path ...string) *mp4Box {
	current := parent
	for _, name := range path {
		children := parseBoxes(current.payload)
		child := findBox(children, name)
		if child == nil {
			return nil
		}
		current = child
	}
	return current
}

func TestReference_SegmentDurationVariance(t *testing.T) {
	if !hasFFmpeg() || !hasFFprobe() {
		t.Skip("ffmpeg/ffprobe not available")
	}

	dir := t.TempDir()
	generateHLSReference(t, dir)

	playlistData, err := os.ReadFile(filepath.Join(dir, "playlist.m3u8"))
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(playlistData)), "\n")
	var durations []float64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#EXTINF:") {
			val := strings.TrimPrefix(line, "#EXTINF:")
			if idx := strings.Index(val, ","); idx >= 0 {
				val = val[:idx]
			}
			dur, err := strconv.ParseFloat(val, 64)
			require.NoError(t, err)
			durations = append(durations, dur)
		}
	}

	require.GreaterOrEqual(t, len(durations), 2)

	targetDur := 2.0
	for _, d := range durations {
		deviation := math.Abs(d-targetDur) / targetDur
		assert.Less(t, deviation, 0.15, "segment duration %.3f deviates >15%% from target %.1f", d, targetDur)
	}
}
