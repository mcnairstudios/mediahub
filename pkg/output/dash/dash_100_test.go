//go:build cgo

package dash

import (
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoTools100(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

type d100ProbeFormat struct {
	Duration string `json:"duration"`
	Size     string `json:"size"`
}

type d100ProbeStream struct {
	CodecName  string `json:"codec_name"`
	CodecType  string `json:"codec_type"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	SampleRate string `json:"sample_rate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
}

type d100ProbePacket struct {
	StreamIndex  int    `json:"stream_index"`
	PtsTime      string `json:"pts_time"`
	DtsTime      string `json:"dts_time"`
	DurationTime string `json:"duration_time"`
	Size         string `json:"size"`
	Flags        string `json:"flags"`
}

func d100ProbeFormatOf(t *testing.T, path string) d100ProbeFormat {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "format=duration,size",
		"-of", "json", path).Output()
	require.NoError(t, err)
	var r struct{ Format d100ProbeFormat }
	require.NoError(t, json.Unmarshal(out, &r))
	return r.Format
}

func d100ProbeStreamsOf(t *testing.T, path string) []d100ProbeStream {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "stream=codec_name,codec_type,width,height,sample_rate,channels",
		"-of", "json", path).Output()
	require.NoError(t, err)
	var r struct{ Streams []d100ProbeStream }
	require.NoError(t, json.Unmarshal(out, &r))
	return r.Streams
}

func d100ProbePacketsOf(t *testing.T, path string) []d100ProbePacket {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "packet=stream_index,pts_time,dts_time,duration_time,size,flags",
		"-of", "json", path).Output()
	require.NoError(t, err)
	var r struct{ Packets []d100ProbePacket }
	require.NoError(t, json.Unmarshal(out, &r))
	return r.Packets
}

func d100ParseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func d100ParseInt(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func d100GenerateAndMux(t *testing.T, videoCodec string, includeAudio bool) (ourDir, refDir string) {
	t.Helper()
	tmpRoot := t.TempDir()
	refDir = filepath.Join(tmpRoot, "ref")
	ourDir = filepath.Join(tmpRoot, "ours")
	require.NoError(t, os.MkdirAll(refDir, 0755))
	require.NoError(t, os.MkdirAll(ourDir, 0755))

	srcPath := generateFATESourceDASH(t, tmpRoot, 5, videoCodec, includeAudio)
	generateFATEReferenceDASH(t, srcPath, refDir)
	muxThroughOurDASH(t, srcPath, ourDir, includeAudio)
	return ourDir, refDir
}

func d100OurVideoFiles(t *testing.T, dir string) (initPath string, segPaths []string) {
	t.Helper()
	initPath = filepath.Join(dir, "init_video.mp4")
	require.FileExists(t, initPath)
	segs, err := filepath.Glob(filepath.Join(dir, "video_*.m4s"))
	require.NoError(t, err)
	sort.Strings(segs)
	return initPath, segs
}

func d100OurAudioFiles(t *testing.T, dir string) (initPath string, segPaths []string) {
	t.Helper()
	initPath = filepath.Join(dir, "init_audio.mp4")
	require.FileExists(t, initPath)
	segs, err := filepath.Glob(filepath.Join(dir, "audio_*.m4s"))
	require.NoError(t, err)
	sort.Strings(segs)
	return initPath, segs
}

func d100RefVideoFiles(t *testing.T, dir string) (initPath string, segPaths []string) {
	t.Helper()
	initPath = filepath.Join(dir, "init-0.m4s")
	require.FileExists(t, initPath)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "chunk-0-") {
			segPaths = append(segPaths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(segPaths)
	return initPath, segPaths
}

func d100RefAudioFiles(t *testing.T, dir string) (initPath string, segPaths []string) {
	t.Helper()
	initPath = filepath.Join(dir, "init-1.m4s")
	require.FileExists(t, initPath)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "chunk-1-") {
			segPaths = append(segPaths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(segPaths)
	return initPath, segPaths
}

func d100ContainsBox(data []byte, boxType string) bool {
	needle := []byte(boxType)
	for i := 0; i <= len(data)-4; i++ {
		if data[i] == needle[0] && data[i+1] == needle[1] &&
			data[i+2] == needle[2] && data[i+3] == needle[3] {
			return true
		}
	}
	return false
}

func d100CombineSegments(t *testing.T, initPath string, segPaths []string) string {
	t.Helper()
	dir := filepath.Dir(initPath)
	out := filepath.Join(dir, "combined_probe.mp4")
	initData, err := os.ReadFile(initPath)
	require.NoError(t, err)
	var combined []byte
	combined = append(combined, initData...)
	for _, seg := range segPaths {
		data, err := os.ReadFile(seg)
		require.NoError(t, err)
		combined = append(combined, data...)
	}
	require.NoError(t, os.WriteFile(out, combined, 0644))
	return out
}

func d100ExtractBoxes(data []byte) []string {
	var boxes []string
	pos := 0
	for pos+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		if size < 8 || pos+size > len(data) {
			break
		}
		boxes = append(boxes, string(data[pos+4:pos+8]))
		pos += size
	}
	return boxes
}

type d100MPD struct {
	XMLName xml.Name    `xml:"MPD"`
	Type    string      `xml:"type,attr"`
	Periods []d100Period `xml:"Period"`
}

type d100Period struct {
	AdaptationSets []d100AS `xml:"AdaptationSet"`
}

type d100AS struct {
	MimeType         string       `xml:"mimeType,attr"`
	ContentType      string       `xml:"contentType,attr"`
	SegmentAlignment string       `xml:"segmentAlignment,attr"`
	StartWithSAP     string       `xml:"startWithSAP,attr"`
	Representations  []d100Rep    `xml:"Representation"`
}

type d100Rep struct {
	ID                string          `xml:"id,attr"`
	MimeType          string          `xml:"mimeType,attr"`
	Codecs            string          `xml:"codecs,attr"`
	Bandwidth         int             `xml:"bandwidth,attr"`
	Width             int             `xml:"width,attr"`
	Height            int             `xml:"height,attr"`
	AudioSamplingRate int             `xml:"audioSamplingRate,attr"`
	SegmentTemplate   *d100SegTmpl    `xml:"SegmentTemplate"`
	AudioChannelCfg   *d100AudioCh    `xml:"AudioChannelConfiguration"`
}

type d100SegTmpl struct {
	Media          string `xml:"media,attr"`
	Initialization string `xml:"initialization,attr"`
	StartNumber    int    `xml:"startNumber,attr"`
	Timescale      int    `xml:"timescale,attr"`
	Duration       int    `xml:"duration,attr"`
}

type d100AudioCh struct {
	SchemeIdUri string `xml:"schemeIdUri,attr"`
	Value       string `xml:"value,attr"`
}

func d100ParseMPD(t *testing.T, dir string) d100MPD {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "manifest.mpd"))
	require.NoError(t, err)
	var doc d100MPD
	require.NoError(t, xml.Unmarshal(data, &doc))
	return doc
}

// --- 50 tests ---

func TestD100_01_VideoInitHasFtypBox(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, _ := d100OurVideoFiles(t, ourDir)
	data, err := os.ReadFile(initPath)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(data), 8)
	assert.Equal(t, "ftyp", string(data[4:8]))
}

func TestD100_02_VideoInitHasMoovBox(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, _ := d100OurVideoFiles(t, ourDir)
	data, err := os.ReadFile(initPath)
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "moov"))
}

func TestD100_03_VideoInitHasAvc1Box(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, _ := d100OurVideoFiles(t, ourDir)
	data, err := os.ReadFile(initPath)
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "avc1"))
}

func TestD100_04_VideoInitHasAvcCBox(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, _ := d100OurVideoFiles(t, ourDir)
	data, err := os.ReadFile(initPath)
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "avcC"))
}

func TestD100_05_H265InitHasHvc1OrHev1Box(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h265", false)
	initPath, _ := d100OurVideoFiles(t, ourDir)
	data, err := os.ReadFile(initPath)
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "hvc1") || d100ContainsBox(data, "hev1"),
		"HEVC init should contain hvc1 or hev1 box")
}

func TestD100_06_AudioInitHasMp4aBox(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", true)
	initPath, _ := d100OurAudioFiles(t, ourDir)
	data, err := os.ReadFile(initPath)
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "mp4a"))
}

func TestD100_07_AudioInitHasEsdsBox(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", true)
	initPath, _ := d100OurAudioFiles(t, ourDir)
	data, err := os.ReadFile(initPath)
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "esds"))
}

func TestD100_08_VideoSegmentHasMoofBox(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	require.Greater(t, len(segs), 0)
	data, err := os.ReadFile(segs[0])
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "moof"))
}

func TestD100_09_VideoSegmentHasMdatBox(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	require.Greater(t, len(segs), 0)
	data, err := os.ReadFile(segs[0])
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "mdat"))
}

func TestD100_10_VideoSegmentHasTrafBox(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	require.Greater(t, len(segs), 0)
	data, err := os.ReadFile(segs[0])
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "traf"))
}

func TestD100_11_VideoSegmentHasTfhdBox(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	require.Greater(t, len(segs), 0)
	data, err := os.ReadFile(segs[0])
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "tfhd"))
}

func TestD100_12_VideoSegmentHasTrunBox(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	require.Greater(t, len(segs), 0)
	data, err := os.ReadFile(segs[0])
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "trun"))
}

func TestD100_13_VideoSegmentHasTfdtBox(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	require.Greater(t, len(segs), 0)
	data, err := os.ReadFile(segs[0])
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "tfdt"))
}

func TestD100_14_AudioSegmentHasMoofAndMdat(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", true)
	_, segs := d100OurAudioFiles(t, ourDir)
	require.Greater(t, len(segs), 0)
	data, err := os.ReadFile(segs[0])
	require.NoError(t, err)
	assert.True(t, d100ContainsBox(data, "moof"))
	assert.True(t, d100ContainsBox(data, "mdat"))
}

func TestD100_15_VideoSegmentCountWithin30PctOfRef(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, refDir := d100GenerateAndMux(t, "h264", false)
	_, ourSegs := d100OurVideoFiles(t, ourDir)
	_, refSegs := d100RefVideoFiles(t, refDir)

	pctDiff := math.Abs(float64(len(ourSegs)-len(refSegs))) / float64(len(refSegs)) * 100
	t.Logf("video segments: ours=%d ref=%d (%.1f%% diff)", len(ourSegs), len(refSegs), pctDiff)
	assert.Less(t, pctDiff, 30.0)
}

func TestD100_16_AudioSegmentCountWithin30PctOfRef(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, refDir := d100GenerateAndMux(t, "h264", true)
	_, ourSegs := d100OurAudioFiles(t, ourDir)
	_, refSegs := d100RefAudioFiles(t, refDir)

	pctDiff := math.Abs(float64(len(ourSegs)-len(refSegs))) / float64(len(refSegs)) * 100
	t.Logf("audio segments: ours=%d ref=%d (%.1f%% diff)", len(ourSegs), len(refSegs), pctDiff)
	assert.Less(t, pctDiff, 30.0)
}

func TestD100_17_VideoInitSizeWithin30PctOfRef(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, refDir := d100GenerateAndMux(t, "h264", false)
	ourInit, _ := d100OurVideoFiles(t, ourDir)
	refInit, _ := d100RefVideoFiles(t, refDir)

	ourStat, _ := os.Stat(ourInit)
	refStat, _ := os.Stat(refInit)
	pctDiff := math.Abs(float64(ourStat.Size()-refStat.Size())) / float64(refStat.Size()) * 100
	t.Logf("video init size: ours=%d ref=%d (%.1f%% diff)", ourStat.Size(), refStat.Size(), pctDiff)
	assert.Less(t, pctDiff, 30.0)
}

func TestD100_18_AudioInitSizeWithin30PctOfRef(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, refDir := d100GenerateAndMux(t, "h264", true)
	ourInit, _ := d100OurAudioFiles(t, ourDir)
	refInit, _ := d100RefAudioFiles(t, refDir)

	ourStat, _ := os.Stat(ourInit)
	refStat, _ := os.Stat(refInit)
	pctDiff := math.Abs(float64(ourStat.Size()-refStat.Size())) / float64(refStat.Size()) * 100
	t.Logf("audio init size: ours=%d ref=%d (%.1f%% diff)", ourStat.Size(), refStat.Size(), pctDiff)
	assert.Less(t, pctDiff, 30.0)
}

func TestD100_19_VideoTotalSizeWithin30PctOfRef(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, refDir := d100GenerateAndMux(t, "h264", false)
	_, ourSegs := d100OurVideoFiles(t, ourDir)
	_, refSegs := d100RefVideoFiles(t, refDir)

	var ourTotal, refTotal int64
	for _, s := range ourSegs {
		info, _ := os.Stat(s)
		ourTotal += info.Size()
	}
	for _, s := range refSegs {
		info, _ := os.Stat(s)
		refTotal += info.Size()
	}
	pctDiff := math.Abs(float64(ourTotal-refTotal)) / float64(refTotal) * 100
	t.Logf("video total size: ours=%d ref=%d (%.1f%% diff)", ourTotal, refTotal, pctDiff)
	assert.Less(t, pctDiff, 30.0)
}

func TestD100_20_VideoDurationWithin5PctOfSource(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, segs := d100OurVideoFiles(t, ourDir)
	combined := d100CombineSegments(t, initPath, segs)
	dur := d100ParseFloat(d100ProbeFormatOf(t, combined).Duration)
	t.Logf("video duration: %.3fs (source=5s)", dur)
	pctDiff := math.Abs(dur-5.0) / 5.0 * 100
	assert.Less(t, pctDiff, 5.0)
}

func TestD100_21_AudioDurationWithin5PctOfSource(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", true)
	initPath, segs := d100OurAudioFiles(t, ourDir)
	combined := d100CombineSegments(t, initPath, segs)
	dur := d100ParseFloat(d100ProbeFormatOf(t, combined).Duration)
	t.Logf("audio duration: %.3fs (source=5s)", dur)
	pctDiff := math.Abs(dur-5.0) / 5.0 * 100
	assert.Less(t, pctDiff, 5.0)
}

func TestD100_22_VideoFrameCountWithin5PctOfRef(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, refDir := d100GenerateAndMux(t, "h264", false)

	ourInit, ourSegs := d100OurVideoFiles(t, ourDir)
	ourCombined := d100CombineSegments(t, ourInit, ourSegs)
	refInit, refSegs := d100RefVideoFiles(t, refDir)
	refCombined := d100CombineSegments(t, refInit, refSegs)

	ourPkts := d100ProbePacketsOf(t, ourCombined)
	refPkts := d100ProbePacketsOf(t, refCombined)

	pctDiff := math.Abs(float64(len(ourPkts)-len(refPkts))) / float64(len(refPkts)) * 100
	t.Logf("video packets: ours=%d ref=%d (%.1f%% diff)", len(ourPkts), len(refPkts), pctDiff)
	assert.Less(t, pctDiff, 5.0)
}

func TestD100_23_SpeedValidation5sSource(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, segs := d100OurVideoFiles(t, ourDir)
	combined := d100CombineSegments(t, initPath, segs)
	dur := d100ParseFloat(d100ProbeFormatOf(t, combined).Duration)
	t.Logf("speed check: %.3fs (expected 4.5-5.5s)", dur)
	assert.Greater(t, dur, 4.5, "too short -- possible double-speed")
	assert.Less(t, dur, 5.5, "too long -- possible half-speed")
}

func TestD100_24_VideoCodecCorrectInInit(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, _ := d100OurVideoFiles(t, ourDir)
	streams := d100ProbeStreamsOf(t, initPath)
	require.GreaterOrEqual(t, len(streams), 1)
	assert.Equal(t, "h264", streams[0].CodecName)
}

func TestD100_25_H265VideoCodecCorrectInInit(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h265", false)
	initPath, _ := d100OurVideoFiles(t, ourDir)
	streams := d100ProbeStreamsOf(t, initPath)
	require.GreaterOrEqual(t, len(streams), 1)
	assert.Equal(t, "hevc", streams[0].CodecName)
}

func TestD100_26_AudioCodecCorrectInInit(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", true)
	initPath, _ := d100OurAudioFiles(t, ourDir)
	streams := d100ProbeStreamsOf(t, initPath)
	require.GreaterOrEqual(t, len(streams), 1)
	assert.Equal(t, "aac", streams[0].CodecName)
}

func TestD100_27_VideoResolutionCorrectInInit(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, _ := d100OurVideoFiles(t, ourDir)
	streams := d100ProbeStreamsOf(t, initPath)
	require.GreaterOrEqual(t, len(streams), 1)
	assert.Equal(t, 640, streams[0].Width)
	assert.Equal(t, 360, streams[0].Height)
}

func TestD100_28_VideoSegmentSizesReasonable(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	for _, seg := range segs {
		info, _ := os.Stat(seg)
		assert.Greater(t, info.Size(), int64(100),
			"segment %s too small", filepath.Base(seg))
		assert.Less(t, info.Size(), int64(10*1024*1024),
			"segment %s too large", filepath.Base(seg))
	}
}

func TestD100_29_AudioSegmentSizesReasonable(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", true)
	_, segs := d100OurAudioFiles(t, ourDir)
	for _, seg := range segs {
		info, _ := os.Stat(seg)
		assert.Greater(t, info.Size(), int64(50),
			"segment %s too small", filepath.Base(seg))
		assert.Less(t, info.Size(), int64(1024*1024),
			"segment %s too large", filepath.Base(seg))
	}
}

func TestD100_30_VideoSegmentPTSMonotonic(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	initPath, _ := d100OurVideoFiles(t, ourDir)

	for i, seg := range segs {
		combined := d100CombineSegments(t, initPath, []string{seg})
		pkts := d100ProbePacketsOf(t, combined)
		var prevDTS float64 = -1
		for j, p := range pkts {
			dts := d100ParseFloat(p.DtsTime)
			if j > 0 {
				assert.GreaterOrEqual(t, dts, prevDTS,
					"segment %d: DTS not monotonic at packet %d", i, j)
			}
			prevDTS = dts
		}
	}
}

func TestD100_31_VideoInitTopLevelBoxesFtypMoov(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, _ := d100OurVideoFiles(t, ourDir)
	data, err := os.ReadFile(initPath)
	require.NoError(t, err)
	boxes := d100ExtractBoxes(data)
	require.GreaterOrEqual(t, len(boxes), 2)
	assert.Equal(t, "ftyp", boxes[0])
	assert.Equal(t, "moov", boxes[1])
}

func TestD100_32_VideoSegmentTopLevelBoxesMoofMdat(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	require.Greater(t, len(segs), 0)
	data, err := os.ReadFile(segs[0])
	require.NoError(t, err)
	boxes := d100ExtractBoxes(data)
	require.GreaterOrEqual(t, len(boxes), 2)
	assert.Equal(t, "moof", boxes[0])
	assert.Equal(t, "mdat", boxes[1])
}

func TestD100_33_AllVideoSegmentsHaveMoofMdat(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	for _, seg := range segs {
		data, err := os.ReadFile(seg)
		require.NoError(t, err)
		assert.True(t, d100ContainsBox(data, "moof"), "%s missing moof", filepath.Base(seg))
		assert.True(t, d100ContainsBox(data, "mdat"), "%s missing mdat", filepath.Base(seg))
	}
}

func TestD100_34_AllAudioSegmentsHaveMoofMdat(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", true)
	_, segs := d100OurAudioFiles(t, ourDir)
	for _, seg := range segs {
		data, err := os.ReadFile(seg)
		require.NoError(t, err)
		assert.True(t, d100ContainsBox(data, "moof"), "%s missing moof", filepath.Base(seg))
		assert.True(t, d100ContainsBox(data, "mdat"), "%s missing mdat", filepath.Base(seg))
	}
}

func TestD100_35_VideoSegmentSizeConsistency(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	if len(segs) < 2 {
		t.Skip("need at least 2 segments")
	}

	var sizes []int64
	for _, seg := range segs {
		info, _ := os.Stat(seg)
		sizes = append(sizes, info.Size())
	}

	inner := sizes
	if len(inner) > 2 {
		inner = inner[:len(inner)-1]
	}
	var total int64
	for _, s := range inner {
		total += s
	}
	avg := float64(total) / float64(len(inner))
	for _, s := range inner {
		ratio := float64(s) / avg
		assert.Greater(t, ratio, 0.3, "segment %d too small vs avg %.0f", s, avg)
		assert.Less(t, ratio, 3.0, "segment %d too large vs avg %.0f", s, avg)
	}
}

func TestD100_36_AvgVideoSegmentSizeWithin30PctOfRef(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, refDir := d100GenerateAndMux(t, "h264", false)
	_, ourSegs := d100OurVideoFiles(t, ourDir)
	_, refSegs := d100RefVideoFiles(t, refDir)

	avgSize := func(paths []string) float64 {
		var total int64
		for _, p := range paths {
			info, _ := os.Stat(p)
			total += info.Size()
		}
		return float64(total) / float64(len(paths))
	}

	ourAvg := avgSize(ourSegs)
	refAvg := avgSize(refSegs)
	pctDiff := math.Abs(ourAvg-refAvg) / refAvg * 100
	t.Logf("avg video segment size: ours=%.0f ref=%.0f (%.1f%% diff)", ourAvg, refAvg, pctDiff)
	assert.Less(t, pctDiff, 30.0)
}

func TestD100_37_H265VideoDecodable(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h265", false)
	initPath, segs := d100OurVideoFiles(t, ourDir)
	combined := d100CombineSegments(t, initPath, segs)

	cmd := exec.Command("ffmpeg", "-v", "error", "-i", combined, "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	errText := strings.TrimSpace(string(out))
	if err != nil {
		t.Fatalf("H265 decode failed: %v\n%s", err, errText)
	}
	assert.Empty(t, errText, "H265 should decode with zero errors")
}

func TestD100_38_H265DurationWithin5Pct(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h265", false)
	initPath, segs := d100OurVideoFiles(t, ourDir)
	combined := d100CombineSegments(t, initPath, segs)
	dur := d100ParseFloat(d100ProbeFormatOf(t, combined).Duration)
	pctDiff := math.Abs(dur-5.0) / 5.0 * 100
	t.Logf("H265 duration: %.3fs (%.1f%% from 5s)", dur, pctDiff)
	assert.Less(t, pctDiff, 5.0)
}

func TestD100_39_H265FrameCountWithin5PctOfRef(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, refDir := d100GenerateAndMux(t, "h265", false)

	ourInit, ourSegs := d100OurVideoFiles(t, ourDir)
	ourCombined := d100CombineSegments(t, ourInit, ourSegs)
	refInit, refSegs := d100RefVideoFiles(t, refDir)
	refCombined := d100CombineSegments(t, refInit, refSegs)

	ourPkts := d100ProbePacketsOf(t, ourCombined)
	refPkts := d100ProbePacketsOf(t, refCombined)

	pctDiff := math.Abs(float64(len(ourPkts)-len(refPkts))) / float64(len(refPkts)) * 100
	t.Logf("H265 packets: ours=%d ref=%d (%.1f%% diff)", len(ourPkts), len(refPkts), pctDiff)
	assert.Less(t, pctDiff, 5.0)
}

func TestD100_40_VideoInitNotEmpty(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, _ := d100OurVideoFiles(t, ourDir)
	info, err := os.Stat(initPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(50))
}

func TestD100_41_AudioInitNotEmpty(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", true)
	initPath, _ := d100OurAudioFiles(t, ourDir)
	info, err := os.Stat(initPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(50))
}

func TestD100_42_AtLeastOneVideoSegmentProduced(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	_, segs := d100OurVideoFiles(t, ourDir)
	assert.GreaterOrEqual(t, len(segs), 1)
}

func TestD100_43_AtLeastOneAudioSegmentProduced(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", true)
	_, segs := d100OurAudioFiles(t, ourDir)
	assert.GreaterOrEqual(t, len(segs), 1)
}

func TestD100_44_VideoOnlyProducesNoAudioFiles(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	audioInit := filepath.Join(ourDir, "init_audio.mp4")
	_, err := os.Stat(audioInit)
	assert.True(t, os.IsNotExist(err), "video-only should not produce audio init")

	audioSegs, _ := filepath.Glob(filepath.Join(ourDir, "audio_*.m4s"))
	assert.Len(t, audioSegs, 0, "video-only should not produce audio segments")
}

func TestD100_45_EachVideoSegmentDecodableAlone(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, segs := d100OurVideoFiles(t, ourDir)
	for i, seg := range segs {
		combined := d100CombineSegments(t, initPath, []string{seg})
		cmd := exec.Command("ffmpeg", "-v", "error", "-i", combined, "-f", "null", "-")
		out, err := cmd.CombinedOutput()
		errText := strings.TrimSpace(string(out))
		if err != nil {
			t.Errorf("segment %d decode failed: %v\n%s", i, err, errText)
		}
	}
}

func TestD100_46_EachAudioSegmentDecodableAlone(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", true)
	initPath, segs := d100OurAudioFiles(t, ourDir)
	for i, seg := range segs {
		combined := d100CombineSegments(t, initPath, []string{seg})
		cmd := exec.Command("ffmpeg", "-v", "error", "-i", combined, "-f", "null", "-")
		out, err := cmd.CombinedOutput()
		errText := strings.TrimSpace(string(out))
		if err != nil {
			t.Errorf("audio segment %d decode failed: %v\n%s", i, err, errText)
		}
	}
}

func TestD100_47_VideoOnlyDecodable(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, _ := d100GenerateAndMux(t, "h264", false)
	initPath, segs := d100OurVideoFiles(t, ourDir)
	combined := d100CombineSegments(t, initPath, segs)

	cmd := exec.Command("ffmpeg", "-v", "error", "-i", combined, "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	errText := strings.TrimSpace(string(out))
	if err != nil {
		t.Fatalf("video-only decode failed: %v\n%s", err, errText)
	}
	assert.Empty(t, errText)
}

func TestD100_48_AudioCodecMatchesRef(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, refDir := d100GenerateAndMux(t, "h264", true)
	ourInit, _ := d100OurAudioFiles(t, ourDir)
	refInit, _ := d100RefAudioFiles(t, refDir)

	ourStreams := d100ProbeStreamsOf(t, ourInit)
	refStreams := d100ProbeStreamsOf(t, refInit)
	require.GreaterOrEqual(t, len(ourStreams), 1)
	require.GreaterOrEqual(t, len(refStreams), 1)
	assert.Equal(t, refStreams[0].CodecName, ourStreams[0].CodecName)
}

func TestD100_49_VideoCodecMatchesRef(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, refDir := d100GenerateAndMux(t, "h264", false)
	ourInit, _ := d100OurVideoFiles(t, ourDir)
	refInit, _ := d100RefVideoFiles(t, refDir)

	ourStreams := d100ProbeStreamsOf(t, ourInit)
	refStreams := d100ProbeStreamsOf(t, refInit)
	require.GreaterOrEqual(t, len(ourStreams), 1)
	require.GreaterOrEqual(t, len(refStreams), 1)
	assert.Equal(t, refStreams[0].CodecName, ourStreams[0].CodecName)
}

func TestD100_50_ResolutionMatchesRef(t *testing.T) {
	skipIfNoTools100(t)
	ourDir, refDir := d100GenerateAndMux(t, "h264", false)
	ourInit, _ := d100OurVideoFiles(t, ourDir)
	refInit, _ := d100RefVideoFiles(t, refDir)

	ourStreams := d100ProbeStreamsOf(t, ourInit)
	refStreams := d100ProbeStreamsOf(t, refInit)
	require.GreaterOrEqual(t, len(ourStreams), 1)
	require.GreaterOrEqual(t, len(refStreams), 1)
	assert.Equal(t, refStreams[0].Width, ourStreams[0].Width)
	assert.Equal(t, refStreams[0].Height, ourStreams[0].Height)
}
