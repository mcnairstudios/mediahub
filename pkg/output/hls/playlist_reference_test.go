package hls

import (
	"embed"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/output/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/ref_vod_playlist.m3u8
//go:embed testdata/ref_live_playlist.m3u8
//go:embed testdata/ref_hevc_playlist.m3u8
//go:embed testdata/ref_segment.ts
var refHLSData embed.FS

func readRefHLS(t *testing.T, name string) []byte {
	t.Helper()
	data, err := refHLSData.ReadFile("testdata/" + name)
	require.NoError(t, err, "reading embedded reference file %s", name)
	return data
}

func ffmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

func ffprobeAvailable() bool {
	_, err := exec.LookPath("ffprobe")
	return err == nil
}

type parsedPlaylist struct {
	lines            []string
	version          int
	targetDuration   float64
	mediaSequence    int
	hasEndList       bool
	segmentDurations []float64
	segmentNames     []string
}

func parsePlaylist(t *testing.T, data []byte) parsedPlaylist {
	t.Helper()
	text := string(data)
	lines := strings.Split(strings.TrimSpace(text), "\n")

	p := parsedPlaylist{lines: lines, version: -1, targetDuration: -1, mediaSequence: -1}

	for i, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "#EXT-X-VERSION:") {
			val := strings.TrimPrefix(line, "#EXT-X-VERSION:")
			v, err := strconv.Atoi(val)
			if err == nil {
				p.version = v
			}
		}

		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") {
			val := strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:")
			td, err := strconv.ParseFloat(val, 64)
			if err == nil {
				p.targetDuration = td
			}
		}

		if strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:") {
			val := strings.TrimPrefix(line, "#EXT-X-MEDIA-SEQUENCE:")
			ms, err := strconv.Atoi(val)
			if err == nil {
				p.mediaSequence = ms
			}
		}

		if strings.HasPrefix(line, "#EXT-X-ENDLIST") {
			p.hasEndList = true
		}

		if strings.HasPrefix(line, "#EXTINF:") {
			val := strings.TrimPrefix(line, "#EXTINF:")
			if idx := strings.Index(val, ","); idx >= 0 {
				val = val[:idx]
			}
			dur, err := strconv.ParseFloat(val, 64)
			if err == nil {
				p.segmentDurations = append(p.segmentDurations, dur)
			}
			if i+1 < len(lines) {
				name := strings.TrimSpace(lines[i+1])
				if !strings.HasPrefix(name, "#") && name != "" {
					p.segmentNames = append(p.segmentNames, name)
				}
			}
		}
	}

	return p
}

func TestVODPlaylistFormat(t *testing.T) {
	data := readRefHLS(t, "ref_vod_playlist.m3u8")
	p := parsePlaylist(t, data)

	assert.Equal(t, "#EXTM3U", strings.TrimSpace(p.lines[0]), "must start with #EXTM3U")
	assert.GreaterOrEqual(t, p.version, 3, "HLS version should be >= 3")
	assert.Greater(t, p.targetDuration, 0.0, "must have #EXT-X-TARGETDURATION > 0")
	assert.GreaterOrEqual(t, p.mediaSequence, 0, "must have #EXT-X-MEDIA-SEQUENCE >= 0")
	assert.True(t, p.hasEndList, "VOD playlist must have #EXT-X-ENDLIST")
	assert.GreaterOrEqual(t, len(p.segmentDurations), 1, "must have at least 1 segment")
	assert.Equal(t, len(p.segmentDurations), len(p.segmentNames), "each EXTINF must have a segment URI")

	numRe := regexp.MustCompile(`(\d+)`)
	var nums []int
	for _, name := range p.segmentNames {
		matches := numRe.FindStringSubmatch(name)
		if len(matches) > 1 {
			n, err := strconv.Atoi(matches[1])
			if err == nil {
				nums = append(nums, n)
			}
		}
	}
	for i := 1; i < len(nums); i++ {
		assert.Equal(t, nums[i-1]+1, nums[i], "segments must be sequential: %d followed by %d", nums[i-1], nums[i])
	}

	for _, dur := range p.segmentDurations {
		assert.LessOrEqual(t, dur, p.targetDuration+0.5,
			"segment duration %.3f must not exceed target duration %.0f + 0.5s", dur, p.targetDuration)
	}
}

func TestLivePlaylistFormat(t *testing.T) {
	data := readRefHLS(t, "ref_live_playlist.m3u8")
	p := parsePlaylist(t, data)

	assert.Equal(t, "#EXTM3U", strings.TrimSpace(p.lines[0]))
	assert.False(t, p.hasEndList, "live playlist must NOT have #EXT-X-ENDLIST")
	assert.Greater(t, p.mediaSequence, 0, "live playlist should have #EXT-X-MEDIA-SEQUENCE > 0 for rolling window")
	assert.GreaterOrEqual(t, len(p.segmentDurations), 1, "must have segments")
	assert.LessOrEqual(t, len(p.segmentDurations), 10, "live playlist should have limited window (not all segments)")
}

func TestSegmentDurationAccuracy(t *testing.T) {
	if !ffmpegAvailable() || !ffprobeAvailable() {
		t.Skip("ffmpeg/ffprobe not available")
	}

	dir := t.TempDir()
	generateHLSWithKeyframes(t, dir, "libx264", 10, 2)

	playlistData, err := os.ReadFile(filepath.Join(dir, "playlist.m3u8"))
	require.NoError(t, err)

	p := parsePlaylist(t, playlistData)
	require.GreaterOrEqual(t, len(p.segmentNames), 3, "should have >= 3 segments for 10s at 2s target")

	for i, segFile := range p.segmentNames {
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

		assert.InDelta(t, p.segmentDurations[i], probeDur, 0.1,
			"segment %s: EXTINF duration %.3f vs probed duration %.3f",
			segFile, p.segmentDurations[i], probeDur)
	}
}

func TestHEVCPlaylist(t *testing.T) {
	data := readRefHLS(t, "ref_hevc_playlist.m3u8")
	p := parsePlaylist(t, data)

	assert.Equal(t, "#EXTM3U", strings.TrimSpace(p.lines[0]))
	assert.GreaterOrEqual(t, p.version, 3, "HEVC HLS version should be >= 3")
	assert.Greater(t, p.targetDuration, 0.0)
	assert.True(t, p.hasEndList, "HEVC VOD playlist should have #EXT-X-ENDLIST")
	assert.GreaterOrEqual(t, len(p.segmentDurations), 1)

	for _, dur := range p.segmentDurations {
		assert.LessOrEqual(t, dur, p.targetDuration+0.5,
			"HEVC segment duration %.3f must not exceed target duration", dur)
	}
}

func TestHLSJSCompatibility(t *testing.T) {
	data := readRefHLS(t, "ref_vod_playlist.m3u8")
	text := string(data)
	lines := strings.Split(strings.TrimSpace(text), "\n")

	assert.Equal(t, "#EXTM3U", strings.TrimSpace(lines[0]),
		"hls.js requires #EXTM3U as first line")

	hasTargetDuration := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#EXT-X-TARGETDURATION:") {
			hasTargetDuration = true
			val := strings.TrimPrefix(strings.TrimSpace(line), "#EXT-X-TARGETDURATION:")
			td, err := strconv.ParseFloat(val, 64)
			require.NoError(t, err)
			assert.Greater(t, td, 0.0, "hls.js requires positive TARGETDURATION")
			assert.Equal(t, math.Floor(td), td, "hls.js expects integer TARGETDURATION")
		}
	}
	assert.True(t, hasTargetDuration, "hls.js requires #EXT-X-TARGETDURATION")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#EXTINF:") {
			assert.Contains(t, line, ",", "hls.js requires comma after EXTINF duration (line %d)", i+1)
			if i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				assert.False(t, strings.HasPrefix(next, "#") || next == "",
					"hls.js requires segment URI immediately after #EXTINF (line %d)", i+1)
			}
		}
	}

	if strings.Contains(text, "#EXT-X-MEDIA-SEQUENCE:") {
		re := regexp.MustCompile(`#EXT-X-MEDIA-SEQUENCE:\s*(\d+)`)
		matches := re.FindStringSubmatch(text)
		require.NotEmpty(t, matches, "MEDIA-SEQUENCE must have a numeric value")
		_, err := strconv.Atoi(matches[1])
		assert.NoError(t, err, "hls.js requires numeric MEDIA-SEQUENCE")
	}

	assert.False(t, strings.Contains(text, "\r\n"), "playlist should use LF line endings (not CRLF)")
}

func TestValidatorAcceptsAllReferencePlaylists(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{"VOD H.264", "ref_vod_playlist.m3u8"},
		{"HEVC", "ref_hevc_playlist.m3u8"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := readRefHLS(t, tc.file)
			errs := validate.ValidateHLSPlaylist(data)
			assert.Empty(t, errs, "ValidateHLSPlaylist rejected %s playlist: %v", tc.name, errs)
		})
	}
}

func TestValidatorAcceptsLivePlaylist(t *testing.T) {
	data := readRefHLS(t, "ref_live_playlist.m3u8")
	errs := validate.ValidateHLSPlaylist(data)
	assert.Empty(t, errs, "ValidateHLSPlaylist rejected live playlist: %v", errs)
}

func TestCompareMuxerOutputAgainstReference(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not available")
	}

	refDir := t.TempDir()
	generateHLSWithKeyframes(t, refDir, "libx264", 10, 2)

	refPlaylist, err := os.ReadFile(filepath.Join(refDir, "playlist.m3u8"))
	require.NoError(t, err)

	refParsed := parsePlaylist(t, refPlaylist)

	assert.Equal(t, "#EXTM3U", strings.TrimSpace(refParsed.lines[0]))
	assert.GreaterOrEqual(t, refParsed.version, 3)
	assert.Greater(t, refParsed.targetDuration, 0.0)
	assert.GreaterOrEqual(t, refParsed.mediaSequence, 0)
	assert.True(t, refParsed.hasEndList)
	assert.GreaterOrEqual(t, len(refParsed.segmentDurations), 3)
	assert.Equal(t, len(refParsed.segmentDurations), len(refParsed.segmentNames))

	for _, name := range refParsed.segmentNames {
		assert.True(t, strings.HasSuffix(name, ".ts"), "segment should be .ts file: %s", name)
	}

	errs := validate.ValidateHLSPlaylist(refPlaylist)
	assert.Empty(t, errs, "our validator should accept ffmpeg reference output: %v", errs)
}

func TestTSSegmentSyncBytes(t *testing.T) {
	data := readRefHLS(t, "ref_segment.ts")
	require.Greater(t, len(data), 188, "segment should be larger than one TS packet")

	packetCount := len(data) / 188
	assert.Equal(t, 0, len(data)%188, "TS data length should be multiple of 188")

	for i := 0; i < packetCount; i++ {
		offset := i * 188
		assert.Equal(t, byte(0x47), data[offset],
			"packet %d at offset %d: expected sync byte 0x47, got 0x%02x", i, offset, data[offset])
	}
}

func TestTSSegmentPATPMTPresent(t *testing.T) {
	data := readRefHLS(t, "ref_segment.ts")
	packetCount := len(data) / 188

	hasPAT := false
	pmtPIDs := map[uint16]bool{}
	hasPMT := false

	for i := 0; i < packetCount; i++ {
		offset := i * 188
		pkt := data[offset : offset+188]
		if pkt[0] != 0x47 {
			continue
		}

		pid := uint16(pkt[1]&0x1f)<<8 | uint16(pkt[2])

		if pid == 0x0000 {
			hasPAT = true

			payloadStart := pkt[1]&0x40 != 0
			if payloadStart {
				adaptFieldCtrl := (pkt[3] >> 4) & 0x03
				headerLen := 4
				if adaptFieldCtrl == 0x03 || adaptFieldCtrl == 0x02 {
					if headerLen < 188 {
						adaptLen := int(pkt[4])
						headerLen = 5 + adaptLen
					}
				}
				if headerLen < 188 {
					pointerField := int(pkt[headerLen])
					tableStart := headerLen + 1 + pointerField
					if tableStart+8 < 188 {
						sectionLen := int(pkt[tableStart+1]&0x0f)<<8 | int(pkt[tableStart+2])
						if sectionLen >= 9 {
							progStart := tableStart + 8
							for progStart+3 < tableStart+3+sectionLen-4 {
								pmtPID := uint16(pkt[progStart+2]&0x1f)<<8 | uint16(pkt[progStart+3])
								if pmtPID > 0 {
									pmtPIDs[pmtPID] = true
								}
								progStart += 4
							}
						}
					}
				}
			}
		}

		if pmtPIDs[pid] {
			hasPMT = true
		}
	}

	assert.True(t, hasPAT, "first segment must contain PAT (PID 0x0000)")
	assert.True(t, hasPMT, "first segment must contain PMT")
}

func TestTSSegmentPESStreamsPresent(t *testing.T) {
	data := readRefHLS(t, "ref_segment.ts")
	packetCount := len(data) / 188

	pids := map[uint16]int{}
	for i := 0; i < packetCount; i++ {
		offset := i * 188
		pkt := data[offset : offset+188]
		if pkt[0] != 0x47 {
			continue
		}
		pid := uint16(pkt[1]&0x1f)<<8 | uint16(pkt[2])
		pids[pid]++
	}

	contentPIDs := 0
	for pid, count := range pids {
		if pid != 0x0000 && pid != 0x0001 && pid != 0x1FFF && count > 0 {
			contentPIDs++
		}
	}

	assert.GreaterOrEqual(t, contentPIDs, 2,
		"TS segment should have at least 2 content PIDs (video + audio + PMT), found %d total PIDs: %v",
		contentPIDs, pids)
}

func TestValidateTSSegmentAcceptsReference(t *testing.T) {
	data := readRefHLS(t, "ref_segment.ts")
	errs := validate.ValidateTSSegment(data)
	assert.Empty(t, errs, "ValidateTSSegment rejected reference segment: %v", errs)
}

func TestGeneratedHEVCSegmentValid(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	generateHLSWithKeyframes(t, dir, "libx265", 6, 2)

	seg0, err := os.ReadFile(filepath.Join(dir, "seg0.ts"))
	require.NoError(t, err)

	errs := validate.ValidateTSSegment(seg0)
	assert.Empty(t, errs, "HEVC TS segment should pass validation: %v", errs)

	assert.Equal(t, byte(0x47), seg0[0])
	assert.Equal(t, 0, len(seg0)%188)
}

func TestGeneratedLivePlaylistNoEndList(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	playlistPath := filepath.Join(dir, "playlist.m3u8")
	segPattern := filepath.Join(dir, "seg%d.ts")

	ctx := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=20:size=320x240:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=20:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-g", "50", "-keyint_min", "50",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "hls", "-hls_time", "2", "-hls_list_size", "3", "-hls_flags", "delete_segments",
		"-hls_segment_filename", segPattern,
		playlistPath,
	)
	out, err := ctx.CombinedOutput()
	require.NoError(t, err, "ffmpeg live HLS generation failed: %s", out)

	data, err := os.ReadFile(playlistPath)
	require.NoError(t, err)
	p := parsePlaylist(t, data)

	assert.Equal(t, "#EXTM3U", strings.TrimSpace(p.lines[0]))
	assert.GreaterOrEqual(t, p.mediaSequence, 0)
	assert.LessOrEqual(t, len(p.segmentDurations), 5, "rolling window should limit segment count")

	errs := validate.ValidateHLSPlaylist(data)
	for _, e := range errs {
		assert.NotEqual(t, "header", e.Field, "playlist header should be valid")
		assert.NotEqual(t, "targetduration", e.Field, "target duration should be valid")
	}
}

func TestSegmentDurationVarianceFromTarget(t *testing.T) {
	if !ffmpegAvailable() || !ffprobeAvailable() {
		t.Skip("ffmpeg/ffprobe not available")
	}

	dir := t.TempDir()
	generateHLSWithKeyframes(t, dir, "libx264", 10, 2)

	playlistData, err := os.ReadFile(filepath.Join(dir, "playlist.m3u8"))
	require.NoError(t, err)

	p := parsePlaylist(t, playlistData)
	require.GreaterOrEqual(t, len(p.segmentDurations), 2)

	for _, d := range p.segmentDurations {
		deviation := math.Abs(d-p.targetDuration) / p.targetDuration
		assert.Less(t, deviation, 0.15,
			"segment duration %.3f deviates >15%% from target %.1f", d, p.targetDuration)
	}

	totalDur := 0.0
	for _, d := range p.segmentDurations {
		totalDur += d
	}
	assert.InDelta(t, 10.0, totalDur, 0.5, "total duration should be ~10s, got %.3f", totalDur)
}

func TestMultipleCodecSegmentsAllValid(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not available")
	}

	codecs := []struct {
		name    string
		encoder string
		extra   []string
	}{
		{"h264", "libx264", nil},
		{"hevc", "libx265", []string{"-tag:v", "hvc1", "-x265-params", "keyint=50:min-keyint=50"}},
	}

	for _, codec := range codecs {
		t.Run(codec.name, func(t *testing.T) {
			dir := t.TempDir()
			playlistPath := filepath.Join(dir, "playlist.m3u8")
			segPattern := filepath.Join(dir, "seg%d.ts")

			args := []string{"-y",
				"-f", "lavfi", "-i", "testsrc2=duration=6:size=320x240:rate=25",
				"-f", "lavfi", "-i", "sine=frequency=440:duration=6:sample_rate=48000",
				"-c:v", codec.encoder, "-preset", "ultrafast",
			}
			if codec.encoder == "libx264" {
				args = append(args, "-g", "50", "-keyint_min", "50")
			}
			args = append(args, codec.extra...)
			args = append(args,
				"-c:a", "aac", "-ac", "2", "-ar", "48000",
				"-f", "hls", "-hls_time", "2", "-hls_list_size", "0",
				"-hls_segment_filename", segPattern,
				playlistPath,
			)

			cmd := exec.Command("ffmpeg", args...)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "ffmpeg %s failed: %s", codec.name, out)

			playlistData, err := os.ReadFile(playlistPath)
			require.NoError(t, err)

			errs := validate.ValidateHLSPlaylist(playlistData)
			assert.Empty(t, errs, "%s playlist validation errors: %v", codec.name, errs)

			seg0, err := os.ReadFile(filepath.Join(dir, "seg0.ts"))
			require.NoError(t, err)

			tsErrs := validate.ValidateTSSegment(seg0)
			assert.Empty(t, tsErrs, "%s segment validation errors: %v", codec.name, tsErrs)

			assert.Equal(t, byte(0x47), seg0[0])
			assert.Equal(t, 0, len(seg0)%188)
		})
	}
}

func TestTSSegmentPESHeaders(t *testing.T) {
	data := readRefHLS(t, "ref_segment.ts")
	packetCount := len(data) / 188

	videoPESFound := false
	audioPESFound := false

	for i := 0; i < packetCount; i++ {
		offset := i * 188
		pkt := data[offset : offset+188]
		if pkt[0] != 0x47 {
			continue
		}

		payloadStart := pkt[1]&0x40 != 0
		if !payloadStart {
			continue
		}

		pid := uint16(pkt[1]&0x1f)<<8 | uint16(pkt[2])
		if pid == 0x0000 || pid == 0x0001 || pid == 0x1FFF {
			continue
		}

		adaptFieldCtrl := (pkt[3] >> 4) & 0x03
		payloadOffset := 4
		if adaptFieldCtrl == 0x03 {
			adaptLen := int(pkt[4])
			payloadOffset = 5 + adaptLen
		}

		if payloadOffset+6 >= 188 {
			continue
		}

		if pkt[payloadOffset] == 0x00 && pkt[payloadOffset+1] == 0x00 && pkt[payloadOffset+2] == 0x01 {
			streamID := pkt[payloadOffset+3]
			if streamID >= 0xE0 && streamID <= 0xEF {
				videoPESFound = true
			}
			if streamID >= 0xC0 && streamID <= 0xDF {
				audioPESFound = true
			}
		}
	}

	assert.True(t, videoPESFound, "TS segment should contain video PES packets (stream IDs 0xE0-0xEF)")
	assert.True(t, audioPESFound, "TS segment should contain audio PES packets (stream IDs 0xC0-0xDF)")
}

func generateHLSWithKeyframes(t *testing.T, dir string, encoder string, durationSec int, segDurSec int) {
	t.Helper()
	playlistPath := filepath.Join(dir, "playlist.m3u8")
	segPattern := filepath.Join(dir, "seg%d.ts")
	durStr := strconv.Itoa(durationSec)
	segDurStr := strconv.Itoa(segDurSec)

	args := []string{"-y",
		"-f", "lavfi", "-i", "testsrc2=duration=" + durStr + ":size=320x240:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=" + durStr + ":sample_rate=48000",
		"-c:v", encoder, "-preset", "ultrafast",
	}

	if encoder == "libx264" {
		args = append(args, "-g", "50", "-keyint_min", "50")
	} else if encoder == "libx265" {
		args = append(args, "-tag:v", "hvc1", "-x265-params", "keyint=50:min-keyint=50")
	}

	args = append(args,
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "hls", "-hls_time", segDurStr, "-hls_list_size", "0",
		"-hls_segment_filename", segPattern,
		playlistPath,
	)

	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "ffmpeg %s HLS generation failed: %s", encoder, out)
}
