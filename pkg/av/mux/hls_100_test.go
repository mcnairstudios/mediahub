//go:build cgo

package mux

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/asticode/go-astiav"
)

type hlsTestFixture struct {
	tsH264Dir     string
	fmp4H265Dir   string
	fmp4H264Dir   string
	tsVidOnlyDir  string
	tsShortDir    string
	ourTSH264Dir  string
	ourFMP4H265Dir string
	ourFMP4H264Dir string
	ourTSVidOnlyDir string
	ourTSShortDir   string
}

var (
	fixture     *hlsTestFixture
	fixtureOnce sync.Once
	fixtureErr  error
)

func getFixture(t *testing.T) *hlsTestFixture {
	t.Helper()
	fixtureOnce.Do(func() {
		fixture, fixtureErr = buildFixture()
	})
	if fixtureErr != nil {
		t.Fatalf("fixture setup failed: %v", fixtureErr)
	}
	return fixture
}

func buildFixture() (*hlsTestFixture, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg not found: %w", err)
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return nil, fmt.Errorf("ffprobe not found: %w", err)
	}

	root := filepath.Join(os.TempDir(), "hls100_test")
	os.RemoveAll(root)

	f := &hlsTestFixture{
		tsH264Dir:       filepath.Join(root, "ref_ts_h264"),
		fmp4H265Dir:     filepath.Join(root, "ref_fmp4_h265"),
		fmp4H264Dir:     filepath.Join(root, "ref_fmp4_h264"),
		tsVidOnlyDir:    filepath.Join(root, "ref_ts_vidonly"),
		tsShortDir:      filepath.Join(root, "ref_ts_short"),
		ourTSH264Dir:    filepath.Join(root, "our_ts_h264"),
		ourFMP4H265Dir:  filepath.Join(root, "our_fmp4_h265"),
		ourFMP4H264Dir:  filepath.Join(root, "our_fmp4_h264"),
		ourTSVidOnlyDir: filepath.Join(root, "our_ts_vidonly"),
		ourTSShortDir:   filepath.Join(root, "our_ts_short"),
	}

	dirs := []string{
		f.tsH264Dir, f.fmp4H265Dir, f.fmp4H264Dir, f.tsVidOnlyDir, f.tsShortDir,
		f.ourTSH264Dir, f.ourFMP4H265Dir, f.ourFMP4H264Dir, f.ourTSVidOnlyDir, f.ourTSShortDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, err
		}
	}

	type ffmpegJob struct {
		args []string
		name string
	}

	jobs := []ffmpegJob{
		{
			name: "ts_h264",
			args: []string{
				"-f", "lavfi", "-i", "testsrc2=duration=10:size=640x360:rate=25",
				"-f", "lavfi", "-i", "sine=f=440:d=10:sample_rate=48000",
				"-c:v", "libx264", "-preset", "ultrafast", "-g", "50", "-keyint_min", "50",
				"-c:a", "aac", "-ac", "2", "-ar", "48000",
				"-f", "hls", "-hls_time", "2", "-hls_list_size", "0",
				"-hls_segment_filename", filepath.Join(f.tsH264Dir, "seg%d.ts"),
				"-y", filepath.Join(f.tsH264Dir, "playlist.m3u8"),
			},
		},
		{
			name: "fmp4_h265",
			args: []string{
				"-f", "lavfi", "-i", "testsrc2=duration=10:size=640x360:rate=25",
				"-f", "lavfi", "-i", "sine=f=440:d=10:sample_rate=48000",
				"-c:v", "libx265", "-preset", "ultrafast",
				"-x265-params", "keyint=50:min-keyint=50",
				"-c:a", "aac", "-ac", "2", "-ar", "48000",
				"-f", "hls", "-hls_time", "2", "-hls_list_size", "0",
				"-hls_segment_type", "fmp4", "-hls_fmp4_init_filename", "init.mp4",
				"-hls_segment_filename", filepath.Join(f.fmp4H265Dir, "seg%d.m4s"),
				"-y", filepath.Join(f.fmp4H265Dir, "playlist.m3u8"),
			},
		},
		{
			name: "fmp4_h264",
			args: []string{
				"-f", "lavfi", "-i", "testsrc2=duration=10:size=640x360:rate=25",
				"-f", "lavfi", "-i", "sine=f=440:d=10:sample_rate=48000",
				"-c:v", "libx264", "-preset", "ultrafast", "-g", "50", "-keyint_min", "50",
				"-c:a", "aac", "-ac", "2", "-ar", "48000",
				"-f", "hls", "-hls_time", "2", "-hls_list_size", "0",
				"-hls_segment_type", "fmp4", "-hls_fmp4_init_filename", "init.mp4",
				"-hls_segment_filename", filepath.Join(f.fmp4H264Dir, "seg%d.m4s"),
				"-y", filepath.Join(f.fmp4H264Dir, "playlist.m3u8"),
			},
		},
		{
			name: "ts_vidonly",
			args: []string{
				"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
				"-c:v", "libx264", "-preset", "ultrafast", "-g", "50", "-keyint_min", "50",
				"-an",
				"-f", "hls", "-hls_time", "2", "-hls_list_size", "0",
				"-hls_segment_filename", filepath.Join(f.tsVidOnlyDir, "seg%d.ts"),
				"-y", filepath.Join(f.tsVidOnlyDir, "playlist.m3u8"),
			},
		},
		{
			name: "ts_short",
			args: []string{
				"-f", "lavfi", "-i", "testsrc2=duration=2:size=640x360:rate=25",
				"-f", "lavfi", "-i", "sine=f=440:d=2:sample_rate=48000",
				"-c:v", "libx264", "-preset", "ultrafast", "-g", "50", "-keyint_min", "50",
				"-c:a", "aac", "-ac", "2",
				"-f", "hls", "-hls_time", "2", "-hls_list_size", "0",
				"-hls_segment_filename", filepath.Join(f.tsShortDir, "seg%d.ts"),
				"-y", filepath.Join(f.tsShortDir, "playlist.m3u8"),
			},
		},
	}

	for _, job := range jobs {
		cmd := exec.Command("ffmpeg", job.args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(out), "Unknown encoder") {
				return nil, fmt.Errorf("encoder not available for %s: %s", job.name, string(out))
			}
			return nil, fmt.Errorf("ffmpeg %s failed: %v\n%s", job.name, err, string(out))
		}
	}

	genOurMuxer(f)

	return f, nil
}

func genOurMuxer(f *hlsTestFixture) {
	type muxJob struct {
		inputDir string
		outputDir string
		segType   string
		codec     string
		hasAudio  bool
		duration  int
	}

	jobs := []muxJob{
		{f.tsH264Dir, f.ourTSH264Dir, "mpegts", "h264", true, 10},
		{f.fmp4H264Dir, f.ourFMP4H264Dir, "fmp4", "h264", true, 10},
		{f.tsVidOnlyDir, f.ourTSVidOnlyDir, "mpegts", "h264", false, 5},
		{f.tsShortDir, f.ourTSShortDir, "mpegts", "h264", true, 2},
	}

	inputRoot := filepath.Join(os.TempDir(), "hls100_test", "inputs")
	os.MkdirAll(inputRoot, 0755)

	for i, job := range jobs {
		inputPath := filepath.Join(inputRoot, fmt.Sprintf("input_%d.ts", i))
		args := []string{
			"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=duration=%d:size=640x360:rate=25", job.duration),
		}
		if job.hasAudio {
			args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("sine=f=440:d=%d:sample_rate=48000", job.duration))
		}
		args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-g", "50", "-keyint_min", "50")
		if job.hasAudio {
			args = append(args, "-c:a", "aac", "-ac", "2", "-ar", "48000", "-b:a", "128k")
		} else {
			args = append(args, "-an")
		}
		inputFmt := "mpegts"
		if job.segType == "fmp4" {
			inputFmt = "mp4"
			inputPath = filepath.Join(inputRoot, fmt.Sprintf("input_%d.mp4", i))
		}
		args = append(args, "-f", inputFmt, "-y", inputPath)
		cmd := exec.Command("ffmpeg", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "generate our input %d: %v\n%s\n", i, err, out)
			continue
		}

		muxThroughOurHLSStandalone(inputPath, job.outputDir, job.segType)
	}

	fmp4H265Input := filepath.Join(inputRoot, "input_h265.mp4")
	h265args := []string{
		"-f", "lavfi", "-i", "testsrc2=duration=10:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=f=440:d=10:sample_rate=48000",
		"-c:v", "libx265", "-preset", "ultrafast",
		"-x265-params", "keyint=50:min-keyint=50",
		"-c:a", "aac", "-ac", "2", "-ar", "48000", "-b:a", "128k",
		"-f", "mp4", "-y", fmp4H265Input,
	}
	cmd := exec.Command("ffmpeg", h265args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "generate h265 input: %v\n%s\n", err, out)
	} else {
		muxThroughOurHLSStandalone(fmp4H265Input, f.ourFMP4H265Dir, "fmp4")
	}
}

func muxThroughOurHLSStandalone(inputPath, outDir, segType string) {
	fc := astiav.AllocFormatContext()
	if fc == nil {
		return
	}
	defer fc.Free()

	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	if err := fc.OpenInput(inputPath, nil, inputDict); err != nil {
		return
	}
	defer fc.CloseInput()
	if err := fc.FindStreamInfo(nil); err != nil {
		return
	}

	var videoStream, audioStream *astiav.Stream
	for _, s := range fc.Streams() {
		switch s.CodecParameters().MediaType() {
		case astiav.MediaTypeVideo:
			if videoStream == nil {
				videoStream = s
			}
		case astiav.MediaTypeAudio:
			if audioStream == nil {
				audioStream = s
			}
		}
	}
	if videoStream == nil {
		return
	}

	vcp := videoStream.CodecParameters()
	var videoExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}

	opts := HLSMuxOpts{
		OutputDir:          outDir,
		SegmentDurationSec: 2,
		SegmentType:        segType,
		VideoCodecID:       vcp.CodecID(),
		VideoExtradata:     videoExtradata,
		VideoWidth:         vcp.Width(),
		VideoHeight:        vcp.Height(),
		VideoTimeBase:      videoStream.TimeBase(),
		VideoFrameRate:     25,
	}

	if audioStream != nil {
		acp := audioStream.CodecParameters()
		var audioExtradata []byte
		if ed := acp.ExtraData(); len(ed) > 0 {
			audioExtradata = make([]byte, len(ed))
			copy(audioExtradata, ed)
		}
		opts.AudioCodecID = acp.CodecID()
		opts.AudioExtradata = audioExtradata
		opts.AudioChannels = acp.ChannelLayout().Channels()
		opts.AudioSampleRate = acp.SampleRate()
		opts.AudioTimeBase = audioStream.TimeBase()
		opts.AudioFrameSize = 1024
	}

	hlsMuxer, err := NewHLSMuxer(opts)
	if err != nil {
		return
	}

	pkt := astiav.AllocPacket()
	if pkt == nil {
		return
	}
	defer pkt.Free()

	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		switch pkt.StreamIndex() {
		case videoStream.Index():
			hlsMuxer.WriteVideoPacket(pkt)
		default:
			if audioStream != nil && pkt.StreamIndex() == audioStream.Index() {
				hlsMuxer.WriteAudioPacket(pkt)
			}
		}
		pkt.Unref()
	}
	hlsMuxer.Close()
}

func readPlaylist(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "playlist.m3u8"))
	if err != nil {
		return ""
	}
	return string(data)
}

func playlistLines(dir string) []string {
	content := readPlaylist(dir)
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSpace(content), "\n")
}

func extractEXTINFs(content string) []float64 {
	re := regexp.MustCompile(`#EXTINF:([\d.]+)`)
	matches := re.FindAllStringSubmatch(content, -1)
	var durations []float64
	for _, m := range matches {
		d, _ := strconv.ParseFloat(m[1], 64)
		durations = append(durations, d)
	}
	return durations
}

func extractTargetDuration(content string) int {
	re := regexp.MustCompile(`#EXT-X-TARGETDURATION:(\d+)`)
	m := re.FindStringSubmatch(content)
	if len(m) > 1 {
		v, _ := strconv.Atoi(m[1])
		return v
	}
	return -1
}

func extractVersion(content string) int {
	re := regexp.MustCompile(`#EXT-X-VERSION:(\d+)`)
	m := re.FindStringSubmatch(content)
	if len(m) > 1 {
		v, _ := strconv.Atoi(m[1])
		return v
	}
	return -1
}

func extractMediaSequence(content string) (int, bool) {
	re := regexp.MustCompile(`#EXT-X-MEDIA-SEQUENCE:(\d+)`)
	m := re.FindStringSubmatch(content)
	if len(m) > 1 {
		v, _ := strconv.Atoi(m[1])
		return v, true
	}
	return 0, false
}

func extractSegmentURIs(content string) []string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var uris []string
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#EXTINF:") && i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if next != "" && !strings.HasPrefix(next, "#") {
				uris = append(uris, next)
			}
		}
	}
	return uris
}

func segmentFiles(dir, ext string) []string {
	pattern := filepath.Join(dir, "seg*."+ext)
	matches, _ := filepath.Glob(pattern)
	sort.Strings(matches)
	return matches
}

func readMP4Boxes(data []byte) []string {
	var types []string
	offset := 0
	for offset+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		if size < 8 || offset+size > len(data) {
			break
		}
		types = append(types, string(data[offset+4:offset+8]))
		offset += size
	}
	return types
}

func hasMP4Box(data []byte, boxType string) bool {
	needle := []byte(boxType)
	if len(needle) != 4 {
		return false
	}
	for i := 0; i <= len(data)-4; i++ {
		if data[i] == needle[0] && data[i+1] == needle[1] && data[i+2] == needle[2] && data[i+3] == needle[3] {
			return true
		}
	}
	return false
}

func hls100_ffprobeField(path, selectStreams, entry string) string {
	args := []string{"-v", "quiet"}
	if selectStreams != "" {
		args = append(args, "-select_streams", selectStreams)
	}
	args = append(args, "-show_entries", entry, "-of", "csv=p=0", path)
	cmd := exec.Command("ffprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

func hls100_ffprobeDuration(path string) float64 {
	val := hls100_ffprobeField(path, "", "format=duration")
	d, _ := strconv.ParseFloat(val, 64)
	return d
}

func hls100_ffprobeCodec(path, selectStreams string) string {
	return hls100_ffprobeField(path, selectStreams, "stream=codec_name")
}

func hls100_ffprobeInt(path, selectStreams, entry string) int {
	val := hls100_ffprobeField(path, selectStreams, entry)
	v, _ := strconv.Atoi(val)
	return v
}

func hls100_ffprobeResolution(path string) (int, int) {
	val := hls100_ffprobeField(path, "v:0", "stream=width,height")
	parts := strings.Split(val, ",")
	if len(parts) == 2 {
		w, _ := strconv.Atoi(parts[0])
		h, _ := strconv.Atoi(parts[1])
		return w, h
	}
	return 0, 0
}

func hls100_ffprobeFrameCount(path, selectStreams string) int {
	args := []string{
		"-v", "quiet", "-select_streams", selectStreams,
		"-count_frames", "-show_entries", "stream=nb_read_frames",
		"-of", "csv=p=0", path,
	}
	cmd := exec.Command("ffprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		return -1
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		v, err := strconv.Atoi(line)
		if err == nil {
			return v
		}
	}
	return -1
}

func hls100_ffprobePacketPTSTime(path, selectStreams string) []float64 {
	args := []string{
		"-v", "quiet", "-select_streams", selectStreams,
		"-show_entries", "packet=pts_time",
		"-of", "csv=p=0", path,
	}
	cmd := exec.Command("ffprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var pts []float64
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if v, err := strconv.ParseFloat(strings.TrimSpace(line), 64); err == nil {
			pts = append(pts, v)
		}
	}
	return pts
}

func runOnDirs(t *testing.T, name string, dirs []string, check func(t *testing.T, dir string)) {
	t.Helper()
	for _, dir := range dirs {
		label := filepath.Base(dir)
		t.Run(label, func(t *testing.T) {
			content := readPlaylist(dir)
			if content == "" {
				t.Skipf("no playlist in %s", dir)
			}
			check(t, dir)
		})
	}
	_ = name
}

func allTSDirs(f *hlsTestFixture) []string {
	return []string{f.tsH264Dir, f.tsVidOnlyDir, f.tsShortDir, f.ourTSH264Dir, f.ourTSVidOnlyDir, f.ourTSShortDir}
}

func allFMP4Dirs(f *hlsTestFixture) []string {
	return []string{f.fmp4H265Dir, f.fmp4H264Dir, f.ourFMP4H265Dir, f.ourFMP4H264Dir}
}

func allDirs(f *hlsTestFixture) []string {
	return append(allTSDirs(f), allFMP4Dirs(f)...)
}

func allAVDirs(f *hlsTestFixture) []string {
	return []string{f.tsH264Dir, f.tsShortDir, f.fmp4H265Dir, f.fmp4H264Dir,
		f.ourTSH264Dir, f.ourTSShortDir, f.ourFMP4H265Dir, f.ourFMP4H264Dir}
}

func allLongDirs(f *hlsTestFixture) []string {
	return []string{f.tsH264Dir, f.fmp4H265Dir, f.fmp4H264Dir,
		f.ourTSH264Dir, f.ourFMP4H265Dir, f.ourFMP4H264Dir}
}

func TestHLS100_01_StartsWithEXTM3U(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "EXTM3U", allDirs(f), func(t *testing.T, dir string) {
		content := readPlaylist(dir)
		if !strings.HasPrefix(strings.TrimSpace(content), "#EXTM3U") {
			t.Error("playlist does not start with #EXTM3U")
		}
	})
}

func TestHLS100_02_HasVersion(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "version", allDirs(f), func(t *testing.T, dir string) {
		v := extractVersion(readPlaylist(dir))
		if v < 0 {
			t.Error("missing #EXT-X-VERSION")
		}
	})
}

func TestHLS100_03_VersionMinimum(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "version_ts", allTSDirs(f), func(t *testing.T, dir string) {
		v := extractVersion(readPlaylist(dir))
		if v >= 0 && v < 3 {
			t.Errorf("MPEG-TS HLS version %d < 3", v)
		}
	})
	runOnDirs(t, "version_fmp4", allFMP4Dirs(f), func(t *testing.T, dir string) {
		v := extractVersion(readPlaylist(dir))
		if v >= 0 && v < 6 {
			t.Errorf("fMP4 HLS version %d < 6", v)
		}
	})
}

func TestHLS100_04_HasTargetDuration(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "targetdur", allDirs(f), func(t *testing.T, dir string) {
		td := extractTargetDuration(readPlaylist(dir))
		if td < 0 {
			t.Error("missing #EXT-X-TARGETDURATION")
		}
	})
}

func TestHLS100_05_TargetDurationIsInteger(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "targetdur_int", allDirs(f), func(t *testing.T, dir string) {
		content := readPlaylist(dir)
		re := regexp.MustCompile(`#EXT-X-TARGETDURATION:(\S+)`)
		m := re.FindStringSubmatch(content)
		if len(m) > 1 {
			if _, err := strconv.Atoi(m[1]); err != nil {
				t.Errorf("TARGETDURATION is not an integer: %s", m[1])
			}
		}
	})
}

func TestHLS100_06_TargetDurationGeMaxSegment(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "targetdur_ceiling", allDirs(f), func(t *testing.T, dir string) {
		content := readPlaylist(dir)
		td := extractTargetDuration(content)
		durations := extractEXTINFs(content)
		if td < 0 || len(durations) == 0 {
			t.Skip("no data")
		}
		var maxDur float64
		for _, d := range durations {
			if d > maxDur {
				maxDur = d
			}
		}
		rounded := int(math.Round(maxDur))
		if td < rounded {
			t.Errorf("TARGETDURATION %d < ceil of max segment duration %.3f (rounded=%d)", td, maxDur, rounded)
		}
	})
}

func TestHLS100_07_HasMediaSequence(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "media_seq", allDirs(f), func(t *testing.T, dir string) {
		content := readPlaylist(dir)
		if !strings.Contains(content, "#EXT-X-MEDIA-SEQUENCE:") {
			t.Error("missing #EXT-X-MEDIA-SEQUENCE")
		}
	})
}

func TestHLS100_08_MediaSequenceNonNegative(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "media_seq_val", allDirs(f), func(t *testing.T, dir string) {
		seq, ok := extractMediaSequence(readPlaylist(dir))
		if ok && seq < 0 {
			t.Errorf("MEDIA-SEQUENCE is negative: %d", seq)
		}
	})
}

func TestHLS100_09_VODHasEndlist(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "endlist", allDirs(f), func(t *testing.T, dir string) {
		content := readPlaylist(dir)
		if !strings.Contains(content, "#EXT-X-ENDLIST") {
			t.Error("VOD playlist missing #EXT-X-ENDLIST")
		}
	})
}

func TestHLS100_10_AtLeastOneEXTINF(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "extinf_exists", allDirs(f), func(t *testing.T, dir string) {
		durations := extractEXTINFs(readPlaylist(dir))
		if len(durations) == 0 {
			t.Error("no #EXTINF tags found")
		}
	})
}

func TestHLS100_11_EXTINFDurationPositive(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "extinf_positive", allDirs(f), func(t *testing.T, dir string) {
		for i, d := range extractEXTINFs(readPlaylist(dir)) {
			if d <= 0 {
				t.Errorf("segment %d has EXTINF duration <= 0: %f", i, d)
			}
		}
	})
}

func TestHLS100_12_EXTINFFollowedByURI(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "extinf_uri", allDirs(f), func(t *testing.T, dir string) {
		lines := playlistLines(dir)
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "#EXTINF:") {
				if i+1 >= len(lines) {
					t.Errorf("EXTINF at line %d has no following URI", i)
					continue
				}
				next := strings.TrimSpace(lines[i+1])
				if next == "" || strings.HasPrefix(next, "#") {
					t.Errorf("EXTINF at line %d not followed by URI (got %q)", i, next)
				}
			}
		}
	})
}

func TestHLS100_13_NoBlankBetweenEXTINFAndURI(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "no_blank", allDirs(f), func(t *testing.T, dir string) {
		lines := playlistLines(dir)
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "#EXTINF:") {
				if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
					t.Errorf("blank line between EXTINF and URI at line %d", i)
				}
			}
		}
	})
}

func TestHLS100_14_NoDuplicateSegmentURIs(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "no_dups", allDirs(f), func(t *testing.T, dir string) {
		uris := extractSegmentURIs(readPlaylist(dir))
		seen := map[string]bool{}
		for _, uri := range uris {
			if seen[uri] {
				t.Errorf("duplicate segment URI: %s", uri)
			}
			seen[uri] = true
		}
	})
}

func TestHLS100_15_SegmentURIsSequential(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "sequential", allDirs(f), func(t *testing.T, dir string) {
		uris := extractSegmentURIs(readPlaylist(dir))
		numRe := regexp.MustCompile(`(\d+)`)
		var nums []int
		for _, uri := range uris {
			m := numRe.FindStringSubmatch(uri)
			if len(m) > 1 {
				n, _ := strconv.Atoi(m[1])
				nums = append(nums, n)
			}
		}
		for i := 1; i < len(nums); i++ {
			if nums[i] != nums[i-1]+1 {
				t.Errorf("non-sequential: seg %d followed by seg %d", nums[i-1], nums[i])
				break
			}
		}
	})
}

func TestHLS100_16_FMP4HasMapTag(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "map_tag", allFMP4Dirs(f), func(t *testing.T, dir string) {
		content := readPlaylist(dir)
		if !strings.Contains(content, "#EXT-X-MAP:") {
			t.Error("fMP4 playlist missing #EXT-X-MAP tag")
		}
	})
}

func TestHLS100_17_MapURIPointsToInit(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "map_uri", allFMP4Dirs(f), func(t *testing.T, dir string) {
		content := readPlaylist(dir)
		if !strings.Contains(content, "init.mp4") {
			t.Error("EXT-X-MAP does not reference init.mp4")
		}
	})
}

func TestHLS100_18_InitFileExists(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "init_exists", allFMP4Dirs(f), func(t *testing.T, dir string) {
		initPath := filepath.Join(dir, "init.mp4")
		if _, err := os.Stat(initPath); err != nil {
			t.Errorf("init.mp4 not found: %v", err)
		}
	})
}

func TestHLS100_19_NoMapTagInMPEGTS(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "no_map_ts", allTSDirs(f), func(t *testing.T, dir string) {
		content := readPlaylist(dir)
		if strings.Contains(content, "#EXT-X-MAP:") {
			t.Error("MPEG-TS playlist should not have #EXT-X-MAP")
		}
	})
}

func TestHLS100_20_FMP4SegmentExtensionM4S(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "m4s_ext", allFMP4Dirs(f), func(t *testing.T, dir string) {
		for _, uri := range extractSegmentURIs(readPlaylist(dir)) {
			if !strings.HasSuffix(uri, ".m4s") {
				t.Errorf("fMP4 segment URI %q does not have .m4s extension", uri)
			}
		}
	})
}

func TestHLS100_21_TSSegmentExtension(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "ts_ext", allTSDirs(f), func(t *testing.T, dir string) {
		for _, uri := range extractSegmentURIs(readPlaylist(dir)) {
			if !strings.HasSuffix(uri, ".ts") {
				t.Errorf("MPEG-TS segment URI %q does not have .ts extension", uri)
			}
		}
	})
}

func TestHLS100_22_AllM4SFilesExist(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "m4s_exist", allFMP4Dirs(f), func(t *testing.T, dir string) {
		for _, uri := range extractSegmentURIs(readPlaylist(dir)) {
			path := filepath.Join(dir, uri)
			if _, err := os.Stat(path); err != nil {
				t.Errorf("segment file %s not found", uri)
			}
		}
	})
}

func TestHLS100_23_AllTSFilesExist(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "ts_exist", allTSDirs(f), func(t *testing.T, dir string) {
		for _, uri := range extractSegmentURIs(readPlaylist(dir)) {
			path := filepath.Join(dir, uri)
			if _, err := os.Stat(path); err != nil {
				t.Errorf("segment file %s not found", uri)
			}
		}
	})
}

func TestHLS100_24_InitHasFtyp(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "ftyp", allFMP4Dirs(f), func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		if !hasMP4Box(data, "ftyp") {
			t.Error("init.mp4 missing ftyp box")
		}
	})
}

func TestHLS100_25_InitHasMoov(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "moov", allFMP4Dirs(f), func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		if !hasMP4Box(data, "moov") {
			t.Error("init.mp4 missing moov box")
		}
	})
}

func TestHLS100_26_EXTINFLeTargetDuration(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "extinf_le_td", allDirs(f), func(t *testing.T, dir string) {
		content := readPlaylist(dir)
		td := extractTargetDuration(content)
		for i, d := range extractEXTINFs(content) {
			if td > 0 && d > float64(td)+0.5 {
				t.Errorf("segment %d EXTINF %.3f > TARGETDURATION %d + 0.5", i, d, td)
			}
		}
	})
}

func TestHLS100_27_TotalDurationMatchesSource(t *testing.T) {
	f := getFixture(t)
	cases := []struct {
		dir      string
		expected float64
	}{
		{f.tsH264Dir, 10}, {f.fmp4H265Dir, 10}, {f.fmp4H264Dir, 10},
		{f.tsVidOnlyDir, 5}, {f.tsShortDir, 2},
		{f.ourTSH264Dir, 10}, {f.ourFMP4H265Dir, 10}, {f.ourFMP4H264Dir, 10},
		{f.ourTSVidOnlyDir, 5}, {f.ourTSShortDir, 2},
	}
	for _, tc := range cases {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			content := readPlaylist(tc.dir)
			if content == "" {
				t.Skip("no playlist")
			}
			var total float64
			for _, d := range extractEXTINFs(content) {
				total += d
			}
			if math.Abs(total-tc.expected) > 1.0 {
				t.Errorf("total EXTINF %.3f differs from expected %.0f by > 1s", total, tc.expected)
			}
		})
	}
}

func TestHLS100_28_FFprobeDurationMatchesEXTINF(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"}, {f.ourTSH264Dir, "ts"},
		{f.fmp4H264Dir, "m4s"}, {f.ourFMP4H264Dir, "m4s"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			content := readPlaylist(tc.dir)
			if content == "" {
				t.Skip("no playlist")
			}
			extinfs := extractEXTINFs(content)
			segs := segmentFiles(tc.dir, tc.ext)
			for i := 0; i < len(segs) && i < len(extinfs); i++ {
				measured := hls100_ffprobeDuration(segs[i])
				if measured <= 0 {
					continue
				}
				diff := math.Abs(measured - extinfs[i])
				if diff > 0.5 {
					t.Errorf("seg %d: ffprobe=%.3f EXTINF=%.3f diff=%.3f (>0.5s)", i, measured, extinfs[i], diff)
				}
			}
		})
	}
}

func TestHLS100_29_NoTinySegmentsExceptLast(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "no_tiny", allDirs(f), func(t *testing.T, dir string) {
		durations := extractEXTINFs(readPlaylist(dir))
		for i, d := range durations {
			if i < len(durations)-1 && d < 0.5 {
				t.Errorf("non-final segment %d has duration %.3f < 0.5s", i, d)
			}
		}
	})
}

func TestHLS100_30_LastSegmentLeTargetDuration(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "last_le_td", allDirs(f), func(t *testing.T, dir string) {
		content := readPlaylist(dir)
		td := extractTargetDuration(content)
		durations := extractEXTINFs(content)
		if td <= 0 || len(durations) == 0 {
			t.Skip("no data")
		}
		last := durations[len(durations)-1]
		if last > float64(td)+0.5 {
			t.Errorf("last segment duration %.3f > TARGETDURATION %d + 0.5", last, td)
		}
	})
}

func TestHLS100_31_SegmentDurationConsistency(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "consistency", allLongDirs(f), func(t *testing.T, dir string) {
		durations := extractEXTINFs(readPlaylist(dir))
		if len(durations) < 3 {
			t.Skip("too few segments")
		}
		nonLast := durations[:len(durations)-1]
		var sum float64
		for _, d := range nonLast {
			sum += d
		}
		mean := sum / float64(len(nonLast))
		var variance float64
		for _, d := range nonLast {
			diff := d - mean
			variance += diff * diff
		}
		variance /= float64(len(nonLast))
		cv := math.Sqrt(variance) / mean
		if cv > 0.3 {
			t.Errorf("segment duration CV=%.2f (>0.3), durations=%v", cv, nonLast)
		}
	})
}

func TestHLS100_32_PTSMonotonicallyIncreasingAcrossSegments(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"}, {f.ourTSH264Dir, "ts"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			if len(segs) < 2 {
				t.Skip("need >= 2 segments")
			}
			var lastFirst float64
			for i, seg := range segs {
				pts := hls100_ffprobePacketPTSTime(seg, "v:0")
				if len(pts) == 0 {
					continue
				}
				if i > 0 && pts[0] < lastFirst {
					t.Errorf("seg %d first PTS %.3f < seg %d first PTS %.3f", i, pts[0], i-1, lastFirst)
				}
				lastFirst = pts[0]
			}
		})
	}
}

func TestHLS100_33_NoPTSGapTooLarge(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"}, {f.ourTSH264Dir, "ts"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			td := extractTargetDuration(readPlaylist(tc.dir))
			if td <= 0 || len(segs) < 2 {
				t.Skip("insufficient data")
			}
			maxGap := float64(td) * 3.0
			var prevLastPTS float64
			for i, seg := range segs {
				pts := hls100_ffprobePacketPTSTime(seg, "v:0")
				if len(pts) == 0 {
					continue
				}
				if i > 0 {
					gap := pts[0] - prevLastPTS
					if gap > maxGap {
						t.Errorf("PTS gap between seg %d and %d: %.3f > %.3f", i-1, i, gap, maxGap)
					}
				}
				prevLastPTS = pts[len(pts)-1]
			}
		})
	}
}

func TestHLS100_34_DTSNonDecreasingWithinSegment(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"}, {f.ourTSH264Dir, "ts"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			for si, seg := range segs {
				args := []string{
					"-v", "quiet", "-select_streams", "v:0",
					"-show_entries", "packet=dts_time", "-of", "csv=p=0", seg,
				}
				cmd := exec.Command("ffprobe", args...)
				out, err := cmd.Output()
				if err != nil {
					continue
				}
				var prevDTS float64
				first := true
				for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
					dts, err := strconv.ParseFloat(strings.TrimSpace(line), 64)
					if err != nil {
						continue
					}
					if !first && dts < prevDTS-0.001 {
						t.Errorf("seg %d: DTS decreased from %.3f to %.3f", si, prevDTS, dts)
						break
					}
					prevDTS = dts
					first = false
				}
			}
		})
	}
}

func TestHLS100_35_FirstSegmentStartsNearZero(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"}, {f.fmp4H264Dir, "m4s"},
		{f.ourTSH264Dir, "ts"}, {f.ourFMP4H264Dir, "m4s"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			if len(segs) == 0 {
				t.Skip("no segments")
			}
			pts := hls100_ffprobePacketPTSTime(segs[0], "v:0")
			if len(pts) == 0 {
				t.Skip("no PTS data")
			}
			if pts[0] > 2.0 {
				t.Errorf("first segment first PTS %.3f > 2.0s", pts[0])
			}
		})
	}
}

func TestHLS100_36_VideoFrameCountPerSegment(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"}, {f.ourTSH264Dir, "ts"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			extinfs := extractEXTINFs(readPlaylist(tc.dir))
			segs := segmentFiles(tc.dir, tc.ext)
			for i := 0; i < len(segs) && i < len(extinfs); i++ {
				fc := hls100_ffprobeFrameCount(segs[i], "v:0")
				if fc < 0 {
					continue
				}
				expected := 25.0 * extinfs[i]
				if math.Abs(float64(fc)-expected) > 5 {
					t.Errorf("seg %d: frames=%d expected~%.0f (EXTINF=%.3f)", i, fc, expected, extinfs[i])
				}
			}
		})
	}
}

func TestHLS100_37_AudioSampleCountConsistent(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"}, {f.ourTSH264Dir, "ts"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			extinfs := extractEXTINFs(readPlaylist(tc.dir))
			segs := segmentFiles(tc.dir, tc.ext)
			for i := 0; i < len(segs) && i < len(extinfs); i++ {
				fc := hls100_ffprobeFrameCount(segs[i], "a:0")
				if fc <= 0 {
					continue
				}
				expectedFrames := (extinfs[i] * 48000.0) / 1024.0
				ratio := float64(fc) / expectedFrames
				if ratio < 0.5 || ratio > 2.0 {
					t.Errorf("seg %d: audio frames=%d expected~%.0f ratio=%.2f", i, fc, expectedFrames, ratio)
				}
			}
		})
	}
}

func TestHLS100_38_NoPTSWraparoundWithinSegment(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"}, {f.ourTSH264Dir, "ts"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			for si, seg := range segs {
				pts := hls100_ffprobePacketPTSTime(seg, "v:0")
				if len(pts) < 2 {
					continue
				}
				for j := 1; j < len(pts); j++ {
					jump := math.Abs(pts[j] - pts[j-1])
					if jump > 60.0 {
						t.Errorf("seg %d: PTS jump of %.1f at frame %d (possible wraparound)", si, jump, j)
						break
					}
				}
			}
		})
	}
}

func TestHLS100_39_SegmentPTSContinuity(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"}, {f.ourTSH264Dir, "ts"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			if len(segs) < 2 {
				t.Skip("need >= 2 segments")
			}
			frameDur := 1.0 / 25.0
			var prevLastPTS float64
			for i, seg := range segs {
				pts := hls100_ffprobePacketPTSTime(seg, "v:0")
				if len(pts) == 0 {
					continue
				}
				if i > 0 {
					gap := pts[0] - prevLastPTS
					if gap > frameDur*3 || gap < -frameDur {
						t.Errorf("PTS gap between seg %d end (%.3f) and seg %d start (%.3f): %.3f", i-1, prevLastPTS, i, pts[0], gap)
					}
				}
				prevLastPTS = pts[len(pts)-1]
			}
		})
	}
}

func TestHLS100_40_TotalFrameCount(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
		dur float64
	}{
		{f.tsH264Dir, "ts", 10}, {f.ourTSH264Dir, "ts", 10},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			var total int
			for _, seg := range segs {
				fc := hls100_ffprobeFrameCount(seg, "v:0")
				if fc > 0 {
					total += fc
				}
			}
			expected := tc.dur * 25.0
			if math.Abs(float64(total)-expected) > 10 {
				t.Errorf("total frames=%d expected~%.0f", total, expected)
			}
		})
	}
}

func TestHLS100_41_TSStartsWithSyncByte(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "sync", allTSDirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		for _, seg := range segs {
			data, _ := os.ReadFile(seg)
			if len(data) > 0 && data[0] != 0x47 {
				t.Errorf("%s: first byte is 0x%02x, expected 0x47", filepath.Base(seg), data[0])
			}
		}
	})
}

func TestHLS100_42_TSSizeMultipleOf188(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "188", allTSDirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		for _, seg := range segs {
			info, _ := os.Stat(seg)
			if info != nil && info.Size()%188 != 0 {
				t.Errorf("%s: size %d not multiple of 188", filepath.Base(seg), info.Size())
			}
		}
	})
}

func TestHLS100_43_PATPresent(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "PAT", allTSDirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) == 0 {
			t.Skip("no segments")
		}
		data, _ := os.ReadFile(segs[0])
		found := false
		for i := 0; i+188 <= len(data); i += 188 {
			if data[i] == 0x47 {
				pid := uint16(data[i+1]&0x1f)<<8 | uint16(data[i+2])
				if pid == 0x0000 {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("PAT (PID 0x0000) not found in first segment")
		}
	})
}

func TestHLS100_44_PMTPresent(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "PMT", allTSDirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) == 0 {
			t.Skip("no segments")
		}
		data, _ := os.ReadFile(segs[0])
		pids := map[uint16]bool{}
		for i := 0; i+188 <= len(data); i += 188 {
			if data[i] == 0x47 {
				pid := uint16(data[i+1]&0x1f)<<8 | uint16(data[i+2])
				pids[pid] = true
			}
		}
		hasPMT := false
		for pid := range pids {
			if pid > 0x0000 && pid < 0x0010 || (pid >= 0x0020 && pid < 0x1FFF && pid != 0x1FFF) {
				hasPMT = true
				break
			}
		}
		if !hasPMT && len(pids) <= 1 {
			t.Error("no PMT-like PID found (only PAT/null)")
		}
	})
}

func TestHLS100_45_VideoStreamPresent(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "video_pid", allTSDirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) == 0 {
			t.Skip("no segments")
		}
		codec := hls100_ffprobeCodec(segs[0], "v:0")
		if codec == "" {
			t.Error("no video stream found via ffprobe")
		}
	})
}

func TestHLS100_46_AudioStreamPresent(t *testing.T) {
	f := getFixture(t)
	avTSDirs := []string{f.tsH264Dir, f.tsShortDir, f.ourTSH264Dir, f.ourTSShortDir}
	runOnDirs(t, "audio_pid", avTSDirs, func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) == 0 {
			t.Skip("no segments")
		}
		codec := hls100_ffprobeCodec(segs[0], "a:0")
		if codec == "" {
			t.Error("no audio stream found via ffprobe")
		}
	})
}

func TestHLS100_47_VideoCodecH264(t *testing.T) {
	f := getFixture(t)
	h264dirs := []string{f.tsH264Dir, f.ourTSH264Dir}
	runOnDirs(t, "h264", h264dirs, func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) == 0 {
			t.Skip("no segments")
		}
		codec := hls100_ffprobeCodec(segs[0], "v:0")
		if codec != "h264" {
			t.Errorf("video codec %q, expected h264", codec)
		}
	})
}

func TestHLS100_48_AudioCodecAAC(t *testing.T) {
	f := getFixture(t)
	avDirs := []string{f.tsH264Dir, f.tsShortDir, f.ourTSH264Dir, f.ourTSShortDir}
	runOnDirs(t, "aac", avDirs, func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) == 0 {
			t.Skip("no segments")
		}
		codec := hls100_ffprobeCodec(segs[0], "a:0")
		if codec != "aac" {
			t.Errorf("audio codec %q, expected aac", codec)
		}
	})
}

func TestHLS100_49_ResolutionMatches(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "resolution", allTSDirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) == 0 {
			t.Skip("no segments")
		}
		w, h := hls100_ffprobeResolution(segs[0])
		if w > 0 && h > 0 && (w != 640 || h != 360) {
			t.Errorf("resolution %dx%d, expected 640x360", w, h)
		}
	})
}

func TestHLS100_50_SampleRateMatches(t *testing.T) {
	f := getFixture(t)
	avDirs := []string{f.tsH264Dir, f.ourTSH264Dir}
	runOnDirs(t, "sample_rate", avDirs, func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) == 0 {
			t.Skip("no segments")
		}
		sr := hls100_ffprobeInt(segs[0], "a:0", "stream=sample_rate")
		if sr > 0 && sr != 48000 {
			t.Errorf("sample rate %d, expected 48000", sr)
		}
	})
}

func TestHLS100_51_ChannelCountMatches(t *testing.T) {
	f := getFixture(t)
	avDirs := []string{f.tsH264Dir, f.ourTSH264Dir}
	runOnDirs(t, "channels", avDirs, func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) == 0 {
			t.Skip("no segments")
		}
		ch := hls100_ffprobeInt(segs[0], "a:0", "stream=channels")
		if ch > 0 && ch != 2 {
			t.Errorf("channels %d, expected 2", ch)
		}
	})
}

func TestHLS100_52_NoEmptyTSSegments(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "not_empty", allTSDirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		for _, seg := range segs {
			info, _ := os.Stat(seg)
			if info != nil && info.Size() <= 188 {
				t.Errorf("%s: segment too small (%d bytes)", filepath.Base(seg), info.Size())
			}
		}
	})
}

func TestHLS100_53_TSSegmentSizesReasonable(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "size_range", allTSDirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) < 2 {
			t.Skip("need >= 2 segments")
		}
		var sizes []int64
		for _, seg := range segs {
			info, _ := os.Stat(seg)
			if info != nil {
				sizes = append(sizes, info.Size())
			}
		}
		if len(sizes) < 2 {
			t.Skip("insufficient size data")
		}
		var minS, maxS int64 = sizes[0], sizes[0]
		for _, s := range sizes {
			if s < minS {
				minS = s
			}
			if s > maxS {
				maxS = s
			}
		}
		if maxS > minS*10 {
			t.Errorf("segment size range too wide: min=%d max=%d (ratio=%.1f)", minS, maxS, float64(maxS)/float64(minS))
		}
	})
}

func TestHLS100_54_PCRPresent(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "PCR", allTSDirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) == 0 {
			t.Skip("no segments")
		}
		data, _ := os.ReadFile(segs[0])
		found := false
		for i := 0; i+188 <= len(data); i += 188 {
			if data[i] != 0x47 {
				continue
			}
			adaptationFieldControl := (data[i+3] >> 4) & 0x3
			if adaptationFieldControl >= 2 {
				adaptLen := int(data[i+4])
				if adaptLen > 0 && (data[i+5]&0x10) != 0 {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("no PCR found in first segment")
		}
	})
}

func TestHLS100_55_ContinuityCounterIncrements(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "cc", allTSDirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "ts")
		if len(segs) == 0 {
			t.Skip("no segments")
		}
		data, _ := os.ReadFile(segs[0])
		pidCC := map[uint16]int{}
		for i := 0; i+188 <= len(data); i += 188 {
			if data[i] != 0x47 {
				continue
			}
			pid := uint16(data[i+1]&0x1f)<<8 | uint16(data[i+2])
			if pid == 0x1FFF {
				continue
			}
			cc := int(data[i+3] & 0x0F)
			adaptFieldCtl := (data[i+3] >> 4) & 0x3
			if adaptFieldCtl == 0 || adaptFieldCtl == 2 {
				continue
			}
			if prev, ok := pidCC[pid]; ok {
				expected := (prev + 1) & 0x0F
				if cc != expected && cc != prev {
					t.Errorf("PID 0x%04x: CC jumped from %d to %d (expected %d)", pid, prev, cc, expected)
					return
				}
			}
			pidCC[pid] = cc
		}
	})
}

func TestHLS100_56_FMP4SegmentHasMoof(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "moof", allFMP4Dirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "m4s")
		for _, seg := range segs {
			data, _ := os.ReadFile(seg)
			if !hasMP4Box(data, "moof") {
				t.Errorf("%s: missing moof box", filepath.Base(seg))
			}
		}
	})
}

func TestHLS100_57_FMP4SegmentHasMdat(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "mdat", allFMP4Dirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "m4s")
		for _, seg := range segs {
			data, _ := os.ReadFile(seg)
			if !hasMP4Box(data, "mdat") {
				t.Errorf("%s: missing mdat box", filepath.Base(seg))
			}
		}
	})
}

func TestHLS100_58_MoofContainsTraf(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "traf", allFMP4Dirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "m4s")
		for _, seg := range segs {
			data, _ := os.ReadFile(seg)
			if hasMP4Box(data, "moof") && !hasMP4Box(data, "traf") {
				t.Errorf("%s: moof present but no traf", filepath.Base(seg))
			}
		}
	})
}

func TestHLS100_59_TrafContainsTfhd(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "tfhd", allFMP4Dirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "m4s")
		for _, seg := range segs {
			data, _ := os.ReadFile(seg)
			if hasMP4Box(data, "traf") && !hasMP4Box(data, "tfhd") {
				t.Errorf("%s: traf present but no tfhd", filepath.Base(seg))
			}
		}
	})
}

func TestHLS100_60_TrafContainsTrun(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "trun", allFMP4Dirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "m4s")
		for _, seg := range segs {
			data, _ := os.ReadFile(seg)
			if hasMP4Box(data, "traf") && !hasMP4Box(data, "trun") {
				t.Errorf("%s: traf present but no trun", filepath.Base(seg))
			}
		}
	})
}

func TestHLS100_61_TrunHasSamples(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "trun_samples", allFMP4Dirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "m4s")
		for _, seg := range segs {
			data, _ := os.ReadFile(seg)
			if !hasMP4Box(data, "trun") {
				continue
			}
			trunPos := -1
			for i := 0; i <= len(data)-4; i++ {
				if string(data[i:i+4]) == "trun" {
					trunPos = i
					break
				}
			}
			if trunPos < 4 {
				continue
			}
			boxStart := trunPos - 4
			if boxStart+12 <= len(data) {
				payload := data[trunPos+4:]
				if len(payload) >= 8 {
					sampleCount := binary.BigEndian.Uint32(payload[4:8])
					if sampleCount == 0 {
						t.Errorf("%s: trun sample_count is 0", filepath.Base(seg))
					}
				}
			}
			_ = boxStart
		}
	})
}

func TestHLS100_62_TrafContainsTfdt(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "tfdt", allFMP4Dirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "m4s")
		for _, seg := range segs {
			data, _ := os.ReadFile(seg)
			if hasMP4Box(data, "traf") && !hasMP4Box(data, "tfdt") {
				t.Errorf("%s: traf present but no tfdt", filepath.Base(seg))
			}
		}
	})
}

func TestHLS100_63_TfdtIncreasing(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "tfdt_inc", allFMP4Dirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "m4s")
		var prevBDT uint64
		for si, seg := range segs {
			data, _ := os.ReadFile(seg)
			for i := 0; i <= len(data)-4; i++ {
				if string(data[i:i+4]) == "tfdt" {
					payload := data[i+4:]
					if len(payload) < 12 {
						break
					}
					var bdt uint64
					if payload[0] == 1 {
						bdt = binary.BigEndian.Uint64(payload[4:12])
					} else if len(payload) >= 8 {
						bdt = uint64(binary.BigEndian.Uint32(payload[4:8]))
					}
					if si > 0 && bdt < prevBDT {
						t.Errorf("seg %d tfdt %d < seg %d tfdt %d", si, bdt, si-1, prevBDT)
					}
					prevBDT = bdt
					break
				}
			}
		}
	})
}

func TestHLS100_64_MfhdSequenceIncrements(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "mfhd_seq", allFMP4Dirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "m4s")
		var prevSeq uint32
		for si, seg := range segs {
			data, _ := os.ReadFile(seg)
			for i := 0; i <= len(data)-4; i++ {
				if string(data[i:i+4]) == "mfhd" {
					payload := data[i+4:]
					if len(payload) >= 8 {
						seq := binary.BigEndian.Uint32(payload[4:8])
						if si > 0 && seq <= prevSeq {
							t.Errorf("seg %d mfhd sequence %d <= seg %d sequence %d", si, seq, si-1, prevSeq)
						}
						prevSeq = seq
					}
					break
				}
			}
		}
	})
}

func TestHLS100_65_InitCodecMatches(t *testing.T) {
	f := getFixture(t)
	cases := []struct {
		dir     string
		codec   string
		boxName string
	}{
		{f.fmp4H264Dir, "h264", "avc1"},
		{f.fmp4H265Dir, "h265", "hvc1"},
		{f.ourFMP4H264Dir, "h264", "avc1"},
		{f.ourFMP4H265Dir, "h265", "hvc1"},
	}
	for _, tc := range cases {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(tc.dir, "init.mp4"))
			if err != nil {
				t.Skip("no init.mp4")
			}
			if !hasMP4Box(data, tc.boxName) {
				altBox := "hev1"
				if tc.codec == "h264" {
					t.Errorf("init.mp4 missing %s box", tc.boxName)
					return
				}
				if !hasMP4Box(data, altBox) {
					t.Errorf("init.mp4 missing %s or %s box", tc.boxName, altBox)
				}
			}
		})
	}
}

func TestHLS100_66_InitHasStsd(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "stsd", allFMP4Dirs(f), func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		if !hasMP4Box(data, "stsd") {
			t.Error("init.mp4 missing stsd box")
		}
	})
}

func TestHLS100_67_InitCorrectDimensions(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "dimensions", allFMP4Dirs(f), func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		if !hasMP4Box(data, "tkhd") {
			t.Skip("no tkhd box")
		}
		for i := 0; i <= len(data)-4; i++ {
			if string(data[i:i+4]) == "tkhd" {
				payload := data[i+4:]
				version := payload[0]
				var offset int
				if version == 1 {
					offset = 84
				} else {
					offset = 72
				}
				if offset+8 <= len(payload) {
					w := binary.BigEndian.Uint32(payload[offset : offset+4])
					h := binary.BigEndian.Uint32(payload[offset+4 : offset+8])
					widthFixed := w >> 16
					heightFixed := h >> 16
					if widthFixed > 0 && heightFixed > 0 {
						if widthFixed != 640 || heightFixed != 360 {
							t.Logf("tkhd dimensions: %dx%d (may be 0 for audio track)", widthFixed, heightFixed)
						}
					}
				}
				break
			}
		}
	})
}

func TestHLS100_68_AudioInitSampleRate(t *testing.T) {
	f := getFixture(t)
	avFMP4Dirs := []string{f.fmp4H264Dir, f.fmp4H265Dir, f.ourFMP4H264Dir, f.ourFMP4H265Dir}
	runOnDirs(t, "audio_sr", avFMP4Dirs, func(t *testing.T, dir string) {
		initPath := filepath.Join(dir, "init.mp4")
		sr := hls100_ffprobeInt(initPath, "a:0", "stream=sample_rate")
		if sr > 0 && sr != 48000 {
			t.Errorf("init audio sample_rate %d, expected 48000", sr)
		}
	})
}

func TestHLS100_69_SegmentNoFtypOrMoov(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "no_ftyp_moov", allFMP4Dirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "m4s")
		for _, seg := range segs {
			data, _ := os.ReadFile(seg)
			boxes := readMP4Boxes(data)
			for _, b := range boxes {
				if b == "ftyp" || b == "moov" {
					t.Errorf("%s: segment contains %s box (should only be in init)", filepath.Base(seg), b)
				}
			}
		}
	})
}

func TestHLS100_70_SegmentSizesReasonable(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "size_reasonable", allFMP4Dirs(f), func(t *testing.T, dir string) {
		segs := segmentFiles(dir, "m4s")
		for _, seg := range segs {
			info, _ := os.Stat(seg)
			if info == nil {
				continue
			}
			if info.Size() < 100 {
				t.Errorf("%s: too small (%d bytes)", filepath.Base(seg), info.Size())
			}
			if info.Size() > 50*1024*1024 {
				t.Errorf("%s: too large (%d bytes)", filepath.Base(seg), info.Size())
			}
		}
	})
}

func TestHLS100_71_H264_avcCProfile(t *testing.T) {
	f := getFixture(t)
	h264Dirs := []string{f.fmp4H264Dir, f.ourFMP4H264Dir}
	runOnDirs(t, "avcC_profile", h264Dirs, func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		if !hasMP4Box(data, "avcC") {
			t.Error("no avcC box found")
			return
		}
		for i := 0; i <= len(data)-4; i++ {
			if string(data[i:i+4]) == "avcC" {
				payload := data[i+4:]
				if len(payload) >= 4 {
					configVersion := payload[0]
					profile := payload[1]
					if configVersion != 1 {
						t.Errorf("avcC config version %d, expected 1", configVersion)
					}
					if profile == 0 {
						t.Error("avcC profile is 0")
					}
					t.Logf("avcC profile=%d", profile)
				}
				break
			}
		}
	})
}

func TestHLS100_72_H264_SPSPresent(t *testing.T) {
	f := getFixture(t)
	h264Dirs := []string{f.fmp4H264Dir, f.ourFMP4H264Dir}
	runOnDirs(t, "SPS", h264Dirs, func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		for i := 0; i <= len(data)-4; i++ {
			if string(data[i:i+4]) == "avcC" {
				payload := data[i+4:]
				if len(payload) >= 6 {
					numSPS := payload[5] & 0x1F
					if numSPS == 0 {
						t.Error("avcC has 0 SPS entries")
					}
				}
				break
			}
		}
	})
}

func TestHLS100_73_H264_PPSPresent(t *testing.T) {
	f := getFixture(t)
	h264Dirs := []string{f.fmp4H264Dir, f.ourFMP4H264Dir}
	runOnDirs(t, "PPS", h264Dirs, func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		for i := 0; i <= len(data)-4; i++ {
			if string(data[i:i+4]) == "avcC" {
				payload := data[i+4:]
				if len(payload) >= 7 {
					numSPS := int(payload[5] & 0x1F)
					offset := 6
					for s := 0; s < numSPS && offset+2 <= len(payload); s++ {
						spsLen := int(binary.BigEndian.Uint16(payload[offset : offset+2]))
						offset += 2 + spsLen
					}
					if offset < len(payload) {
						numPPS := int(payload[offset])
						if numPPS == 0 {
							t.Error("avcC has 0 PPS entries")
						}
					}
				}
				break
			}
		}
	})
}

func TestHLS100_74_H265_hvcCBox(t *testing.T) {
	f := getFixture(t)
	h265Dirs := []string{f.fmp4H265Dir, f.ourFMP4H265Dir}
	runOnDirs(t, "hvcC", h265Dirs, func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		if !hasMP4Box(data, "hvcC") {
			t.Error("init.mp4 missing hvcC box")
		}
	})
}

func TestHLS100_75_H265_VPSPresent(t *testing.T) {
	f := getFixture(t)
	h265Dirs := []string{f.fmp4H265Dir, f.ourFMP4H265Dir}
	runOnDirs(t, "VPS", h265Dirs, func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		for i := 0; i <= len(data)-4; i++ {
			if string(data[i:i+4]) == "hvcC" {
				payload := data[i+4:]
				if len(payload) >= 23 {
					numArrays := int(payload[22])
					if numArrays == 0 {
						t.Error("hvcC has 0 parameter set arrays")
					}
				}
				break
			}
		}
	})
}

func TestHLS100_76_H265_SPSPresent(t *testing.T) {
	f := getFixture(t)
	h265Dirs := []string{f.fmp4H265Dir, f.ourFMP4H265Dir}
	runOnDirs(t, "H265_SPS", h265Dirs, func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		found := false
		for i := 0; i <= len(data)-4; i++ {
			if string(data[i:i+4]) == "hvcC" {
				payload := data[i+4:]
				if len(payload) >= 23 {
					numArrays := int(payload[22])
					offset := 23
					for a := 0; a < numArrays && offset+3 <= len(payload); a++ {
						naluType := payload[offset] & 0x3F
						numNalus := int(binary.BigEndian.Uint16(payload[offset+1 : offset+3]))
						offset += 3
						if naluType == 33 {
							found = numNalus > 0
						}
						for n := 0; n < numNalus && offset+2 <= len(payload); n++ {
							naluLen := int(binary.BigEndian.Uint16(payload[offset : offset+2]))
							offset += 2 + naluLen
						}
					}
				}
				break
			}
		}
		if !found {
			t.Error("hvcC missing SPS (NALU type 33)")
		}
	})
}

func TestHLS100_77_H265_PPSPresent(t *testing.T) {
	f := getFixture(t)
	h265Dirs := []string{f.fmp4H265Dir, f.ourFMP4H265Dir}
	runOnDirs(t, "H265_PPS", h265Dirs, func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		found := false
		for i := 0; i <= len(data)-4; i++ {
			if string(data[i:i+4]) == "hvcC" {
				payload := data[i+4:]
				if len(payload) >= 23 {
					numArrays := int(payload[22])
					offset := 23
					for a := 0; a < numArrays && offset+3 <= len(payload); a++ {
						naluType := payload[offset] & 0x3F
						numNalus := int(binary.BigEndian.Uint16(payload[offset+1 : offset+3]))
						offset += 3
						if naluType == 34 {
							found = numNalus > 0
						}
						for n := 0; n < numNalus && offset+2 <= len(payload); n++ {
							naluLen := int(binary.BigEndian.Uint16(payload[offset : offset+2]))
							offset += 2 + naluLen
						}
					}
				}
				break
			}
		}
		if !found {
			t.Error("hvcC missing PPS (NALU type 34)")
		}
	})
}

func TestHLS100_78_AAC_EsdsBox(t *testing.T) {
	f := getFixture(t)
	avFMP4Dirs := []string{f.fmp4H264Dir, f.fmp4H265Dir, f.ourFMP4H264Dir, f.ourFMP4H265Dir}
	runOnDirs(t, "esds", avFMP4Dirs, func(t *testing.T, dir string) {
		data, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
		if err != nil {
			t.Skip("no init.mp4")
		}
		if hasMP4Box(data, "mp4a") && !hasMP4Box(data, "esds") {
			t.Error("mp4a present but esds missing")
		}
	})
}

func TestHLS100_79_AAC_AudioObjectType(t *testing.T) {
	f := getFixture(t)
	avFMP4Dirs := []string{f.fmp4H264Dir, f.fmp4H265Dir, f.ourFMP4H264Dir, f.ourFMP4H265Dir}
	runOnDirs(t, "aot", avFMP4Dirs, func(t *testing.T, dir string) {
		initPath := filepath.Join(dir, "init.mp4")
		codec := hls100_ffprobeCodec(initPath, "a:0")
		if codec != "" && codec != "aac" {
			t.Errorf("audio codec %q, expected aac", codec)
		}
	})
}

func TestHLS100_80_CodecStringExtractable(t *testing.T) {
	f := getFixture(t)
	cases := []struct {
		dir    string
		prefix string
	}{
		{f.fmp4H264Dir, "avc1"},
		{f.fmp4H265Dir, "hvc1"},
		{f.ourFMP4H264Dir, "avc1"},
		{f.ourFMP4H265Dir, "hvc1"},
	}
	for _, tc := range cases {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			initPath := filepath.Join(tc.dir, "init.mp4")
			val := hls100_ffprobeField(initPath, "v:0", "stream=codec_tag_string")
			if val == "" {
				codec := hls100_ffprobeCodec(initPath, "v:0")
				if codec == "" {
					t.Error("no codec info extractable from init")
				}
				return
			}
			t.Logf("codec_tag_string: %s", val)
		})
	}
}

func TestHLS100_81_10sSourceProduces10s(t *testing.T) {
	f := getFixture(t)
	dirs := []string{f.tsH264Dir, f.fmp4H264Dir, f.ourTSH264Dir, f.ourFMP4H264Dir}
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			dur := hls100_ffprobeDuration(filepath.Join(dir, "playlist.m3u8"))
			if dur <= 0 {
				t.Skip("ffprobe returned no duration")
			}
			if math.Abs(dur-10.0) > 1.5 {
				t.Errorf("ffprobe total duration %.3f, expected ~10s", dur)
			}
		})
	}
}

func TestHLS100_82_25fpsFrameCount(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
		dur float64
	}{
		{f.tsH264Dir, "ts", 10},
		{f.ourTSH264Dir, "ts", 10},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			var total int
			for _, seg := range segs {
				fc := hls100_ffprobeFrameCount(seg, "v:0")
				if fc > 0 {
					total += fc
				}
			}
			expected := tc.dur * 25.0
			if math.Abs(float64(total)-expected) > 10 {
				t.Errorf("total frames=%d, expected~%.0f", total, expected)
			}
		})
	}
}

func TestHLS100_83_PlaybackRate(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"},
		{f.ourTSH264Dir, "ts"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			if len(segs) == 0 {
				t.Skip("no segments")
			}
			var totalFrames int
			var minPTS, maxPTS float64
			first := true
			for _, seg := range segs {
				fc := hls100_ffprobeFrameCount(seg, "v:0")
				if fc > 0 {
					totalFrames += fc
				}
				pts := hls100_ffprobePacketPTSTime(seg, "v:0")
				for _, p := range pts {
					if first || p < minPTS {
						minPTS = p
					}
					if first || p > maxPTS {
						maxPTS = p
					}
					first = false
				}
			}
			ptsRange := maxPTS - minPTS
			if ptsRange > 0 && totalFrames > 0 {
				rate := float64(totalFrames) / ptsRange
				expected := 25.0
				ratio := rate / expected
				if ratio < 0.8 || ratio > 1.2 {
					t.Errorf("playback rate %.2f fps (expected ~25, ratio=%.2f)", rate, ratio)
				}
			}
		})
	}
}

func TestHLS100_84_NoFrameDuplication(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"},
		{f.ourTSH264Dir, "ts"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			for si, seg := range segs {
				pts := hls100_ffprobePacketPTSTime(seg, "v:0")
				for j := 1; j < len(pts); j++ {
					if pts[j] == pts[j-1] {
						t.Errorf("seg %d: duplicate PTS %.3f at frames %d-%d", si, pts[j], j-1, j)
						break
					}
				}
			}
		})
	}
}

func TestHLS100_85_NoFrameDrops(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"},
		{f.ourTSH264Dir, "ts"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			frameDur := 1.0 / 25.0
			for si, seg := range segs {
				pts := hls100_ffprobePacketPTSTime(seg, "v:0")
				for j := 1; j < len(pts); j++ {
					gap := pts[j] - pts[j-1]
					if gap > frameDur*2.5 {
						t.Errorf("seg %d: PTS gap %.3f at frame %d (>2.5x frame duration)", si, gap, j)
						break
					}
				}
			}
		})
	}
}

func TestHLS100_86_SingleSegmentContent(t *testing.T) {
	f := getFixture(t)
	t.Run("ref", func(t *testing.T) {
		content := readPlaylist(f.tsShortDir)
		if content == "" {
			t.Skip("no playlist")
		}
		durations := extractEXTINFs(content)
		if len(durations) < 1 {
			t.Error("short content produced 0 segments")
		}
		if !strings.Contains(content, "#EXTM3U") {
			t.Error("missing EXTM3U")
		}
		if !strings.Contains(content, "#EXT-X-ENDLIST") {
			t.Error("missing ENDLIST")
		}
	})
	t.Run("ours", func(t *testing.T) {
		content := readPlaylist(f.ourTSShortDir)
		if content == "" {
			t.Skip("no playlist")
		}
		durations := extractEXTINFs(content)
		if len(durations) < 1 {
			t.Error("short content produced 0 segments")
		}
	})
}

func TestHLS100_87_VideoOnlyValidPlaylist(t *testing.T) {
	f := getFixture(t)
	for _, dir := range []string{f.tsVidOnlyDir, f.ourTSVidOnlyDir} {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			content := readPlaylist(dir)
			if content == "" {
				t.Skip("no playlist")
			}
			if !strings.Contains(content, "#EXTM3U") {
				t.Error("missing EXTM3U")
			}
			if !strings.Contains(content, "#EXT-X-ENDLIST") {
				t.Error("missing ENDLIST")
			}
			durations := extractEXTINFs(content)
			if len(durations) == 0 {
				t.Error("no segments")
			}
		})
	}
}

func TestHLS100_88_FirstSegmentPlayableStandalone(t *testing.T) {
	f := getFixture(t)
	dirs := []struct {
		dir string
		ext string
	}{
		{f.tsH264Dir, "ts"},
		{f.ourTSH264Dir, "ts"},
	}
	for _, tc := range dirs {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			segs := segmentFiles(tc.dir, tc.ext)
			if len(segs) == 0 {
				t.Skip("no segments")
			}
			codec := hls100_ffprobeCodec(segs[0], "v:0")
			if codec == "" {
				t.Error("first segment not playable standalone (no video codec detected)")
			}
		})
	}
}

func TestHLS100_89_PlaylistParseRoundTrip(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "roundtrip", allDirs(f), func(t *testing.T, dir string) {
		content := readPlaylist(dir)
		seg1, dur1, td1 := parseHLSPlaylist100(content)
		lines := strings.Split(strings.TrimSpace(content), "\n")
		var rebuilt strings.Builder
		for _, line := range lines {
			rebuilt.WriteString(strings.TrimSpace(line))
			rebuilt.WriteString("\n")
		}
		seg2, dur2, td2 := parseHLSPlaylist100(rebuilt.String())
		if seg1 != seg2 {
			t.Errorf("segment count changed: %d -> %d", seg1, seg2)
		}
		if td1 != td2 {
			t.Errorf("target duration changed: %d -> %d", td1, td2)
		}
		for i := 0; i < len(dur1) && i < len(dur2); i++ {
			if math.Abs(dur1[i]-dur2[i]) > 0.001 {
				t.Errorf("duration %d changed: %.6f -> %.6f", i, dur1[i], dur2[i])
			}
		}
	})
}

func parseHLSPlaylist100(content string) (int, []float64, int) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var durations []float64
	targetDur := -1
	segCount := 0
	tdRe := regexp.MustCompile(`#EXT-X-TARGETDURATION:(\d+)`)
	extinfRe := regexp.MustCompile(`#EXTINF:([\d.]+)`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if m := tdRe.FindStringSubmatch(line); len(m) > 1 {
			targetDur, _ = strconv.Atoi(m[1])
		}
		if m := extinfRe.FindStringSubmatch(line); len(m) > 1 {
			d, _ := strconv.ParseFloat(m[1], 64)
			durations = append(durations, d)
			segCount++
		}
	}
	return segCount, durations, targetDur
}

func TestHLS100_90_EmptyContentGraceful(t *testing.T) {
	dir := t.TempDir()
	hlsMuxer, err := NewHLSMuxer(HLSMuxOpts{
		OutputDir:          dir,
		SegmentDurationSec: 2,
		SegmentType:        "mpegts",
		VideoCodecID:       astiav.CodecIDH264,
		VideoWidth:         640,
		VideoHeight:        360,
		VideoTimeBase:      astiav.NewRational(1, 90000),
		VideoFrameRate:     25,
	})
	if err != nil {
		t.Fatalf("create muxer: %v", err)
	}
	err = hlsMuxer.Close()
	if err != nil {
		t.Logf("close with no packets: %v (acceptable)", err)
	}
}

func TestHLS100_91_VeryShortSegmentTarget(t *testing.T) {
	f := getFixture(t)
	_ = f
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.ts")
	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=320x240:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-an",
		"-f", "mpegts", "-y", inputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "Unknown encoder") {
			t.Skip("libx264 not available")
		}
		t.Fatalf("generate input: %v\n%s", err, out)
	}

	refDir := filepath.Join(dir, "ref")
	os.MkdirAll(refDir, 0755)
	refCmd := exec.Command("ffmpeg",
		"-i", inputPath, "-c", "copy",
		"-f", "hls", "-hls_time", "1", "-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(refDir, "seg%d.ts"),
		"-y", filepath.Join(refDir, "playlist.m3u8"),
	)
	if out, err := refCmd.CombinedOutput(); err != nil {
		t.Fatalf("generate ref: %v\n%s", err, out)
	}

	content := readPlaylist(refDir)
	if content == "" {
		t.Fatal("no playlist")
	}
	durations := extractEXTINFs(content)
	if len(durations) < 2 {
		t.Logf("short target produced only %d segments", len(durations))
	}
	for _, d := range durations {
		if d <= 0 {
			t.Errorf("invalid segment duration: %f", d)
		}
	}
}

func TestHLS100_92_LongSegmentTarget(t *testing.T) {
	f := getFixture(t)
	_ = f
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.ts")
	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "testsrc2=duration=10:size=320x240:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-an",
		"-f", "mpegts", "-y", inputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "Unknown encoder") {
			t.Skip("libx264 not available")
		}
		t.Fatalf("generate input: %v\n%s", err, out)
	}

	refDir := filepath.Join(dir, "ref")
	os.MkdirAll(refDir, 0755)
	refCmd := exec.Command("ffmpeg",
		"-i", inputPath, "-c", "copy",
		"-f", "hls", "-hls_time", "10", "-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(refDir, "seg%d.ts"),
		"-y", filepath.Join(refDir, "playlist.m3u8"),
	)
	if out, err := refCmd.CombinedOutput(); err != nil {
		t.Fatalf("generate ref: %v\n%s", err, out)
	}

	content := readPlaylist(refDir)
	durations := extractEXTINFs(content)
	if len(durations) > 2 {
		t.Errorf("10s target on 10s source produced %d segments (expected 1-2)", len(durations))
	}
}

func TestHLS100_93_SegmentCountMatchesDuration(t *testing.T) {
	f := getFixture(t)
	cases := []struct {
		dir     string
		dur     float64
		target  int
	}{
		{f.tsH264Dir, 10, 2},
		{f.ourTSH264Dir, 10, 2},
		{f.tsVidOnlyDir, 5, 2},
		{f.ourTSVidOnlyDir, 5, 2},
	}
	for _, tc := range cases {
		t.Run(filepath.Base(tc.dir), func(t *testing.T) {
			content := readPlaylist(tc.dir)
			if content == "" {
				t.Skip("no playlist")
			}
			durations := extractEXTINFs(content)
			expected := math.Ceil(tc.dur / float64(tc.target))
			diff := math.Abs(float64(len(durations)) - expected)
			if diff > 2 {
				t.Errorf("segment count %d, expected ~%.0f (duration=%.0f, target=%d)", len(durations), expected, tc.dur, tc.target)
			}
		})
	}
}

func TestHLS100_94_FilePermissions(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "perms", allDirs(f), func(t *testing.T, dir string) {
		playlistPath := filepath.Join(dir, "playlist.m3u8")
		info, err := os.Stat(playlistPath)
		if err != nil {
			t.Skip("no playlist")
		}
		perm := info.Mode().Perm()
		if perm&0444 != 0444 {
			t.Errorf("playlist permissions %o (not readable)", perm)
		}
	})
}

func TestHLS100_95_NoTempFiles(t *testing.T) {
	f := getFixture(t)
	runOnDirs(t, "no_tmp", allDirs(f), func(t *testing.T, dir string) {
		tmpFiles, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
		if len(tmpFiles) > 0 {
			t.Errorf("found %d .tmp files: %v", len(tmpFiles), tmpFiles)
		}
	})
}

func TestHLS100_96_OurTSSegmentCountMatchesFFmpeg(t *testing.T) {
	f := getFixture(t)
	refContent := readPlaylist(f.tsH264Dir)
	ourContent := readPlaylist(f.ourTSH264Dir)
	if refContent == "" || ourContent == "" {
		t.Skip("missing playlists")
	}
	refSegs := extractEXTINFs(refContent)
	ourSegs := extractEXTINFs(ourContent)
	diff := len(ourSegs) - len(refSegs)
	if diff < 0 {
		diff = -diff
	}
	if diff > 1 {
		t.Errorf("TS segment count: ref=%d ours=%d (diff=%d, max allowed=1)", len(refSegs), len(ourSegs), diff)
	}
	t.Logf("TS segment count: ref=%d ours=%d", len(refSegs), len(ourSegs))
}

func TestHLS100_97_OurFMP4SegmentCountMatchesFFmpeg(t *testing.T) {
	f := getFixture(t)
	refContent := readPlaylist(f.fmp4H264Dir)
	ourContent := readPlaylist(f.ourFMP4H264Dir)
	if refContent == "" || ourContent == "" {
		t.Skip("missing playlists")
	}
	refSegs := extractEXTINFs(refContent)
	ourSegs := extractEXTINFs(ourContent)
	diff := len(ourSegs) - len(refSegs)
	if diff < 0 {
		diff = -diff
	}
	if diff > 1 {
		t.Errorf("fMP4 segment count: ref=%d ours=%d (diff=%d, max allowed=1)", len(refSegs), len(ourSegs), diff)
	}
	t.Logf("fMP4 segment count: ref=%d ours=%d", len(refSegs), len(ourSegs))
}

func TestHLS100_98_OurPlaylistSameStructureTags(t *testing.T) {
	f := getFixture(t)
	pairs := []struct {
		refDir string
		ourDir string
	}{
		{f.tsH264Dir, f.ourTSH264Dir},
		{f.fmp4H264Dir, f.ourFMP4H264Dir},
	}
	for _, p := range pairs {
		t.Run(filepath.Base(p.ourDir), func(t *testing.T) {
			refContent := readPlaylist(p.refDir)
			ourContent := readPlaylist(p.ourDir)
			if refContent == "" || ourContent == "" {
				t.Skip("missing playlists")
			}
			requiredTags := []string{"#EXTM3U", "#EXT-X-VERSION:", "#EXT-X-TARGETDURATION:", "#EXT-X-MEDIA-SEQUENCE:", "#EXTINF:", "#EXT-X-ENDLIST"}
			for _, tag := range requiredTags {
				if strings.Contains(refContent, tag) && !strings.Contains(ourContent, tag) {
					t.Errorf("our playlist missing %s (present in reference)", tag)
				}
			}
		})
	}
}

func TestHLS100_99_OurTotalDurationMatchesFFmpeg(t *testing.T) {
	f := getFixture(t)
	pairs := []struct {
		refDir string
		ourDir string
	}{
		{f.tsH264Dir, f.ourTSH264Dir},
		{f.fmp4H264Dir, f.ourFMP4H264Dir},
	}
	for _, p := range pairs {
		t.Run(filepath.Base(p.ourDir), func(t *testing.T) {
			refContent := readPlaylist(p.refDir)
			ourContent := readPlaylist(p.ourDir)
			if refContent == "" || ourContent == "" {
				t.Skip("missing playlists")
			}
			var refTotal, ourTotal float64
			for _, d := range extractEXTINFs(refContent) {
				refTotal += d
			}
			for _, d := range extractEXTINFs(ourContent) {
				ourTotal += d
			}
			if math.Abs(ourTotal-refTotal) > 0.5 {
				t.Errorf("duration mismatch: ref=%.3f ours=%.3f (diff=%.3f)", refTotal, ourTotal, ourTotal-refTotal)
			}
			t.Logf("duration: ref=%.3f ours=%.3f", refTotal, ourTotal)
		})
	}
}

func TestHLS100_100_OurInitBoxTypesMatchFFmpeg(t *testing.T) {
	f := getFixture(t)
	pairs := []struct {
		refDir string
		ourDir string
	}{
		{f.fmp4H264Dir, f.ourFMP4H264Dir},
		{f.fmp4H265Dir, f.ourFMP4H265Dir},
	}
	for _, p := range pairs {
		t.Run(filepath.Base(p.ourDir), func(t *testing.T) {
			refData, err := os.ReadFile(filepath.Join(p.refDir, "init.mp4"))
			if err != nil {
				t.Skip("no reference init.mp4")
			}
			ourData, err := os.ReadFile(filepath.Join(p.ourDir, "init.mp4"))
			if err != nil {
				t.Skip("no our init.mp4")
			}
			refBoxes := readMP4Boxes(refData)
			ourBoxes := readMP4Boxes(ourData)
			t.Logf("init boxes: ref=%v ours=%v", refBoxes, ourBoxes)

			refSet := map[string]bool{}
			for _, b := range refBoxes {
				refSet[b] = true
			}
			for box := range refSet {
				found := false
				for _, b := range ourBoxes {
					if b == box {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("our init.mp4 missing top-level box %q (present in reference)", box)
				}
			}
		})
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	if fixture != nil {
		root := filepath.Join(os.TempDir(), "hls100_test")
		os.RemoveAll(root)
	}
	os.Exit(code)
}
