package selector

import "github.com/mcnairstudios/mediahub/pkg/media"

type AudioPrefs struct {
	Language string
}

func SelectAudio(tracks []media.AudioTrack, prefs AudioPrefs) int {
	if len(tracks) == 0 {
		return -1
	}

	candidates := make([]media.AudioTrack, 0, len(tracks))
	for _, t := range tracks {
		if !t.IsAD {
			candidates = append(candidates, t)
		}
	}
	if len(candidates) == 0 {
		candidates = tracks
	}

	if prefs.Language != "" {
		langMatches := make([]media.AudioTrack, 0, len(candidates))
		for _, t := range candidates {
			if t.Language == prefs.Language {
				langMatches = append(langMatches, t)
			}
		}
		if len(langMatches) > 0 {
			candidates = langMatches
		}
	}

	for _, t := range candidates {
		if t.Channels <= 2 {
			return t.Index
		}
	}

	return candidates[0].Index
}
