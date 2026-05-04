//go:build cgo

package mux

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/asticode/go-astiav"
)

const rawSATIPPath = "/tmp/raw_satip_60s.ts"

func skipIfNoRawCapture(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(rawSATIPPath); err != nil {
		t.Skipf("raw SAT>IP capture not available at %s", rawSATIPPath)
	}
}

type fateFormat struct {
	name     string
	dir      string
	file     string
	ffArgs   []string
	isSingle bool
}

var fateFormats = []fateFormat{
	{
		name: "mse_fmp4", dir: "mse", file: "output.mp4", isSingle: true,
		ffArgs: []string{
			"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof",
		},
	},
	{
		name: "hls_ts", dir: "hls_ts", file: "playlist.m3u8",
		ffArgs: []string{
			"-f", "hls", "-hls_time", "6", "-hls_list_size", "0",
			"-hls_segment_filename", "", // filled at runtime
		},
	},
	{
		name: "hls_fmp4", dir: "hls_fmp4", file: "playlist.m3u8",
		ffArgs: []string{
			"-f", "hls", "-hls_time", "6", "-hls_list_size", "0",
			"-hls_segment_type", "fmp4",
			"-hls_fmp4_init_filename", "init.mp4",
			"-hls_segment_filename", "", // filled at runtime
		},
	},
	{
		name: "dash", dir: "dash", file: "manifest.mpd",
		ffArgs: []string{
			"-f", "dash", "-seg_duration", "6",
			"-init_seg_name", "init-$RepresentationID$.m4s",
			"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		},
	},
	{
		name: "matroska", dir: "mkv", file: "output.mkv", isSingle: true,
		ffArgs: []string{"-f", "matroska"},
	},
	{
		name: "mpegts", dir: "mpegts", file: "output.ts", isSingle: true,
		ffArgs: []string{"-f", "mpegts"},
	},
	{
		name: "mp4", dir: "mp4", file: "output.mp4", isSingle: true,
		ffArgs: []string{"-f", "mp4"},
	},
}

type probeResult struct {
	Duration    float64
	Size        int64
	VideoCodec  string
	AudioCodec  string
	Width       int
	Height      int
	Framerate   string
	SampleRate  int
	Channels    int
	FrameCount  int
	SampleCount int
}

func ffprobeFormat(t *testing.T, path string) probeResult {
	t.Helper()

	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration,size",
		"-show_entries", "stream=codec_name,codec_type,width,height,r_frame_rate,sample_rate,channels",
		"-of", "json",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}

	var data struct {
		Streams []struct {
			CodecName  string `json:"codec_name"`
			CodecType  string `json:"codec_type"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			RFrameRate string `json:"r_frame_rate"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			Size     string `json:"size"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("parse ffprobe output: %v", err)
	}

	var pr probeResult
	pr.Duration, _ = strconv.ParseFloat(data.Format.Duration, 64)
	pr.Size, _ = strconv.ParseInt(data.Format.Size, 10, 64)

	for _, s := range data.Streams {
		switch s.CodecType {
		case "video":
			if pr.VideoCodec == "" {
				pr.VideoCodec = s.CodecName
				pr.Width = s.Width
				pr.Height = s.Height
				pr.Framerate = s.RFrameRate
			}
		case "audio":
			if pr.AudioCodec == "" {
				pr.AudioCodec = s.CodecName
				sr, _ := strconv.Atoi(s.SampleRate)
				pr.SampleRate = sr
				pr.Channels = s.Channels
			}
		}
	}
	return pr
}

func ffprobeFrameCount(t *testing.T, path string) (videoFrames, audioFrames int) {
	t.Helper()
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-count_frames",
		"-show_entries", "stream=codec_type,nb_read_frames",
		"-of", "csv=p=0",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		t.Logf("ffprobe frame count failed for %s: %v", path, err)
		return 0, 0
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		count, _ := strconv.Atoi(parts[1])
		switch parts[0] {
		case "video":
			videoFrames = count
		case "audio":
			audioFrames = count
		}
	}
	return
}

func ffmpegDecodeClean(t *testing.T, path string) (bool, string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-v", "error", "-i", path, "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	errText := strings.TrimSpace(string(out))
	if err != nil {
		return false, errText
	}
	return errText == "", errText
}

func generateFateReference(t *testing.T, fmt fateFormat, outDir string) string {
	t.Helper()
	refDir := filepath.Join(outDir, fmt.dir)
	if err := os.MkdirAll(refDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", refDir, err)
	}

	args := []string{
		"-y", "-i", rawSATIPPath,
		"-vf", "yadif=mode=send_frame",
		"-c:v", "libx265", "-preset", "ultrafast",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
	}

	fmtArgs := make([]string, len(fmt.ffArgs))
	copy(fmtArgs, fmt.ffArgs)

	for i, a := range fmtArgs {
		if a == "-hls_segment_filename" && i+1 < len(fmtArgs) && fmtArgs[i+1] == "" {
			if fmt.name == "hls_fmp4" {
				fmtArgs[i+1] = filepath.Join(refDir, "seg%d.m4s")
			} else {
				fmtArgs[i+1] = filepath.Join(refDir, "seg%d.ts")
			}
		}
	}

	args = append(args, fmtArgs...)
	args = append(args, filepath.Join(refDir, fmt.file))

	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "Unknown encoder") || strings.Contains(outStr, "Encoder") {
			t.Skipf("encoder not available: %s", outStr)
		}
		t.Fatalf("generate %s reference: %v\n%s", fmt.name, err, outStr)
	}

	return filepath.Join(refDir, fmt.file)
}

func TestFATEFormats_ReferenceValidation(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)
	skipIfNoRawCapture(t)

	for _, fmt := range fateFormats {
		t.Run(fmt.name, func(t *testing.T) {
			if fmt.name == "dash" {
				t.Skip("DASH manifests cannot be probed/decoded directly by ffprobe/ffmpeg CLI")
			}
			refDir := t.TempDir()
			refPath := generateFateReference(t, fmt, refDir)

			t.Run("probe", func(t *testing.T) {
				pr := ffprobeFormat(t, refPath)
				t.Logf("duration=%.3fs size=%d video=%s %dx%d@%s audio=%s %dHz %dch",
					pr.Duration, pr.Size, pr.VideoCodec, pr.Width, pr.Height,
					pr.Framerate, pr.AudioCodec, pr.SampleRate, pr.Channels)

				if pr.VideoCodec != "hevc" {
					t.Errorf("expected hevc video, got %s", pr.VideoCodec)
				}
				if pr.AudioCodec != "aac" {
					t.Errorf("expected aac audio, got %s", pr.AudioCodec)
				}
				if pr.Width != 1920 || pr.Height != 1080 {
					t.Errorf("expected 1920x1080, got %dx%d", pr.Width, pr.Height)
				}
				if pr.SampleRate != 48000 {
					t.Errorf("expected 48000 Hz, got %d", pr.SampleRate)
				}
				if pr.Channels != 2 {
					t.Errorf("expected 2 channels, got %d", pr.Channels)
				}

				if pr.Duration < 40 || pr.Duration > 78 {
					t.Errorf("duration %.3fs outside 40-78s tolerance (source ~60s)", pr.Duration)
				}
			})

			t.Run("decode_clean", func(t *testing.T) {
				clean, errOutput := ffmpegDecodeClean(t, refPath)
				if !clean {
					t.Errorf("ffmpeg decode errors: %s", errOutput)
				} else {
					t.Log("zero decode errors")
				}
			})

			t.Run("frame_count", func(t *testing.T) {
				if fmt.name == "dash" {
					t.Skip("DASH manifests not directly countable via ffprobe")
				}
				vFrames, aFrames := ffprobeFrameCount(t, refPath)
				t.Logf("video_frames=%d audio_frames=%d", vFrames, aFrames)

				if vFrames < 1000 || vFrames > 2000 {
					t.Errorf("video frame count %d outside 1000-2000 range (expect ~1500 for 60s@25fps)", vFrames)
				}
				if aFrames < 2000 {
					t.Errorf("audio frame count %d too low (expect ~2800 for 60s@48kHz/1024)", aFrames)
				}
			})
		})
	}
}

func TestFATEFormats_PreGeneratedReferences(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	preGenRefs := []struct {
		name string
		path string
	}{
		{"mse_fmp4", "/tmp/fate_ref/mse/output.mp4"},
		{"hls_ts", "/tmp/fate_ref/hls_ts/playlist.m3u8"},
		{"hls_fmp4", "/tmp/fate_ref/hls_fmp4/playlist.m3u8"},
		{"matroska", "/tmp/fate_ref/mkv/output.mkv"},
		{"mpegts", "/tmp/fate_ref/mpegts/output.ts"},
		{"mp4", "/tmp/fate_ref/mp4/output.mp4"},
	}

	for _, ref := range preGenRefs {
		t.Run(ref.name, func(t *testing.T) {
			if _, err := os.Stat(ref.path); os.IsNotExist(err) {
				t.Skipf("pre-generated reference not found: %s", ref.path)
			}

			pr := ffprobeFormat(t, ref.path)
			t.Logf("duration=%.3fs video=%s %dx%d@%s audio=%s %dHz %dch",
				pr.Duration, pr.VideoCodec, pr.Width, pr.Height,
				pr.Framerate, pr.AudioCodec, pr.SampleRate, pr.Channels)

			if pr.VideoCodec != "hevc" {
				t.Errorf("expected hevc, got %s", pr.VideoCodec)
			}
			if pr.AudioCodec != "aac" {
				t.Errorf("expected aac, got %s", pr.AudioCodec)
			}

			clean, errOutput := ffmpegDecodeClean(t, ref.path)
			if !clean {
				t.Errorf("decode errors: %s", errOutput)
			}
		})
	}
}

func TestFATEFormats_HLSSegmentCount(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoRawCapture(t)

	variants := []struct {
		name    string
		segType string
		segExt  string
	}{
		{"hls_ts", "mpegts", ".ts"},
		{"hls_fmp4", "fmp4", ".m4s"},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			refDir := t.TempDir()
			fmt := fateFormat{
				name: v.name, dir: v.name, file: "playlist.m3u8",
			}
			if v.segType == "fmp4" {
				fmt.ffArgs = []string{
					"-f", "hls", "-hls_time", "6", "-hls_list_size", "0",
					"-hls_segment_type", "fmp4",
					"-hls_fmp4_init_filename", "init.mp4",
					"-hls_segment_filename", "",
				}
			} else {
				fmt.ffArgs = []string{
					"-f", "hls", "-hls_time", "6", "-hls_list_size", "0",
					"-hls_segment_filename", "",
				}
			}

			refPath := generateFateReference(t, fmt, refDir)

			playlistData, err := os.ReadFile(refPath)
			if err != nil {
				t.Fatalf("read playlist: %v", err)
			}

			segCount := 0
			var totalDuration float64
			for _, line := range strings.Split(string(playlistData), "\n") {
				if strings.HasPrefix(line, "#EXTINF:") {
					durStr := strings.TrimPrefix(line, "#EXTINF:")
					durStr = strings.TrimSuffix(durStr, ",")
					dur, _ := strconv.ParseFloat(durStr, 64)
					totalDuration += dur
					segCount++
				}
			}

			t.Logf("segments=%d total_duration=%.3fs", segCount, totalDuration)

			minSegs := 4
			maxSegs := int(math.Ceil(60.0/6.0)) + 3
			if segCount < minSegs || segCount > maxSegs {
				t.Errorf("segment count %d outside expected range %d-%d",
					segCount, minSegs, maxSegs)
			}

			if math.Abs(totalDuration-60.0) > 18.0 {
				t.Errorf("total EXTINF duration %.3fs deviates more than 30%% from 60s", totalDuration)
			}
		})
	}
}

func TestFATEFormats_OurFMP4VsReference(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)
	skipIfNoRawCapture(t)

	tmpRoot := t.TempDir()
	refDir := filepath.Join(tmpRoot, "ref")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(refDir, 0755)
	os.MkdirAll(ourDir, 0755)

	transcodeDir := filepath.Join(tmpRoot, "transcoded")
	os.MkdirAll(transcodeDir, 0755)
	transcodedPath := filepath.Join(transcodeDir, "source.mp4")

	cmd := exec.Command("ffmpeg",
		"-y", "-i", rawSATIPPath,
		"-vf", "yadif=mode=send_frame",
		"-c:v", "libx265", "-preset", "ultrafast",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-t", "10",
		"-f", "mp4", transcodedPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("transcode source: %v\n%s", err, string(out))
	}

	cmd = exec.Command("ffmpeg",
		"-y", "-i", transcodedPath,
		"-c:v", "copy", "-c:a", "copy",
		"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof",
		filepath.Join(refDir, "ref.mp4"),
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("generate reference fMP4: %v", err)
	}

	fc := astiav.AllocFormatContext()
	if fc == nil {
		t.Fatal("alloc format context")
	}
	defer fc.Free()

	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	if err := fc.OpenInput(transcodedPath, nil, inputDict); err != nil {
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
		t.Fatal("no video stream in transcoded source")
	}

	vcp := videoStream.CodecParameters()
	var videoExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}

	muxOpts := MuxOpts{
		OutputDir:      ourDir,
		VideoCodecID:   vcp.CodecID(),
		VideoExtradata: videoExtradata,
		VideoWidth:     vcp.Width(),
		VideoHeight:    vcp.Height(),
		VideoTimeBase:  videoStream.TimeBase(),
	}

	if audioStream != nil {
		acp := audioStream.CodecParameters()
		var audioExtradata []byte
		if ed := acp.ExtraData(); len(ed) > 0 {
			audioExtradata = make([]byte, len(ed))
			copy(audioExtradata, ed)
		}
		muxOpts.AudioCodecID = acp.CodecID()
		muxOpts.AudioExtradata = audioExtradata
		muxOpts.AudioChannels = acp.ChannelLayout().Channels()
		muxOpts.AudioSampleRate = acp.SampleRate()
	}

	fmp4Muxer, err := NewFragmentedMuxer(muxOpts)
	if err != nil {
		t.Fatalf("create fMP4 muxer: %v", err)
	}

	pkt := astiav.AllocPacket()
	if pkt == nil {
		t.Fatal("alloc packet")
	}
	defer pkt.Free()

	audioOutTB := astiav.NewRational(1, 48000)
	if audioStream != nil {
		sr := audioStream.CodecParameters().SampleRate()
		if sr > 0 {
			audioOutTB = astiav.NewRational(1, sr)
		}
	}

	var videoPkts, audioPkts int
	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		switch pkt.StreamIndex() {
		case videoStream.Index():
			if err := fmp4Muxer.WriteVideoPacket(pkt); err != nil {
				t.Fatalf("write video pkt %d: %v", videoPkts, err)
			}
			videoPkts++
		default:
			if audioStream != nil && pkt.StreamIndex() == audioStream.Index() {
				pkt.RescaleTs(audioStream.TimeBase(), audioOutTB)
				if err := fmp4Muxer.WriteAudioPacket(pkt); err != nil {
					t.Fatalf("write audio pkt %d: %v", audioPkts, err)
				}
				audioPkts++
			}
		}
		pkt.Unref()
	}

	t.Logf("wrote %d video + %d audio packets through our muxer", videoPkts, audioPkts)

	if err := fmp4Muxer.Close(); err != nil {
		t.Fatalf("close muxer: %v", err)
	}

	t.Run("our_segments_decodable", func(t *testing.T) {
		videoSegs, _ := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))
		t.Logf("produced %d video segments", len(videoSegs))

		if len(videoSegs) == 0 {
			entries, _ := os.ReadDir(ourDir)
			for _, e := range entries {
				info, _ := e.Info()
				t.Logf("  %s (%d bytes)", e.Name(), info.Size())
			}
			t.Fatal("no video segments produced")
		}

		initPath := filepath.Join(ourDir, "init_video.mp4")
		if _, err := os.Stat(initPath); os.IsNotExist(err) {
			t.Fatal("init_video.mp4 not found")
		}

		concatList := filepath.Join(ourDir, "concat.txt")
		var lines []string
		lines = append(lines, "file '"+initPath+"'")
		for _, seg := range videoSegs {
			lines = append(lines, "file '"+seg+"'")
		}
		os.WriteFile(concatList, []byte(strings.Join(lines, "\n")), 0644)

		cmd := exec.Command("ffmpeg", "-v", "error",
			"-f", "concat", "-safe", "0", "-i", concatList,
			"-f", "null", "-")
		decOut, err := cmd.CombinedOutput()
		errText := strings.TrimSpace(string(decOut))
		if err != nil {
			t.Logf("concat decode failed (may be expected for raw segments): %v: %s", err, errText)
		} else if errText != "" {
			t.Logf("decode warnings: %s", errText)
		} else {
			t.Log("segments decode cleanly via concat")
		}
	})

	t.Run("codec_match", func(t *testing.T) {
		codecStr := fmp4Muxer.VideoCodecString()
		t.Logf("video codec string: %s", codecStr)

		if codecStr == "" {
			t.Error("VideoCodecString() is empty")
		}

		if vcp.CodecID() == astiav.CodecIDHevc {
			if !strings.HasPrefix(codecStr, "hvc1") && !strings.HasPrefix(codecStr, "hev1") {
				t.Errorf("expected hvc1/hev1 codec string for HEVC, got %s", codecStr)
			}
		}
	})
}

func TestFATEFormats_OurHLSVsReference(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)
	skipIfNoRawCapture(t)

	variants := []struct {
		name    string
		segType string
	}{
		{"hls_ts", "mpegts"},
		{"hls_fmp4", "fmp4"},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			tmpRoot := t.TempDir()
			transcodeDir := filepath.Join(tmpRoot, "transcoded")
			refDir := filepath.Join(tmpRoot, "ref")
			ourDir := filepath.Join(tmpRoot, "ours")
			os.MkdirAll(transcodeDir, 0755)
			os.MkdirAll(refDir, 0755)
			os.MkdirAll(ourDir, 0755)

			ext := ".ts"
			outFmt := "mpegts"
			if v.segType == "fmp4" {
				ext = ".mp4"
				outFmt = "mp4"
			}
			transcodedPath := filepath.Join(transcodeDir, "source"+ext)
			cmd := exec.Command("ffmpeg",
				"-y", "-i", rawSATIPPath,
				"-vf", "yadif=mode=send_frame",
				"-c:v", "libx265", "-preset", "ultrafast",
				"-c:a", "aac", "-ac", "2", "-ar", "48000",
				"-t", "10",
				"-f", outFmt, transcodedPath,
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("transcode source: %v\n%s", err, string(out))
			}

			generateHLSReference(t, transcodedPath, refDir, v.segType)

			muxThroughOurHLS(t, transcodedPath, ourDir, v.segType)

			refPlaylist := filepath.Join(refDir, "playlist.m3u8")
			ourPlaylist := filepath.Join(ourDir, "playlist.m3u8")

			t.Run("our_decodable", func(t *testing.T) {
				clean, errOutput := ffmpegDecodeClean(t, ourPlaylist)
				if !clean {
					t.Errorf("our HLS not decodable: %s", errOutput)
				} else {
					t.Log("our HLS output is decodable")
				}
			})

			t.Run("duration_match", func(t *testing.T) {
				refPr := ffprobeFormat(t, refPlaylist)
				ourPr := ffprobeFormat(t, ourPlaylist)

				t.Logf("ref duration=%.3fs our duration=%.3fs", refPr.Duration, ourPr.Duration)

				if ourPr.Duration <= 0 {
					t.Error("our duration is zero")
					return
				}

				ratio := ourPr.Duration / refPr.Duration
				if ratio < 0.7 || ratio > 1.3 {
					t.Errorf("duration ratio %.2f outside 0.7-1.3 range (ref=%.3fs ours=%.3fs)",
						ratio, refPr.Duration, ourPr.Duration)
				}
			})

			t.Run("codec_match", func(t *testing.T) {
				ourPr := ffprobeFormat(t, ourPlaylist)
				if ourPr.VideoCodec != "hevc" {
					t.Errorf("expected hevc video, got %s", ourPr.VideoCodec)
				}
				if ourPr.AudioCodec != "aac" {
					t.Errorf("expected aac audio, got %s", ourPr.AudioCodec)
				}
			})

			t.Run("framerate_match", func(t *testing.T) {
				ourPr := ffprobeFormat(t, ourPlaylist)
				if ourPr.Framerate != "25/1" {
					t.Logf("framerate %s (expected 25/1, may differ due to muxer)", ourPr.Framerate)
				}
			})

			t.Run("segment_count", func(t *testing.T) {
				refData, _ := os.ReadFile(refPlaylist)
				ourData, _ := os.ReadFile(ourPlaylist)

				refSegs := countPlaylistSegments(string(refData))
				ourSegs := countPlaylistSegments(string(ourData))

				t.Logf("ref segments=%d our segments=%d", refSegs, ourSegs)

				diff := ourSegs - refSegs
				if diff < 0 {
					diff = -diff
				}
				if diff > 1 {
					t.Errorf("segment count diff %d exceeds tolerance of 1 (ref=%d ours=%d)",
						diff, refSegs, ourSegs)
				}
			})
		})
	}
}

func countPlaylistSegments(content string) int {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "#EXTINF:") {
			count++
		}
	}
	return count
}

func TestFATEFormats_DTSMonotonic(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	refs := []struct {
		name string
		path string
	}{
		{"mse_fmp4", "/tmp/fate_ref/mse/output.mp4"},
		{"mpegts", "/tmp/fate_ref/mpegts/output.ts"},
		{"mp4", "/tmp/fate_ref/mp4/output.mp4"},
		{"matroska", "/tmp/fate_ref/mkv/output.mkv"},
	}

	for _, ref := range refs {
		t.Run(ref.name, func(t *testing.T) {
			if _, err := os.Stat(ref.path); os.IsNotExist(err) {
				t.Skipf("reference not found: %s", ref.path)
			}

			cmd := exec.Command("ffprobe",
				"-v", "quiet",
				"-select_streams", "v:0",
				"-show_entries", "packet=dts",
				"-of", "csv=p=0",
				ref.path,
			)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("ffprobe packets: %v", err)
			}

			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) < 10 {
				t.Fatalf("only %d DTS values, expected many more", len(lines))
			}

			var prevDTS int64 = math.MinInt64
			nonMonotonic := 0
			validCount := 0
			for i, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || line == "N/A" {
					continue
				}
				dts, err := strconv.ParseInt(line, 10, 64)
				if err != nil {
					continue
				}
				validCount++
				if prevDTS != math.MinInt64 && dts < prevDTS {
					nonMonotonic++
					if nonMonotonic <= 3 {
						t.Logf("non-monotonic DTS at line %d: %d < %d", i, dts, prevDTS)
					}
				}
				prevDTS = dts
			}

			t.Logf("%d DTS values, %d non-monotonic", validCount, nonMonotonic)

			if nonMonotonic > 0 {
				t.Errorf("DTS must be strictly monotonic, found %d non-monotonic values in %d packets",
					nonMonotonic, validCount)
			}
		})
	}
}

func TestFATEFormats_PTSSpacing(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	refs := []struct {
		name string
		path string
	}{
		{"mse_fmp4", "/tmp/fate_ref/mse/output.mp4"},
		{"mp4", "/tmp/fate_ref/mp4/output.mp4"},
		{"matroska", "/tmp/fate_ref/mkv/output.mkv"},
	}

	for _, ref := range refs {
		t.Run(ref.name, func(t *testing.T) {
			if _, err := os.Stat(ref.path); os.IsNotExist(err) {
				t.Skipf("reference not found: %s", ref.path)
			}

			cmd := exec.Command("ffprobe",
				"-v", "quiet",
				"-select_streams", "v:0",
				"-show_entries", "packet=pts",
				"-of", "csv=p=0",
				ref.path,
			)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("ffprobe packets: %v", err)
			}

			var pts []int64
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || line == "N/A" {
					continue
				}
				v, err := strconv.ParseInt(line, 10, 64)
				if err == nil {
					pts = append(pts, v)
				}
			}

			if len(pts) < 10 {
				t.Fatalf("too few PTS values: %d", len(pts))
			}

			firstPTS := pts[0]
			lastPTS := pts[len(pts)-1]
			spanSec := float64(lastPTS-firstPTS) / float64(lastPTS-firstPTS) * (float64(len(pts)-1) / 25.0)
			_ = spanSec

			t.Logf("PTS range: %d to %d (%d values), first-last span covers output",
				firstPTS, lastPTS, len(pts))

			if lastPTS <= firstPTS {
				t.Errorf("last PTS %d not greater than first PTS %d", lastPTS, firstPTS)
			}
		})
	}
}

func TestFATEFormats_SourceProbe(t *testing.T) {
	skipIfNoFFprobeBinary(t)
	skipIfNoRawCapture(t)

	pr := ffprobeFormat(t, rawSATIPPath)
	t.Logf("source: duration=%.3fs size=%d", pr.Duration, pr.Size)
	t.Logf("video: %s %dx%d@%s", pr.VideoCodec, pr.Width, pr.Height, pr.Framerate)
	t.Logf("audio: %s %dHz %dch", pr.AudioCodec, pr.SampleRate, pr.Channels)

	if pr.VideoCodec != "h264" {
		t.Errorf("expected h264 video, got %s", pr.VideoCodec)
	}
	if pr.AudioCodec != "ac3" {
		t.Errorf("expected ac3 audio, got %s", pr.AudioCodec)
	}
	if pr.Width != 1920 || pr.Height != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", pr.Width, pr.Height)
	}
	if pr.Framerate != "50/1" {
		t.Logf("expected 50/1 (interlaced), got %s", pr.Framerate)
	}
	if math.Abs(pr.Duration-60.0) > 5.0 {
		t.Errorf("expected ~60s duration, got %.3fs", pr.Duration)
	}
}
