package conv

import (
	"testing"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/media"
)

func TestCodecIDFromString(t *testing.T) {
	tests := []struct {
		name    string
		codec   string
		want    astiav.CodecID
		wantErr bool
	}{
		{"h264", "h264", astiav.CodecIDH264, false},
		{"hevc", "hevc", astiav.CodecIDHevc, false},
		{"h265 alias", "h265", astiav.CodecIDHevc, false},
		{"aac", "aac", astiav.CodecIDAac, false},
		{"opus", "opus", astiav.CodecIDOpus, false},
		{"case insensitive", "H264", astiav.CodecIDH264, false},
		{"unknown codec", "notacodec", astiav.CodecIDNone, true},
		{"empty string", "", astiav.CodecIDNone, true},
		{"aac_latm", "aac_latm", astiav.CodecIDAacLatm, false},
		{"mp3", "mp3", astiav.CodecIDMp3, false},
		{"av1", "av1", astiav.CodecIDAv1, false},
		{"subrip", "subrip", astiav.CodecIDSubrip, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CodecIDFromString(tt.codec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for codec %q, got nil", tt.codec)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for codec %q: %v", tt.codec, err)
			}
			if got != tt.want {
				t.Errorf("CodecIDFromString(%q) = %v, want %v", tt.codec, got, tt.want)
			}
		})
	}
}

func TestCodecIDFromStringCoversAllVideo(t *testing.T) {
	videoCodecs := []string{"h264", "hevc", "h265", "mpeg2video", "mpeg4", "vp8", "vp9", "av1", "theora"}
	for _, c := range videoCodecs {
		if _, err := CodecIDFromString(c); err != nil {
			t.Errorf("video codec %q not in map: %v", c, err)
		}
	}
}

func TestCodecIDFromStringCoversAllAudio(t *testing.T) {
	audioCodecs := []string{"aac", "aac_latm", "ac3", "eac3", "dts", "mp2", "mp3", "flac", "vorbis", "opus", "truehd", "pcm_s16le"}
	for _, c := range audioCodecs {
		if _, err := CodecIDFromString(c); err != nil {
			t.Errorf("audio codec %q not in map: %v", c, err)
		}
	}
}

func TestToAVPacket(t *testing.T) {
	tb := astiav.NewRational(1, 90000)
	pkt := &av.Packet{
		Type:     av.Video,
		Data:     []byte{0x00, 0x00, 0x00, 0x01, 0x65},
		PTS:      1_000_000_000,
		DTS:      1_000_000_000,
		Duration: 33_333_333,
		Keyframe: true,
	}

	avPkt, err := ToAVPacket(pkt, tb)
	if err != nil {
		t.Fatalf("ToAVPacket: %v", err)
	}
	defer avPkt.Free()

	if avPkt.Pts() != 90000 {
		t.Errorf("PTS = %d, want 90000", avPkt.Pts())
	}
	if avPkt.Dts() != 90000 {
		t.Errorf("DTS = %d, want 90000", avPkt.Dts())
	}
	if avPkt.Size() != 5 {
		t.Errorf("Size = %d, want 5", avPkt.Size())
	}
	if !avPkt.Flags().Has(astiav.PacketFlagKey) {
		t.Error("expected keyframe flag to be set")
	}
}

func TestToAVPacketNonKeyframe(t *testing.T) {
	tb := astiav.NewRational(1, 90000)
	pkt := &av.Packet{
		Type:     av.Video,
		Data:     []byte{0x00, 0x00, 0x00, 0x01, 0x41},
		PTS:      2_000_000_000,
		DTS:      2_000_000_000,
		Duration: 33_333_333,
		Keyframe: false,
	}

	avPkt, err := ToAVPacket(pkt, tb)
	if err != nil {
		t.Fatalf("ToAVPacket: %v", err)
	}
	defer avPkt.Free()

	if avPkt.Flags().Has(astiav.PacketFlagKey) {
		t.Error("expected keyframe flag NOT to be set")
	}
}

func TestToAVPacketEmptyData(t *testing.T) {
	tb := astiav.NewRational(1, 90000)
	pkt := &av.Packet{
		Type: av.Audio,
		Data: nil,
		PTS:  0,
		DTS:  0,
	}

	avPkt, err := ToAVPacket(pkt, tb)
	if err != nil {
		t.Fatalf("ToAVPacket: %v", err)
	}
	defer avPkt.Free()

	if avPkt.Size() != 0 {
		t.Errorf("expected size 0 for empty data, got %d", avPkt.Size())
	}
}

func TestCodecParamsFromVideoProbe(t *testing.T) {
	vi := &media.VideoInfo{
		Codec:     "h264",
		Width:     1920,
		Height:    1080,
		Extradata: []byte{0x01, 0x64, 0x00, 0x28},
	}

	cp, err := CodecParamsFromVideoProbe(vi)
	if err != nil {
		t.Fatalf("CodecParamsFromVideoProbe: %v", err)
	}
	defer cp.Free()

	if cp.CodecID() != astiav.CodecIDH264 {
		t.Errorf("CodecID = %v, want H264", cp.CodecID())
	}
	if cp.MediaType() != astiav.MediaTypeVideo {
		t.Errorf("MediaType = %v, want Video", cp.MediaType())
	}
	if cp.Width() != 1920 {
		t.Errorf("Width = %d, want 1920", cp.Width())
	}
	if cp.Height() != 1080 {
		t.Errorf("Height = %d, want 1080", cp.Height())
	}
}

func TestCodecParamsFromVideoProbeUnknownCodec(t *testing.T) {
	vi := &media.VideoInfo{
		Codec:  "notacodec",
		Width:  1920,
		Height: 1080,
	}

	_, err := CodecParamsFromVideoProbe(vi)
	if err == nil {
		t.Fatal("expected error for unknown codec")
	}
}

func TestCodecParamsFromAudioProbe(t *testing.T) {
	at := &media.AudioTrack{
		Codec:      "aac",
		SampleRate: 48000,
		Channels:   2,
	}

	cp, err := CodecParamsFromAudioProbe(at)
	if err != nil {
		t.Fatalf("CodecParamsFromAudioProbe: %v", err)
	}
	defer cp.Free()

	if cp.CodecID() != astiav.CodecIDAac {
		t.Errorf("CodecID = %v, want AAC", cp.CodecID())
	}
	if cp.MediaType() != astiav.MediaTypeAudio {
		t.Errorf("MediaType = %v, want Audio", cp.MediaType())
	}
	if cp.SampleRate() != 48000 {
		t.Errorf("SampleRate = %d, want 48000", cp.SampleRate())
	}
}

func TestCodecParamsFromAudioProbeUnknownCodec(t *testing.T) {
	at := &media.AudioTrack{
		Codec:      "notacodec",
		SampleRate: 44100,
	}

	_, err := CodecParamsFromAudioProbe(at)
	if err == nil {
		t.Fatal("expected error for unknown codec")
	}
}
