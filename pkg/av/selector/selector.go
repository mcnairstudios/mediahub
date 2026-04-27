package selector

import "github.com/mcnairstudios/mediahub/pkg/media"

type AudioPrefs struct {
	Language string
}

func codecPriority(codec string) int {
	switch codec {
	case "aac":
		return 50
	case "mp2":
		return 40
	case "ac3":
		return 30
	case "eac3":
		return 20
	case "dts":
		return 10
	default:
		return 0
	}
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

	best := candidates[0]
	for _, t := range candidates[1:] {
		tp := codecPriority(t.Codec)
		bp := codecPriority(best.Codec)
		if tp > bp || (tp == bp && t.Channels > best.Channels) {
			best = t
		}
	}

	return best.Index
}
