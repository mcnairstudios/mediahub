package jellyfin

import (
	"context"
	"strings"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

func (s *Server) buildStreamItems(ctx context.Context, searchTerm string) []BaseItemDto {
	if s.streams == nil {
		return nil
	}
	streams, err := s.streams.List(ctx)
	if err != nil {
		return nil
	}

	var items []BaseItemDto
	for _, st := range streams {
		if searchTerm != "" && !strings.Contains(strings.ToLower(st.Name), searchTerm) {
			continue
		}
		items = append(items, s.streamToItem(&st))
	}
	return items
}

func (s *Server) streamToItem(st *media.Stream) BaseItemDto {
	itemID := stripDashes(st.ID)

	item := BaseItemDto{
		Name:         st.Name,
		SortName:     sortName(st.Name),
		Container:    "mp4",
		ServerID:     s.serverID,
		ID:           itemID,
		Type:         "Movie",
		MediaType:    "Video",
		IsFolder:     false,
		LocationType: "FileSystem",
		ImageTags:    map[string]string{},
		UserData:     &UserItemData{Key: st.ID},
		MediaSources: []MediaSource{{
			Protocol: "Http", ID: itemID, Type: "Default", Name: st.Name,
			Container: "mp4", IsRemote: true, SupportsTranscoding: true,
			SupportsDirectStream: true, SupportsDirectPlay: false,
			TranscodingURL:         channelStreamURL(itemID),
			TranscodingSubProtocol: "http", TranscodingContainer: "mp4",
			MediaStreams: buildMediaStreams(st),
		}},
	}

	if st.Duration > 0 {
		item.RunTimeTicks = secondsToTicks(st.Duration)
		item.MediaSources[0].RunTimeTicks = item.RunTimeTicks
	}

	if st.Width > 0 && st.Height > 0 {
		item.Width = st.Width
		item.Height = st.Height
	}

	if s.tmdbCache != nil {
		if m, ok := s.tmdbCache.GetMovie(st.Name); ok && m != nil {
			item.Overview = m.Overview
			item.CommunityRating = m.Rating
			item.OfficialRating = m.Certification
			item.Genres = m.Genres
			item.GenreItems = genreItems(m.Genres)
			if m.ReleaseDate != "" {
				year := m.ReleaseDate
				if len(year) >= 4 {
					year = year[:4]
				}
				item.PremiereDate = m.ReleaseDate + "T00:00:00.0000000Z"
				item.DateCreated = m.ReleaseDate + "T00:00:00.0000000Z"
			}
			if m.PosterPath != "" {
				item.ImageTags["Primary"] = "tmdb"
			}
			if m.BackdropPath != "" {
				item.BackdropImageTags = []string{"tmdb"}
			}
		}
	}

	return item
}

func buildMediaStreams(st *media.Stream) []MediaStream {
	videoCodec := "h264"
	audioCodec := "aac"
	width, height := 1920, 1080

	if st.VideoCodec != "" {
		vc := strings.ToLower(st.VideoCodec)
		switch {
		case vc == "hevc" || vc == "h265":
			videoCodec = "hevc"
		case vc == "h264" || vc == "avc":
			videoCodec = "h264"
		case vc == "av1":
			videoCodec = "av1"
		default:
			videoCodec = vc
		}
	}
	if st.AudioCodec != "" {
		ac := strings.ToLower(st.AudioCodec)
		switch {
		case strings.Contains(ac, "aac"):
			audioCodec = "aac"
		case strings.Contains(ac, "ac3") || strings.Contains(ac, "ac-3"):
			audioCodec = "ac3"
		case strings.Contains(ac, "eac3") || strings.Contains(ac, "e-ac-3"):
			audioCodec = "eac3"
		case strings.Contains(ac, "dts"):
			audioCodec = "dca"
		case strings.Contains(ac, "opus"):
			audioCodec = "opus"
		case strings.Contains(ac, "mp3"):
			audioCodec = "mp3"
		default:
			audioCodec = ac
		}
	}
	if st.Width > 0 && st.Height > 0 {
		width = st.Width
		height = st.Height
	}

	return []MediaStream{
		{Type: "Video", Codec: videoCodec, Index: 0, IsDefault: true, Width: width, Height: height},
		{Type: "Audio", Codec: audioCodec, Index: 1, IsDefault: true, Channels: 2},
	}
}
