//go:build cgo

package session

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/demux"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/output/dash"
	"github.com/mcnairstudios/mediahub/pkg/output/hls"
	"github.com/mcnairstudios/mediahub/pkg/output/mse"
	"github.com/mcnairstudios/mediahub/pkg/output/record"
	"github.com/mcnairstudios/mediahub/pkg/output/validate"
	"github.com/mcnairstudios/mediahub/pkg/output/webrtc"
)

func generateTestInput(t *testing.T) string {
	t.Helper()

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found in PATH, skipping pipeline e2e test")
	}

	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.ts")

	cmd := exec.Command(ffmpegPath,
		"-f", "lavfi", "-i", "testsrc2=duration=3:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "mpegts", inputPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("ffmpeg failed to generate test input: %v", err)
	}

	info, err := os.Stat(inputPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("test input file not created or empty")
	}

	return inputPath
}

func openDemuxer(t *testing.T, inputPath string) *demux.Demuxer {
	t.Helper()

	dm, err := demux.NewDemuxer(inputPath, demux.DemuxOpts{
		AudioTrack:      -1,
		ProbeSize:       5000000,
		AnalyzeDuration: 2000000,
	})
	if err != nil {
		t.Fatalf("failed to open demuxer: %v", err)
	}
	return dm
}

func pluginConfig(t *testing.T, dm *demux.Demuxer, outputDir string) output.PluginConfig {
	t.Helper()

	si := dm.StreamInfo()

	cfg := output.PluginConfig{
		OutputDir: outputDir,
		IsLive:    false,
	}

	if si != nil && si.Video != nil {
		cfg.Video = si.Video
	}

	vcp := dm.VideoCodecParameters()
	if vcp != nil {
		cfg.VideoCodecParams = vcp
		cfg.VideoExtradata = vcp.ExtraData()
	}

	acp := dm.AudioCodecParameters()
	if acp != nil {
		cfg.AudioCodecParams = acp
		cfg.AudioExtradata = acp.ExtraData()
		if si != nil && len(si.AudioTracks) > 0 {
			cfg.Audio = &si.AudioTracks[0]
		}
	}

	return cfg
}

type pipelineStats struct {
	videoPackets int
	audioPackets int
	errors       int
}

func drainDemuxer(t *testing.T, dm *demux.Demuxer, fo *output.FanOut) pipelineStats {
	t.Helper()

	var stats pipelineStats
	for {
		pkt, err := dm.ReadPacket()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("demuxer read error: %v", err)
		}

		switch pkt.Type {
		case av.Video:
			if err := fo.PushVideo(pkt.Data, pkt.PTS, pkt.DTS, pkt.Duration, pkt.Keyframe); err != nil {
				stats.errors++
			}
			stats.videoPackets++
		case av.Audio:
			if err := fo.PushAudio(pkt.Data, pkt.PTS, pkt.DTS, pkt.Duration); err != nil {
				stats.errors++
			}
			stats.audioPackets++
		}
	}
	fo.EndOfStream()
	return stats
}

func TestPipelineE2E_MSE(t *testing.T) {
	inputPath := generateTestInput(t)
	dm := openDemuxer(t, inputPath)
	defer dm.Close()

	outputDir := t.TempDir()
	cfg := pluginConfig(t, dm, outputDir)

	plugin, err := mse.New(cfg)
	if err != nil {
		t.Fatalf("mse.New: %v", err)
	}

	fo := output.NewFanOut(plugin)
	stats := drainDemuxer(t, dm, fo)
	plugin.Stop()

	t.Logf("MSE: %d video, %d audio packets pushed, %d errors", stats.videoPackets, stats.audioPackets, stats.errors)

	if stats.videoPackets == 0 {
		t.Fatal("no video packets pushed")
	}

	status := plugin.Status()
	t.Logf("MSE status: segments=%d bytes=%d healthy=%v", status.SegmentCount, status.BytesWritten, status.Healthy)

	if status.BytesWritten == 0 {
		t.Error("MSE plugin wrote 0 bytes")
	}

	segDir := filepath.Join(outputDir, "segments")
	entries, err := os.ReadDir(segDir)
	if err != nil {
		t.Fatalf("read segments dir: %v", err)
	}

	var initFiles, segFiles int
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "init") {
			initFiles++
			data, _ := os.ReadFile(filepath.Join(segDir, name))
			errs := validate.ValidateFMP4Init(data)
			for _, ve := range errs {
				t.Errorf("MSE init validation: %s", ve)
			}
		}
		if strings.HasSuffix(name, ".m4s") {
			segFiles++
			data, _ := os.ReadFile(filepath.Join(segDir, name))
			errs := validate.ValidateFMP4Segment(data)
			for _, ve := range errs {
				t.Errorf("MSE segment %s validation: %s", name, ve)
			}
		}
	}

	t.Logf("MSE: %d init files, %d segment files", initFiles, segFiles)
	if initFiles == 0 {
		t.Error("no init segments produced")
	}
	if segFiles == 0 {
		t.Error("no media segments produced")
	}
}

func TestPipelineE2E_HLS(t *testing.T) {
	inputPath := generateTestInput(t)
	dm := openDemuxer(t, inputPath)
	defer dm.Close()

	outputDir := t.TempDir()
	cfg := pluginConfig(t, dm, outputDir)
	cfg.SegmentDurationSec = 2

	plugin, err := hls.New(cfg)
	if err != nil {
		t.Fatalf("hls.New: %v", err)
	}

	fo := output.NewFanOut(plugin)
	stats := drainDemuxer(t, dm, fo)
	plugin.Stop()

	t.Logf("HLS: %d video, %d audio packets pushed, %d errors", stats.videoPackets, stats.audioPackets, stats.errors)

	if stats.videoPackets == 0 {
		t.Fatal("no video packets pushed")
	}

	status := plugin.Status()
	t.Logf("HLS status: segments=%d healthy=%v err=%q", status.SegmentCount, status.Healthy, status.Error)

	segDir := filepath.Join(outputDir, "segments")
	playlistPath := filepath.Join(segDir, "playlist.m3u8")
	playlistData, err := os.ReadFile(playlistPath)
	if err != nil {
		t.Fatalf("read playlist: %v", err)
	}

	errs := validate.ValidateHLSPlaylist(playlistData)
	for _, ve := range errs {
		t.Errorf("HLS playlist validation: %s", ve)
	}

	tsFiles := 0
	entries, _ := os.ReadDir(segDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ts") {
			tsFiles++
			data, _ := os.ReadFile(filepath.Join(segDir, e.Name()))
			tsErrs := validate.ValidateTSSegment(data)
			for _, ve := range tsErrs {
				t.Errorf("HLS TS segment %s validation: %s", e.Name(), ve)
			}
		}
	}

	t.Logf("HLS: playlist present, %d .ts files", tsFiles)
	if tsFiles == 0 {
		t.Error("no .ts segments produced")
	}
}

func TestPipelineE2E_DASH(t *testing.T) {
	inputPath := generateTestInput(t)
	dm := openDemuxer(t, inputPath)
	defer dm.Close()

	outputDir := t.TempDir()
	cfg := pluginConfig(t, dm, outputDir)

	plugin, err := dash.New(cfg)
	if err != nil {
		t.Fatalf("dash.New: %v", err)
	}

	fo := output.NewFanOut(plugin)
	stats := drainDemuxer(t, dm, fo)
	plugin.Stop()

	t.Logf("DASH: %d video, %d audio packets pushed, %d errors", stats.videoPackets, stats.audioPackets, stats.errors)

	if stats.videoPackets == 0 {
		t.Fatal("no video packets pushed")
	}

	status := plugin.Status()
	t.Logf("DASH status: segments=%d bytes=%d healthy=%v", status.SegmentCount, status.BytesWritten, status.Healthy)

	if status.BytesWritten == 0 {
		t.Error("DASH plugin wrote 0 bytes")
	}

	segDir := filepath.Join(outputDir, "segments")
	entries, err := os.ReadDir(segDir)
	if err != nil {
		t.Fatalf("read segments dir: %v", err)
	}

	var initFiles, segFiles int
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "init") {
			initFiles++
			data, _ := os.ReadFile(filepath.Join(segDir, name))
			errs := validate.ValidateFMP4Init(data)
			for _, ve := range errs {
				t.Errorf("DASH init %s validation: %s", name, ve)
			}
		}
		if strings.HasSuffix(name, ".m4s") {
			segFiles++
			data, _ := os.ReadFile(filepath.Join(segDir, name))
			errs := validate.ValidateFMP4Segment(data)
			for _, ve := range errs {
				t.Errorf("DASH segment %s validation: %s", name, ve)
			}
		}
	}

	t.Logf("DASH: %d init files, %d segment files", initFiles, segFiles)
	if initFiles == 0 {
		t.Error("no init segments produced")
	}
	if segFiles == 0 {
		t.Error("no media segments produced")
	}
}

func TestPipelineE2E_WebRTC(t *testing.T) {
	inputPath := generateTestInput(t)
	dm := openDemuxer(t, inputPath)
	defer dm.Close()

	outputDir := t.TempDir()
	cfg := pluginConfig(t, dm, outputDir)

	plugin, err := webrtc.New(cfg)
	if err != nil {
		t.Fatalf("webrtc.New: %v", err)
	}

	fo := output.NewFanOut(plugin)
	stats := drainDemuxer(t, dm, fo)
	plugin.Stop()

	t.Logf("WebRTC: %d video, %d audio packets pushed, %d errors", stats.videoPackets, stats.audioPackets, stats.errors)

	if stats.videoPackets == 0 {
		t.Fatal("no video packets pushed")
	}

	status := plugin.Status()
	t.Logf("WebRTC status: bytes=%d healthy=%v", status.BytesWritten, status.Healthy)
}

func TestPipelineE2E_Record(t *testing.T) {
	inputPath := generateTestInput(t)
	dm := openDemuxer(t, inputPath)
	defer dm.Close()

	outputDir := t.TempDir()
	outputFile := filepath.Join(outputDir, "recording.ts")
	cfg := pluginConfig(t, dm, outputDir)
	cfg.OutputFilePath = outputFile
	cfg.OutputFormat = "mpegts"

	plugin, err := record.New(cfg)
	if err != nil {
		t.Fatalf("record.New: %v", err)
	}

	fo := output.NewFanOut(plugin)
	stats := drainDemuxer(t, dm, fo)
	plugin.Stop()

	t.Logf("Record: %d video, %d audio packets pushed, %d errors", stats.videoPackets, stats.audioPackets, stats.errors)

	if stats.videoPackets == 0 {
		t.Fatal("no video packets pushed")
	}

	info, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("recording file not found: %v", err)
	}

	t.Logf("Record: output file size = %d bytes", info.Size())
	if info.Size() == 0 {
		t.Error("recording file is empty")
	}

	status := plugin.Status()
	t.Logf("Record status: bytes=%d healthy=%v", status.BytesWritten, status.Healthy)

	if info.Size() > 188 {
		data, _ := os.ReadFile(outputFile)
		tsErrs := validate.ValidateTSSegment(data)
		for _, ve := range tsErrs {
			t.Errorf("Record TS validation: %s", ve)
		}
	}
}

func TestPipelineE2E_MultiOutput(t *testing.T) {
	inputPath := generateTestInput(t)
	dm := openDemuxer(t, inputPath)
	defer dm.Close()

	mseDir := t.TempDir()
	hlsDir := t.TempDir()
	recDir := t.TempDir()

	si := dm.StreamInfo()

	baseCfg := output.PluginConfig{
		IsLive: false,
	}
	if si != nil && si.Video != nil {
		baseCfg.Video = si.Video
	}
	vcp := dm.VideoCodecParameters()
	if vcp != nil {
		baseCfg.VideoCodecParams = vcp
		baseCfg.VideoExtradata = vcp.ExtraData()
	}
	acp := dm.AudioCodecParameters()
	if acp != nil {
		baseCfg.AudioCodecParams = acp
		baseCfg.AudioExtradata = acp.ExtraData()
		if si != nil && len(si.AudioTracks) > 0 {
			baseCfg.Audio = &si.AudioTracks[0]
		}
	}

	mseCfg := baseCfg
	mseCfg.OutputDir = mseDir
	msePlugin, err := mse.New(mseCfg)
	if err != nil {
		t.Fatalf("mse.New: %v", err)
	}

	hlsCfg := baseCfg
	hlsCfg.OutputDir = hlsDir
	hlsCfg.SegmentDurationSec = 2
	hlsPlugin, err := hls.New(hlsCfg)
	if err != nil {
		t.Fatalf("hls.New: %v", err)
	}

	recFile := filepath.Join(recDir, "recording.ts")
	recCfg := baseCfg
	recCfg.OutputDir = recDir
	recCfg.OutputFilePath = recFile
	recCfg.OutputFormat = "mpegts"
	recPlugin, err := record.New(recCfg)
	if err != nil {
		t.Fatalf("record.New: %v", err)
	}

	fo := output.NewFanOut(msePlugin, hlsPlugin, recPlugin)

	if fo.PluginCount() != 3 {
		t.Fatalf("expected 3 plugins, got %d", fo.PluginCount())
	}

	stats := drainDemuxer(t, dm, fo)
	msePlugin.Stop()
	hlsPlugin.Stop()
	recPlugin.Stop()

	t.Logf("Multi-output: %d video, %d audio packets pushed, %d errors", stats.videoPackets, stats.audioPackets, stats.errors)

	mseStatus := msePlugin.Status()
	if mseStatus.BytesWritten == 0 {
		t.Error("MSE plugin wrote 0 bytes in multi-output mode")
	}

	hlsStatus := hlsPlugin.Status()
	t.Logf("Multi-output HLS: segments=%d", hlsStatus.SegmentCount)

	recInfo, err := os.Stat(recFile)
	if err != nil {
		t.Fatalf("recording file not found in multi-output mode: %v", err)
	}
	if recInfo.Size() == 0 {
		t.Error("recording file is empty in multi-output mode")
	}

	mseSegDir := filepath.Join(mseDir, "segments")
	mseEntries, _ := os.ReadDir(mseSegDir)
	mseSegCount := 0
	for _, e := range mseEntries {
		if strings.HasSuffix(e.Name(), ".m4s") || strings.HasPrefix(e.Name(), "init") {
			mseSegCount++
		}
	}

	hlsSegDir := filepath.Join(hlsDir, "segments")
	playlistData, err := os.ReadFile(filepath.Join(hlsSegDir, "playlist.m3u8"))
	if err != nil {
		t.Errorf("HLS playlist missing in multi-output mode: %v", err)
	} else {
		errs := validate.ValidateHLSPlaylist(playlistData)
		for _, ve := range errs {
			t.Errorf("Multi-output HLS playlist validation: %s", ve)
		}
	}

	recData, _ := os.ReadFile(recFile)
	if len(recData) > 188 {
		tsErrs := validate.ValidateTSSegment(recData)
		for _, ve := range tsErrs {
			t.Errorf("Multi-output Record TS validation: %s", ve)
		}
	}

	t.Logf("Multi-output results: MSE=%d files, HLS playlist=%d bytes, Record=%d bytes",
		mseSegCount, len(playlistData), recInfo.Size())
}

func TestPipelineE2E_ProbeResult(t *testing.T) {
	inputPath := generateTestInput(t)
	dm := openDemuxer(t, inputPath)
	defer dm.Close()

	si := dm.StreamInfo()
	if si == nil {
		t.Fatal("probe returned nil StreamInfo")
	}

	if si.Video == nil {
		t.Fatal("probe found no video track")
	}

	t.Logf("Probe: video=%s %dx%d, %d audio tracks",
		si.Video.Codec, si.Video.Width, si.Video.Height, len(si.AudioTracks))

	if si.Video.Codec != "h264" {
		t.Errorf("expected h264 video codec, got %q", si.Video.Codec)
	}
	if si.Video.Width != 640 || si.Video.Height != 360 {
		t.Errorf("expected 640x360, got %dx%d", si.Video.Width, si.Video.Height)
	}

	if len(si.AudioTracks) == 0 {
		t.Fatal("probe found no audio tracks")
	}

	audio := si.AudioTracks[0]
	t.Logf("Probe: audio=%s ch=%d rate=%d", audio.Codec, audio.Channels, audio.SampleRate)

	if audio.SampleRate != 48000 {
		t.Errorf("expected 48000 sample rate, got %d", audio.SampleRate)
	}
	if audio.Channels != 2 {
		t.Errorf("expected 2 channels, got %d", audio.Channels)
	}

	_ = &media.ProbeResult{}
}
