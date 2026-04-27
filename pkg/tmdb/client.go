package tmdb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
)

type Client struct {
	httpClient *http.Client
	apiKeyFn   func() string
	cache      *tmdbcache.Cache
	mu         sync.Mutex
}

func NewClient(apiKeyFn func() string, cache *tmdbcache.Cache) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiKeyFn:   apiKeyFn,
		cache:      cache,
	}
}

func (c *Client) apiKey() string {
	if c.apiKeyFn == nil {
		return ""
	}
	return c.apiKeyFn()
}

func (c *Client) SearchMovie(query string, year int) (*tmdbcache.Movie, error) {
	key := c.apiKey()
	if key == "" {
		return nil, fmt.Errorf("no TMDB API key configured")
	}

	cacheKey := "search_movie_" + query
	if year > 0 {
		cacheKey += "_" + strconv.Itoa(year)
	}
	if m, ok := c.cache.GetMovie(cacheKey); ok {
		return m, nil
	}

	u := "https://api.themoviedb.org/3/search/movie?api_key=" + url.QueryEscape(key) +
		"&query=" + url.QueryEscape(query) + "&language=en-GB"
	if year > 0 {
		u += "&year=" + strconv.Itoa(year)
	}

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}

	if len(result.Results) == 0 {
		return nil, nil
	}

	first := result.Results[0]
	movie, err := c.MovieDetail(first.ID)
	if err != nil {
		return nil, err
	}

	c.cache.SetMovie(cacheKey, movie)
	return movie, nil
}

func (c *Client) SearchTV(query string) (*tmdbcache.Series, error) {
	key := c.apiKey()
	if key == "" {
		return nil, fmt.Errorf("no TMDB API key configured")
	}

	clean, year := cleanVODName(query)
	searchQuery := clean
	cacheKey := "search_tv_" + query

	if s, ok := c.cache.GetSeries(cacheKey); ok {
		return s, nil
	}

	u := "https://api.themoviedb.org/3/search/tv?api_key=" + url.QueryEscape(key) +
		"&query=" + url.QueryEscape(searchQuery) + "&language=en-GB"
	if year != "" {
		u += "&first_air_date_year=" + year
	}

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}

	if len(result.Results) == 0 {
		return nil, nil
	}

	first := result.Results[0]
	series, err := c.TVDetail(first.ID)
	if err != nil {
		return nil, err
	}

	c.cache.SetSeries(cacheKey, series)
	return series, nil
}

func (c *Client) MovieDetail(tmdbID int) (*tmdbcache.Movie, error) {
	key := c.apiKey()
	if key == "" {
		return nil, fmt.Errorf("no TMDB API key configured")
	}

	cacheKey := "detail_movie_" + strconv.Itoa(tmdbID)
	if m, ok := c.cache.GetMovie(cacheKey); ok {
		return m, nil
	}

	u := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d?api_key=%s&language=en-GB&append_to_response=credits,release_dates",
		tmdbID, url.QueryEscape(key))

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("detail request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB returned %d", resp.StatusCode)
	}

	var raw movieDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode detail: %w", err)
	}

	movie := &tmdbcache.Movie{
		ID:           raw.ID,
		Title:        raw.Title,
		Overview:     raw.Overview,
		PosterPath:   raw.PosterPath,
		BackdropPath: raw.BackdropPath,
		ReleaseDate:  raw.ReleaseDate,
		Rating:       raw.VoteAverage,
		Runtime:      raw.Runtime,
	}

	for _, g := range raw.Genres {
		movie.Genres = append(movie.Genres, g.Name)
	}

	if raw.BelongsToCollection.ID > 0 {
		movie.CollectionID = raw.BelongsToCollection.ID
		movie.CollectionName = raw.BelongsToCollection.Name
	}

	movie.Certification = extractMovieCertification(raw.ReleaseDates, "GB", "US")

	if raw.Credits.Cast != nil {
		for i, p := range raw.Credits.Cast {
			if i >= 20 {
				break
			}
			movie.Cast = append(movie.Cast, tmdbcache.CastMember{
				Name:        p.Name,
				Character:   p.Character,
				ProfilePath: p.ProfilePath,
				TMDBID:      p.ID,
			})
		}
	}

	if raw.Credits.Crew != nil {
		for _, p := range raw.Credits.Crew {
			if p.Job == "Director" || p.Job == "Writer" || p.Job == "Screenplay" || p.Department == "Directing" {
				movie.Crew = append(movie.Crew, tmdbcache.CrewMember{
					Name:        p.Name,
					Job:         p.Job,
					Department:  p.Department,
					ProfilePath: p.ProfilePath,
					TMDBID:      p.ID,
				})
			}
		}
	}

	c.cache.SetMovie(cacheKey, movie)
	return movie, nil
}

func (c *Client) TVDetail(tmdbID int) (*tmdbcache.Series, error) {
	key := c.apiKey()
	if key == "" {
		return nil, fmt.Errorf("no TMDB API key configured")
	}

	cacheKey := "detail_tv_" + strconv.Itoa(tmdbID)
	if s, ok := c.cache.GetSeries(cacheKey); ok {
		return s, nil
	}

	u := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d?api_key=%s&language=en-GB&append_to_response=credits,content_ratings",
		tmdbID, url.QueryEscape(key))

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("detail request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB returned %d", resp.StatusCode)
	}

	var raw tvDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode detail: %w", err)
	}

	series := &tmdbcache.Series{
		ID:           raw.ID,
		Name:         raw.Name,
		Overview:     raw.Overview,
		PosterPath:   raw.PosterPath,
		BackdropPath: raw.BackdropPath,
		FirstAirDate: raw.FirstAirDate,
		Rating:       raw.VoteAverage,
	}

	for _, g := range raw.Genres {
		series.Genres = append(series.Genres, g.Name)
	}

	for _, s := range raw.Seasons {
		series.Seasons = append(series.Seasons, tmdbcache.Season{
			SeasonNumber: s.SeasonNumber,
			Name:         s.Name,
			Overview:     s.Overview,
			PosterPath:   s.PosterPath,
			EpisodeCount: s.EpisodeCount,
		})
	}

	c.cache.SetSeries(cacheKey, series)
	return series, nil
}

func (c *Client) SeasonDetail(tvID, seasonNum int) (*tmdbcache.Season, error) {
	key := c.apiKey()
	if key == "" {
		return nil, fmt.Errorf("no TMDB API key configured")
	}

	cacheKey := fmt.Sprintf("season_%d_%d", tvID, seasonNum)
	if s, ok := c.cache.GetSeries(cacheKey); ok && len(s.Seasons) > 0 {
		return &s.Seasons[0], nil
	}

	u := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/season/%d?api_key=%s&language=en-GB",
		tvID, seasonNum, url.QueryEscape(key))

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("season request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB returned %d", resp.StatusCode)
	}

	var raw seasonDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode season: %w", err)
	}

	season := &tmdbcache.Season{
		SeasonNumber: raw.SeasonNumber,
		Name:         raw.Name,
		Overview:     raw.Overview,
		PosterPath:   raw.PosterPath,
		EpisodeCount: len(raw.Episodes),
	}

	for _, ep := range raw.Episodes {
		season.Episodes = append(season.Episodes, tmdbcache.Episode{
			EpisodeNumber: ep.EpisodeNumber,
			Name:          ep.Name,
			Overview:      ep.Overview,
			StillPath:     ep.StillPath,
			AirDate:       ep.AirDate,
			Runtime:       ep.Runtime,
		})
	}

	return season, nil
}

func (c *Client) PersonProfilePath(tmdbID string) string {
	key := c.apiKey()
	if key == "" {
		return ""
	}

	u := fmt.Sprintf("https://api.themoviedb.org/3/person/%s?api_key=%s", url.QueryEscape(tmdbID), url.QueryEscape(key))
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var raw struct {
		ProfilePath string `json:"profile_path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return ""
	}
	return raw.ProfilePath
}

func ImageURL(path string, size string) string {
	if path == "" {
		return ""
	}
	if size == "" {
		size = "w500"
	}
	return "https://image.tmdb.org/t/p/" + size + path
}

func (c *Client) SyncStream(streamName, mediaType, tmdbIDStr string) {
	if c.apiKey() == "" {
		return
	}

	clean, yearStr := cleanVODName(streamName)
	year := 0
	if yearStr != "" {
		year, _ = strconv.Atoi(yearStr)
	}

	if tmdbIDStr != "" {
		tmdbID, err := strconv.Atoi(tmdbIDStr)
		if err == nil && tmdbID > 0 {
			if mediaType == "movie" {
				c.MovieDetail(tmdbID)
			} else {
				c.TVDetail(tmdbID)
			}
			return
		}
	}

	if mediaType == "movie" {
		c.SearchMovie(clean, year)
	} else {
		c.SearchTV(streamName)
	}
}

func (c *Client) SyncBatch(items []SyncItem) {
	if c.apiKey() == "" {
		return
	}

	go func() {
		for _, item := range items {
			c.SyncStream(item.Name, item.MediaType, item.TMDBID)
			time.Sleep(250 * time.Millisecond)
		}
	}()
}

type SyncItem struct {
	Name      string
	MediaType string
	TMDBID    string
}

type searchResponse struct {
	Results []searchResult `json:"results"`
}

type searchResult struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	ReleaseDate  string  `json:"release_date"`
	FirstAirDate string  `json:"first_air_date"`
	VoteAverage  float64 `json:"vote_average"`
	MediaType    string  `json:"media_type"`
}

type genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type creditsPerson struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}

type credits struct {
	Cast []creditsPerson `json:"cast"`
	Crew []creditsPerson `json:"crew"`
}

type collectionRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type movieDetailResponse struct {
	ID                  int           `json:"id"`
	Title               string        `json:"title"`
	Overview            string        `json:"overview"`
	PosterPath          string        `json:"poster_path"`
	BackdropPath        string        `json:"backdrop_path"`
	ReleaseDate         string        `json:"release_date"`
	VoteAverage         float64       `json:"vote_average"`
	Runtime             int           `json:"runtime"`
	Genres              []genre       `json:"genres"`
	BelongsToCollection collectionRef `json:"belongs_to_collection"`
	Credits             credits       `json:"credits"`
	ReleaseDates        releaseDates  `json:"release_dates"`
}

type releaseDates struct {
	Results []releaseDateCountry `json:"results"`
}

type releaseDateCountry struct {
	ISO        string        `json:"iso_3166_1"`
	Dates      []releaseDate `json:"release_dates"`
}

type releaseDate struct {
	Certification string `json:"certification"`
}

type tvDetailResponse struct {
	ID           int            `json:"id"`
	Name         string         `json:"name"`
	Overview     string         `json:"overview"`
	PosterPath   string         `json:"poster_path"`
	BackdropPath string         `json:"backdrop_path"`
	FirstAirDate string         `json:"first_air_date"`
	VoteAverage  float64        `json:"vote_average"`
	Genres       []genre        `json:"genres"`
	Seasons      []seasonBrief  `json:"seasons"`
	Credits      credits        `json:"credits"`
}

type seasonBrief struct {
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	EpisodeCount int    `json:"episode_count"`
}

type seasonDetailResponse struct {
	SeasonNumber int             `json:"season_number"`
	Name         string          `json:"name"`
	Overview     string          `json:"overview"`
	PosterPath   string          `json:"poster_path"`
	Episodes     []episodeDetail `json:"episodes"`
}

type episodeDetail struct {
	EpisodeNumber int    `json:"episode_number"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	StillPath     string `json:"still_path"`
	AirDate       string `json:"air_date"`
	Runtime       int    `json:"runtime"`
}

var (
	editionTag = regexp.MustCompile(`\{[^}]+\}`)
	yearParen  = regexp.MustCompile(`\((\d{4})\)`)
)

func cleanVODName(name string) (clean string, year string) {
	cleaned := editionTag.ReplaceAllString(name, "")
	if m := yearParen.FindStringSubmatch(cleaned); len(m) > 1 {
		year = m[1]
		cleaned = yearParen.ReplaceAllString(cleaned, "")
	}
	return strings.TrimSpace(cleaned), year
}

func extractMovieCertification(rd releaseDates, countries ...string) string {
	for _, country := range countries {
		for _, r := range rd.Results {
			if r.ISO != country {
				continue
			}
			for _, d := range r.Dates {
				if d.Certification != "" {
					return d.Certification
				}
			}
		}
	}
	return ""
}
