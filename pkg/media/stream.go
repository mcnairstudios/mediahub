package media

type Stream struct {
	ID         string `json:"id"`
	SourceType string `json:"source_type"`
	SourceID   string `json:"source_id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Group      string `json:"group"`
	TvgID      string `json:"tvg_id"`
	TvgName    string `json:"tvg_name"`
	TvgLogo    string `json:"tvg_logo"`
	IsActive   bool   `json:"is_active"`

	VideoCodec string `json:"video_codec,omitempty"`
	AudioCodec string `json:"audio_codec,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	BitDepth   int    `json:"bit_depth,omitempty"`
	Interlaced bool   `json:"interlaced,omitempty"`
	FramerateN int    `json:"framerate_n,omitempty"`
	FramerateD int    `json:"framerate_d,omitempty"`
	Duration   float64 `json:"duration,omitempty"`

	VODType        string `json:"vod_type,omitempty"`
	TMDBID         string `json:"tmdb_id,omitempty"`
	Year           string `json:"year,omitempty"`
	Season         int    `json:"season,omitempty"`
	Episode        int    `json:"episode,omitempty"`
	EpisodeName    string `json:"episode_name,omitempty"`
	CollectionName string `json:"collection_name,omitempty"`
	CollectionID   string `json:"collection_id,omitempty"`
	IsLocal        bool   `json:"is_local,omitempty"`
}
