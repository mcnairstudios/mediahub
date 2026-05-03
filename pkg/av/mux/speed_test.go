//go:build cgo

package mux

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/asticode/go-astiav"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type speedProbeFormat struct {
	Duration string `json:"duration"`
}

type speedProbeFormatResult struct {
	Format speedProbeFormat `json:"format"`
}

func speedSkipIfNoTools(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

func speedProbe(t *testing.T, path string) float64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "json", path).Output()
	require.NoError(t, err, "ffprobe failed on %s", path)
	var result speedProbeFormatResult
	require.NoError(t, json.Unmarshal(out, &result))
	dur, err := strconv.ParseFloat(result.Format.Duration, 64)
	require.NoError(t, err, "parse duration %q", result.Format.Duration)
	return dur
}

func speedGenerateInput(t *testing.T, dir string, duration int, withAudio bool) string {
	t.Helper()
	inputPath := filepath.Join(dir, "input.mp4")
	args := []string{"-y",
		"-f", "lavfi", "-i", "testsrc2=duration=" + strconv.Itoa(duration) + ":size=640x360:rate=25",
	}
	if withAudio {
		args = append(args,
			"-f", "lavfi", "-i", "sine=frequency=440:duration="+strconv.Itoa(duration)+":sample_rate=48000",
			"-c:a", "aac", "-ac", "2", "-ar", "48000",
		)
	} else {
		args = append(args, "-an")
	}
	args = append(args,
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-g", "25",
		"-f", "mp4", inputPath,
	)
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "generate test input")
	return inputPath
}

func speedOpenInput(t *testing.T, inputPath string) (*astiav.FormatContext, *astiav.Stream, *astiav.Stream) {
	t.Helper()
	fc := astiav.AllocFormatContext()
	require.NotNil(t, fc)

	inputDict := astiav.NewDictionary()
	require.NoError(t, fc.OpenInput(inputPath, nil, inputDict))
	inputDict.Free()
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
	require.NotNil(t, videoStream, "input must have video stream")
	return fc, videoStream, audioStream
}

func speedCopyExtradata(cp *astiav.CodecParameters) []byte {
	ed := cp.ExtraData()
	if len(ed) == 0 {
		return nil
	}
	out := make([]byte, len(ed))
	copy(out, ed)
	return out
}

// TestSpeed_FragmentedMuxer verifies that 5s of 25fps H.264 pushed through
// FragmentedMuxer produces output measured at 4.5-5.5s by ffprobe.
// This is the critical speed/timestamp test for MSE/DASH fMP4 output.
func TestSpeed_FragmentedMuxer(t *testing.T) {
	speedSkipIfNoTools(t)

	dir := t.TempDir()
	ourDir := filepath.Join(dir, "fmp4")
	require.NoError(t, os.MkdirAll(ourDir, 0755))

	inputPath := speedGenerateInput(t, dir, 5, false)
	fc, videoStream, _ := speedOpenInput(t, inputPath)
	defer fc.Free()
	defer fc.CloseInput()

	vcp := videoStream.CodecParameters()
	fmp4Muxer, err := NewFragmentedMuxer(MuxOpts{
		OutputDir:      ourDir,
		VideoCodecID:   vcp.CodecID(),
		VideoExtradata: speedCopyExtradata(vcp),
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

	initData, err := os.ReadFile(filepath.Join(ourDir, "init_video.mp4"))
	require.NoError(t, err)
	segs, _ := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))
	require.Greater(t, len(segs), 0)

	combinedPath := filepath.Join(dir, "fmp4_combined.mp4")
	var combined []byte
	combined = append(combined, initData...)
	for _, seg := range segs {
		data, _ := os.ReadFile(seg)
		combined = append(combined, data...)
	}
	require.NoError(t, os.WriteFile(combinedPath, combined, 0644))

	dur := speedProbe(t, combinedPath)
	t.Logf("FragmentedMuxer: %d packets, %d segments, ffprobe duration = %.3fs", count, len(segs), dur)
	t.Logf("  expected: 4.5-5.5s | if ~2.5s: double-speed | if ~10s: half-speed")

	assert.Greater(t, dur, 4.5, "FragmentedMuxer output too short (double-speed?)")
	assert.Less(t, dur, 5.5, "FragmentedMuxer output too long (half-speed?)")
}

// TestSpeed_HLSMuxer_MPEGTS verifies that 5s of 25fps H.264+AAC pushed
// through HLSMuxer (MPEG-TS segments) produces total playlist duration
// measured at 4.5-5.5s by ffprobe.
func TestSpeed_HLSMuxer_MPEGTS(t *testing.T) {
	speedSkipIfNoTools(t)

	dir := t.TempDir()
	ourDir := filepath.Join(dir, "hls")
	require.NoError(t, os.MkdirAll(ourDir, 0755))

	inputPath := speedGenerateInput(t, dir, 5, true)
	fc, videoStream, audioStream := speedOpenInput(t, inputPath)
	defer fc.Free()
	defer fc.CloseInput()

	require.NotNil(t, audioStream, "input should have audio for HLS test")

	vcp := videoStream.CodecParameters()
	acp := audioStream.CodecParameters()

	hlsMuxer, err := NewHLSMuxer(HLSMuxOpts{
		OutputDir:          ourDir,
		SegmentDurationSec: 2,
		VideoCodecID:       vcp.CodecID(),
		VideoExtradata:     speedCopyExtradata(vcp),
		VideoWidth:         vcp.Width(),
		VideoHeight:        vcp.Height(),
		VideoTimeBase:      videoStream.TimeBase(),
		VideoFrameRate:     25,
		AudioCodecID:       acp.CodecID(),
		AudioExtradata:     speedCopyExtradata(acp),
		AudioChannels:      acp.ChannelLayout().Channels(),
		AudioSampleRate:    acp.SampleRate(),
		AudioTimeBase:      audioStream.TimeBase(),
		AudioFrameSize:     1024,
	})
	require.NoError(t, err)

	pkt := astiav.AllocPacket()
	require.NotNil(t, pkt)
	defer pkt.Free()

	var videoPkts, audioPkts int
	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		switch pkt.StreamIndex() {
		case videoStream.Index():
			require.NoError(t, hlsMuxer.WriteVideoPacket(pkt))
			videoPkts++
		case audioStream.Index():
			require.NoError(t, hlsMuxer.WriteAudioPacket(pkt))
			audioPkts++
		}
		pkt.Unref()
	}
	require.NoError(t, hlsMuxer.Close())

	playlistPath := filepath.Join(ourDir, "playlist.m3u8")
	_, err = os.Stat(playlistPath)
	require.NoError(t, err, "HLS playlist should exist")

	probeOut, err := exec.Command("ffprobe", "-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "json", playlistPath).Output()
	require.NoError(t, err, "ffprobe HLS playlist")
	var result speedProbeFormatResult
	require.NoError(t, json.Unmarshal(probeOut, &result))
	dur, err := strconv.ParseFloat(result.Format.Duration, 64)
	require.NoError(t, err)

	t.Logf("HLSMuxer MPEG-TS: %d video + %d audio packets, ffprobe duration = %.3fs",
		videoPkts, audioPkts, dur)
	t.Logf("  expected: 4.5-5.5s | if ~2.5s: double-speed | if ~10s: half-speed")

	assert.Greater(t, dur, 4.5, "HLS output too short (double-speed?)")
	assert.Less(t, dur, 5.5, "HLS output too long (half-speed?)")
}

// TestSpeed_HLSMuxer_VideoOnly verifies speed with video-only HLS.
func TestSpeed_HLSMuxer_VideoOnly(t *testing.T) {
	speedSkipIfNoTools(t)

	dir := t.TempDir()
	ourDir := filepath.Join(dir, "hls_video")
	require.NoError(t, os.MkdirAll(ourDir, 0755))

	inputPath := speedGenerateInput(t, dir, 5, false)
	fc, videoStream, _ := speedOpenInput(t, inputPath)
	defer fc.Free()
	defer fc.CloseInput()

	vcp := videoStream.CodecParameters()

	hlsMuxer, err := NewHLSMuxer(HLSMuxOpts{
		OutputDir:          ourDir,
		SegmentDurationSec: 2,
		VideoCodecID:       vcp.CodecID(),
		VideoExtradata:     speedCopyExtradata(vcp),
		VideoWidth:         vcp.Width(),
		VideoHeight:        vcp.Height(),
		VideoTimeBase:      videoStream.TimeBase(),
		VideoFrameRate:     25,
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
			require.NoError(t, hlsMuxer.WriteVideoPacket(pkt))
			count++
		}
		pkt.Unref()
	}
	require.NoError(t, hlsMuxer.Close())

	playlistPath := filepath.Join(ourDir, "playlist.m3u8")
	dur := speedProbe(t, playlistPath)

	t.Logf("HLSMuxer video-only: %d packets, ffprobe duration = %.3fs", count, dur)
	assert.Greater(t, dur, 4.5, "HLS video-only too short")
	assert.Less(t, dur, 5.5, "HLS video-only too long")
}

// TestSpeed_FragmentedMuxer_WithAudio verifies speed with A/V fMP4.
func TestSpeed_FragmentedMuxer_WithAudio(t *testing.T) {
	speedSkipIfNoTools(t)

	dir := t.TempDir()
	ourDir := filepath.Join(dir, "fmp4_av")
	require.NoError(t, os.MkdirAll(ourDir, 0755))

	inputPath := speedGenerateInput(t, dir, 5, true)
	fc, videoStream, audioStream := speedOpenInput(t, inputPath)
	defer fc.Free()
	defer fc.CloseInput()
	require.NotNil(t, audioStream)

	vcp := videoStream.CodecParameters()
	acp := audioStream.CodecParameters()

	fmp4Muxer, err := NewFragmentedMuxer(MuxOpts{
		OutputDir:       ourDir,
		VideoCodecID:    vcp.CodecID(),
		VideoExtradata:  speedCopyExtradata(vcp),
		VideoWidth:      vcp.Width(),
		VideoHeight:     vcp.Height(),
		VideoTimeBase:   videoStream.TimeBase(),
		AudioCodecID:    acp.CodecID(),
		AudioExtradata:  speedCopyExtradata(acp),
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

	initData, err := os.ReadFile(filepath.Join(ourDir, "init_video.mp4"))
	require.NoError(t, err)
	segs, _ := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))

	combinedPath := filepath.Join(dir, "fmp4_av_combined.mp4")
	var combined []byte
	combined = append(combined, initData...)
	for _, seg := range segs {
		data, _ := os.ReadFile(seg)
		combined = append(combined, data...)
	}
	require.NoError(t, os.WriteFile(combinedPath, combined, 0644))

	dur := speedProbe(t, combinedPath)
	t.Logf("FragmentedMuxer A/V: %d video + %d audio, ffprobe duration = %.3fs",
		videoPkts, audioPkts, dur)

	assert.Greater(t, dur, 4.5, "fMP4 A/V too short")
	assert.Less(t, dur, 5.5, "fMP4 A/V too long")
}
