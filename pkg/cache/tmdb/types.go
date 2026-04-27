package tmdb

type Movie struct {
	ID             int
	Title          string
	Overview       string
	PosterPath     string
	BackdropPath   string
	ReleaseDate    string
	Rating         float64
	Genres         []string
	Runtime        int
	Certification  string
	Cast           []CastMember
	Crew           []CrewMember
	CollectionID   int
	CollectionName string
}

type Series struct {
	ID           int
	Name         string
	Overview     string
	PosterPath   string
	BackdropPath string
	FirstAirDate string
	Rating       float64
	Genres       []string
	Seasons      []Season
}

type Season struct {
	SeasonNumber int
	Name         string
	Overview     string
	PosterPath   string
	EpisodeCount int
	Episodes     []Episode
}

type Episode struct {
	EpisodeNumber int
	Name          string
	Overview      string
	StillPath     string
	AirDate       string
	Runtime       int
}

type CastMember struct {
	Name        string
	Character   string
	ProfilePath string
	TMDBID      int
}

type CrewMember struct {
	Name        string
	Job         string
	Department  string
	ProfilePath string
	TMDBID      int
}
