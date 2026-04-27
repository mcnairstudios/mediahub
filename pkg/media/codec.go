package media

import "strings"

type VideoCodec string
type AudioCodec string
type Container string

const (
	VideoH264  VideoCodec = "h264"
	VideoH265  VideoCodec = "h265"
	VideoAV1   VideoCodec = "av1"
	VideoMPEG2 VideoCodec = "mpeg2video"
	VideoCopy  VideoCodec = "copy"

	AudioAAC  AudioCodec = "aac"
	AudioAC3  AudioCodec = "ac3"
	AudioMP2  AudioCodec = "mp2"
	AudioMP3  AudioCodec = "mp3"
	AudioOpus AudioCodec = "opus"
	AudioCopy AudioCodec = "copy"

	ContainerMP4    Container = "mp4"
	ContainerMPEGTS Container = "mpegts"
	ContainerMKV    Container = "mkv"
)

func NormalizeVideoCodec(s string) VideoCodec {
	lower := strings.ToLower(s)
	switch lower {
	case "h264", "h.264", "avc", "avc1":
		return VideoH264
	case "h265", "h.265", "hevc", "hvc1", "hev1":
		return VideoH265
	case "av1", "av01":
		return VideoAV1
	case "mpeg2video", "mpeg2":
		return VideoMPEG2
	case "copy":
		return VideoCopy
	default:
		return VideoCodec(lower)
	}
}

func NormalizeAudioCodec(s string) AudioCodec {
	lower := strings.ToLower(s)
	switch lower {
	case "aac", "aac_latm":
		return AudioAAC
	case "ac3", "eac3":
		return AudioAC3
	case "mp2":
		return AudioMP2
	case "mp3":
		return AudioMP3
	case "opus":
		return AudioOpus
	case "copy":
		return AudioCopy
	default:
		return AudioCodec(lower)
	}
}

func BaseVideoCodec(vc VideoCodec) string {
	switch vc {
	case VideoH264, "avc1", "avc":
		return "h264"
	case VideoH265, "hvc1", "hev1":
		return "h265"
	case VideoAV1, "av01":
		return "av1"
	case VideoMPEG2:
		return "mpeg2video"
	case VideoCopy:
		return "copy"
	default:
		return string(vc)
	}
}
