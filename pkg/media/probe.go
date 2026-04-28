package media

type ProbeResult struct {
	Video       *VideoInfo      `json:"video,omitempty"`
	AudioTracks []AudioTrack    `json:"audio_tracks,omitempty"`
	SubTracks   []SubtitleTrack `json:"sub_tracks,omitempty"`
	DurationMs  int64           `json:"duration_ms"`
}

type VideoInfo struct {
	Index      int    `json:"index"`
	Codec      string `json:"codec"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	BitDepth   int    `json:"bit_depth"`
	Interlaced bool   `json:"interlaced"`
	FramerateN int    `json:"framerate_n"`
	FramerateD int    `json:"framerate_d"`
	Extradata  []byte `json:"extradata,omitempty"`
	Profile    string `json:"profile,omitempty"`
	PixFmt     string `json:"pix_fmt,omitempty"`
}

type SubtitleTrack struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language,omitempty"`
}

func (vi *VideoInfo) FPS() float64 {
	if vi.FramerateD == 0 {
		return 0
	}
	return float64(vi.FramerateN) / float64(vi.FramerateD)
}

type AudioTrack struct {
	Index      int    `json:"index"`
	Codec      string `json:"codec"`
	Language   string `json:"language,omitempty"`
	Channels   int    `json:"channels"`
	SampleRate int    `json:"sample_rate"`
	BitRate    int    `json:"bit_rate,omitempty"`
	IsAD       bool   `json:"is_ad"`
}
