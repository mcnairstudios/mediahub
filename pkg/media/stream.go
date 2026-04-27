package media

type Stream struct {
	ID         string
	SourceType string
	SourceID   string
	Name       string
	URL        string
	Group      string
	TvgID      string
	TvgName    string
	TvgLogo    string
	IsActive   bool

	VideoCodec string
	AudioCodec string
	Width      int
	Height     int
	BitDepth   int
	Interlaced bool
	FramerateN int
	FramerateD int
	Duration   float64

	VODType        string
	TMDBID         string
	Year           string
	Season         int
	Episode        int
	EpisodeName    string
	CollectionName string
	CollectionID   string
	IsLocal        bool
}
