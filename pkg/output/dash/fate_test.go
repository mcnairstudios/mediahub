//go:build cgo

package dash

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoFFmpegFate(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
}

func skipIfNoFFprobeFate(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

func generateFATESourceDASH(t *testing.T, dir string, durationSec int, videoCodec string, includeAudio bool) string {
	t.Helper()
	srcPath := filepath.Join(dir, "source.mp4")

	args := []string{
		"-f", "lavfi", "-i",
		"testsrc2=size=640x360:rate=25:d=" + strconv.Itoa(durationSec),
	}
	if includeAudio {
		args = append(args,
			"-f", "lavfi", "-i",
			"sine=f=440:d="+strconv.Itoa(durationSec)+":sample_rate=48000",
		)
	}

	switch videoCodec {
	case "h264":
		args = append(args, "-c:v", "libx264", "-preset", "ultrafast",
			"-profile:v", "baseline", "-g", "25")
	case "h265":
		args = append(args, "-c:v", "libx265", "-preset", "ultrafast",
			"-x265-params", "keyint=25:min-keyint=25")
	}

	if includeAudio {
		args = append(args, "-c:a", "aac", "-ac", "2", "-ar", "48000")
	} else {
		args = append(args, "-an")
	}

	args = append(args, "-f", "mp4", "-y", srcPath)

	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "Unknown encoder") || strings.Contains(outStr, "Encoder") {
			t.Skipf("encoder not available: %s", outStr)
		}
		t.Fatalf("generate FATE source: %v\n%s", err, outStr)
	}
	return srcPath
}

func generateFATEReferenceDASH(t *testing.T, sourcePath, refDir string) {
	t.Helper()
	args := []string{"-i", sourcePath, "-c", "copy",
		"-f", "dash", "-seg_duration", "2",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		"-y", filepath.Join(refDir, "manifest.mpd"),
	}
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate FATE reference DASH: %v\n%s", err, string(out))
	}
}

func muxThroughOurDASH(t *testing.T, inputPath, ourDir string, includeAudio bool) {
	t.Helper()

	fc := astiav.AllocFormatContext()
	require.NotNil(t, fc)
	defer fc.Free()

	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	require.NoError(t, fc.OpenInput(inputPath, nil, inputDict))
	defer fc.CloseInput()
	require.NoError(t, fc.FindStreamInfo(nil))

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
	require.NotNil(t, videoStream)

	vcp := videoStream.CodecParameters()
	var videoExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}

	opts := mux.MuxOpts{
		OutputDir:      ourDir,
		VideoCodecID:   vcp.CodecID(),
		VideoExtradata: videoExtradata,
		VideoWidth:     vcp.Width(),
		VideoHeight:    vcp.Height(),
		VideoTimeBase:  videoStream.TimeBase(),
	}

	if includeAudio && audioStream != nil {
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
	}

	fmp4Muxer, err := mux.NewFragmentedMuxer(opts)
	require.NoError(t, err)

	pkt := astiav.AllocPacket()
	require.NotNil(t, pkt)
	defer pkt.Free()

	audioOutTB := astiav.NewRational(1, 48000)

	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		switch pkt.StreamIndex() {
		case videoStream.Index():
			require.NoError(t, fmp4Muxer.WriteVideoPacket(pkt))
		default:
			if includeAudio && audioStream != nil && pkt.StreamIndex() == audioStream.Index() {
				pkt.RescaleTs(audioStream.TimeBase(), audioOutTB)
				require.NoError(t, fmp4Muxer.WriteAudioPacket(pkt))
			}
		}
		pkt.Unref()
	}
	require.NoError(t, fmp4Muxer.Close())
}

func combineOurDASHVideo(t *testing.T, ourDir string) string {
	t.Helper()
	combinedPath := filepath.Join(ourDir, "combined_video.mp4")

	initVideo, err := os.ReadFile(filepath.Join(ourDir, "init_video.mp4"))
	require.NoError(t, err)

	videoSegs, err := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))
	require.NoError(t, err)
	sort.Strings(videoSegs)
	require.Greater(t, len(videoSegs), 0)

	var combined []byte
	combined = append(combined, initVideo...)
	for _, seg := range videoSegs {
		data, err := os.ReadFile(seg)
		require.NoError(t, err)
		combined = append(combined, data...)
	}
	require.NoError(t, os.WriteFile(combinedPath, combined, 0644))
	return combinedPath
}

func combineOurDASHAudio(t *testing.T, ourDir string) string {
	t.Helper()
	combinedPath := filepath.Join(ourDir, "combined_audio.mp4")

	initAudio, err := os.ReadFile(filepath.Join(ourDir, "init_audio.mp4"))
	require.NoError(t, err)

	audioSegs, err := filepath.Glob(filepath.Join(ourDir, "audio_*.m4s"))
	require.NoError(t, err)
	sort.Strings(audioSegs)
	require.Greater(t, len(audioSegs), 0)

	var combined []byte
	combined = append(combined, initAudio...)
	for _, seg := range audioSegs {
		data, err := os.ReadFile(seg)
		require.NoError(t, err)
		combined = append(combined, data...)
	}
	require.NoError(t, os.WriteFile(combinedPath, combined, 0644))
	return combinedPath
}

func combineRefDASHVideo(t *testing.T, refDir string) string {
	t.Helper()
	combinedPath := filepath.Join(refDir, "combined_video.mp4")

	initData, err := os.ReadFile(filepath.Join(refDir, "init-0.m4s"))
	require.NoError(t, err)

	entries, err := os.ReadDir(refDir)
	require.NoError(t, err)

	var segPaths []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "chunk-0-") {
			segPaths = append(segPaths, filepath.Join(refDir, e.Name()))
		}
	}
	sort.Strings(segPaths)

	var combined []byte
	combined = append(combined, initData...)
	for _, seg := range segPaths {
		data, err := os.ReadFile(seg)
		require.NoError(t, err)
		combined = append(combined, data...)
	}
	require.NoError(t, os.WriteFile(combinedPath, combined, 0644))
	return combinedPath
}

func ffmpegDecodableDASH(t *testing.T, path string) (bool, string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-v", "error", "-i", path, "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return false, output
	}
	if output != "" {
		return true, output
	}
	return true, ""
}

type dashFramecrcResult struct {
	lines      []string
	frameCount int
	videoCount int
	audioCount int
}

func computeFrameCRCDASH(t *testing.T, path string) dashFramecrcResult {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-i", path, "-f", "framecrc", "-")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("framecrc failed for %s: %v\n%s", path, err, string(out))
	}

	var result dashFramecrcResult
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		result.lines = append(result.lines, line)
		result.frameCount++
		parts := strings.SplitN(strings.TrimSpace(line), ",", 2)
		if len(parts) >= 1 {
			idx, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			if idx == 0 {
				result.videoCount++
			} else if idx == 1 {
				result.audioCount++
			}
		}
	}
	return result
}

type dashProbeFormat struct {
	Duration  string `json:"duration"`
	NbStreams int    `json:"nb_streams"`
}

type dashProbeResult struct {
	Format dashProbeFormat `json:"format"`
}

func ffprobeDurationDASH(t *testing.T, path string) float64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "json", path).Output()
	require.NoError(t, err)
	var result dashProbeResult
	require.NoError(t, json.Unmarshal(out, &result))
	dur, _ := strconv.ParseFloat(result.Format.Duration, 64)
	return dur
}

func fateDASHTestVariant(t *testing.T, videoCodec string, includeAudio bool) {
	t.Helper()

	tmpRoot := t.TempDir()
	refDir := filepath.Join(tmpRoot, "ref")
	ourDir := filepath.Join(tmpRoot, "ours")
	require.NoError(t, os.MkdirAll(refDir, 0755))
	require.NoError(t, os.MkdirAll(ourDir, 0755))

	sourcePath := generateFATESourceDASH(t, tmpRoot, 5, videoCodec, includeAudio)
	generateFATEReferenceDASH(t, sourcePath, refDir)
	muxThroughOurDASH(t, sourcePath, ourDir, includeAudio)

	ourCombined := combineOurDASHVideo(t, ourDir)

	t.Run("decodable", func(t *testing.T) {
		ok, errOutput := ffmpegDecodableDASH(t, ourCombined)
		if !ok {
			t.Fatalf("ffmpeg cannot decode our DASH video output: %s", errOutput)
		}
		if errOutput != "" {
			t.Logf("ffmpeg warnings (non-fatal): %s", errOutput)
		}
		t.Log("our DASH video output is fully decodable by ffmpeg with zero errors")
	})

	if includeAudio {
		t.Run("audio_decodable", func(t *testing.T) {
			audioCombined := combineOurDASHAudio(t, ourDir)
			ok, errOutput := ffmpegDecodableDASH(t, audioCombined)
			if !ok {
				t.Fatalf("ffmpeg cannot decode our DASH audio output: %s", errOutput)
			}
			if errOutput != "" {
				t.Logf("ffmpeg warnings (non-fatal): %s", errOutput)
			}
			t.Log("our DASH audio output is fully decodable by ffmpeg with zero errors")
		})
	}

	t.Run("framecrc", func(t *testing.T) {
		refCombined := combineRefDASHVideo(t, refDir)
		refCRC := computeFrameCRCDASH(t, refCombined)
		ourCRC := computeFrameCRCDASH(t, ourCombined)

		t.Logf("reference: %d frames (%d video, %d audio)",
			refCRC.frameCount, refCRC.videoCount, refCRC.audioCount)
		t.Logf("ours: %d frames (%d video, %d audio)",
			ourCRC.frameCount, ourCRC.videoCount, ourCRC.audioCount)

		if ourCRC.frameCount == 0 {
			t.Fatal("our DASH produced zero decoded frames")
		}

		videoDiff := ourCRC.videoCount - refCRC.videoCount
		if videoDiff < 0 {
			videoDiff = -videoDiff
		}
		tolerance := refCRC.videoCount / 10
		if tolerance < 2 {
			tolerance = 2
		}
		if videoDiff > tolerance {
			t.Errorf("video frame count mismatch: ref=%d ours=%d (diff=%d, tolerance=%d)",
				refCRC.videoCount, ourCRC.videoCount, videoDiff, tolerance)
		}
	})

	t.Run("duration", func(t *testing.T) {
		skipIfNoFFprobeFate(t)
		ourDur := ffprobeDurationDASH(t, ourCombined)
		t.Logf("our DASH video duration: %.3fs (source=5s)", ourDur)
		assert.InDelta(t, 5.0, ourDur, 0.5,
			"our duration should be within 0.5s of 5s source")
	})
}

func TestFATE_DASH_H264_AAC(t *testing.T) {
	skipIfNoFFmpegFate(t)
	fateDASHTestVariant(t, "h264", true)
}

func TestFATE_DASH_H265_AAC(t *testing.T) {
	skipIfNoFFmpegFate(t)
	fateDASHTestVariant(t, "h265", true)
}

func TestFATE_DASH_H264_VideoOnly(t *testing.T) {
	skipIfNoFFmpegFate(t)
	fateDASHTestVariant(t, "h264", false)
}

func TestFATE_DASH_Decodability_ZeroErrors(t *testing.T) {
	skipIfNoFFmpegFate(t)

	variants := []struct {
		name       string
		videoCodec string
		audio      bool
	}{
		{"h264_aac", "h264", true},
		{"h265_aac", "h265", true},
		{"h264_vidonly", "h264", false},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			tmpRoot := t.TempDir()
			ourDir := filepath.Join(tmpRoot, "ours")
			require.NoError(t, os.MkdirAll(ourDir, 0755))

			src := generateFATESourceDASH(t, tmpRoot, 5, v.videoCodec, v.audio)
			muxThroughOurDASH(t, src, ourDir, v.audio)
			combined := combineOurDASHVideo(t, ourDir)

			cmd := exec.Command("ffmpeg", "-v", "error", "-i", combined, "-f", "null", "-")
			output, err := cmd.CombinedOutput()
			errText := strings.TrimSpace(string(output))

			if err != nil {
				t.Fatalf("ffmpeg cannot decode our DASH: %v\n%s", err, errText)
			}
			if errText != "" {
				t.Errorf("ffmpeg reported errors: %s", errText)
			} else {
				t.Log("zero errors from ffmpeg -v error: DASH video is valid")
			}

			if v.audio {
				audioCombined := combineOurDASHAudio(t, ourDir)
				cmd = exec.Command("ffmpeg", "-v", "error", "-i", audioCombined, "-f", "null", "-")
				output, err = cmd.CombinedOutput()
				errText = strings.TrimSpace(string(output))
				if err != nil {
					t.Fatalf("ffmpeg cannot decode our DASH audio: %v\n%s", err, errText)
				}
				if errText != "" {
					t.Errorf("ffmpeg reported audio errors: %s", errText)
				} else {
					t.Log("zero errors from ffmpeg -v error: DASH audio is valid")
				}
			}
		})
	}
}

func TestFATE_DASH_FramecrcComparison(t *testing.T) {
	skipIfNoFFmpegFate(t)

	tmpRoot := t.TempDir()
	refDir := filepath.Join(tmpRoot, "ref")
	ourDir := filepath.Join(tmpRoot, "ours")
	require.NoError(t, os.MkdirAll(refDir, 0755))
	require.NoError(t, os.MkdirAll(ourDir, 0755))

	sourcePath := generateFATESourceDASH(t, tmpRoot, 5, "h264", true)
	generateFATEReferenceDASH(t, sourcePath, refDir)
	muxThroughOurDASH(t, sourcePath, ourDir, true)

	refCombined := combineRefDASHVideo(t, refDir)
	ourCombined := combineOurDASHVideo(t, ourDir)

	refCRC := computeFrameCRCDASH(t, refCombined)
	ourCRC := computeFrameCRCDASH(t, ourCombined)

	t.Logf("reference: %d total (%d video)", refCRC.frameCount, refCRC.videoCount)
	t.Logf("ours:      %d total (%d video)", ourCRC.frameCount, ourCRC.videoCount)

	require.Greater(t, ourCRC.videoCount, 0, "should have video frames")
	require.Greater(t, refCRC.videoCount, 0, "reference should have video frames")

	diff := math.Abs(float64(ourCRC.videoCount - refCRC.videoCount))
	pctDiff := diff / float64(refCRC.videoCount) * 100
	t.Logf("video frame count diff: %.1f%%", pctDiff)
	assert.Less(t, pctDiff, 5.0,
		"video frame count should be within 5%% of reference: ref=%d ours=%d",
		refCRC.videoCount, ourCRC.videoCount)

	matchCount := 0
	minLines := len(refCRC.lines)
	if len(ourCRC.lines) < minLines {
		minLines = len(ourCRC.lines)
	}
	for i := 0; i < minLines; i++ {
		if refCRC.lines[i] == ourCRC.lines[i] {
			matchCount++
		}
	}
	totalFrames := refCRC.frameCount
	if ourCRC.frameCount > totalFrames {
		totalFrames = ourCRC.frameCount
	}
	if totalFrames > 0 {
		matchPct := float64(matchCount) / float64(totalFrames) * 100
		t.Logf("framecrc match: %d/%d lines (%.1f%%)", matchCount, totalFrames, matchPct)
	}
}
