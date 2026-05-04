package mux_test

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av/conv"
	"github.com/mcnairstudios/mediahub/pkg/av/decode"
	"github.com/mcnairstudios/mediahub/pkg/av/demux"
	"github.com/mcnairstudios/mediahub/pkg/av/encode"
	"github.com/mcnairstudios/mediahub/pkg/av/filter"
	"github.com/mcnairstudios/mediahub/pkg/av/mux"
	"github.com/mcnairstudios/mediahub/pkg/av/resample"
)

const rawSATIPPath = "/tmp/raw_satip_60s.ts"
const refFMP4Path = "/tmp/ref_fmp4_60/output.mp4"

func skipIfNoRawCapture(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(rawSATIPPath); err != nil {
		t.Skipf("raw SAT>IP capture not available at %s", rawSATIPPath)
	}
}

func skipIfNoReference(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(refFMP4Path); err != nil {
		t.Skipf("reference fMP4 not available at %s", refFMP4Path)
	}
}

func skipIfNoFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
}

func skipIfNoFFprobe(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

type fateProbeResult struct {
	Duration   float64
	VideoCodec string
	AudioCodec string
	Framerate  string
	Width      int
	Height     int
}

func fateFFprobeFormat(t *testing.T, path string) fateProbeResult {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "format=duration",
		"-show_entries", "stream=codec_name,codec_type,r_frame_rate,width,height",
		"-of", "flat", path).Output()
	if err != nil {
		t.Fatalf("ffprobe failed on %s: %v", path, err)
	}

	var pr fateProbeResult
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := strings.Trim(parts[1], "\"")

		switch {
		case key == "format.duration":
			pr.Duration, _ = strconv.ParseFloat(val, 64)
		case strings.HasSuffix(key, "codec_name") && pr.VideoCodec == "":
			ct := ""
			for _, l2 := range strings.Split(string(out), "\n") {
				if strings.Contains(l2, strings.Replace(key, "codec_name", "codec_type", 1)) {
					ct = strings.Trim(strings.SplitN(l2, "=", 2)[1], "\"")
				}
			}
			if ct == "" || ct == "video" {
				pr.VideoCodec = val
			}
		}
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := strings.Trim(parts[1], "\"")

		switch {
		case strings.HasSuffix(key, "codec_type"):
			// handled above
		case strings.HasSuffix(key, "codec_name"):
			// find the codec_type for this stream index
			prefix := strings.TrimSuffix(key, "codec_name")
			typeKey := prefix + "codec_type"
			for _, l2 := range strings.Split(string(out), "\n") {
				l2 = strings.TrimSpace(l2)
				if strings.HasPrefix(l2, typeKey) {
					tp := strings.Trim(strings.SplitN(l2, "=", 2)[1], "\"")
					if tp == "audio" && pr.AudioCodec == "" {
						pr.AudioCodec = val
					}
					if tp == "video" {
						pr.VideoCodec = val
					}
				}
			}
		case strings.HasSuffix(key, "r_frame_rate"):
			if pr.Framerate == "" {
				pr.Framerate = val
			}
		case strings.HasSuffix(key, "width"):
			if pr.Width == 0 {
				pr.Width, _ = strconv.Atoi(val)
			}
		case strings.HasSuffix(key, "height"):
			if pr.Height == 0 {
				pr.Height, _ = strconv.Atoi(val)
			}
		}
	}
	return pr
}

func fateProbePTS(t *testing.T, path string, streamType string) []int64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet", "-show_packets",
		"-show_entries", "packet=pts,codec_type",
		"-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("ffprobe packets failed: %v", err)
	}
	var pts []int64
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Split(strings.TrimSpace(scanner.Text()), ",")
		if len(fields) < 2 || fields[0] != streamType {
			continue
		}
		if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
			pts = append(pts, v)
		}
	}
	return pts
}

func fateProbeDTS(t *testing.T, path string, streamType string) []int64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet", "-show_packets",
		"-show_entries", "packet=dts,codec_type",
		"-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("ffprobe packets failed: %v", err)
	}
	var dts []int64
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Split(strings.TrimSpace(scanner.Text()), ",")
		if len(fields) < 2 || fields[0] != streamType {
			continue
		}
		if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
			dts = append(dts, v)
		}
	}
	return dts
}

type fatePipelineResult struct {
	outDir      string
	segDir      string
	videoFrames int
	audioFrames int
	videoSegs   []string
	audioSegs   []string
}

func runFATEPipeline(t *testing.T, inputPath string, maxPackets int) fatePipelineResult {
	t.Helper()

	astiav.SetLogLevel(astiav.LogLevelWarning)

	outDir := t.TempDir()
	segDir := filepath.Join(outDir, "segments")
	os.MkdirAll(segDir, 0755)

	dm, err := demux.NewDemuxer(inputPath, demux.DemuxOpts{
		ProbeSize:       5000000,
		AnalyzeDuration: 2000000,
	})
	if err != nil {
		t.Fatalf("demuxer: %v", err)
	}
	defer dm.Close()

	info := dm.StreamInfo()
	if info == nil || info.Video == nil {
		t.Fatal("no video stream info")
	}

	videoCodecID, err := conv.CodecIDFromString(info.Video.Codec)
	if err != nil {
		t.Fatalf("video codec: %v", err)
	}
	videoDec, err := decode.NewVideoDecoder(videoCodecID, info.Video.Extradata, decode.DecodeOpts{})
	if err != nil {
		t.Fatalf("video decoder: %v", err)
	}
	defer videoDec.Close()

	deint, err := filter.NewDeinterlacer(
		info.Video.Width, info.Video.Height,
		astiav.PixelFormatYuv420P,
		astiav.NewRational(1, 90000),
	)
	if err != nil {
		t.Fatalf("deinterlacer: %v", err)
	}
	defer deint.Close()

	videoEnc, err := encode.NewVideoEncoder(encode.EncodeOpts{
		Codec:     "h265",
		HWAccel:   "none",
		Width:     info.Video.Width,
		Height:    info.Video.Height,
		Framerate: 25,
		Preset:    "ultrafast",
	})
	if err != nil {
		t.Fatalf("video encoder: %v", err)
	}
	defer videoEnc.Close()

	var audioDec *decode.Decoder
	var audioResamp *resample.Resampler
	var audioEnc *encode.Encoder
	hasAudio := false

	if len(info.AudioTracks) > 0 {
		a := info.AudioTracks[0]
		audioCodecID, cerr := conv.CodecIDFromString(a.Codec)
		if cerr != nil {
			t.Fatalf("audio codec: %v", cerr)
		}
		audioDec, err = decode.NewAudioDecoder(audioCodecID, nil)
		if err != nil {
			t.Fatalf("audio decoder: %v", err)
		}
		defer audioDec.Close()

		audioResamp, err = resample.NewResampler(
			a.Channels, a.SampleRate, astiav.SampleFormatFltp,
			2, 48000, astiav.SampleFormatFltp,
		)
		if err != nil {
			t.Fatalf("resampler: %v", err)
		}
		defer audioResamp.Close()

		audioEnc, err = encode.NewAACEncoder(2, 48000)
		if err != nil {
			t.Fatalf("audio encoder: %v", err)
		}
		defer audioEnc.Close()
		hasAudio = true
	}

	outVideoCodecID, _ := conv.CodecIDFromString("h265")
	muxOpts := mux.MuxOpts{
		OutputDir:     segDir,
		VideoCodecID:  outVideoCodecID,
		VideoWidth:    info.Video.Width,
		VideoHeight:   info.Video.Height,
		VideoTimeBase: astiav.NewRational(1, 90000),
	}
	if ed := videoEnc.Extradata(); len(ed) > 0 {
		muxOpts.VideoExtradata = make([]byte, len(ed))
		copy(muxOpts.VideoExtradata, ed)
	}
	if hasAudio {
		outAudioCodecID, _ := conv.CodecIDFromString("aac")
		muxOpts.AudioCodecID = outAudioCodecID
		muxOpts.AudioChannels = 2
		muxOpts.AudioSampleRate = 48000
		if ed := audioEnc.Extradata(); len(ed) > 0 {
			muxOpts.AudioExtradata = make([]byte, len(ed))
			copy(muxOpts.AudioExtradata, ed)
		}
	}

	fmuxer, err := mux.NewFragmentedMuxer(muxOpts)
	if err != nil {
		t.Fatalf("muxer: %v", err)
	}

	videoTB := astiav.NewRational(1, 90000)
	audioTB := astiav.NewRational(1, 48000)

	var res fatePipelineResult
	res.outDir = outDir
	res.segDir = segDir

	pktCount := 0
	for {
		if maxPackets > 0 && pktCount >= maxPackets {
			break
		}

		pkt, err := dm.ReadPacket()
		if err != nil {
			break
		}
		pktCount++

		if pkt.Type == av.Video {
			avPkt, cerr := conv.ToAVPacket(pkt, videoTB)
			if cerr != nil {
				continue
			}

			frames, derr := videoDec.Decode(avPkt)
			avPkt.Free()
			if derr != nil {
				continue
			}

			for _, frame := range frames {
				deintFrame, ferr := deint.Process(frame)
				frame.Free()
				if ferr != nil || deintFrame == nil {
					continue
				}

				encPkts, eerr := videoEnc.Encode(deintFrame)
				deintFrame.Free()
				if eerr != nil {
					continue
				}

				for _, ep := range encPkts {
					fmuxer.WriteVideoPacket(ep)
					ep.Free()
					res.videoFrames++
				}
			}
		} else if pkt.Type == av.Audio && hasAudio {
			avPkt, cerr := conv.ToAVPacket(pkt, audioTB)
			if cerr != nil {
				continue
			}

			frames, derr := audioDec.Decode(avPkt)
			avPkt.Free()
			if derr != nil {
				continue
			}

			for _, frame := range frames {
				resampFrame, rerr := audioResamp.Convert(frame)
				frame.Free()
				if rerr != nil || resampFrame == nil {
					continue
				}

				encPkts, eerr := audioEnc.Encode(resampFrame)
				resampFrame.Free()
				if eerr != nil {
					continue
				}

				for _, ep := range encPkts {
					fmuxer.WriteAudioPacket(ep)
					ep.Free()
					res.audioFrames++
				}
			}
		}
	}

	fmuxer.Close()

	res.videoSegs, _ = filepath.Glob(filepath.Join(segDir, "video_*.m4s"))
	res.audioSegs, _ = filepath.Glob(filepath.Join(segDir, "audio_*.m4s"))

	return res
}

func concatFATEVideoSegments(t *testing.T, segDir string, n int) string {
	t.Helper()
	initPath := filepath.Join(segDir, "init_video.mp4")
	if _, err := os.Stat(initPath); err != nil {
		t.Fatalf("init_video.mp4 not found in %s", segDir)
	}

	var parts []string
	parts = append(parts, initPath)
	for i := 1; i <= n; i++ {
		seg := filepath.Join(segDir, fmt.Sprintf("video_%04d.m4s", i))
		if _, err := os.Stat(seg); err != nil {
			break
		}
		parts = append(parts, seg)
	}

	if len(parts) < 2 {
		t.Fatalf("need at least init + 1 segment, got %d files", len(parts))
	}

	outPath := filepath.Join(segDir, "concat.mp4")
	cmd := exec.Command("sh", "-c", "cat "+strings.Join(parts, " ")+" > "+outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("concat segments: %v: %s", err, string(out))
	}
	return outPath
}

func concatAllFATEVideoSegments(t *testing.T, segDir string) string {
	t.Helper()
	segs, _ := filepath.Glob(filepath.Join(segDir, "video_*.m4s"))
	return concatFATEVideoSegments(t, segDir, len(segs))
}

func TestFATE_Pipeline_ReferenceIsValid(t *testing.T) {
	skipIfNoFFmpeg(t)
	skipIfNoFFprobe(t)
	skipIfNoRawCapture(t)
	skipIfNoReference(t)

	pr := fateFFprobeFormat(t, refFMP4Path)
	t.Logf("reference duration: %.2fs", pr.Duration)
	if pr.Duration < 50 || pr.Duration > 70 {
		t.Errorf("reference duration %.2fs not in expected range [50,70]", pr.Duration)
	}
	if pr.VideoCodec != "hevc" {
		t.Errorf("expected hevc video, got %s", pr.VideoCodec)
	}
	if pr.AudioCodec != "aac" {
		t.Errorf("expected aac audio, got %s", pr.AudioCodec)
	}
	if pr.Framerate != "25/1" {
		t.Errorf("expected 25/1 fps, got %s", pr.Framerate)
	}
	t.Logf("reference: codec=%s fps=%s %dx%d audio=%s",
		pr.VideoCodec, pr.Framerate, pr.Width, pr.Height, pr.AudioCodec)
}

func TestFATE_Pipeline_OurDurationMatchesReference(t *testing.T) {
	skipIfNoFFmpeg(t)
	skipIfNoFFprobe(t)
	skipIfNoRawCapture(t)
	skipIfNoReference(t)

	res := runFATEPipeline(t, rawSATIPPath, 5000)
	t.Logf("pipeline: %d video frames, %d audio frames", res.videoFrames, res.audioFrames)
	t.Logf("segments: %d video, %d audio", len(res.videoSegs), len(res.audioSegs))

	if len(res.videoSegs) < 3 {
		t.Fatalf("need at least 3 video segments, got %d", len(res.videoSegs))
	}

	ourConcat := concatFATEVideoSegments(t, res.segDir, 3)
	ourPr := fateFFprobeFormat(t, ourConcat)
	t.Logf("our 3-segment duration: %.2fs", ourPr.Duration)

	refConcat := filepath.Join(t.TempDir(), "ref3.mp4")
	refCmd := exec.Command("ffmpeg", "-v", "quiet", "-i", refFMP4Path,
		"-t", fmt.Sprintf("%.1f", ourPr.Duration*1.5),
		"-c", "copy", "-y", refConcat)
	if out, rerr := refCmd.CombinedOutput(); rerr != nil {
		t.Logf("ref trim: %v: %s (using full reference)", rerr, string(out))
		refConcat = refFMP4Path
	}
	refPr := fateFFprobeFormat(t, refConcat)
	t.Logf("reference comparison duration: %.2fs", refPr.Duration)

	if ourPr.Duration <= 0 {
		t.Fatal("our duration is zero or negative")
	}
	if ourPr.Duration > 1000 {
		t.Fatalf("our duration is absurdly large: %.2fs (PTS overflow?)", ourPr.Duration)
	}

	if refPr.Duration > 0 {
		ratio := ourPr.Duration / refPr.Duration
		if ratio < 0.1 || ratio > 3.0 {
			t.Errorf("duration ratio %.2f out of range [0.1, 3.0]: ours=%.2fs ref=%.2fs",
				ratio, ourPr.Duration, refPr.Duration)
		} else {
			t.Logf("duration ratio: %.2f (acceptable)", ratio)
		}
	}
}

func TestFATE_Pipeline_FramerateCorrect(t *testing.T) {
	skipIfNoFFmpeg(t)
	skipIfNoFFprobe(t)
	skipIfNoRawCapture(t)

	res := runFATEPipeline(t, rawSATIPPath, 3000)
	if len(res.videoSegs) < 1 {
		t.Fatal("no video segments produced")
	}

	n := len(res.videoSegs)
	if n > 3 {
		n = 3
	}
	ourConcat := concatFATEVideoSegments(t, res.segDir, n)
	pr := fateFFprobeFormat(t, ourConcat)
	t.Logf("our framerate: %s", pr.Framerate)

	badRates := []string{"90000/1", "25/2", "50/1", "90000/3600"}
	for _, bad := range badRates {
		if pr.Framerate == bad {
			t.Errorf("framerate is %s (known bad value)", pr.Framerate)
		}
	}

	if pr.Framerate != "25/1" {
		parts := strings.Split(pr.Framerate, "/")
		if len(parts) == 2 {
			num, _ := strconv.ParseFloat(parts[0], 64)
			den, _ := strconv.ParseFloat(parts[1], 64)
			if den > 0 {
				actual := num / den
				if actual < 20 || actual > 30 {
					t.Errorf("framerate %.2f fps (from %s) outside 20-30 range", actual, pr.Framerate)
				} else {
					t.Logf("framerate %.2f fps (acceptable)", actual)
				}
			}
		}
	}
}

func TestFATE_Pipeline_DecodableZeroErrors(t *testing.T) {
	skipIfNoFFmpeg(t)
	skipIfNoFFprobe(t)
	skipIfNoRawCapture(t)

	res := runFATEPipeline(t, rawSATIPPath, 3000)
	if len(res.videoSegs) < 1 {
		t.Fatal("no video segments produced")
	}

	n := len(res.videoSegs)
	if n > 3 {
		n = 3
	}
	ourConcat := concatFATEVideoSegments(t, res.segDir, n)

	cmd := exec.Command("ffmpeg", "-v", "error", "-i", ourConcat, "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	errText := strings.TrimSpace(string(out))

	if err != nil {
		t.Fatalf("ffmpeg decode failed: %v\n%s", err, errText)
	}
	if errText != "" {
		t.Errorf("ffmpeg reported errors:\n%s", errText)
	} else {
		t.Log("zero decode errors")
	}
}

func TestFATE_Pipeline_PTSMonotonic(t *testing.T) {
	skipIfNoFFmpeg(t)
	skipIfNoFFprobe(t)
	skipIfNoRawCapture(t)

	res := runFATEPipeline(t, rawSATIPPath, 3000)
	if len(res.videoSegs) < 1 {
		t.Fatal("no video segments produced")
	}

	n := len(res.videoSegs)
	if n > 3 {
		n = 3
	}
	ourConcat := concatFATEVideoSegments(t, res.segDir, n)

	dts := fateProbeDTS(t, ourConcat, "video")
	if len(dts) < 10 {
		t.Fatalf("only %d video DTS values", len(dts))
	}
	t.Logf("DTS: %d values, first=%d last=%d", len(dts), dts[0], dts[len(dts)-1])

	nonMonoDTS := 0
	for i := 1; i < len(dts); i++ {
		if dts[i] < dts[i-1] {
			nonMonoDTS++
			if nonMonoDTS <= 5 {
				t.Logf("non-monotonic DTS at %d: %d < %d", i, dts[i], dts[i-1])
			}
		}
	}
	if nonMonoDTS > 0 {
		t.Errorf("%d/%d DTS values non-monotonic", nonMonoDTS, len(dts)-1)
	} else {
		t.Log("DTS monotonically increasing")
	}

	pts := fateProbePTS(t, ourConcat, "video")
	if len(pts) < 10 {
		t.Fatalf("only %d video PTS values", len(pts))
	}
	t.Logf("PTS: %d values, first=%d last=%d", len(pts), pts[0], pts[len(pts)-1])

	ptsNonMono := 0
	for i := 1; i < len(pts); i++ {
		if pts[i] <= pts[i-1] {
			ptsNonMono++
		}
	}
	if ptsNonMono > 0 {
		t.Logf("PTS non-monotonic: %d/%d (expected with B-frames)", ptsNonMono, len(pts)-1)
	}

	totalSpan := pts[len(pts)-1] - pts[0]
	if totalSpan <= 0 {
		t.Errorf("PTS span non-positive: first=%d last=%d", pts[0], pts[len(pts)-1])
	} else {
		avgSpacing := float64(totalSpan) / float64(len(pts)-1)
		t.Logf("average PTS spacing: %.1f (span=%d, %d packets)", avgSpacing, totalSpan, len(pts))

		ratio := avgSpacing / 3600.0
		if ratio < 0.5 || ratio > 2.0 {
			t.Errorf("PTS spacing %.1f far from 3600 (1/25s@90kHz), ratio=%.2f", avgSpacing, ratio)
		} else {
			t.Logf("PTS spacing ratio to 3600: %.2f (acceptable)", ratio)
		}
	}
}

func TestFATE_Pipeline_VideoAudioSegmentRatio(t *testing.T) {
	skipIfNoFFmpeg(t)
	skipIfNoRawCapture(t)

	res := runFATEPipeline(t, rawSATIPPath, 5000)
	t.Logf("segments: %d video, %d audio", len(res.videoSegs), len(res.audioSegs))

	if len(res.videoSegs) == 0 {
		t.Fatal("no video segments")
	}
	if len(res.audioSegs) == 0 {
		t.Fatal("no audio segments")
	}

	vf := float64(len(res.videoSegs))
	af := float64(len(res.audioSegs))
	ratio := vf / af
	if ratio < 1 {
		ratio = af / vf
	}
	t.Logf("segment ratio: %.2f", ratio)

	if ratio > 5.0 {
		t.Errorf("video/audio segment ratio %.2f exceeds 5.0 (broken timing?)", ratio)
	}
}

func TestFATE_Pipeline_SegmentDurationReasonable(t *testing.T) {
	skipIfNoFFmpeg(t)
	skipIfNoFFprobe(t)
	skipIfNoRawCapture(t)

	res := runFATEPipeline(t, rawSATIPPath, 3000)
	if len(res.videoSegs) < 1 {
		t.Fatal("no video segments")
	}

	initPath := filepath.Join(res.segDir, "init_video.mp4")
	checkCount := len(res.videoSegs)
	if checkCount > 3 {
		checkCount = 3
	}

	for i := 1; i <= checkCount; i++ {
		segPath := filepath.Join(res.segDir, fmt.Sprintf("video_%04d.m4s", i))
		concatPath := filepath.Join(res.segDir, fmt.Sprintf("check_%04d.mp4", i))

		cmd := exec.Command("sh", "-c", fmt.Sprintf("cat %s %s > %s", initPath, segPath, concatPath))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Logf("segment %d concat: %v: %s", i, err, string(out))
			continue
		}

		pr := fateFFprobeFormat(t, concatPath)
		t.Logf("segment %d duration: %.3fs", i, pr.Duration)

		if pr.Duration < 0.5 {
			t.Errorf("segment %d duration %.3fs too short (< 0.5s)", i, pr.Duration)
		}
		if pr.Duration > 30 {
			t.Errorf("segment %d duration %.3fs too long (> 30s)", i, pr.Duration)
		}
	}
}

func TestFATE_Pipeline_OverflowSafeRescale(t *testing.T) {
	safeRescale := func(v int64, num, den int64) int64 {
		if den == 0 {
			return 0
		}
		return (v/den)*num + (v%den)*num/den
	}

	tests := []struct {
		name     string
		v, num   int64
		den      int64
		expected int64
	}{
		{
			name: "normal 40ms", v: 3600,
			num: 1_000_000_000, den: 90000,
			expected: 40_000_000,
		},
		{
			name: "100s", v: 9_000_000,
			num: 1_000_000_000, den: 90000,
			expected: 100_000_000_000,
		},
		{
			name: "large value no overflow", v: 900_000_000_000,
			num: 1_000_000_000, den: 90000,
			expected: 10_000_000_000_000_000,
		},
		{
			name: "zero denominator", v: 1000,
			num: 1, den: 0,
			expected: 0,
		},
		{
			name: "zero value", v: 0,
			num: 1_000_000_000, den: 90000,
			expected: 0,
		},
		{
			name: "negative PTS", v: -3600,
			num: 1_000_000_000, den: 90000,
			expected: -40_000_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeRescale(tt.v, tt.num, tt.den)
			if tt.expected != 0 {
				diff := math.Abs(float64(got - tt.expected))
				tol := math.Abs(float64(tt.expected)) * 0.001
				if tol < 1 {
					tol = 1
				}
				if diff > tol {
					t.Errorf("safeRescale(%d, %d, %d) = %d, want %d",
						tt.v, tt.num, tt.den, got, tt.expected)
				}
			} else if got != 0 {
				t.Errorf("safeRescale(%d, %d, %d) = %d, want 0", tt.v, tt.num, tt.den, got)
			}
		})
	}

	t.Run("overflow safety", func(t *testing.T) {
		result := safeRescale(900_000_000_000, 1_000_000_000, 90000)
		if result <= 0 {
			t.Errorf("overflow: safeRescale(900B, 1B, 90K) = %d (should be positive)", result)
		}
		t.Logf("safeRescale(900_000_000_000, 1_000_000_000, 90000) = %d", result)
	})
}

func TestFATE_Pipeline_NanosConversionRoundTrip(t *testing.T) {
	avTSToNanos := func(ts int64, tbNum, tbDen int64) int64 {
		if tbDen == 0 {
			return 0
		}
		return (ts/tbDen)*(1_000_000_000*tbNum) + (ts%tbDen)*(1_000_000_000*tbNum)/tbDen
	}

	nanosToAVTS := func(nanos int64, tbNum, tbDen int64) int64 {
		if tbNum == 0 {
			return 0
		}
		d := 1_000_000_000 * tbNum
		return (nanos/d)*tbDen + (nanos%d)*tbDen/d
	}

	tests := []struct {
		name  string
		pts   int64
		tbNum int64
		tbDen int64
	}{
		{"video 1/90000 small", 3600, 1, 90000},
		{"video 1/90000 medium", 900000, 1, 90000},
		{"video 1/90000 large", 90000000, 1, 90000},
		{"audio 1/48000 small", 1024, 1, 48000},
		{"audio 1/48000 medium", 480000, 1, 48000},
		{"audio 1/48000 large", 48000000, 1, 48000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nanos := avTSToNanos(tt.pts, tt.tbNum, tt.tbDen)
			roundTrip := nanosToAVTS(nanos, tt.tbNum, tt.tbDen)

			diff := tt.pts - roundTrip
			if diff < 0 {
				diff = -diff
			}

			t.Logf("pts=%d -> nanos=%d -> roundtrip=%d (diff=%d)", tt.pts, nanos, roundTrip, diff)

			if diff > 1 {
				t.Errorf("round-trip loss: %d -> %d -> %d (diff=%d)", tt.pts, nanos, roundTrip, diff)
			}
		})
	}
}

func TestFATE_Pipeline_60sPlayback(t *testing.T) {
	skipIfNoFFmpeg(t)
	skipIfNoFFprobe(t)
	skipIfNoRawCapture(t)

	if testing.Short() {
		t.Skip("full 60s pipeline test skipped in short mode")
	}

	res := runFATEPipeline(t, rawSATIPPath, 0)
	t.Logf("full pipeline: %d video, %d audio frames", res.videoFrames, res.audioFrames)
	t.Logf("segments: %d video, %d audio", len(res.videoSegs), len(res.audioSegs))

	if len(res.videoSegs) < 3 {
		t.Fatalf("expected 3+ video segments for 60s, got %d", len(res.videoSegs))
	}

	ourConcat := concatAllFATEVideoSegments(t, res.segDir)
	pr := fateFFprobeFormat(t, ourConcat)
	t.Logf("full output duration: %.2fs", pr.Duration)

	if pr.Duration < 55 || pr.Duration > 65 {
		t.Errorf("duration %.2fs not in [55,65] for 60s source", pr.Duration)
	}

	if pr.Duration > 100 {
		t.Errorf("duration %.2fs wildly wrong (PTS overflow?)", pr.Duration)
	}
}
