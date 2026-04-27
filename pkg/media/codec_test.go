package media

import "testing"

func TestNormalizeVideoCodec(t *testing.T) {
	tests := []struct {
		input string
		want  VideoCodec
	}{
		{"hevc", VideoH265},
		{"h265", VideoH265},
		{"h.265", VideoH265},
		{"HEVC", VideoH265},
		{"hvc1", VideoH265},
		{"hev1", VideoH265},
		{"h264", VideoH264},
		{"h.264", VideoH264},
		{"H264", VideoH264},
		{"avc1", VideoH264},
		{"avc", VideoH264},
		{"av1", VideoAV1},
		{"AV1", VideoAV1},
		{"av01", VideoAV1},
		{"mpeg2video", VideoMPEG2},
		{"MPEG2VIDEO", VideoMPEG2},
		{"mpeg2", VideoMPEG2},
		{"copy", VideoCopy},
		{"COPY", VideoCopy},
		{"unknown_codec", VideoCodec("unknown_codec")},
		{"", VideoCodec("")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeVideoCodec(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeVideoCodec(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeAudioCodec(t *testing.T) {
	tests := []struct {
		input string
		want  AudioCodec
	}{
		{"aac", AudioAAC},
		{"AAC", AudioAAC},
		{"aac_latm", AudioAAC},
		{"ac3", AudioAC3},
		{"eac3", AudioAC3},
		{"EAC3", AudioAC3},
		{"mp2", AudioMP2},
		{"MP2", AudioMP2},
		{"mp3", AudioMP3},
		{"MP3", AudioMP3},
		{"opus", AudioOpus},
		{"OPUS", AudioOpus},
		{"copy", AudioCopy},
		{"COPY", AudioCopy},
		{"unknown_audio", AudioCodec("unknown_audio")},
		{"", AudioCodec("")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeAudioCodec(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeAudioCodec(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBaseVideoCodec(t *testing.T) {
	tests := []struct {
		input VideoCodec
		want  string
	}{
		{VideoH265, "h265"},
		{VideoCodec("hvc1"), "h265"},
		{VideoCodec("hev1"), "h265"},
		{VideoH264, "h264"},
		{VideoCodec("avc1"), "h264"},
		{VideoCodec("avc"), "h264"},
		{VideoAV1, "av1"},
		{VideoCodec("av01"), "av1"},
		{VideoMPEG2, "mpeg2video"},
		{VideoCopy, "copy"},
		{VideoCodec("something_else"), "something_else"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := BaseVideoCodec(tt.input)
			if got != tt.want {
				t.Errorf("BaseVideoCodec(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
