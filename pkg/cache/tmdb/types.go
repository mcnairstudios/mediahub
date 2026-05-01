package tmdb

type Movie struct {
	ID             int          `json:"id"`
	Title          string       `json:"title"`
	Overview       string       `json:"overview"`
	PosterPath     string       `json:"poster_path,omitempty"`
	BackdropPath   string       `json:"backdrop_path,omitempty"`
	ReleaseDate    string       `json:"release_date,omitempty"`
	Rating         float64      `json:"rating"`
	Genres         []string     `json:"genres,omitempty"`
	Runtime        int          `json:"runtime"`
	Certification  string       `json:"certification,omitempty"`
	Cast           []CastMember `json:"cast,omitempty"`
	Crew           []CrewMember `json:"crew,omitempty"`
	CollectionID           int          `json:"collection_id,omitempty"`
	CollectionName         string       `json:"collection_name,omitempty"`
	CollectionPosterPath   string       `json:"collection_poster_path,omitempty"`
	CollectionBackdropPath string       `json:"collection_backdrop_path,omitempty"`
}

type Series struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Overview     string   `json:"overview"`
	PosterPath   string   `json:"poster_path,omitempty"`
	BackdropPath string   `json:"backdrop_path,omitempty"`
	FirstAirDate string   `json:"first_air_date,omitempty"`
	Rating       float64  `json:"rating"`
	Genres       []string `json:"genres,omitempty"`
	Seasons      []Season `json:"seasons,omitempty"`
}

type Season struct {
	SeasonNumber int       `json:"season_number"`
	Name         string    `json:"name"`
	Overview     string    `json:"overview"`
	PosterPath   string    `json:"poster_path,omitempty"`
	EpisodeCount int       `json:"episode_count"`
	Episodes     []Episode `json:"episodes,omitempty"`
}

type Episode struct {
	EpisodeNumber int    `json:"episode_number"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	StillPath     string `json:"still_path,omitempty"`
	AirDate       string `json:"air_date,omitempty"`
	Runtime       int    `json:"runtime"`
}

type CastMember struct {
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path,omitempty"`
	TMDBID      int    `json:"tmdb_id"`
}

type CrewMember struct {
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path,omitempty"`
	TMDBID      int    `json:"tmdb_id"`
}
