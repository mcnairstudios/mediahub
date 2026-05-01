package tmdb

type MovieBlob struct {
	ID             int          `json:"id"`
	MediaType      string       `json:"media_type"`
	Title          string       `json:"title"`
	Overview       string       `json:"overview"`
	Year           string       `json:"year"`
	Rating         float64      `json:"rating"`
	Runtime        int          `json:"runtime"`
	Genres         []string     `json:"genres,omitempty"`
	Certification  string       `json:"certification,omitempty"`
	CollectionID   int          `json:"collection_id,omitempty"`
	CollectionName string       `json:"collection_name,omitempty"`
	Cast           []CastMember `json:"cast,omitempty"`
	Crew           []CrewMember `json:"crew,omitempty"`
}

type SeriesBlob struct {
	ID        int          `json:"id"`
	MediaType string       `json:"media_type"`
	Name      string       `json:"name"`
	Overview  string       `json:"overview"`
	Year      string       `json:"year"`
	Rating    float64      `json:"rating"`
	Genres    []string     `json:"genres,omitempty"`
	Seasons   []SeasonBlob `json:"seasons,omitempty"`
}

type SeasonBlob struct {
	SeasonNumber int           `json:"season_number"`
	Name         string        `json:"name"`
	Overview     string        `json:"overview,omitempty"`
	EpisodeCount int           `json:"episode_count"`
	Episodes     []EpisodeBlob `json:"episodes,omitempty"`
}

type EpisodeBlob struct {
	EpisodeNumber int    `json:"episode_number"`
	Name          string `json:"name"`
	Overview      string `json:"overview,omitempty"`
	AirDate       string `json:"air_date,omitempty"`
	Runtime       int    `json:"runtime,omitempty"`
}

type CastMember struct {
	Name      string `json:"name"`
	Character string `json:"character"`
	TMDBID    int    `json:"tmdb_id"`
}

type CrewMember struct {
	Name       string `json:"name"`
	Job        string `json:"job"`
	Department string `json:"department,omitempty"`
	TMDBID     int    `json:"tmdb_id"`
}

type QueueEntry struct {
	TMDBID      int    `json:"tmdb_id"`
	MediaType   string `json:"media_type"`
	Status      string `json:"status"`
	SeasonsDone []int  `json:"seasons_done,omitempty"`
	CreatedAt   int64  `json:"created_at"`
}

type ImageQueueEntry struct {
	TMDBPath string `json:"tmdb_path"`
	Size     string `json:"size"`
}

type StreamToResolve struct {
	StreamID  string
	Name      string
	Year      string
	MediaType string
	TMDBID    string
}

type ResolvedStream struct {
	StreamID string
	TMDBID   int
}
