//go:build cgo

package dash

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

func skipIfNoToolsDash(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

func probeFormatDash(t *testing.T, path string) ffprobeFormat {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "format=duration,nb_streams",
		"-of", "json", path).Output()
	require.NoError(t, err, "ffprobe format failed")
	var result ffprobeFormatResult
	require.NoError(t, json.Unmarshal(out, &result))
	return result.Format
}

func probeStreamsDash(t *testing.T, path string) []ffprobeStream {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "stream=codec_name,width,height,r_frame_rate,sample_rate,channels",
		"-of", "json", path).Output()
	require.NoError(t, err, "ffprobe streams failed")
	var result ffprobeStreamResult
	require.NoError(t, json.Unmarshal(out, &result))
	return result.Streams
}

func probePacketsDash(t *testing.T, path string) []ffprobePacket {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "packet=stream_index,pts_time,dts_time,duration_time,size",
		"-of", "json", path).Output()
	require.NoError(t, err, "ffprobe packets failed")
	var result ffprobePacketResult
	require.NoError(t, json.Unmarshal(out, &result))
	return result.Packets
}

func parseDurationDash(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// TestConformance_DASH_ReferenceProperties verifies that the ffmpeg-generated
// DASH reference has the expected properties as measured by ffprobe on the
// individual init+segment files. Measured: segment count, codec, resolution.
func TestConformance_DASH_ReferenceProperties(t *testing.T) {
	skipIfNoToolsDash(t)

	dir := t.TempDir()

	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-g", "50",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "dash", "-seg_duration", "2",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		filepath.Join(dir, "manifest.mpd"),
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "ffmpeg DASH generation failed: %s", out)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var videoSegs, audioSegs []string
	for _, e := range entries {
		switch {
		case strings.HasPrefix(e.Name(), "chunk-0-"):
			videoSegs = append(videoSegs, filepath.Join(dir, e.Name()))
		case strings.HasPrefix(e.Name(), "chunk-1-"):
			audioSegs = append(audioSegs, filepath.Join(dir, e.Name()))
		}
	}

	t.Logf("reference DASH: %d video segments, %d audio segments", len(videoSegs), len(audioSegs))
	assert.GreaterOrEqual(t, len(videoSegs), 2, "should have >= 2 video segments for 5s at 2s")
	assert.GreaterOrEqual(t, len(audioSegs), 2, "should have >= 2 audio segments for 5s at 2s")

	videoInitPath := filepath.Join(dir, "init-0.m4s")
	videoInitStreams := probeStreamsDash(t, videoInitPath)
	require.GreaterOrEqual(t, len(videoInitStreams), 1)
	assert.Equal(t, "h264", videoInitStreams[0].CodecName)
	assert.Equal(t, 640, videoInitStreams[0].Width)
	assert.Equal(t, 360, videoInitStreams[0].Height)
	t.Logf("video init: codec=%s %dx%d", videoInitStreams[0].CodecName,
		videoInitStreams[0].Width, videoInitStreams[0].Height)

	audioInitPath := filepath.Join(dir, "init-1.m4s")
	audioInitStreams := probeStreamsDash(t, audioInitPath)
	require.GreaterOrEqual(t, len(audioInitStreams), 1)
	assert.Equal(t, "aac", audioInitStreams[0].CodecName)
	t.Logf("audio init: codec=%s rate=%s channels=%d",
		audioInitStreams[0].CodecName, audioInitStreams[0].SampleRate, audioInitStreams[0].Channels)

	for _, seg := range videoSegs {
		info, err := os.Stat(seg)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(100),
			"video segment %s should be > 100 bytes", filepath.Base(seg))
	}
}

// TestConformance_DASH_OurOutputMatchesReference generates a DASH reference
// with ffmpeg, then pushes the same source through our FragmentedMuxer (which
// DASH plugin wraps), concatenates init+segments, probes both with ffprobe,
// and compares codecs, resolution, and duration.
func TestConformance_DASH_OurOutputMatchesReference(t *testing.T) {
	skipIfNoToolsDash(t)

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.mp4")
	refDir := filepath.Join(dir, "ref")
	ourDir := filepath.Join(dir, "ours")
	ourCombined := filepath.Join(dir, "ours_combined.mp4")
	require.NoError(t, os.MkdirAll(refDir, 0755))
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
		"-f", "dash", "-seg_duration", "2",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		filepath.Join(refDir, "manifest.mpd"),
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "generate DASH reference: %s", out)

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
	require.Greater(t, len(videoSegs), 0)

	var combined []byte
	combined = append(combined, initVideo...)
	for _, seg := range videoSegs {
		data, err := os.ReadFile(seg)
		require.NoError(t, err)
		combined = append(combined, data...)
	}
	require.NoError(t, os.WriteFile(ourCombined, combined, 0644))

	refVideoInit := filepath.Join(refDir, "init-0.m4s")
	refEntries, _ := os.ReadDir(refDir)
	var refVideoSegs []string
	for _, e := range refEntries {
		if strings.HasPrefix(e.Name(), "chunk-0-") {
			refVideoSegs = append(refVideoSegs, filepath.Join(refDir, e.Name()))
		}
	}
	var refCombined []byte
	refInitData, err := os.ReadFile(refVideoInit)
	require.NoError(t, err)
	refCombined = append(refCombined, refInitData...)
	for _, seg := range refVideoSegs {
		data, err := os.ReadFile(seg)
		require.NoError(t, err)
		refCombined = append(refCombined, data...)
	}
	refCombinedPath := filepath.Join(dir, "ref_combined.mp4")
	require.NoError(t, os.WriteFile(refCombinedPath, refCombined, 0644))

	refStreams := probeStreamsDash(t, refCombinedPath)
	ourStreams := probeStreamsDash(t, ourCombined)

	require.GreaterOrEqual(t, len(refStreams), 1)
	require.GreaterOrEqual(t, len(ourStreams), 1)

	assert.Equal(t, refStreams[0].CodecName, ourStreams[0].CodecName, "video codec should match")
	assert.Equal(t, refStreams[0].Width, ourStreams[0].Width, "width should match")
	assert.Equal(t, refStreams[0].Height, ourStreams[0].Height, "height should match")

	t.Logf("reference video: %s %dx%d", refStreams[0].CodecName, refStreams[0].Width, refStreams[0].Height)
	t.Logf("ours video:      %s %dx%d", ourStreams[0].CodecName, ourStreams[0].Width, ourStreams[0].Height)

	refFmt := probeFormatDash(t, refCombinedPath)
	ourFmt := probeFormatDash(t, ourCombined)
	refDur := parseDurationDash(refFmt.Duration)
	ourDur := parseDurationDash(ourFmt.Duration)

	t.Logf("duration: ref=%.3fs ours=%.3fs", refDur, ourDur)
	assert.InDelta(t, refDur, ourDur, 0.5,
		"video duration should match within 0.5s")
}

// TestConformance_DASH_SpeedCheck verifies that 5 seconds of content produces
// output with ~5s duration. Catches timestamp double/half speed bugs.
func TestConformance_DASH_SpeedCheck(t *testing.T) {
	skipIfNoToolsDash(t)

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

	fmt := probeFormatDash(t, ourCombined)
	dur := parseDurationDash(fmt.Duration)
	t.Logf("SPEED CHECK (DASH): ffprobe measured duration = %.3fs (expected 4.5-5.5s)", dur)
	t.Logf("  %d video packets written, %d segments produced", count, len(videoSegs))

	assert.Greater(t, dur, 4.5, "duration too short -- possible double-speed bug")
	assert.Less(t, dur, 5.5, "duration too long -- possible half-speed bug")
}

// TestConformance_DASH_PerSegmentProbe probes each individual segment of the
// ffmpeg DASH reference to verify per-segment properties (non-empty, has video
// packets with reasonable timing).
func TestConformance_DASH_PerSegmentProbe(t *testing.T) {
	skipIfNoToolsDash(t)

	dir := t.TempDir()

	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-g", "50",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "dash", "-seg_duration", "2",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		filepath.Join(dir, "manifest.mpd"),
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "ffmpeg DASH generation failed: %s", out)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var videoSegs []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "chunk-0-") {
			videoSegs = append(videoSegs, filepath.Join(dir, e.Name()))
		}
	}
	require.GreaterOrEqual(t, len(videoSegs), 2)

	videoInitPath := filepath.Join(dir, "init-0.m4s")
	for i, seg := range videoSegs {
		combinedPath := filepath.Join(dir, "probe_seg.mp4")
		initData, err := os.ReadFile(videoInitPath)
		require.NoError(t, err)
		segData, err := os.ReadFile(seg)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(combinedPath, append(initData, segData...), 0644))

		packets := probePacketsDash(t, combinedPath)
		require.Greater(t, len(packets), 0,
			"segment %d (%s) should have packets", i, filepath.Base(seg))

		var pts []float64
		for _, p := range packets {
			pts = append(pts, parseDurationDash(p.PtsTime))
		}
		for j := 1; j < len(pts); j++ {
			assert.GreaterOrEqual(t, pts[j], pts[j-1],
				"segment %d: PTS should be monotonic at packet %d", i, j)
		}

		t.Logf("segment %d (%s): %d packets, PTS range [%.3f, %.3f]",
			i, filepath.Base(seg), len(packets), pts[0], pts[len(pts)-1])
	}
}

func videoSpacingsDash(packets []ffprobePacket, streamIdx int) []float64 {
	var pts []float64
	for _, p := range packets {
		if p.StreamIndex == streamIdx {
			pts = append(pts, parseDurationDash(p.PtsTime))
		}
	}
	var spacings []float64
	for i := 1; i < len(pts); i++ {
		spacings = append(spacings, pts[i]-pts[i-1])
	}
	return spacings
}

func avgFloatDash(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += math.Abs(v)
	}
	return sum / float64(len(vals))
}
