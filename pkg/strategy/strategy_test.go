package strategy

import (
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

func TestResolve_VideoCopy(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4"},
	)
	if d.NeedsTranscode {
		t.Error("expected video copy, got transcode")
	}
	if d.VideoCodec != media.VideoCopy {
		t.Errorf("expected video codec copy, got %s", d.VideoCodec)
	}
}

func TestResolve_VideoTranscode_DifferentCodec(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "h265", AudioCodec: "aac", Container: "mp4"},
	)
	if !d.NeedsTranscode {
		t.Error("expected transcode for h264->h265")
	}
	if d.VideoCodec != media.VideoH265 {
		t.Errorf("expected h265, got %s", d.VideoCodec)
	}
}

func TestResolve_DefaultMatchesSource(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "default", AudioCodec: "aac", Container: "mp4"},
	)
	if d.NeedsTranscode {
		t.Error("expected copy when output is default and source matches")
	}
	if d.VideoCodec != media.VideoCopy {
		t.Errorf("expected copy, got %s", d.VideoCodec)
	}
}

func TestResolve_H265SourceH264Output(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h265", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "h264", AudioCodec: "aac", Container: "mp4"},
	)
	if !d.NeedsTranscode {
		t.Error("expected transcode for h265->h264")
	}
	if d.VideoCodec != media.VideoH264 {
		t.Errorf("expected h264, got %s", d.VideoCodec)
	}
}

func TestResolve_Interlaced(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080, Interlaced: true},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4"},
	)
	if !d.Deinterlace {
		t.Error("expected deinterlace for interlaced source")
	}
	if !d.NeedsTranscode {
		t.Error("expected transcode when deinterlace is needed")
	}
}

func TestResolve_BitDepthExceedsMax(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h265", AudioCodec: "aac", Width: 3840, Height: 2160, BitDepth: 10},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4", MaxBitDepth: 8},
	)
	if !d.NeedsTranscode {
		t.Error("expected transcode when bit depth exceeds max")
	}
}

func TestResolve_BitDepthWithinMax(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h265", AudioCodec: "aac", Width: 3840, Height: 2160, BitDepth: 8},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4", MaxBitDepth: 8},
	)
	if d.NeedsTranscode {
		t.Error("expected copy when bit depth is within max")
	}
}

func TestResolve_AudioCopy(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4"},
	)
	if d.NeedsAudioTranscode {
		t.Error("expected audio copy when source and output are both aac")
	}
	if d.AudioCodec != media.AudioCopy {
		t.Errorf("expected audio copy, got %s", d.AudioCodec)
	}
}

func TestResolve_AudioTranscode_AC3ToAAC(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "ac3", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4"},
	)
	if !d.NeedsAudioTranscode {
		t.Error("expected audio transcode for ac3->aac")
	}
	if d.AudioCodec != media.AudioAAC {
		t.Errorf("expected aac, got %s", d.AudioCodec)
	}
}

func TestResolve_AudioTranscode_LATMToAAC(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac_latm", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4"},
	)
	if d.NeedsAudioTranscode {
		t.Error("expected audio copy for aac_latm->aac (same base codec)")
	}
	if d.AudioCodec != media.AudioCopy {
		t.Errorf("expected audio copy, got %s", d.AudioCodec)
	}
}

func TestResolve_HeightReduction(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4", OutputHeight: 720},
	)
	if !d.NeedsTranscode {
		t.Error("expected transcode when output height is less than source")
	}
}

func TestResolve_SameHeight(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4", OutputHeight: 1080},
	)
	if d.NeedsTranscode {
		t.Error("expected copy when output height matches source")
	}
}

func TestResolve_OutputHeightHigherThanSource(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1280, Height: 720},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4", OutputHeight: 1080},
	)
	if d.NeedsTranscode {
		t.Error("expected copy when output height ceiling exceeds source")
	}
}

func TestResolve_Container(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mpegts"},
	)
	if d.Container != media.ContainerMPEGTS {
		t.Errorf("expected mpegts container, got %s", d.Container)
	}
}

func TestResolve_HWAccelPassthrough(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "h265", AudioCodec: "aac", Container: "mp4", HWAccel: "vaapi"},
	)
	if d.HWAccel != "vaapi" {
		t.Errorf("expected vaapi, got %s", d.HWAccel)
	}
}

func TestResolve_HWAccelNotSetOnCopy(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4", HWAccel: "vaapi"},
	)
	if d.HWAccel != "" {
		t.Errorf("expected no hwaccel on copy, got %s", d.HWAccel)
	}
}

func TestResolve_AudioCopyOutput(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "ac3", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "copy", Container: "mpegts"},
	)
	if d.NeedsAudioTranscode {
		t.Error("expected audio copy when output is copy")
	}
	if d.AudioCodec != media.AudioCopy {
		t.Errorf("expected audio copy, got %s", d.AudioCodec)
	}
}

func TestResolve_DefaultAudioMatchesSource(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "ac3", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "default", Container: "mpegts"},
	)
	if d.NeedsAudioTranscode {
		t.Error("expected audio copy when output is default")
	}
	if d.AudioCodec != media.AudioCopy {
		t.Errorf("expected audio copy, got %s", d.AudioCodec)
	}
}

func TestResolve_TranscodeVideoCodecResolved(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080, Interlaced: true},
		Output{VideoCodec: "default", AudioCodec: "aac", Container: "mp4"},
	)
	if d.VideoCodec != media.VideoH264 {
		t.Errorf("expected h264 when default + interlaced forces transcode, got %s", d.VideoCodec)
	}
	if !d.NeedsTranscode {
		t.Error("expected transcode for interlaced source")
	}
}

func TestResolve_DefaultVideoWithHeightReduction(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h265", AudioCodec: "aac", Width: 3840, Height: 2160},
		Output{VideoCodec: "default", AudioCodec: "aac", Container: "mp4", OutputHeight: 1080},
	)
	if !d.NeedsTranscode {
		t.Error("expected transcode for height reduction")
	}
	if d.VideoCodec != media.VideoH265 {
		t.Errorf("expected h265 (match source) on default+transcode, got %s", d.VideoCodec)
	}
}
