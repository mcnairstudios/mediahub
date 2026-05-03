//go:build cgo

package mux

import (
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/output/validate"
)

func skipIfNoFFmpegBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
}

func skipIfNoFFprobeBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

func generateTestInput(t *testing.T, dir string, durationSec int, codec string, includeAudio bool) string {
	t.Helper()
	return generateTestInputFormat(t, dir, durationSec, codec, includeAudio, "mpegts")
}

func generateTestInputFormat(t *testing.T, dir string, durationSec int, codec string, includeAudio bool, format string) string {
	t.Helper()
	ext := ".ts"
	if format == "mp4" {
		ext = ".mp4"
	}
	inputPath := filepath.Join(dir, "input"+ext)

	args := []string{
		"-f", "lavfi", "-i",
		"testsrc2=duration=" + strconv.Itoa(durationSec) + ":size=640x360:rate=25",
	}

	if includeAudio {
		args = append(args,
			"-f", "lavfi", "-i",
			"sine=frequency=440:duration="+strconv.Itoa(durationSec)+":sample_rate=48000",
		)
	}

	switch codec {
	case "h264":
		args = append(args,
			"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
			"-g", "50", "-keyint_min", "50",
		)
	case "h265":
		args = append(args,
			"-c:v", "libx265", "-preset", "ultrafast",
			"-x265-params", "keyint=50:min-keyint=50",
		)
	}

	if includeAudio {
		args = append(args, "-c:a", "aac", "-ac", "2", "-ar", "48000", "-b:a", "128k")
	} else {
		args = append(args, "-an")
	}

	args = append(args, "-f", format, "-y", inputPath)

	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "Unknown encoder") || strings.Contains(string(out), "Encoder") {
			t.Skipf("encoder not available: %s", string(out))
		}
		t.Fatalf("generate test input (%s, %s): %v\n%s", codec, format, err, out)
	}
	return inputPath
}

func generateHLSReference(t *testing.T, inputPath, refDir string, segType string) {
	t.Helper()

	args := []string{
		"-i", inputPath,
		"-c", "copy",
		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "0",
	}

	if segType == "fmp4" {
		args = append(args,
			"-bsf:a", "aac_adtstoasc",
			"-hls_segment_type", "fmp4",
			"-hls_fmp4_init_filename", "init.mp4",
			"-hls_segment_filename", filepath.Join(refDir, "seg%d.m4s"),
		)
	} else {
		args = append(args,
			"-hls_segment_filename", filepath.Join(refDir, "seg%d.ts"),
		)
	}

	args = append(args, "-y", filepath.Join(refDir, "playlist.m3u8"))

	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate reference HLS: %v\n%s", err, out)
	}
}

func muxThroughOurHLS(t *testing.T, inputPath, ourDir string, segType string) {
	t.Helper()

	fc := astiav.AllocFormatContext()
	if fc == nil {
		t.Fatal("alloc format context")
	}
	defer fc.Free()

	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	if err := fc.OpenInput(inputPath, nil, inputDict); err != nil {
		t.Fatalf("open input: %v", err)
	}
	defer fc.CloseInput()
	if err := fc.FindStreamInfo(nil); err != nil {
		t.Fatalf("find stream info: %v", err)
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
		t.Fatal("no video stream in test input")
	}

	vcp := videoStream.CodecParameters()
	var videoExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}

	opts := HLSMuxOpts{
		OutputDir:          ourDir,
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
		t.Fatalf("create HLS muxer: %v", err)
	}

	pkt := astiav.AllocPacket()
	if pkt == nil {
		t.Fatal("alloc packet")
	}
	defer pkt.Free()

	var videoPkts, audioPkts int
	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		switch pkt.StreamIndex() {
		case videoStream.Index():
			if err := hlsMuxer.WriteVideoPacket(pkt); err != nil {
				t.Fatalf("write video packet %d: %v", videoPkts, err)
			}
			videoPkts++
		default:
			if audioStream != nil && pkt.StreamIndex() == audioStream.Index() {
				if err := hlsMuxer.WriteAudioPacket(pkt); err != nil {
					t.Fatalf("write audio packet %d: %v", audioPkts, err)
				}
				audioPkts++
			}
		}
		pkt.Unref()
	}

	t.Logf("wrote %d video + %d audio packets", videoPkts, audioPkts)

	if err := hlsMuxer.Close(); err != nil {
		t.Fatalf("close muxer: %v", err)
	}
}

func ffprobePlaylistDuration(t *testing.T, playlistPath string) float64 {
	t.Helper()
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		playlistPath,
	)
	out, err := cmd.Output()
	if err != nil {
		t.Logf("ffprobe playlist duration failed: %v", err)
		return -1
	}
	val := strings.TrimSpace(string(out))
	dur, err := strconv.ParseFloat(val, 64)
	if err != nil {
		t.Logf("ffprobe returned non-numeric duration: %q", val)
		return -1
	}
	return dur
}

func ffprobeSegmentDuration(t *testing.T, segPath string) float64 {
	t.Helper()
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		segPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return -1
	}
	val := strings.TrimSpace(string(out))
	dur, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return -1
	}
	return dur
}

func ffprobeStreamCodec(t *testing.T, path string, mediaType string) string {
	t.Helper()
	selectStreams := "v:0"
	if mediaType == "audio" {
		selectStreams = "a:0"
	}
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", selectStreams,
		"-show_entries", "stream=codec_name",
		"-of", "csv=p=0",
		path,
	)
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

func ffprobeStreamResolution(t *testing.T, path string) (int, int) {
	t.Helper()
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return 0, 0
	}
	parts := strings.Split(strings.TrimSpace(lines[0]), ",")
	if len(parts) != 2 {
		return 0, 0
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return w, h
}

func ffprobeFrameRate(t *testing.T, path string) float64 {
	t.Helper()
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", "v:0",
		"-show_entries", "stream=r_frame_rate",
		"-of", "csv=p=0",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return 0
	}
	val := strings.TrimSpace(lines[0])
	parts := strings.Split(val, "/")
	if len(parts) == 2 {
		num, _ := strconv.ParseFloat(parts[0], 64)
		den, _ := strconv.ParseFloat(parts[1], 64)
		if den > 0 {
			return num / den
		}
	}
	fps, _ := strconv.ParseFloat(val, 64)
	return fps
}

func ffprobeValidateNoErrors(t *testing.T, playlistPath string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-v", "error", "-i", playlistPath, "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("ffmpeg error-check of HLS output failed: %v\n%s", err, string(out))
	} else if len(strings.TrimSpace(string(out))) > 0 {
		t.Logf("ffmpeg warnings: %s", string(out))
	}
}

func containsFMP4Box(data []byte, boxType string) bool {
	needle := []byte(boxType)
	for i := 0; i <= len(data)-4; i++ {
		if data[i] == needle[0] && data[i+1] == needle[1] && data[i+2] == needle[2] && data[i+3] == needle[3] {
			return true
		}
	}
	return false
}

func parseFMP4Boxes(data []byte) []string {
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

func TestHLSConformance_H264_AAC_MPEGTS(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	tmpRoot := t.TempDir()
	refDir := filepath.Join(tmpRoot, "ref")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(refDir, 0755)
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInput(t, tmpRoot, 10, "h264", true)

	generateHLSReference(t, inputPath, refDir, "mpegts")
	muxThroughOurHLS(t, inputPath, ourDir, "mpegts")

	refPlaylist, err := os.ReadFile(filepath.Join(refDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read ref playlist: %v", err)
	}
	ourPlaylist, err := os.ReadFile(filepath.Join(ourDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read our playlist: %v", err)
	}

	t.Logf("reference playlist:\n%s", string(refPlaylist))
	t.Logf("our playlist:\n%s", string(ourPlaylist))

	refErrs := validate.ValidateHLSPlaylist(refPlaylist)
	for _, e := range refErrs {
		t.Errorf("validator rejected reference playlist: %v", e)
	}
	ourErrs := validate.ValidateHLSPlaylist(ourPlaylist)
	for _, e := range ourErrs {
		t.Errorf("validator rejected our playlist: %v", e)
	}

	refSegCount, refDurations, refTargetDur := parseHLSPlaylist(t, string(refPlaylist))
	ourSegCount, ourDurations, ourTargetDur := parseHLSPlaylist(t, string(ourPlaylist))

	t.Logf("segment count: ref=%d ours=%d", refSegCount, ourSegCount)
	t.Logf("target duration: ref=%d ours=%d", refTargetDur, ourTargetDur)

	segDiff := ourSegCount - refSegCount
	if segDiff < 0 {
		segDiff = -segDiff
	}
	if segDiff > 2 {
		t.Errorf("segment count difference too large: ref=%d ours=%d", refSegCount, ourSegCount)
	}

	var refTotal, ourTotal float64
	for _, d := range refDurations {
		refTotal += d
	}
	for _, d := range ourDurations {
		ourTotal += d
	}
	t.Logf("total duration: ref=%.3fs ours=%.3fs (source=10s)", refTotal, ourTotal)

	if math.Abs(ourTotal-10.0) > 1.0 {
		t.Errorf("our total duration %.3fs deviates from 10s by more than 1s", ourTotal)
	}

	if ourTargetDur > 0 {
		for i, d := range ourDurations {
			if d > float64(ourTargetDur)+0.5 {
				t.Errorf("segment %d duration %.3fs exceeds target %d", i, d, ourTargetDur)
			}
		}
	}

	ourSegs, _ := filepath.Glob(filepath.Join(ourDir, "seg*.ts"))
	for _, seg := range ourSegs {
		videoCodec := ffprobeStreamCodec(t, seg, "video")
		if videoCodec != "" && videoCodec != "h264" {
			t.Errorf("segment %s: video codec=%s, expected h264", filepath.Base(seg), videoCodec)
		}

		audioCodec := ffprobeStreamCodec(t, seg, "audio")
		if audioCodec != "" && audioCodec != "aac" {
			t.Errorf("segment %s: audio codec=%s, expected aac", filepath.Base(seg), audioCodec)
		}

		w, h := ffprobeStreamResolution(t, seg)
		if w > 0 && h > 0 {
			if w != 640 || h != 360 {
				t.Errorf("segment %s: resolution %dx%d, expected 640x360", filepath.Base(seg), w, h)
			}
		}
	}

	for i, seg := range ourSegs {
		measured := ffprobeSegmentDuration(t, seg)
		if measured < 0 {
			continue
		}
		if i < len(ourDurations) {
			extinf := ourDurations[i]
			if extinf > 0 {
				ratio := measured / extinf
				if ratio < 0.7 || ratio > 1.3 {
					t.Errorf("segment %d: measured=%.3fs EXTINF=%.3fs ratio=%.2f (outside 30%% tolerance)",
						i, measured, extinf, ratio)
				} else {
					t.Logf("segment %d: measured=%.3fs EXTINF=%.3fs ratio=%.2f", i, measured, extinf, ratio)
				}
			}
		}
	}

	ffprobeValidateNoErrors(t, filepath.Join(ourDir, "playlist.m3u8"))
}

func TestHLSConformance_H265_AAC_FMP4(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	tmpRoot := t.TempDir()
	refDir := filepath.Join(tmpRoot, "ref")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(refDir, 0755)
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInputFormat(t, tmpRoot, 10, "h265", true, "mp4")

	generateHLSReference(t, inputPath, refDir, "fmp4")
	muxThroughOurHLS(t, inputPath, ourDir, "fmp4")

	refPlaylist, err := os.ReadFile(filepath.Join(refDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read ref playlist: %v", err)
	}
	ourPlaylist, err := os.ReadFile(filepath.Join(ourDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read our playlist: %v", err)
	}

	t.Logf("reference playlist:\n%s", string(refPlaylist))
	t.Logf("our playlist:\n%s", string(ourPlaylist))

	ourErrs := validate.ValidateHLSPlaylist(ourPlaylist)
	for _, e := range ourErrs {
		t.Errorf("validator rejected our playlist: %v", e)
	}

	refSegCount, _, _ := parseHLSPlaylist(t, string(refPlaylist))
	ourSegCount, ourDurations, ourTargetDur := parseHLSPlaylist(t, string(ourPlaylist))

	segDiff := ourSegCount - refSegCount
	if segDiff < 0 {
		segDiff = -segDiff
	}
	if segDiff > 2 {
		t.Errorf("segment count difference too large: ref=%d ours=%d", refSegCount, ourSegCount)
	}

	var ourTotal float64
	for _, d := range ourDurations {
		ourTotal += d
	}
	t.Logf("total duration: ours=%.3fs (source=10s)", ourTotal)

	if math.Abs(ourTotal-10.0) > 1.0 {
		t.Errorf("our total duration %.3fs deviates from 10s by more than 1s", ourTotal)
	}

	if ourTargetDur > 0 {
		for i, d := range ourDurations {
			if d > float64(ourTargetDur)+0.5 {
				t.Errorf("segment %d duration %.3fs exceeds target %d", i, d, ourTargetDur)
			}
		}
	}

	initPath := filepath.Join(ourDir, "init.mp4")
	initData, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatalf("read init.mp4: %v", err)
	}
	t.Logf("init.mp4: %d bytes", len(initData))

	if !containsFMP4Box(initData, "ftyp") {
		t.Error("init.mp4 missing ftyp box")
	}
	if !containsFMP4Box(initData, "moov") {
		t.Error("init.mp4 missing moov box")
	}

	initBoxes := parseFMP4Boxes(initData)
	t.Logf("init.mp4 boxes: %v", initBoxes)

	if containsFMP4Box(initData, "hvc1") || containsFMP4Box(initData, "hev1") {
		t.Log("init.mp4 contains HEVC codec config")
	}

	ourSegs, _ := filepath.Glob(filepath.Join(ourDir, "seg*.m4s"))
	for _, seg := range ourSegs {
		segData, err := os.ReadFile(seg)
		if err != nil {
			continue
		}
		if !containsFMP4Box(segData, "moof") {
			t.Errorf("segment %s missing moof box", filepath.Base(seg))
		}
		if !containsFMP4Box(segData, "mdat") {
			t.Errorf("segment %s missing mdat box", filepath.Base(seg))
		}

		boxes := parseFMP4Boxes(segData)
		t.Logf("segment %s: %d bytes, boxes=%v", filepath.Base(seg), len(segData), boxes)
	}

	for i, seg := range ourSegs {
		measured := ffprobeSegmentDuration(t, seg)
		if measured < 0 {
			continue
		}
		if i < len(ourDurations) && ourDurations[i] > 0 {
			ratio := measured / ourDurations[i]
			t.Logf("segment %d: measured=%.3fs EXTINF=%.3fs ratio=%.2f", i, measured, ourDurations[i], ratio)
		}
	}

	ffprobeValidateNoErrors(t, filepath.Join(ourDir, "playlist.m3u8"))
}

func TestHLSConformance_H264_AAC_FMP4(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	tmpRoot := t.TempDir()
	refDir := filepath.Join(tmpRoot, "ref")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(refDir, 0755)
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInputFormat(t, tmpRoot, 10, "h264", true, "mp4")

	generateHLSReference(t, inputPath, refDir, "fmp4")
	muxThroughOurHLS(t, inputPath, ourDir, "fmp4")

	refPlaylist, err := os.ReadFile(filepath.Join(refDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read ref playlist: %v", err)
	}
	ourPlaylist, err := os.ReadFile(filepath.Join(ourDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read our playlist: %v", err)
	}

	t.Logf("reference playlist:\n%s", string(refPlaylist))
	t.Logf("our playlist:\n%s", string(ourPlaylist))

	ourErrs := validate.ValidateHLSPlaylist(ourPlaylist)
	for _, e := range ourErrs {
		t.Errorf("validator rejected our playlist: %v", e)
	}

	refSegCount, _, _ := parseHLSPlaylist(t, string(refPlaylist))
	ourSegCount, ourDurations, _ := parseHLSPlaylist(t, string(ourPlaylist))

	segDiff := ourSegCount - refSegCount
	if segDiff < 0 {
		segDiff = -segDiff
	}
	if segDiff > 2 {
		t.Errorf("segment count: ref=%d ours=%d", refSegCount, ourSegCount)
	}

	var ourTotal float64
	for _, d := range ourDurations {
		ourTotal += d
	}
	if math.Abs(ourTotal-10.0) > 1.0 {
		t.Errorf("total duration %.3fs deviates from 10s by more than 1s", ourTotal)
	}

	initPath := filepath.Join(ourDir, "init.mp4")
	initData, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatalf("read init.mp4: %v", err)
	}

	if !containsFMP4Box(initData, "ftyp") {
		t.Error("init.mp4 missing ftyp box")
	}
	if !containsFMP4Box(initData, "moov") {
		t.Error("init.mp4 missing moov box")
	}

	if containsFMP4Box(initData, "avc1") || containsFMP4Box(initData, "avcC") {
		t.Log("init.mp4 contains H.264 codec config")
	}

	ourSegs, _ := filepath.Glob(filepath.Join(ourDir, "seg*.m4s"))
	for _, seg := range ourSegs {
		segData, _ := os.ReadFile(seg)
		if !containsFMP4Box(segData, "moof") {
			t.Errorf("segment %s missing moof box", filepath.Base(seg))
		}
		if !containsFMP4Box(segData, "mdat") {
			t.Errorf("segment %s missing mdat box", filepath.Base(seg))
		}
	}

	refInitPath := filepath.Join(refDir, "init.mp4")
	if refInitData, err := os.ReadFile(refInitPath); err == nil {
		refBoxes := parseFMP4Boxes(refInitData)
		ourBoxes := parseFMP4Boxes(initData)
		t.Logf("init box comparison: ref=%v ours=%v", refBoxes, ourBoxes)
	}

	ffprobeValidateNoErrors(t, filepath.Join(ourDir, "playlist.m3u8"))
}

func TestHLSConformance_SpeedValidation(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	tmpRoot := t.TempDir()
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInput(t, tmpRoot, 5, "h264", true)

	muxThroughOurHLS(t, inputPath, ourDir, "mpegts")

	playlistPath := filepath.Join(ourDir, "playlist.m3u8")
	totalDuration := ffprobePlaylistDuration(t, playlistPath)

	if totalDuration > 0 {
		t.Logf("ffprobe measured total duration: %.3fs (expected ~5s)", totalDuration)
		if totalDuration < 4.0 || totalDuration > 6.5 {
			t.Errorf("SPEED BUG: ffprobe measured %.3fs for 5s source (expected 4.0-6.5s)", totalDuration)
		}
	}

	playlistData, _ := os.ReadFile(playlistPath)
	_, durations, _ := parseHLSPlaylist(t, string(playlistData))

	var extinfTotal float64
	for _, d := range durations {
		extinfTotal += d
	}
	t.Logf("EXTINF total: %.3fs (expected ~5s)", extinfTotal)

	if math.Abs(extinfTotal-5.0) > 1.0 {
		t.Errorf("SPEED BUG: EXTINF total %.3fs deviates from 5s by more than 1s", extinfTotal)
	}

	segs, _ := filepath.Glob(filepath.Join(ourDir, "seg*.ts"))
	var measuredTotal float64
	for i, seg := range segs {
		measured := ffprobeSegmentDuration(t, seg)
		if measured < 0 {
			continue
		}
		measuredTotal += measured
		t.Logf("segment %d: ffprobe=%.3fs", i, measured)
	}

	if measuredTotal > 0 {
		t.Logf("sum of measured segment durations: %.3fs (expected ~5s)", measuredTotal)
		if measuredTotal < 4.0 || measuredTotal > 7.0 {
			t.Errorf("SPEED BUG: measured total %.3fs for 5s source", measuredTotal)
		}
	}
}

func TestHLSConformance_SpeedValidation_FMP4(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	tmpRoot := t.TempDir()
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInputFormat(t, tmpRoot, 5, "h264", true, "mp4")

	muxThroughOurHLS(t, inputPath, ourDir, "fmp4")

	playlistPath := filepath.Join(ourDir, "playlist.m3u8")
	playlistData, _ := os.ReadFile(playlistPath)
	_, durations, _ := parseHLSPlaylist(t, string(playlistData))

	var extinfTotal float64
	for _, d := range durations {
		extinfTotal += d
	}
	t.Logf("fMP4 EXTINF total: %.3fs (expected ~5s)", extinfTotal)

	if math.Abs(extinfTotal-5.0) > 1.0 {
		t.Errorf("SPEED BUG (fMP4): EXTINF total %.3fs deviates from 5s by more than 1s", extinfTotal)
	}

	ffprobeValidateNoErrors(t, playlistPath)
}

func TestHLSConformance_FMP4_InitSegmentStructure(t *testing.T) {
	skipIfNoFFmpegBinary(t)

	tmpRoot := t.TempDir()
	refDir := filepath.Join(tmpRoot, "ref")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(refDir, 0755)
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInputFormat(t, tmpRoot, 5, "h264", true, "mp4")

	generateHLSReference(t, inputPath, refDir, "fmp4")
	muxThroughOurHLS(t, inputPath, ourDir, "fmp4")

	refInit, err := os.ReadFile(filepath.Join(refDir, "init.mp4"))
	if err != nil {
		t.Fatalf("read ref init.mp4: %v", err)
	}
	ourInit, err := os.ReadFile(filepath.Join(ourDir, "init.mp4"))
	if err != nil {
		t.Fatalf("read our init.mp4: %v", err)
	}

	refBoxes := parseFMP4Boxes(refInit)
	ourBoxes := parseFMP4Boxes(ourInit)

	t.Logf("reference init boxes: %v (%d bytes)", refBoxes, len(refInit))
	t.Logf("our init boxes: %v (%d bytes)", ourBoxes, len(ourInit))

	for _, required := range []string{"ftyp", "moov"} {
		found := false
		for _, b := range ourBoxes {
			if b == required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("our init.mp4 missing required %s box (reference has it)", required)
		}
	}

	if containsFMP4Box(refInit, "avc1") && !containsFMP4Box(ourInit, "avc1") {
		t.Error("our init.mp4 missing avc1 codec box (reference has it)")
	}
	if containsFMP4Box(refInit, "avcC") && !containsFMP4Box(ourInit, "avcC") {
		t.Error("our init.mp4 missing avcC config box (reference has it)")
	}
	if containsFMP4Box(refInit, "mp4a") && !containsFMP4Box(ourInit, "mp4a") {
		t.Error("our init.mp4 missing mp4a codec box (reference has it)")
	}
}

func TestHLSConformance_FMP4_SegmentBoxStructure(t *testing.T) {
	skipIfNoFFmpegBinary(t)

	tmpRoot := t.TempDir()
	refDir := filepath.Join(tmpRoot, "ref")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(refDir, 0755)
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInputFormat(t, tmpRoot, 5, "h264", true, "mp4")

	generateHLSReference(t, inputPath, refDir, "fmp4")
	muxThroughOurHLS(t, inputPath, ourDir, "fmp4")

	refSegs, _ := filepath.Glob(filepath.Join(refDir, "seg*.m4s"))
	ourSegs, _ := filepath.Glob(filepath.Join(ourDir, "seg*.m4s"))

	t.Logf("segment count: ref=%d ours=%d", len(refSegs), len(ourSegs))

	if len(refSegs) > 0 {
		refData, _ := os.ReadFile(refSegs[0])
		refBoxes := parseFMP4Boxes(refData)
		t.Logf("reference first segment boxes: %v", refBoxes)

		if !containsFMP4Box(refData, "moof") {
			t.Error("reference segment missing moof")
		}
		if !containsFMP4Box(refData, "mdat") {
			t.Error("reference segment missing mdat")
		}
	}

	for _, seg := range ourSegs {
		data, _ := os.ReadFile(seg)
		boxes := parseFMP4Boxes(data)

		hasMoof := false
		hasMdat := false
		for _, b := range boxes {
			if b == "moof" {
				hasMoof = true
			}
			if b == "mdat" {
				hasMdat = true
			}
		}

		if !hasMoof {
			t.Errorf("our segment %s missing moof box", filepath.Base(seg))
		}
		if !hasMdat {
			t.Errorf("our segment %s missing mdat box", filepath.Base(seg))
		}

		if containsFMP4Box(data, "tfdt") {
			t.Logf("segment %s has tfdt (decode time) box", filepath.Base(seg))
		}

		t.Logf("segment %s: %d bytes, boxes=%v", filepath.Base(seg), len(data), boxes)
	}
}

func TestHLSConformance_PlaylistContainsMapForFMP4(t *testing.T) {
	skipIfNoFFmpegBinary(t)

	tmpRoot := t.TempDir()
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInputFormat(t, tmpRoot, 5, "h264", true, "mp4")
	muxThroughOurHLS(t, inputPath, ourDir, "fmp4")

	playlistData, err := os.ReadFile(filepath.Join(ourDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read playlist: %v", err)
	}

	content := string(playlistData)
	t.Logf("fMP4 playlist:\n%s", content)

	if !strings.Contains(content, "#EXT-X-MAP:") {
		t.Error("fMP4 playlist missing #EXT-X-MAP tag (required for fMP4 HLS)")
	}

	if !strings.Contains(content, "init.mp4") {
		t.Error("fMP4 playlist does not reference init.mp4")
	}

	if !strings.Contains(content, ".m4s") {
		t.Error("fMP4 playlist does not reference .m4s segments")
	}

	if strings.Contains(content, ".ts") && !strings.Contains(content, "#EXT-X-VERSION") {
		t.Log("fMP4 playlist unexpectedly references .ts segments")
	}
}

func TestHLSConformance_TargetDurationCeiling(t *testing.T) {
	skipIfNoFFmpegBinary(t)

	for _, segType := range []string{"mpegts", "fmp4"} {
		t.Run(segType, func(t *testing.T) {
			tmpRoot := t.TempDir()
			ourDir := filepath.Join(tmpRoot, "ours")
			os.MkdirAll(ourDir, 0755)

			inputFormat := "mpegts"
			if segType == "fmp4" {
				inputFormat = "mp4"
			}
			inputPath := generateTestInputFormat(t, tmpRoot, 10, "h264", true, inputFormat)
			muxThroughOurHLS(t, inputPath, ourDir, segType)

			playlistData, _ := os.ReadFile(filepath.Join(ourDir, "playlist.m3u8"))
			_, durations, targetDur := parseHLSPlaylist(t, string(playlistData))

			if targetDur <= 0 || len(durations) == 0 {
				t.Skip("no segments or target duration")
			}

			var maxDuration float64
			for _, d := range durations {
				if d > maxDuration {
					maxDuration = d
				}
			}

			expectedTarget := int(math.Ceil(maxDuration))
			if targetDur < expectedTarget {
				t.Errorf("targetduration %d is less than ceiling of max segment duration %.3fs (expected >= %d)",
					targetDur, maxDuration, expectedTarget)
			}

			t.Logf("targetduration=%d maxSegDur=%.3fs ceil=%d", targetDur, maxDuration, expectedTarget)
		})
	}
}

func TestHLSConformance_FrameratePreserved(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	tmpRoot := t.TempDir()
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInput(t, tmpRoot, 5, "h264", false)
	muxThroughOurHLS(t, inputPath, ourDir, "mpegts")

	segs, _ := filepath.Glob(filepath.Join(ourDir, "seg*.ts"))
	if len(segs) == 0 {
		t.Fatal("no segments produced")
	}

	fps := ffprobeFrameRate(t, segs[0])
	if fps > 0 {
		if math.Abs(fps-25.0) > 1.0 {
			t.Errorf("framerate %.2f deviates from source 25fps", fps)
		} else {
			t.Logf("framerate: %.2f (expected 25)", fps)
		}
	}
}

func TestHLSConformance_FMP4_Reset(t *testing.T) {
	skipIfNoFFmpegBinary(t)

	tmpRoot := t.TempDir()
	inputPath := generateTestInputFormat(t, tmpRoot, 5, "h264", true, "mp4")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	fc := astiav.AllocFormatContext()
	if fc == nil {
		t.Fatal("alloc format context")
	}
	defer fc.Free()

	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	if err := fc.OpenInput(inputPath, nil, inputDict); err != nil {
		t.Fatalf("open input: %v", err)
	}
	defer fc.CloseInput()
	fc.FindStreamInfo(nil)

	var videoStream *astiav.Stream
	for _, s := range fc.Streams() {
		if s.CodecParameters().MediaType() == astiav.MediaTypeVideo {
			videoStream = s
			break
		}
	}
	if videoStream == nil {
		t.Fatal("no video stream")
	}

	vcp := videoStream.CodecParameters()
	var videoExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}

	hlsMuxer, err := NewHLSMuxer(HLSMuxOpts{
		OutputDir:          ourDir,
		SegmentDurationSec: 2,
		SegmentType:        "fmp4",
		VideoCodecID:       vcp.CodecID(),
		VideoExtradata:     videoExtradata,
		VideoWidth:         vcp.Width(),
		VideoHeight:        vcp.Height(),
		VideoTimeBase:      videoStream.TimeBase(),
		VideoFrameRate:     25,
	})
	if err != nil {
		t.Fatalf("create muxer: %v", err)
	}

	pkt := astiav.AllocPacket()
	defer pkt.Free()

	count := 0
	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		if pkt.StreamIndex() == videoStream.Index() {
			hlsMuxer.WriteVideoPacket(pkt) //nolint:errcheck
			count++
			if count >= 50 {
				break
			}
		}
		pkt.Unref()
	}

	t.Logf("before reset: segmentCount=%d", hlsMuxer.SegmentCount())

	if err := hlsMuxer.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}

	if hlsMuxer.SegmentCount() != 0 {
		t.Errorf("segment count after reset = %d, want 0", hlsMuxer.SegmentCount())
	}

	m4sFiles, _ := filepath.Glob(filepath.Join(ourDir, "seg*.m4s"))
	if len(m4sFiles) != 0 {
		t.Errorf("found %d .m4s files after reset, want 0", len(m4sFiles))
	}

	hlsMuxer.Close()
}
