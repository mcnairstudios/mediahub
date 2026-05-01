package session

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

type SubprocessConfig struct {
	InputURL     string
	OutputDir    string
	VideoCodec   string
	AudioCodec   string
	VideoBitrate int
	AudioBitrate int
	HWAccel      string
	OutputHeight int
	Deinterlace  bool
	IsLive       bool
}

func RunSubprocessPipeline(ctx context.Context, cfg SubprocessConfig, log zerolog.Logger) error {
	segDir := filepath.Join(cfg.OutputDir, "segments")
	if err := os.MkdirAll(segDir, 0755); err != nil {
		return fmt.Errorf("subprocess: create segment dir: %w", err)
	}

	args := buildFFmpegArgs(cfg, segDir)

	log.Info().Strs("args", args).Msg("subprocess: starting ffmpeg")

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("subprocess: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("subprocess: start ffmpeg: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "Error") || strings.Contains(line, "error") {
				log.Error().Str("ffmpeg", line).Msg("subprocess: ffmpeg stderr")
			} else {
				log.Debug().Str("ffmpeg", line).Msg("subprocess: ffmpeg stderr")
			}
		}
	}()

	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if waitErr != nil {
		return fmt.Errorf("subprocess: ffmpeg exited: %w", waitErr)
	}
	return nil
}

func buildFFmpegArgs(cfg SubprocessConfig, segDir string) []string {
	var args []string

	args = append(args, "-hide_banner", "-loglevel", "warning", "-y")

	hwaccel := cfg.HWAccel
	if hwaccel != "" && hwaccel != "none" && hwaccel != "default" {
		switch hwaccel {
		case "vaapi":
			args = append(args, "-hwaccel", "vaapi", "-hwaccel_device", "/dev/dri/renderD128", "-hwaccel_output_format", "vaapi")
		case "qsv":
			args = append(args, "-hwaccel", "qsv", "-hwaccel_device", "/dev/dri/renderD128", "-hwaccel_output_format", "qsv")
		case "cuda", "nvenc":
			args = append(args, "-hwaccel", "cuda", "-hwaccel_output_format", "cuda")
		case "videotoolbox":
			args = append(args, "-hwaccel", "videotoolbox")
		}
	}

	if strings.HasPrefix(cfg.InputURL, "rtsp://") {
		args = append(args,
			"-rtsp_transport", "tcp",
			"-analyzeduration", "1000000",
			"-probesize", "1000000",
		)
	} else {
		args = append(args,
			"-analyzeduration", "3000000",
			"-probesize", "3000000",
		)
	}
	args = append(args,
		"-err_detect", "ignore_err",
		"-fflags", "+genpts+discardcorrupt",
		"-i", cfg.InputURL,
	)

	args = append(args,
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		"-map", "0:v:0?",
		"-map", "0:a:0?",
	)

	videoEncoder := resolveVideoEncoder(cfg.VideoCodec, cfg.HWAccel)
	args = append(args, "-c:v", videoEncoder)

	if isHEVC(videoEncoder) {
		args = append(args, "-tag:v:0", "hvc1")
	}

	videoBitrate := cfg.VideoBitrate
	if videoBitrate <= 0 {
		videoBitrate = 4000
	}
	if videoEncoder != "copy" {
		args = append(args, "-b:v", fmt.Sprintf("%dk", videoBitrate))
		isSoftware := videoEncoder == "libx264" || videoEncoder == "libx265" || videoEncoder == "libaom-av1"
		if isSoftware {
			args = append(args, "-preset", "ultrafast")
		}
	}

	var vf []string
	if cfg.Deinterlace {
		switch cfg.HWAccel {
		case "vaapi":
			vf = append(vf, "deinterlace_vaapi")
		case "qsv":
			vf = append(vf, "vpp_qsv=deinterlace=2")
		default:
			vf = append(vf, "yadif")
		}
	}
	if cfg.OutputHeight > 0 {
		switch cfg.HWAccel {
		case "vaapi":
			vf = append(vf, fmt.Sprintf("scale_vaapi=w=-2:h=%d", cfg.OutputHeight))
		case "qsv":
			vf = append(vf, fmt.Sprintf("vpp_qsv=w=-2:h=%d", cfg.OutputHeight))
		default:
			vf = append(vf, fmt.Sprintf("scale=-2:%d", cfg.OutputHeight))
		}
	}
	if len(vf) > 0 {
		args = append(args, "-vf", strings.Join(vf, ","))
	}

	audioEncoder := resolveAudioEncoder(cfg.AudioCodec)
	args = append(args, "-c:a", audioEncoder)
	if audioEncoder != "copy" {
		audioBitrate := cfg.AudioBitrate
		if audioBitrate <= 0 {
			audioBitrate = 128
		}
		args = append(args, "-b:a", fmt.Sprintf("%dk", audioBitrate), "-ac", "2")
	}

	hlsFlags := "independent_segments+append_list"
	hlsListSize := "0"
	if cfg.IsLive {
		hlsFlags = "delete_segments+independent_segments+append_list"
		hlsListSize = "10"
	}

	args = append(args,
		"-f", "hls",
		"-hls_time", "2",
		"-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", "init.mp4",
		"-hls_flags", hlsFlags,
		"-hls_list_size", hlsListSize,
	)

	segPattern := filepath.Join(segDir, "seg%04d.m4s")
	playlistPath := filepath.Join(segDir, "playlist.m3u8")

	args = append(args,
		"-hls_segment_filename", segPattern,
		playlistPath,
	)

	return args
}

func resolveVideoEncoder(codec, hwaccel string) string {
	base := strings.ToLower(codec)
	if base == "" || base == "copy" {
		return "copy"
	}

	switch hwaccel {
	case "videotoolbox":
		switch base {
		case "h264":
			return "h264_videotoolbox"
		case "h265", "hevc":
			return "hevc_videotoolbox"
		}
	case "vaapi":
		switch base {
		case "h264":
			return "h264_vaapi"
		case "h265", "hevc":
			return "hevc_vaapi"
		case "av1":
			return "av1_vaapi"
		}
	case "nvenc", "cuda":
		switch base {
		case "h264":
			return "h264_nvenc"
		case "h265", "hevc":
			return "hevc_nvenc"
		case "av1":
			return "av1_nvenc"
		}
	case "qsv":
		switch base {
		case "h264":
			return "h264_qsv"
		case "h265", "hevc":
			return "hevc_qsv"
		case "av1":
			return "av1_qsv"
		}
	}

	switch base {
	case "h264":
		return "libx264"
	case "h265", "hevc":
		return "libx265"
	case "av1":
		return "libaom-av1"
	default:
		return base
	}
}

func resolveAudioEncoder(codec string) string {
	base := strings.ToLower(codec)
	switch base {
	case "", "aac":
		return "aac"
	case "opus":
		return "libopus"
	case "mp3":
		return "libmp3lame"
	case "ac3":
		return "ac3"
	case "copy":
		return "copy"
	default:
		return base
	}
}

func isHEVC(encoder string) bool {
	e := strings.ToLower(encoder)
	return strings.Contains(e, "hevc") || strings.Contains(e, "265")
}
