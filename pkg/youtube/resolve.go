package youtube

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/kkdai/youtube/v2"
)

// IsYouTubeURL returns true if the given URL is a YouTube watch URL or short link.
func IsYouTubeURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := strings.ToLower(u.Hostname())

	switch {
	case host == "youtu.be":
		return strings.TrimPrefix(u.Path, "/") != ""
	case host == "www.youtube.com" || host == "youtube.com" || host == "m.youtube.com":
		if u.Path == "/watch" && u.Query().Get("v") != "" {
			return true
		}
		if strings.HasPrefix(u.Path, "/shorts/") || strings.HasPrefix(u.Path, "/live/") {
			return true
		}
		return false
	default:
		return false
	}
}

// maxDesiredHeight is the upper bound for automatic quality selection.
// We prefer up to 1080p to balance quality and bandwidth.
const maxDesiredHeight = 1080

// ResolveStreamURL resolves a YouTube watch URL to a direct streamable URL.
// It prefers mp4 formats with both video and audio (progressive) at up to 1080p.
func ResolveStreamURL(ctx context.Context, videoURL string) (string, error) {
	client := youtube.Client{}

	video, err := client.GetVideoContext(ctx, videoURL)
	if err != nil {
		return "", fmt.Errorf("youtube: get video %q: %w", videoURL, err)
	}

	if len(video.Formats) == 0 {
		return "", fmt.Errorf("youtube: no formats available for %q", videoURL)
	}

	// Filter to progressive formats (have both video and audio) in mp4 container.
	var candidates []youtube.Format
	for _, f := range video.Formats {
		if f.AudioChannels == 0 {
			continue // DASH video-only
		}
		if !strings.Contains(f.MimeType, "video/mp4") {
			continue
		}
		candidates = append(candidates, f)
	}

	// If no mp4 progressive formats, try any format with audio.
	if len(candidates) == 0 {
		for _, f := range video.Formats {
			if f.AudioChannels > 0 {
				candidates = append(candidates, f)
			}
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("youtube: no suitable formats with audio for %q", videoURL)
	}

	// Pick the best candidate: highest quality up to maxDesiredHeight.
	best := candidates[0]
	for _, f := range candidates[1:] {
		if isBetterFormat(f, best) {
			best = f
		}
	}

	streamURL, err := client.GetStreamURLContext(ctx, video, &best)
	if err != nil {
		return "", fmt.Errorf("youtube: get stream URL for %q (itag %d): %w", videoURL, best.ItagNo, err)
	}

	return streamURL, nil
}

// isBetterFormat returns true if a is a better choice than b.
func isBetterFormat(a, b youtube.Format) bool {
	aOK := a.Height <= maxDesiredHeight
	bOK := b.Height <= maxDesiredHeight

	switch {
	case aOK && !bOK:
		return true
	case !aOK && bOK:
		return false
	case aOK && bOK:
		return a.Height > b.Height
	default:
		// Both exceed max — pick the smaller one (closer to max).
		return a.Height < b.Height
	}
}
