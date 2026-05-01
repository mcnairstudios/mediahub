package selector

import (
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

func TestSelectAudio(t *testing.T) {
	tests := []struct {
		name   string
		tracks []media.AudioTrack
		prefs  AudioPrefs
		want   int
	}{
		{
			name:   "empty tracks",
			tracks: nil,
			want:   -1,
		},
		{
			name:   "single track",
			tracks: []media.AudioTrack{{Index: 1, Codec: "aac", Language: "en", Channels: 2}},
			want:   1,
		},
		{
			name: "prefer language",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "aac", Language: "en", Channels: 2},
				{Index: 2, Codec: "aac", Language: "fr", Channels: 2},
			},
			prefs: AudioPrefs{Language: "fr"},
			want:  2,
		},
		{
			name: "prefer stereo over multichannel",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "ac3", Language: "en", Channels: 6},
				{Index: 2, Codec: "aac", Language: "en", Channels: 2},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  2,
		},
		{
			name: "skip AD tracks",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "aac", Language: "en", Channels: 2, IsAD: true},
				{Index: 2, Codec: "ac3", Language: "en", Channels: 6},
			},
			want: 2,
		},
		{
			name: "all AD - use anyway",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "aac", IsAD: true, Channels: 2},
			},
			want: 1,
		},
		{
			name: "all AD multiple - prefer stereo",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "ac3", IsAD: true, Channels: 6},
				{Index: 2, Codec: "aac", IsAD: true, Channels: 2},
			},
			want: 2,
		},
		{
			name: "same channels - first wins",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "aac", Language: "en", Channels: 2},
				{Index: 2, Codec: "aac", Language: "en", Channels: 6},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  1,
		},
		{
			name: "no language match - prefer stereo",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "ac3", Language: "de", Channels: 6},
				{Index: 2, Codec: "aac", Language: "de", Channels: 2},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  2,
		},
		{
			name: "AD skipped even if first",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "aac", Language: "en", Channels: 6, IsAD: true},
				{Index: 2, Codec: "ac3", Language: "en", Channels: 2},
				{Index: 3, Codec: "mp2", Language: "en", Channels: 2},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  2,
		},
		{
			name: "language filter then prefer stereo",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "aac", Language: "fr", Channels: 6},
				{Index: 2, Codec: "ac3", Language: "en", Channels: 6},
				{Index: 3, Codec: "aac", Language: "en", Channels: 2},
				{Index: 4, Codec: "aac", Language: "en", Channels: 6},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  3,
		},
		{
			name: "no prefs - prefer stereo",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "ac3", Language: "en", Channels: 6},
				{Index: 2, Codec: "aac", Language: "fr", Channels: 2},
			},
			want: 2,
		},
		{
			name: "all multichannel - first wins",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "dts", Language: "en", Channels: 8},
				{Index: 2, Codec: "ac3", Language: "en", Channels: 6},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectAudio(tt.tracks, tt.prefs)
			if got != tt.want {
				t.Errorf("SelectAudio() = %d, want %d", got, tt.want)
			}
		})
	}
}
