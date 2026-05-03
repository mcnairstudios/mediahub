//go:build cgo

package mux

import (
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

func ffprobePacketCount(t *testing.T, path string, mediaType string) int {
	t.Helper()
	selectStreams := "v:0"
	if mediaType == "audio" {
		selectStreams = "a:0"
	}
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", selectStreams,
		"-show_entries", "packet=pts",
		"-of", "csv=p=0",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return -1
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func ffprobePacketPTS(t *testing.T, path string, mediaType string) []int64 {
	t.Helper()
	selectStreams := "v:0"
	if mediaType == "audio" {
		selectStreams = "a:0"
	}
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", selectStreams,
		"-show_entries", "packet=pts",
		"-of", "csv=p=0",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var pts []int64
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "N/A" {
			continue
		}
		v, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			continue
		}
		pts = append(pts, v)
	}
	return pts
}

func ffprobeDuration(t *testing.T, path string) float64 {
	t.Helper()
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		path,
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

func TestFMP4Conformance_PacketCount(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	tmpRoot := t.TempDir()
	inputPath := filepath.Join(tmpRoot, "input.mp4")
	refPath := filepath.Join(tmpRoot, "ref.mp4")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
		"-g", "25",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "mp4", "-y", inputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate input: %v\n%s", err, out)
	}

	cmd = exec.Command("ffmpeg",
		"-i", inputPath,
		"-c:v", "copy", "-c:a", "copy",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-y", refPath,
	)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate reference fMP4: %v\n%s", err, out)
	}

	refVideoCount := ffprobePacketCount(t, refPath, "video")
	refAudioCount := ffprobePacketCount(t, refPath, "audio")
	t.Logf("reference: %d video packets, %d audio packets", refVideoCount, refAudioCount)

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
		t.Fatalf("create muxer: %v", err)
	}

	pkt := astiav.AllocPacket()
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
			fmp4Muxer.WriteVideoPacket(pkt) //nolint:errcheck
			videoPkts++
		default:
			if audioStream != nil && pkt.StreamIndex() == audioStream.Index() {
				pkt.RescaleTs(audioStream.TimeBase(), audioOutTB)
				fmp4Muxer.WriteAudioPacket(pkt) //nolint:errcheck
				audioPkts++
			}
		}
		pkt.Unref()
	}

	fmp4Muxer.Close()

	t.Logf("wrote %d video + %d audio packets through our muxer", videoPkts, audioPkts)

	if refVideoCount > 0 {
		ratio := float64(videoPkts) / float64(refVideoCount)
		if ratio < 0.95 || ratio > 1.05 {
			t.Errorf("video packet count mismatch: ours=%d ref=%d ratio=%.2f", videoPkts, refVideoCount, ratio)
		}
	}
}

func TestFMP4Conformance_PTSMonotonic(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	tmpRoot := t.TempDir()
	inputPath := filepath.Join(tmpRoot, "input.mp4")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
		"-g", "25", "-an",
		"-f", "mp4", "-y", inputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate input: %v\n%s", err, out)
	}

	fc := astiav.AllocFormatContext()
	if fc == nil {
		t.Fatal("alloc format context")
	}
	defer fc.Free()

	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	fc.OpenInput(inputPath, nil, inputDict)
	defer fc.CloseInput()
	fc.FindStreamInfo(nil)

	var videoStream *astiav.Stream
	for _, s := range fc.Streams() {
		if s.CodecParameters().MediaType() == astiav.MediaTypeVideo {
			videoStream = s
			break
		}
	}

	vcp := videoStream.CodecParameters()
	var videoExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}

	fmp4Muxer, _ := NewFragmentedMuxer(MuxOpts{
		OutputDir:      ourDir,
		VideoCodecID:   vcp.CodecID(),
		VideoExtradata: videoExtradata,
		VideoWidth:     vcp.Width(),
		VideoHeight:    vcp.Height(),
		VideoTimeBase:  videoStream.TimeBase(),
	})

	pkt := astiav.AllocPacket()
	defer pkt.Free()

	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		if pkt.StreamIndex() == videoStream.Index() {
			fmp4Muxer.WriteVideoPacket(pkt) //nolint:errcheck
		}
		pkt.Unref()
	}
	fmp4Muxer.Close()

	videoSegs, _ := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))
	if len(videoSegs) == 0 {
		t.Fatal("no video segments")
	}

	for _, seg := range videoSegs {
		pts := ffprobePacketPTS(t, seg, "video")
		if len(pts) < 2 {
			continue
		}

		nonDecreasing := true
		for i := 1; i < len(pts); i++ {
			if pts[i] < pts[i-1] {
				nonDecreasing = false
				t.Logf("segment %s: PTS[%d]=%d < PTS[%d]=%d",
					filepath.Base(seg), i, pts[i], i-1, pts[i-1])
			}
		}

		if nonDecreasing {
			t.Logf("segment %s: %d packets, PTS monotonic OK (range %d-%d)",
				filepath.Base(seg), len(pts), pts[0], pts[len(pts)-1])
		}
	}
}

func TestFMP4Conformance_Duration(t *testing.T) {
	skipIfNoFFmpegBinary(t)
	skipIfNoFFprobeBinary(t)

	tmpRoot := t.TempDir()
	inputPath := filepath.Join(tmpRoot, "input.mp4")
	refPath := filepath.Join(tmpRoot, "ref.mp4")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
		"-g", "25",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "mp4", "-y", inputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate input: %v\n%s", err, out)
	}

	cmd = exec.Command("ffmpeg",
		"-i", inputPath,
		"-c:v", "copy", "-c:a", "copy",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-y", refPath,
	)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate reference: %v\n%s", err, out)
	}

	refDuration := ffprobeDuration(t, refPath)
	t.Logf("reference fMP4 duration: %.3fs", refDuration)

	fc := astiav.AllocFormatContext()
	if fc == nil {
		t.Fatal("alloc format context")
	}
	defer fc.Free()

	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	fc.OpenInput(inputPath, nil, inputDict)
	defer fc.CloseInput()
	fc.FindStreamInfo(nil)

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

	fmp4Muxer, _ := NewFragmentedMuxer(muxOpts)

	pkt := astiav.AllocPacket()
	defer pkt.Free()

	audioOutTB := astiav.NewRational(1, 48000)

	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		switch pkt.StreamIndex() {
		case videoStream.Index():
			fmp4Muxer.WriteVideoPacket(pkt) //nolint:errcheck
		default:
			if audioStream != nil && pkt.StreamIndex() == audioStream.Index() {
				pkt.RescaleTs(audioStream.TimeBase(), audioOutTB)
				fmp4Muxer.WriteAudioPacket(pkt) //nolint:errcheck
			}
		}
		pkt.Unref()
	}
	fmp4Muxer.Close()

	videoSegs, _ := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))
	var totalMeasured float64
	for _, seg := range videoSegs {
		d := ffprobeDuration(t, seg)
		if d > 0 {
			totalMeasured += d
		}
	}

	t.Logf("our total measured video duration: %.3fs (expected ~5s)", totalMeasured)

	if totalMeasured > 0 && math.Abs(totalMeasured-5.0) > 1.0 {
		t.Errorf("SPEED BUG: total measured duration %.3fs deviates from 5s by more than 1s", totalMeasured)
	}

	initData, _ := os.ReadFile(filepath.Join(ourDir, "init_video.mp4"))
	if len(initData) > 0 {
		errs := validate.ValidateFMP4Init(initData)
		for _, e := range errs {
			t.Errorf("init validation: %v", e)
		}
	}
}

func TestFMP4Conformance_SegmentValidation(t *testing.T) {
	skipIfNoFFmpegBinary(t)

	tmpRoot := t.TempDir()
	inputPath := filepath.Join(tmpRoot, "input.mp4")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
		"-g", "25",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "mp4", "-y", inputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate input: %v\n%s", err, out)
	}

	fc := astiav.AllocFormatContext()
	if fc == nil {
		t.Fatal("alloc format context")
	}
	defer fc.Free()

	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	fc.OpenInput(inputPath, nil, inputDict)
	defer fc.CloseInput()
	fc.FindStreamInfo(nil)

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

	fmp4Muxer, _ := NewFragmentedMuxer(muxOpts)
	pkt := astiav.AllocPacket()
	defer pkt.Free()

	audioOutTB := astiav.NewRational(1, 48000)

	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		switch pkt.StreamIndex() {
		case videoStream.Index():
			fmp4Muxer.WriteVideoPacket(pkt) //nolint:errcheck
		default:
			if audioStream != nil && pkt.StreamIndex() == audioStream.Index() {
				pkt.RescaleTs(audioStream.TimeBase(), audioOutTB)
				fmp4Muxer.WriteAudioPacket(pkt) //nolint:errcheck
			}
		}
		pkt.Unref()
	}
	fmp4Muxer.Close()

	videoSegs, _ := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))
	audioSegs, _ := filepath.Glob(filepath.Join(ourDir, "audio_*.m4s"))

	t.Logf("produced %d video segments, %d audio segments", len(videoSegs), len(audioSegs))

	for _, seg := range videoSegs {
		data, _ := os.ReadFile(seg)
		errs := validate.ValidateFMP4Segment(data)
		for _, e := range errs {
			t.Errorf("video segment %s: %v", filepath.Base(seg), e)
		}
	}

	for _, seg := range audioSegs {
		data, _ := os.ReadFile(seg)
		errs := validate.ValidateFMP4Segment(data)
		for _, e := range errs {
			t.Errorf("audio segment %s: %v", filepath.Base(seg), e)
		}
	}
}
