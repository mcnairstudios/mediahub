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
			name: "prefer AAC over AC3",
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
			name: "all AD multiple - pick best",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "ac3", IsAD: true, Channels: 6},
				{Index: 2, Codec: "aac", IsAD: true, Channels: 2},
			},
			want: 2,
		},
		{
			name: "prefer higher channels same codec",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "aac", Language: "en", Channels: 2},
				{Index: 2, Codec: "aac", Language: "en", Channels: 6},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  2,
		},
		{
			name: "no language match - keep all",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "ac3", Language: "de", Channels: 6},
				{Index: 2, Codec: "aac", Language: "de", Channels: 2},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  2,
		},
		{
			name: "codec priority order mp2 over ac3",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "ac3", Language: "en", Channels: 6},
				{Index: 2, Codec: "mp2", Language: "en", Channels: 2},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  2,
		},
		{
			name: "codec priority order ac3 over eac3",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "eac3", Language: "en", Channels: 6},
				{Index: 2, Codec: "ac3", Language: "en", Channels: 2},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  2,
		},
		{
			name: "codec priority order eac3 over dts",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "dts", Language: "en", Channels: 6},
				{Index: 2, Codec: "eac3", Language: "en", Channels: 2},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  2,
		},
		{
			name: "unknown codec loses to known",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "opus", Language: "en", Channels: 6},
				{Index: 2, Codec: "dts", Language: "en", Channels: 2},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  2,
		},
		{
			name: "AD skipped even if better codec",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "aac", Language: "en", Channels: 6, IsAD: true},
				{Index: 2, Codec: "ac3", Language: "en", Channels: 2},
				{Index: 3, Codec: "mp2", Language: "en", Channels: 2},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  3,
		},
		{
			name: "language filter then codec then channels",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "aac", Language: "fr", Channels: 6},
				{Index: 2, Codec: "ac3", Language: "en", Channels: 6},
				{Index: 3, Codec: "aac", Language: "en", Channels: 2},
				{Index: 4, Codec: "aac", Language: "en", Channels: 6},
			},
			prefs: AudioPrefs{Language: "en"},
			want:  4,
		},
		{
			name: "no prefs - best codec wins",
			tracks: []media.AudioTrack{
				{Index: 1, Codec: "ac3", Language: "en", Channels: 6},
				{Index: 2, Codec: "aac", Language: "fr", Channels: 2},
			},
			want: 2,
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
