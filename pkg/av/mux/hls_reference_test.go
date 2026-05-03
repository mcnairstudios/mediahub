//go:build cgo

package mux

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/output/validate"
)

const knownGoodHLSPlaylist = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:3
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:2.000000,
seg0.ts
#EXTINF:2.000000,
seg1.ts
#EXTINF:2.000000,
seg2.ts
#EXTINF:2.000000,
seg3.ts
#EXTINF:2.000000,
seg4.ts
#EXT-X-ENDLIST
`

func TestHLSReference_ValidatorAcceptsKnownGood(t *testing.T) {
	errs := validate.ValidateHLSPlaylist([]byte(knownGoodHLSPlaylist))
	for _, e := range errs {
		t.Errorf("validator rejected known-good playlist: %v", e)
	}
}

func TestHLSReference_EndToEnd(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg binary not found, skipping reference test")
	}
	t.Logf("using ffmpeg at %s", ffmpegPath)

	tmpRoot := t.TempDir()
	inputPath := filepath.Join(tmpRoot, "input.ts")
	refDir := filepath.Join(tmpRoot, "ref")
	ourDir := filepath.Join(tmpRoot, "ours")

	if err := os.MkdirAll(refDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ourDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(ffmpegPath,
		"-f", "lavfi", "-i", "testsrc2=duration=10:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=10:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
		"-g", "50",
		"-c:a", "aac", "-ac", "2", "-ar", "48000", "-b:a", "128k",
		"-f", "mpegts", "-y", inputPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("generate test input: %v", err)
	}

	info, err := os.Stat(inputPath)
	if err != nil || info.Size() == 0 {
		t.Fatal("test input file is empty or missing")
	}
	t.Logf("generated test input: %d bytes", info.Size())

	refPlaylistPath := filepath.Join(refDir, "playlist.m3u8")
	cmd = exec.Command(ffmpegPath,
		"-i", inputPath,
		"-c", "copy",
		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(refDir, "seg%d.ts"),
		"-y", refPlaylistPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("generate reference HLS: %v", err)
	}

	refPlaylistData, err := os.ReadFile(refPlaylistPath)
	if err != nil {
		t.Fatalf("read reference playlist: %v", err)
	}
	t.Logf("reference playlist:\n%s", string(refPlaylistData))

	refErrs := validate.ValidateHLSPlaylist(refPlaylistData)
	for _, e := range refErrs {
		t.Errorf("validator rejected ffmpeg reference playlist: %v", e)
	}

	refSegCount, refSegDurations, refTargetDur := parseHLSPlaylist(t, string(refPlaylistData))
	t.Logf("reference: %d segments, target duration %d", refSegCount, refTargetDur)

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
	if audioStream == nil {
		t.Fatal("no audio stream in test input")
	}

	vcp := videoStream.CodecParameters()
	acp := audioStream.CodecParameters()

	var videoExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}
	var audioExtradata []byte
	if ed := acp.ExtraData(); len(ed) > 0 {
		audioExtradata = make([]byte, len(ed))
		copy(audioExtradata, ed)
	}

	t.Logf("video: codec=%v %dx%d tb=%d/%d extradata=%d bytes",
		vcp.CodecID(), vcp.Width(), vcp.Height(),
		videoStream.TimeBase().Num(), videoStream.TimeBase().Den(),
		len(videoExtradata))
	t.Logf("audio: codec=%v rate=%d channels=%d tb=%d/%d extradata=%d bytes",
		acp.CodecID(), acp.SampleRate(), acp.ChannelLayout().Channels(),
		audioStream.TimeBase().Num(), audioStream.TimeBase().Den(),
		len(audioExtradata))

	hlsMuxer, err := NewHLSMuxer(HLSMuxOpts{
		OutputDir:          ourDir,
		SegmentDurationSec: 2,
		VideoCodecID:       vcp.CodecID(),
		VideoExtradata:     videoExtradata,
		VideoWidth:         vcp.Width(),
		VideoHeight:        vcp.Height(),
		VideoTimeBase:      videoStream.TimeBase(),
		VideoFrameRate:     25,
		AudioCodecID:       acp.CodecID(),
		AudioExtradata:     audioExtradata,
		AudioChannels:      acp.ChannelLayout().Channels(),
		AudioSampleRate:    acp.SampleRate(),
		AudioTimeBase:      audioStream.TimeBase(),
		AudioFrameSize:     1024,
	})
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
		case audioStream.Index():
			if err := hlsMuxer.WriteAudioPacket(pkt); err != nil {
				t.Fatalf("write audio packet %d: %v", audioPkts, err)
			}
			audioPkts++
		}
		pkt.Unref()
	}

	t.Logf("wrote %d video + %d audio packets through our muxer", videoPkts, audioPkts)

	if err := hlsMuxer.Close(); err != nil {
		t.Fatalf("close muxer: %v", err)
	}

	ourPlaylistData, err := os.ReadFile(filepath.Join(ourDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read our playlist: %v", err)
	}
	t.Logf("our playlist:\n%s", string(ourPlaylistData))

	ourErrs := validate.ValidateHLSPlaylist(ourPlaylistData)
	for _, e := range ourErrs {
		t.Errorf("validator rejected our playlist: %v", e)
	}

	ourSegCount, ourSegDurations, ourTargetDur := parseHLSPlaylist(t, string(ourPlaylistData))

	if !strings.Contains(string(ourPlaylistData), "#EXTM3U") {
		t.Error("our playlist missing #EXTM3U header")
	}
	if !strings.Contains(string(ourPlaylistData), "#EXT-X-TARGETDURATION:") {
		t.Error("our playlist missing #EXT-X-TARGETDURATION")
	}
	if !strings.Contains(string(ourPlaylistData), "#EXT-X-ENDLIST") {
		t.Error("our playlist missing #EXT-X-ENDLIST (Close should finalize)")
	}

	segDiff := ourSegCount - refSegCount
	if segDiff < 0 {
		segDiff = -segDiff
	}
	if segDiff > 1 {
		t.Errorf("segment count mismatch: ours=%d reference=%d (diff=%d, max allowed=1)",
			ourSegCount, refSegCount, segDiff)
	} else {
		t.Logf("segment count: ours=%d reference=%d (within tolerance)", ourSegCount, refSegCount)
	}

	if ourTargetDur > 0 && refTargetDur > 0 {
		t.Logf("target duration: ours=%d reference=%d", ourTargetDur, refTargetDur)
	}

	minLen := len(ourSegDurations)
	if len(refSegDurations) < minLen {
		minLen = len(refSegDurations)
	}
	for i := 0; i < minLen; i++ {
		if refSegDurations[i] == 0 {
			continue
		}
		ratio := ourSegDurations[i] / refSegDurations[i]
		if ratio < 0.8 || ratio > 1.2 {
			t.Errorf("segment %d duration mismatch: ours=%.3fs reference=%.3fs (ratio=%.2f, tolerance=20%%)",
				i, ourSegDurations[i], refSegDurations[i], ratio)
		}
	}

	ourSegFiles, _ := filepath.Glob(filepath.Join(ourDir, "seg*.ts"))
	for _, seg := range ourSegFiles {
		info, err := os.Stat(seg)
		if err != nil {
			t.Errorf("stat segment %s: %v", filepath.Base(seg), err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("segment %s is empty", filepath.Base(seg))
		}
	}

	refSegFiles, _ := filepath.Glob(filepath.Join(refDir, "seg*.ts"))
	t.Logf("segment file count: ours=%d reference=%d", len(ourSegFiles), len(refSegFiles))

	for i := 0; i < len(ourSegFiles) && i < len(refSegFiles); i++ {
		ourInfo, _ := os.Stat(ourSegFiles[i])
		refInfo, _ := os.Stat(refSegFiles[i])
		if ourInfo != nil && refInfo != nil {
			ourSize := float64(ourInfo.Size())
			refSize := float64(refInfo.Size())
			if refSize > 0 {
				sizeRatio := ourSize / refSize
				t.Logf("segment %d size: ours=%d ref=%d ratio=%.2f",
					i, ourInfo.Size(), refInfo.Size(), sizeRatio)
			}
		}
	}

	probeCmd := exec.Command(ffmpegPath, "-v", "error", "-i",
		filepath.Join(ourDir, "playlist.m3u8"), "-f", "null", "-")
	probeOutput, probeErr := probeCmd.CombinedOutput()
	if probeErr != nil {
		t.Errorf("ffmpeg probe of our HLS output failed: %v\n%s", probeErr, string(probeOutput))
	} else if len(probeOutput) > 0 {
		t.Logf("ffmpeg probe warnings:\n%s", string(probeOutput))
	} else {
		t.Log("ffmpeg validates our HLS output with no errors")
	}
}

func TestHLSReference_VideoOnly(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg binary not found")
	}

	tmpRoot := t.TempDir()
	inputPath := filepath.Join(tmpRoot, "input.ts")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	cmd := exec.Command(ffmpegPath,
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=320x240:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
		"-g", "25",
		"-an",
		"-f", "mpegts", "-y", inputPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("generate test input: %v", err)
	}

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
		VideoCodecID:       vcp.CodecID(),
		VideoExtradata:     videoExtradata,
		VideoWidth:         vcp.Width(),
		VideoHeight:        vcp.Height(),
		VideoTimeBase:      videoStream.TimeBase(),
		VideoFrameRate:     25,
	})
	if err != nil {
		t.Fatalf("create HLS muxer: %v", err)
	}

	pkt := astiav.AllocPacket()
	defer pkt.Free()

	var count int
	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		if pkt.StreamIndex() == videoStream.Index() {
			if err := hlsMuxer.WriteVideoPacket(pkt); err != nil {
				t.Fatalf("write video pkt %d: %v", count, err)
			}
			count++
		}
		pkt.Unref()
	}

	hlsMuxer.Close()
	t.Logf("wrote %d video packets", count)

	playlistData, err := os.ReadFile(filepath.Join(ourDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read playlist: %v", err)
	}

	errs := validate.ValidateHLSPlaylist(playlistData)
	for _, e := range errs {
		t.Errorf("validator error: %v", e)
	}

	if !strings.Contains(string(playlistData), "#EXT-X-ENDLIST") {
		t.Error("missing #EXT-X-ENDLIST")
	}

	segCount, _, _ := parseHLSPlaylist(t, string(playlistData))
	if segCount == 0 {
		t.Error("no segments in video-only HLS output")
	}
	t.Logf("video-only: %d segments\n%s", segCount, string(playlistData))
}

func parseHLSPlaylist(t *testing.T, content string) (segCount int, durations []float64, targetDur int) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(content), "\n")

	targetDurRe := regexp.MustCompile(`#EXT-X-TARGETDURATION:(\d+)`)
	extinfRe := regexp.MustCompile(`#EXTINF:([\d.]+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if m := targetDurRe.FindStringSubmatch(line); len(m) > 1 {
			targetDur, _ = strconv.Atoi(m[1])
		}

		if m := extinfRe.FindStringSubmatch(line); len(m) > 1 {
			dur, _ := strconv.ParseFloat(m[1], 64)
			durations = append(durations, dur)
			segCount++
		}
	}
	return
}

func TestHLSReference_DurationAccuracy(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg binary not found")
	}

	tmpRoot := t.TempDir()
	inputPath := filepath.Join(tmpRoot, "input.ts")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	cmd := exec.Command(ffmpegPath,
		"-f", "lavfi", "-i", "testsrc2=duration=10:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=10:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
		"-g", "50",
		"-c:a", "aac", "-ac", "2", "-ar", "48000", "-b:a", "128k",
		"-f", "mpegts", "-y", inputPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("generate input: %v", err)
	}

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

	vcp := videoStream.CodecParameters()
	acp := audioStream.CodecParameters()

	var videoExtradata, audioExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}
	if ed := acp.ExtraData(); len(ed) > 0 {
		audioExtradata = make([]byte, len(ed))
		copy(audioExtradata, ed)
	}

	hlsMuxer, err := NewHLSMuxer(HLSMuxOpts{
		OutputDir:          ourDir,
		SegmentDurationSec: 2,
		VideoCodecID:       vcp.CodecID(),
		VideoExtradata:     videoExtradata,
		VideoWidth:         vcp.Width(),
		VideoHeight:        vcp.Height(),
		VideoTimeBase:      videoStream.TimeBase(),
		VideoFrameRate:     25,
		AudioCodecID:       acp.CodecID(),
		AudioExtradata:     audioExtradata,
		AudioChannels:      acp.ChannelLayout().Channels(),
		AudioSampleRate:    acp.SampleRate(),
		AudioTimeBase:      audioStream.TimeBase(),
		AudioFrameSize:     1024,
	})
	if err != nil {
		t.Fatalf("create muxer: %v", err)
	}

	pkt := astiav.AllocPacket()
	defer pkt.Free()
	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		switch pkt.StreamIndex() {
		case videoStream.Index():
			hlsMuxer.WriteVideoPacket(pkt) //nolint:errcheck
		case audioStream.Index():
			hlsMuxer.WriteAudioPacket(pkt) //nolint:errcheck
		}
		pkt.Unref()
	}
	hlsMuxer.Close()

	playlistData, _ := os.ReadFile(filepath.Join(ourDir, "playlist.m3u8"))
	_, durations, targetDur := parseHLSPlaylist(t, string(playlistData))

	var totalDuration float64
	for _, d := range durations {
		totalDuration += d
	}

	if math.Abs(totalDuration-10.0) > 1.0 {
		t.Errorf("total duration %.3fs deviates from expected 10s by more than 1s", totalDuration)
	} else {
		t.Logf("total duration: %.3fs (expected ~10s)", totalDuration)
	}

	for i, dur := range durations {
		if i == len(durations)-1 {
			continue
		}
		if targetDur > 0 && dur > float64(targetDur)+0.5 {
			t.Errorf("segment %d duration %.3fs exceeds target duration %d", i, dur, targetDur)
		}
	}

	t.Logf("segment durations: %v", durations)
}
