package api

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

func (s *Server) handleStreamDetail(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("id")
	if streamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "stream ID required")
		return
	}

	stream, err := s.deps.StreamStore.Get(r.Context(), streamID)
	if err != nil || stream == nil {
		httputil.RespondError(w, http.StatusNotFound, "stream not found")
		return
	}

	logoURL := stream.TvgLogo
	if s.deps.LogoCache != nil && logoURL != "" {
		logoURL = s.deps.LogoCache.Resolve(logoURL)
	}

	result := map[string]any{
		"id":        stream.ID,
		"name":      stream.Name,
		"group":     stream.Group,
		"tvg_logo":  logoURL,
		"vod_type":  stream.VODType,
		"tmdb_id":   stream.TMDBID,
		"year":      stream.Year,
		"season":    stream.Season,
		"episode":   stream.Episode,
		"is_active": stream.IsActive,
	}

	if s.deps.TMDBClient == nil {
		httputil.RespondJSON(w, http.StatusOK, result)
		return
	}

	mediaType := stream.VODType
	if mediaType == "" || mediaType == "episode" {
		if stream.Season > 0 || stream.Episode > 0 {
			mediaType = "series"
		} else {
			mediaType = "movie"
		}
	}

	if stream.TMDBID != "" {
		tmdbID, err := strconv.Atoi(stream.TMDBID)
		if err == nil && tmdbID > 0 {
			if mediaType == "movie" {
				movie, err := s.deps.TMDBClient.MovieDetail(tmdbID)
				if err == nil && movie != nil {
					enrichMovieResult(result, movie)
					httputil.RespondJSON(w, http.StatusOK, result)
					return
				}
			} else {
				series, err := s.deps.TMDBClient.TVDetail(tmdbID)
				if err == nil && series != nil {
					enrichSeriesResult(result, series)
					httputil.RespondJSON(w, http.StatusOK, result)
					return
				}
			}
		}
	}

	clean, yearStr := vodCleanName(stream.Name)
	year := 0
	if yearStr != "" {
		year, _ = strconv.Atoi(yearStr)
	} else if stream.Year != "" {
		year, _ = strconv.Atoi(stream.Year)
	}

	if mediaType == "movie" {
		movie, err := s.deps.TMDBClient.SearchMovie(clean, year)
		if err == nil && movie != nil {
			enrichMovieResult(result, movie)
		}
	} else {
		series, err := s.deps.TMDBClient.SearchTV(stream.Name)
		if err == nil && series != nil {
			enrichSeriesResult(result, series)
		}
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}

func (s *Server) handleTMDBImage(w http.ResponseWriter, r *http.Request) {
	if s.deps.TMDBImages == nil {
		http.Error(w, "TMDB images not configured", http.StatusServiceUnavailable)
		return
	}
	s.deps.TMDBImages.ServeHTTP(w, r)
}

func enrichMovieResult(result map[string]any, m *tmdbcache.Movie) {
	result["media_type"] = "movie"
	result["title"] = m.Title
	result["overview"] = m.Overview
	result["poster_path"] = m.PosterPath
	result["backdrop_path"] = m.BackdropPath
	result["release_date"] = m.ReleaseDate
	result["rating"] = m.Rating
	result["runtime"] = m.Runtime
	result["genres"] = m.Genres
	result["certification"] = m.Certification
	result["collection_id"] = m.CollectionID
	result["collection_name"] = m.CollectionName

	if m.PosterPath != "" {
		result["poster_url"] = "/api/tmdb/image?size=w500&path=" + m.PosterPath
	}
	if m.BackdropPath != "" {
		result["backdrop_url"] = "/api/tmdb/image?size=w1280&path=" + m.BackdropPath
	}

	cast := make([]map[string]any, 0, len(m.Cast))
	for _, c := range m.Cast {
		entry := map[string]any{
			"name":      c.Name,
			"character": c.Character,
			"tmdb_id":   c.TMDBID,
		}
		if c.ProfilePath != "" {
			entry["profile_path"] = c.ProfilePath
			entry["profile_url"] = "/api/tmdb/image?size=w185&path=" + c.ProfilePath
		}
		cast = append(cast, entry)
	}
	result["cast"] = cast

	crew := make([]map[string]any, 0, len(m.Crew))
	for _, c := range m.Crew {
		entry := map[string]any{
			"name":       c.Name,
			"job":        c.Job,
			"department": c.Department,
			"tmdb_id":    c.TMDBID,
		}
		if c.ProfilePath != "" {
			entry["profile_path"] = c.ProfilePath
			entry["profile_url"] = "/api/tmdb/image?size=w185&path=" + c.ProfilePath
		}
		crew = append(crew, entry)
	}
	result["crew"] = crew
}

func enrichSeriesResult(result map[string]any, s *tmdbcache.Series) {
	result["media_type"] = "series"
	result["title"] = s.Name
	result["overview"] = s.Overview
	result["poster_path"] = s.PosterPath
	result["backdrop_path"] = s.BackdropPath
	result["first_air_date"] = s.FirstAirDate
	result["rating"] = s.Rating
	result["genres"] = s.Genres

	if s.PosterPath != "" {
		result["poster_url"] = "/api/tmdb/image?size=w500&path=" + s.PosterPath
	}
	if s.BackdropPath != "" {
		result["backdrop_url"] = "/api/tmdb/image?size=w1280&path=" + s.BackdropPath
	}

	seasons := make([]map[string]any, 0, len(s.Seasons))
	for _, sn := range s.Seasons {
		entry := map[string]any{
			"season_number": sn.SeasonNumber,
			"name":          sn.Name,
			"overview":      sn.Overview,
			"episode_count": sn.EpisodeCount,
		}
		if sn.PosterPath != "" {
			entry["poster_path"] = sn.PosterPath
			entry["poster_url"] = "/api/tmdb/image?size=w342&path=" + sn.PosterPath
		}
		eps := make([]map[string]any, 0, len(sn.Episodes))
		for _, ep := range sn.Episodes {
			epEntry := map[string]any{
				"episode_number": ep.EpisodeNumber,
				"name":           ep.Name,
				"overview":       ep.Overview,
				"air_date":       ep.AirDate,
				"runtime":        ep.Runtime,
			}
			if ep.StillPath != "" {
				epEntry["still_path"] = ep.StillPath
				epEntry["still_url"] = "/api/tmdb/image?size=w300&path=" + ep.StillPath
			}
			eps = append(eps, epEntry)
		}
		entry["episodes"] = eps
		seasons = append(seasons, entry)
	}
	result["seasons"] = seasons
}

var vodYearParen = regexp.MustCompile(`\((\d{4})\)`)

func vodCleanName(name string) (string, string) {
	year := ""
	cleaned := name
	if m := vodYearParen.FindStringSubmatch(cleaned); len(m) > 1 {
		year = m[1]
		cleaned = vodYearParen.ReplaceAllString(cleaned, "")
	}
	return strings.TrimSpace(cleaned), year
}
