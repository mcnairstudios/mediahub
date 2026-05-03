package strategy

import (
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

func TestResolve_AudioOpusOutput(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "opus", Container: "mp4"},
	)
	if !d.NeedsAudioTranscode {
		t.Error("expected audio transcode for aac->opus")
	}
	if d.AudioCodec != media.AudioOpus {
		t.Errorf("expected opus, got %s", d.AudioCodec)
	}
}

func TestResolve_AudioMP3Output(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "ac3", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "mp3", Container: "mpegts"},
	)
	if !d.NeedsAudioTranscode {
		t.Error("expected audio transcode for ac3->mp3")
	}
	if d.AudioCodec != media.AudioMP3 {
		t.Errorf("expected mp3, got %s", d.AudioCodec)
	}
}

func TestResolve_AV1Transcode(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "av1", AudioCodec: "aac", Container: "mp4"},
	)
	if !d.NeedsTranscode {
		t.Error("expected transcode for h264->av1")
	}
	if d.VideoCodec != media.VideoAV1 {
		t.Errorf("expected av1, got %s", d.VideoCodec)
	}
}

func TestResolve_MPEG2Source(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "mpeg2video", AudioCodec: "mp2", Width: 720, Height: 576, Interlaced: true},
		Output{VideoCodec: "h264", AudioCodec: "aac", Container: "mp4"},
	)
	if !d.NeedsTranscode {
		t.Error("expected transcode for mpeg2->h264")
	}
	if d.VideoCodec != media.VideoH264 {
		t.Errorf("expected h264, got %s", d.VideoCodec)
	}
	if !d.Deinterlace {
		t.Error("expected deinterlace for interlaced mpeg2")
	}
	if !d.NeedsAudioTranscode {
		t.Error("expected audio transcode mp2->aac")
	}
}

func TestResolve_SameCodecStillCopy(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "h264", AudioCodec: "aac", Container: "mp4"},
	)
	if d.NeedsTranscode {
		t.Error("expected copy when source and output codec match")
	}
	if d.VideoCodec != media.VideoCopy {
		t.Errorf("expected copy, got %s", d.VideoCodec)
	}
}

func TestResolve_HevcVariantNames(t *testing.T) {
	for _, name := range []string{"hevc", "hvc1", "hev1", "h.265"} {
		d := Resolve(
			Input{VideoCodec: name, AudioCodec: "aac", Width: 1920, Height: 1080},
			Output{VideoCodec: "h265", AudioCodec: "aac", Container: "mp4"},
		)
		if d.NeedsTranscode {
			t.Errorf("input %q should match h265 output without transcode", name)
		}
	}
}

func TestResolve_H264VariantNames(t *testing.T) {
	for _, name := range []string{"avc", "avc1", "h.264"} {
		d := Resolve(
			Input{VideoCodec: name, AudioCodec: "aac", Width: 1920, Height: 1080},
			Output{VideoCodec: "h264", AudioCodec: "aac", Container: "mp4"},
		)
		if d.NeedsTranscode {
			t.Errorf("input %q should match h264 output without transcode", name)
		}
	}
}

func TestResolve_InterlacedWithDefaultCodec(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h265", AudioCodec: "aac", Width: 1920, Height: 1080, Interlaced: true},
		Output{VideoCodec: "default", AudioCodec: "aac", Container: "mp4"},
	)
	if !d.NeedsTranscode {
		t.Error("expected transcode for interlaced source")
	}
	if d.VideoCodec != media.VideoH265 {
		t.Errorf("expected h265 (match source), got %s", d.VideoCodec)
	}
}

func TestResolve_BitDepth10WithHeight(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h265", AudioCodec: "aac", Width: 3840, Height: 2160, BitDepth: 10},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4", MaxBitDepth: 8, OutputHeight: 1080},
	)
	if !d.NeedsTranscode {
		t.Error("expected transcode for both bit depth and height exceed")
	}
}

func TestResolve_QSVHWAccel(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "h265", AudioCodec: "aac", Container: "mp4", HWAccel: "qsv"},
	)
	if d.HWAccel != "qsv" {
		t.Errorf("expected qsv, got %s", d.HWAccel)
	}
}

func TestResolve_NVENCHWAccel(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "h265", AudioCodec: "aac", Container: "mp4", HWAccel: "nvenc"},
	)
	if d.HWAccel != "nvenc" {
		t.Errorf("expected nvenc, got %s", d.HWAccel)
	}
}

func TestResolve_VideoToolboxHWAccel(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "h265", AudioCodec: "aac", Container: "mp4", HWAccel: "videotoolbox"},
	)
	if d.HWAccel != "videotoolbox" {
		t.Errorf("expected videotoolbox, got %s", d.HWAccel)
	}
}

func TestResolve_ContainerMKV(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "copy", Container: "mkv"},
	)
	if d.Container != media.ContainerMKV {
		t.Errorf("expected mkv container, got %s", d.Container)
	}
}

func TestResolve_ContainerMatroska(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "copy", Container: "matroska"},
	)
	if d.Container != media.ContainerMatroska {
		t.Errorf("expected matroska container, got %s", d.Container)
	}
	if d.AudioCodec != media.AudioCopy {
		t.Errorf("matroska should allow any audio codec, got %s", d.AudioCodec)
	}
}

func TestResolve_ContainerWebM(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "webm"},
	)
	if d.Container != media.ContainerWebM {
		t.Errorf("expected webm container, got %s", d.Container)
	}
	if d.AudioCodec != media.AudioOpus {
		t.Errorf("webm should force opus audio, got %s", d.AudioCodec)
	}
	if !d.NeedsAudioTranscode {
		t.Error("webm with non-opus audio should need audio transcode")
	}
}

func TestResolve_ContainerWebMOpusPassthrough(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "opus", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "opus", Container: "webm"},
	)
	if d.AudioCodec != media.AudioOpus {
		t.Errorf("expected opus, got %s", d.AudioCodec)
	}
}

func TestResolve_ContainerWebMCopyAudioAllowed(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "opus", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "copy", Container: "webm"},
	)
	if d.AudioCodec != media.AudioCopy {
		t.Errorf("webm with copy audio should stay copy, got %s", d.AudioCodec)
	}
}

func TestResolve_AudioEAC3NormalizedToAC3(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "eac3", Width: 1920, Height: 1080},
		Output{VideoCodec: "copy", AudioCodec: "copy", Container: "mpegts"},
	)
	if d.NeedsAudioTranscode {
		t.Error("expected audio copy")
	}
	if d.AudioCodec != media.AudioCopy {
		t.Errorf("expected copy, got %s", d.AudioCodec)
	}
}

func TestResolve_MaxBitDepthZeroIgnored(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h265", AudioCodec: "aac", Width: 3840, Height: 2160, BitDepth: 10},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4", MaxBitDepth: 0},
	)
	if d.NeedsTranscode {
		t.Error("MaxBitDepth=0 should be ignored")
	}
}

func TestResolve_OutputHeightZeroIgnored(t *testing.T) {
	d := Resolve(
		Input{VideoCodec: "h264", AudioCodec: "aac", Width: 3840, Height: 2160},
		Output{VideoCodec: "copy", AudioCodec: "aac", Container: "mp4", OutputHeight: 0},
	)
	if d.NeedsTranscode {
		t.Error("OutputHeight=0 should be ignored")
	}
}
