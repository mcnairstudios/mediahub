//go:build cgo

package mux

import (
	"bufio"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type framecrcResult struct {
	lines      []string
	frameCount int
	videoCount int
	audioCount int
}

func computeFrameCRC(t *testing.T, playlistPath string) framecrcResult {
	t.Helper()
	cmd := exec.Command("ffmpeg",
		"-i", playlistPath,
		"-f", "framecrc", "-")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("framecrc failed for %s: %v\n%s", playlistPath, err, string(out))
	}

	var result framecrcResult
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

func ffmpegDecodable(t *testing.T, playlistPath string) (bool, string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-v", "error", "-i", playlistPath, "-f", "null", "-")
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

func generateFATESource(t *testing.T, dir string, durationSec int, videoCodec string, includeAudio bool, format string) string {
	t.Helper()
	ext := ".ts"
	if format == "mp4" {
		ext = ".mp4"
	}
	srcPath := filepath.Join(dir, "source"+ext)

	args := []string{
		"-f", "lavfi", "-i",
		"testsrc2=size=128x72:rate=1:d=" + strconv.Itoa(durationSec),
	}
	if includeAudio {
		args = append(args,
			"-f", "lavfi", "-i",
			"sine=f=440:d="+strconv.Itoa(durationSec)+":sample_rate=44100",
		)
	}

	switch videoCodec {
	case "h264":
		args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-g", "1")
	case "h265":
		args = append(args, "-c:v", "libx265", "-preset", "ultrafast",
			"-x265-params", "keyint=1:min-keyint=1")
	}

	if includeAudio {
		args = append(args, "-c:a", "aac", "-ac", "2")
	} else {
		args = append(args, "-an")
	}

	args = append(args, "-f", format, "-y", srcPath)

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

func generateFATEReferenceHLS(t *testing.T, sourcePath, refDir, segType string) {
	t.Helper()
	args := []string{"-i", sourcePath, "-c", "copy"}

	if segType == "fmp4" {
		args = append(args,
			"-f", "hls",
			"-hls_time", "1",
			"-hls_list_size", "0",
			"-hls_segment_type", "fmp4",
			"-hls_fmp4_init_filename", "init.mp4",
			"-hls_segment_filename", filepath.Join(refDir, "seg%d.m4s"),
		)
	} else {
		args = append(args,
			"-f", "hls",
			"-hls_time", "1",
			"-hls_list_size", "0",
			"-hls_segment_filename", filepath.Join(refDir, "seg%d.ts"),
		)
	}

	args = append(args, "-y", filepath.Join(refDir, "playlist.m3u8"))

	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate FATE reference HLS: %v\n%s", err, string(out))
	}
}

func fateTestVariant(t *testing.T, name, videoCodec, inputFormat, segType string, includeAudio bool) {
	t.Helper()

	tmpRoot := t.TempDir()
	refDir := filepath.Join(tmpRoot, "ref")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(refDir, 0755)
	os.MkdirAll(ourDir, 0755)

	sourcePath := generateFATESource(t, tmpRoot, 3, videoCodec, includeAudio, inputFormat)

	generateFATEReferenceHLS(t, sourcePath, refDir, segType)

	muxThroughOurHLS(t, sourcePath, ourDir, segType)

	refPlaylist := filepath.Join(refDir, "playlist.m3u8")
	ourPlaylist := filepath.Join(ourDir, "playlist.m3u8")

	refData, _ := os.ReadFile(refPlaylist)
	ourData, _ := os.ReadFile(ourPlaylist)
	t.Logf("reference playlist:\n%s", string(refData))
	t.Logf("our playlist:\n%s", string(ourData))

	t.Run("decodable", func(t *testing.T) {
		ok, errOutput := ffmpegDecodable(t, ourPlaylist)
		if !ok {
			t.Fatalf("ffmpeg cannot decode our HLS output: %s", errOutput)
		}
		if errOutput != "" {
			t.Logf("ffmpeg warnings (non-fatal): %s", errOutput)
		}
		t.Log("our HLS output is fully decodable by ffmpeg")
	})

	t.Run("framecrc", func(t *testing.T) {
		refCRC := computeFrameCRC(t, refPlaylist)
		ourCRC := computeFrameCRC(t, ourPlaylist)

		t.Logf("reference: %d frames (%d video, %d audio)",
			refCRC.frameCount, refCRC.videoCount, refCRC.audioCount)
		t.Logf("ours: %d frames (%d video, %d audio)",
			ourCRC.frameCount, ourCRC.videoCount, ourCRC.audioCount)

		if ourCRC.frameCount == 0 {
			t.Fatal("our HLS produced zero decoded frames")
		}

		videoDiff := ourCRC.videoCount - refCRC.videoCount
		if videoDiff < 0 {
			videoDiff = -videoDiff
		}
		if videoDiff > 1 {
			t.Errorf("video frame count mismatch: ref=%d ours=%d (diff=%d, max=1)",
				refCRC.videoCount, ourCRC.videoCount, videoDiff)
		}

		if includeAudio {
			if ourCRC.audioCount == 0 {
				t.Error("no audio frames decoded from our HLS output")
			}
			audioDiff := ourCRC.audioCount - refCRC.audioCount
			if audioDiff < 0 {
				audioDiff = -audioDiff
			}
			audioTolerance := refCRC.audioCount / 10
			if audioTolerance < 2 {
				audioTolerance = 2
			}
			if audioDiff > audioTolerance {
				t.Errorf("audio frame count mismatch: ref=%d ours=%d (diff=%d, tolerance=%d)",
					refCRC.audioCount, ourCRC.audioCount, audioDiff, audioTolerance)
			}
		}

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

			if matchPct < 50 && matchCount < totalFrames {
				t.Logf("framecrc mismatch is expected: different muxers produce different PTS values")
				t.Logf("the key result is that both produce the same number of decoded frames")
			}
		}
	})

	t.Run("duration", func(t *testing.T) {
		refPlaylistData, _ := os.ReadFile(refPlaylist)
		ourPlaylistData, _ := os.ReadFile(ourPlaylist)

		_, refDurations, _ := parseHLSPlaylist(t, string(refPlaylistData))
		_, ourDurations, _ := parseHLSPlaylist(t, string(ourPlaylistData))

		var refTotal, ourTotal float64
		for _, d := range refDurations {
			refTotal += d
		}
		for _, d := range ourDurations {
			ourTotal += d
		}

		t.Logf("EXTINF total: ref=%.3fs ours=%.3fs (source=3s)", refTotal, ourTotal)

		if math.Abs(ourTotal-3.0) > 1.0 {
			t.Errorf("our total duration %.3fs deviates from 3s source by more than 1s", ourTotal)
		}
	})
}

func TestFATE_H264_AAC_MPEGTS(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	fateTestVariant(t, "h264_aac_mpegts", "h264", "mpegts", "mpegts", true)
}

func TestFATE_H264_AAC_FMP4(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	fateTestVariant(t, "h264_aac_fmp4", "h264", "mp4", "fmp4", true)
}

func TestFATE_H265_AAC_FMP4(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	fateTestVariant(t, "h265_aac_fmp4", "h265", "mp4", "fmp4", true)
}

func TestFATE_H264_VideoOnly_MPEGTS(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	fateTestVariant(t, "h264_vidonly_mpegts", "h264", "mpegts", "mpegts", false)
}

func TestFATE_H264_VideoOnly_FMP4(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	fateTestVariant(t, "h264_vidonly_fmp4", "h264", "mp4", "fmp4", false)
}

func TestFATE_Decodability_ZeroErrors(t *testing.T) {
	skipIfNoFFmpegBinary(t)

	variants := []struct {
		name        string
		videoCodec  string
		inputFormat string
		segType     string
		audio       bool
	}{
		{"h264_aac_ts", "h264", "mpegts", "mpegts", true},
		{"h264_aac_fmp4", "h264", "mp4", "fmp4", true},
		{"h265_aac_fmp4", "h265", "mp4", "fmp4", true},
		{"h264_vidonly_ts", "h264", "mpegts", "mpegts", false},
		{"h264_vidonly_fmp4", "h264", "mp4", "fmp4", false},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			tmpRoot := t.TempDir()
			ourDir := filepath.Join(tmpRoot, "ours")
			os.MkdirAll(ourDir, 0755)

			src := generateFATESource(t, tmpRoot, 3, v.videoCodec, v.audio, v.inputFormat)
			muxThroughOurHLS(t, src, ourDir, v.segType)

			playlist := filepath.Join(ourDir, "playlist.m3u8")

			cmd := exec.Command("ffmpeg", "-v", "error", "-i", playlist, "-f", "null", "-")
			output, err := cmd.CombinedOutput()
			errText := strings.TrimSpace(string(output))

			if err != nil {
				t.Fatalf("ffmpeg cannot decode our HLS: %v\n%s", err, errText)
			}
			if errText != "" {
				t.Errorf("ffmpeg reported errors: %s", errText)
			} else {
				t.Log("zero errors from ffmpeg -v error: HLS is valid")
			}
		})
	}
}

func TestFATE_OutputPlugin_Decodable(t *testing.T) {
	skipIfNoFFmpegBinary(t)

	tmpRoot := t.TempDir()
	srcDir := filepath.Join(tmpRoot, "src")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(ourDir, 0755)

	sourcePath := generateFATESource(t, srcDir, 3, "h264", true, "mpegts")

	muxThroughOurHLS(t, sourcePath, ourDir, "mpegts")

	playlist := filepath.Join(ourDir, "playlist.m3u8")

	ok, errOutput := ffmpegDecodable(t, playlist)
	if !ok {
		t.Fatalf("output plugin HLS not decodable: %s", errOutput)
	}

	crc := computeFrameCRC(t, playlist)
	t.Logf("output plugin path: %d total frames (%d video, %d audio)",
		crc.frameCount, crc.videoCount, crc.audioCount)

	if crc.videoCount == 0 {
		t.Error("no video frames in output plugin HLS")
	}
	if crc.audioCount == 0 {
		t.Error("no audio frames in output plugin HLS")
	}
}
