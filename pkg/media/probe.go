package media

type ProbeResult struct {
	Video       *VideoInfo
	AudioTracks []AudioTrack
	SubTracks   []SubtitleTrack
	DurationMs  int64
}

type VideoInfo struct {
	Index      int
	Codec      string
	Width      int
	Height     int
	BitDepth   int
	Interlaced bool
	FramerateN int
	FramerateD int
	Extradata  []byte
	Profile    string
	PixFmt     string
}

type SubtitleTrack struct {
	Index    int
	Codec    string
	Language string
}

func (vi *VideoInfo) FPS() float64 {
	if vi.FramerateD == 0 {
		return 0
	}
	return float64(vi.FramerateN) / float64(vi.FramerateD)
}

type AudioTrack struct {
	Index      int
	Codec      string
	Language   string
	Channels   int
	SampleRate int
	BitRate    int
	IsAD       bool
}
