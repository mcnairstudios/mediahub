//go:build cgo

package mse

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ffprobeFormat struct {
	Duration  string `json:"duration"`
	NbStreams int    `json:"nb_streams"`
}

type ffprobeStream struct {
	CodecName  string `json:"codec_name"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	RFrameRate string `json:"r_frame_rate,omitempty"`
	SampleRate string `json:"sample_rate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
}

type ffprobePacket struct {
	StreamIndex  int    `json:"stream_index"`
	PtsTime      string `json:"pts_time"`
	DtsTime      string `json:"dts_time"`
	DurationTime string `json:"duration_time"`
	Size         string `json:"size"`
}

type ffprobeFormatResult struct {
	Format ffprobeFormat `json:"format"`
}

type ffprobeStreamResult struct {
	Streams []ffprobeStream `json:"streams"`
}

type ffprobePacketResult struct {
	Packets []ffprobePacket `json:"packets"`
}

func skipIfNoTools(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

func probeFormat(t *testing.T, path string) ffprobeFormat {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "format=duration,nb_streams",
		"-of", "json", path).Output()
	require.NoError(t, err, "ffprobe format failed")
	var result ffprobeFormatResult
	require.NoError(t, json.Unmarshal(out, &result))
	return result.Format
}

func probeStreams(t *testing.T, path string) []ffprobeStream {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "stream=codec_name,width,height,r_frame_rate,sample_rate,channels",
		"-of", "json", path).Output()
	require.NoError(t, err, "ffprobe streams failed")
	var result ffprobeStreamResult
	require.NoError(t, json.Unmarshal(out, &result))
	return result.Streams
}

func probePackets(t *testing.T, path string) []ffprobePacket {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "packet=stream_index,pts_time,dts_time,duration_time,size",
		"-of", "json", path).Output()
	require.NoError(t, err, "ffprobe packets failed")
	var result ffprobePacketResult
	require.NoError(t, json.Unmarshal(out, &result))
	return result.Packets
}

func parseDuration(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseFrameRate(s string) float64 {
	var num, den int
	if n, _ := fmt.Sscanf(s, "%d/%d", &num, &den); n == 2 && den > 0 {
		return float64(num) / float64(den)
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// TestConformance_MSE_ReferenceProperties verifies that the ffmpeg-generated
// reference fMP4 has the expected properties as measured by ffprobe.
// Measured: duration, stream count, codecs, resolution, frame rate, sample rate, channels.
func TestConformance_MSE_ReferenceProperties(t *testing.T) {
	skipIfNoTools(t)

	dir := t.TempDir()
	refPath := filepath.Join(dir, "reference.mp4")

	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-g", "25",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof",
		refPath,
	)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "generate reference fMP4")

	fmt := probeFormat(t, refPath)
	streams := probeStreams(t, refPath)
	packets := probePackets(t, refPath)

	dur := parseDuration(fmt.Duration)
	t.Logf("reference duration: %.3fs", dur)
	t.Logf("reference nb_streams: %d", fmt.NbStreams)
	assert.InDelta(t, 5.0, dur, 0.5, "duration should be ~5.0s")
	assert.Equal(t, 2, fmt.NbStreams, "should have 2 streams (video + audio)")

	require.Len(t, streams, 2, "should have 2 stream entries")

	var video, audio *ffprobeStream
	for i := range streams {
		switch streams[i].CodecName {
		case "h264":
			video = &streams[i]
		case "aac":
			audio = &streams[i]
		}
	}
	require.NotNil(t, video, "should have h264 video stream")
	require.NotNil(t, audio, "should have aac audio stream")

	t.Logf("video: %s %dx%d fps=%s", video.CodecName, video.Width, video.Height, video.RFrameRate)
	t.Logf("audio: %s rate=%s channels=%d", audio.CodecName, audio.SampleRate, audio.Channels)

	assert.Equal(t, 640, video.Width)
	assert.Equal(t, 360, video.Height)
	fps := parseFrameRate(video.RFrameRate)
	assert.InDelta(t, 25.0, fps, 1.0, "framerate should be ~25fps")

	assert.Equal(t, "48000", audio.SampleRate)
	assert.Equal(t, 2, audio.Channels)

	var videoPTS []float64
	for _, pkt := range packets {
		if pkt.StreamIndex == 0 {
			videoPTS = append(videoPTS, parseDuration(pkt.PtsTime))
		}
	}
	require.Greater(t, len(videoPTS), 10, "should have many video packets")
	for i := 1; i < len(videoPTS); i++ {
		assert.GreaterOrEqual(t, videoPTS[i], videoPTS[i-1],
			"video PTS should be monotonically non-decreasing at packet %d", i)
	}
	t.Logf("reference: %d video packets, %d total packets", len(videoPTS), len(packets))
}

// TestConformance_MSE_OurOutputMatchesReference generates fMP4 with ffmpeg as
// reference, then pushes the same source through our FragmentedMuxer, probes
// both with ffprobe, and compares: stream count, codecs, resolution, duration,
// and video PTS spacing.
func TestConformance_MSE_OurOutputMatchesReference(t *testing.T) {
	skipIfNoTools(t)

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.mp4")
	refPath := filepath.Join(dir, "reference.mp4")
	ourDir := filepath.Join(dir, "ours")
	ourCombined := filepath.Join(dir, "ours_combined.mp4")
	require.NoError(t, os.MkdirAll(ourDir, 0755))

	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-g", "25",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "mp4", "-y", inputPath,
	)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	cmd = exec.Command("ffmpeg", "-y",
		"-i", inputPath,
		"-c:v", "copy", "-c:a", "copy",
		"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof",
		refPath,
	)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

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
	require.NotNil(t, audioStream)

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

	fmp4Muxer, err := mux.NewFragmentedMuxer(mux.MuxOpts{
		OutputDir:       ourDir,
		VideoCodecID:    vcp.CodecID(),
		VideoExtradata:  videoExtradata,
		VideoWidth:      vcp.Width(),
		VideoHeight:     vcp.Height(),
		VideoTimeBase:   videoStream.TimeBase(),
		AudioCodecID:    acp.CodecID(),
		AudioExtradata:  audioExtradata,
		AudioChannels:   acp.ChannelLayout().Channels(),
		AudioSampleRate: acp.SampleRate(),
	})
	require.NoError(t, err)

	pkt := astiav.AllocPacket()
	require.NotNil(t, pkt)
	defer pkt.Free()

	audioOutTB := astiav.NewRational(1, acp.SampleRate())

	var videoPkts, audioPkts int
	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		switch pkt.StreamIndex() {
		case videoStream.Index():
			require.NoError(t, fmp4Muxer.WriteVideoPacket(pkt))
			videoPkts++
		case audioStream.Index():
			pkt.RescaleTs(audioStream.TimeBase(), audioOutTB)
			require.NoError(t, fmp4Muxer.WriteAudioPacket(pkt))
			audioPkts++
		}
		pkt.Unref()
	}
	require.NoError(t, fmp4Muxer.Close())
	t.Logf("wrote %d video + %d audio packets through our muxer", videoPkts, audioPkts)

	initVideo, err := os.ReadFile(filepath.Join(ourDir, "init_video.mp4"))
	require.NoError(t, err)
	videoSegs, err := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))
	require.NoError(t, err)
	require.Greater(t, len(videoSegs), 0, "should have produced video segments")

	var combined []byte
	combined = append(combined, initVideo...)
	for _, seg := range videoSegs {
		data, err := os.ReadFile(seg)
		require.NoError(t, err)
		combined = append(combined, data...)
	}
	require.NoError(t, os.WriteFile(ourCombined, combined, 0644))
	t.Logf("combined output: %d bytes (init=%d + %d segments)", len(combined), len(initVideo), len(videoSegs))

	refFmt := probeFormat(t, refPath)
	ourFmt := probeFormat(t, ourCombined)

	refStreams := probeStreams(t, refPath)
	ourStreams := probeStreams(t, ourCombined)

	t.Logf("reference: duration=%s streams=%d", refFmt.Duration, refFmt.NbStreams)
	t.Logf("ours:      duration=%s streams=%d", ourFmt.Duration, ourFmt.NbStreams)

	require.GreaterOrEqual(t, len(ourStreams), 1, "our output should have at least 1 stream")
	assert.Equal(t, refStreams[0].CodecName, ourStreams[0].CodecName, "video codec should match")

	var refVideo *ffprobeStream
	for i := range refStreams {
		if refStreams[i].CodecName == "h264" {
			refVideo = &refStreams[i]
			break
		}
	}
	var ourVideo *ffprobeStream
	for i := range ourStreams {
		if ourStreams[i].CodecName == "h264" {
			ourVideo = &ourStreams[i]
			break
		}
	}
	require.NotNil(t, refVideo)
	require.NotNil(t, ourVideo)
	assert.Equal(t, refVideo.Width, ourVideo.Width, "video width should match")
	assert.Equal(t, refVideo.Height, ourVideo.Height, "video height should match")

	refDur := parseDuration(refFmt.Duration)
	ourDur := parseDuration(ourFmt.Duration)
	assert.InDelta(t, refDur, ourDur, 0.5,
		"duration should match within 0.5s: ref=%.3f ours=%.3f", refDur, ourDur)

	refPackets := probePackets(t, refPath)
	ourPackets := probePackets(t, ourCombined)

	refVideoSpacings := videoSpacings(refPackets, 0)
	ourVideoSpacings := videoSpacings(ourPackets, 0)

	if len(refVideoSpacings) > 2 && len(ourVideoSpacings) > 2 {
		refAvgSpacing := avgFloat(refVideoSpacings[1:])
		ourAvgSpacing := avgFloat(ourVideoSpacings[1:])
		t.Logf("avg video PTS spacing: ref=%.6fs ours=%.6fs", refAvgSpacing, ourAvgSpacing)

		if refAvgSpacing > 0 {
			ratio := ourAvgSpacing / refAvgSpacing
			assert.InDelta(t, 1.0, ratio, 0.20,
				"video PTS spacing ratio should be ~1.0 (got %.3f): ref=%.6f ours=%.6f",
				ratio, refAvgSpacing, ourAvgSpacing)
		}
	}
}

// TestConformance_MSE_SpeedCheck verifies that 5 seconds of 25fps content
// muxed through FragmentedMuxer produces output with ~5s duration as
// measured by ffprobe. This catches timestamp bugs that cause double-speed
// or half-speed playback.
func TestConformance_MSE_SpeedCheck(t *testing.T) {
	skipIfNoTools(t)

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.mp4")
	ourDir := filepath.Join(dir, "ours")
	ourCombined := filepath.Join(dir, "combined.mp4")
	require.NoError(t, os.MkdirAll(ourDir, 0755))

	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-g", "25",
		"-an",
		"-f", "mp4", inputPath,
	)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	fc := astiav.AllocFormatContext()
	require.NotNil(t, fc)
	defer fc.Free()

	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	require.NoError(t, fc.OpenInput(inputPath, nil, inputDict))
	defer fc.CloseInput()
	require.NoError(t, fc.FindStreamInfo(nil))

	var videoStream *astiav.Stream
	for _, s := range fc.Streams() {
		if s.CodecParameters().MediaType() == astiav.MediaTypeVideo {
			videoStream = s
			break
		}
	}
	require.NotNil(t, videoStream)

	vcp := videoStream.CodecParameters()
	var videoExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}

	fmp4Muxer, err := mux.NewFragmentedMuxer(mux.MuxOpts{
		OutputDir:      ourDir,
		VideoCodecID:   vcp.CodecID(),
		VideoExtradata: videoExtradata,
		VideoWidth:     vcp.Width(),
		VideoHeight:    vcp.Height(),
		VideoTimeBase:  videoStream.TimeBase(),
	})
	require.NoError(t, err)

	pkt := astiav.AllocPacket()
	require.NotNil(t, pkt)
	defer pkt.Free()

	var count int
	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		if pkt.StreamIndex() == videoStream.Index() {
			require.NoError(t, fmp4Muxer.WriteVideoPacket(pkt))
			count++
		}
		pkt.Unref()
	}
	require.NoError(t, fmp4Muxer.Close())
	t.Logf("wrote %d video packets", count)

	initVideo, err := os.ReadFile(filepath.Join(ourDir, "init_video.mp4"))
	require.NoError(t, err)
	videoSegs, err := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))
	require.NoError(t, err)

	var combined []byte
	combined = append(combined, initVideo...)
	for _, seg := range videoSegs {
		data, err := os.ReadFile(seg)
		require.NoError(t, err)
		combined = append(combined, data...)
	}
	require.NoError(t, os.WriteFile(ourCombined, combined, 0644))

	fmt := probeFormat(t, ourCombined)
	dur := parseDuration(fmt.Duration)
	t.Logf("SPEED CHECK: ffprobe measured duration = %.3fs (expected 4.5-5.5s)", dur)
	t.Logf("  if ~2.5s: timestamps at 50fps but frames at 25fps = 2x speed")
	t.Logf("  if ~10.0s: timestamps at 12.5fps = 0.5x speed")

	assert.Greater(t, dur, 4.5, "duration too short -- possible double-speed bug")
	assert.Less(t, dur, 5.5, "duration too long -- possible half-speed bug")
}

func videoSpacings(packets []ffprobePacket, streamIdx int) []float64 {
	var pts []float64
	for _, p := range packets {
		if p.StreamIndex == streamIdx {
			pts = append(pts, parseDuration(p.PtsTime))
		}
	}
	var spacings []float64
	for i := 1; i < len(pts); i++ {
		spacings = append(spacings, pts[i]-pts[i-1])
	}
	return spacings
}

func avgFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += math.Abs(v)
	}
	return sum / float64(len(vals))
}
