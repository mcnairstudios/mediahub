package api

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/tmdb"
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

func (s *Server) handleVODLibrary(w http.ResponseWriter, r *http.Request) {
	vodType := r.URL.Query().Get("type")
	sourceID := r.URL.Query().Get("source_id")

	cacheKey := "vod:" + vodType + ":" + sourceID
	if entry, ok := s.vodCache.Load(cacheKey); ok {
		if ce, ok := entry.(*vodCacheEntry); ok {
			httputil.RespondJSON(w, http.StatusOK, ce.data)
			return
		}
	}

	streams, err := s.deps.StreamStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list streams")
		return
	}

	var filtered []media.Stream
	for _, st := range streams {
		if st.VODType == "" {
			continue
		}
		if vodType != "" && st.VODType != vodType {
			continue
		}
		if sourceID != "" && st.SourceID != sourceID {
			continue
		}
		filtered = append(filtered, st)
	}

	type vodAlt struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Group    string `json:"group,omitempty"`
		SourceID string `json:"source_id,omitempty"`
	}

	type vodItem struct {
		ID                 string   `json:"id"`
		Name               string   `json:"name"`
		PosterURL          string   `json:"poster_url,omitempty"`
		BackdropURL        string   `json:"backdrop_url,omitempty"`
		Overview           string   `json:"overview,omitempty"`
		Rating             float64  `json:"rating,omitempty"`
		Year               string   `json:"year,omitempty"`
		Genres             []string `json:"genres,omitempty"`
		Certification      string   `json:"certification,omitempty"`
		VODType            string   `json:"vod_type"`
		Group              string   `json:"group,omitempty"`
		CollectionName     string   `json:"collection_name,omitempty"`
		CollectionID       int      `json:"collection_id,omitempty"`
		CollectionPoster   string   `json:"collection_poster,omitempty"`
		CollectionBackdrop string   `json:"collection_backdrop,omitempty"`
		Season             int      `json:"season,omitempty"`
		Episode            int      `json:"episode,omitempty"`
		EpisodeName        string   `json:"episode_name,omitempty"`
		SourceID           string   `json:"source_id,omitempty"`
		SourceType         string   `json:"source_type,omitempty"`
		Alternates         []vodAlt `json:"alternates,omitempty"`
	}

	type cachedLookup struct {
		movie  *tmdbcache.Movie
		series *tmdbcache.Series
	}

	lookupCache := make(map[string]*cachedLookup)
	var uncached []tmdb.SyncItem

	lookupTMDB := func(st media.Stream) *cachedLookup {
		mediaType := st.VODType
		if mediaType == "" || mediaType == "episode" {
			if st.Season > 0 || st.Episode > 0 {
				mediaType = "series"
			} else {
				mediaType = "movie"
			}
		}

		lookupName := st.Name
		if mediaType == "series" && st.CollectionName != "" {
			lookupName = st.CollectionName
		}

		cacheKey := lookupName + "_" + mediaType
		if cached, ok := lookupCache[cacheKey]; ok {
			return cached
		}

		cached := &cachedLookup{}
		lookupCache[cacheKey] = cached

		if s.deps.TMDBCache == nil {
			return cached
		}

		clean, yearStr := vodCleanName(lookupName)
		year := 0
		if yearStr != "" {
			year, _ = strconv.Atoi(yearStr)
		} else if st.Year != "" {
			year, _ = strconv.Atoi(st.Year)
		}

		if mediaType == "movie" {
			movieCacheKey := "search_movie_" + clean
			if year > 0 {
				movieCacheKey += "_" + strconv.Itoa(year)
			}
			if m, ok := s.deps.TMDBCache.GetMovie(movieCacheKey); ok {
				cached.movie = m
			} else {
				uncached = append(uncached, tmdb.SyncItem{Name: lookupName, MediaType: mediaType, TMDBID: st.TMDBID})
			}
		} else {
			seriesCacheKey := "search_tv_" + lookupName
			if _, ok := s.deps.TMDBCache.GetSeries(seriesCacheKey); !ok {
				seriesCacheKey = "search_tv_" + clean
			}
			if sr, ok := s.deps.TMDBCache.GetSeries(seriesCacheKey); ok {
				cached.series = sr
			} else {
				uncached = append(uncached, tmdb.SyncItem{Name: lookupName, MediaType: mediaType, TMDBID: st.TMDBID})
			}
		}

		return cached
	}

	var items []vodItem
	for _, st := range filtered {
		cached := lookupTMDB(st)

		item := vodItem{
			ID:         st.ID,
			Name:       st.Name,
			VODType:    st.VODType,
			Group:      st.Group,
			Season:     st.Season,
			Episode:    st.Episode,
			EpisodeName: st.EpisodeName,
			SourceID:   st.SourceID,
			SourceType: st.SourceType,
			Year:       st.Year,
		}

		if st.CollectionName != "" {
			item.CollectionName = st.CollectionName
		}
		if st.CollectionID != "" {
			cid, _ := strconv.Atoi(st.CollectionID)
			item.CollectionID = cid
		}

		if cached.movie != nil {
			m := cached.movie
			item.Overview = m.Overview
			item.Rating = m.Rating
			item.Genres = m.Genres
			item.Certification = m.Certification
			if m.ReleaseDate != "" && len(m.ReleaseDate) >= 4 {
				item.Year = m.ReleaseDate[:4]
			}
			if m.PosterPath != "" {
				item.PosterURL = "/api/tmdb/image?size=w500&path=" + m.PosterPath
			}
			if m.BackdropPath != "" {
				item.BackdropURL = "/api/tmdb/image?size=w1280&path=" + m.BackdropPath
			}
			if m.CollectionID > 0 {
				item.CollectionID = m.CollectionID
				item.CollectionName = m.CollectionName
			}
		} else if cached.series != nil {
			sr := cached.series
			item.Overview = sr.Overview
			item.Rating = sr.Rating
			item.Genres = sr.Genres
			if sr.FirstAirDate != "" && len(sr.FirstAirDate) >= 4 {
				item.Year = sr.FirstAirDate[:4]
			}
			if sr.PosterPath != "" {
				item.PosterURL = "/api/tmdb/image?size=w500&path=" + sr.PosterPath
			}
			if sr.BackdropPath != "" {
				item.BackdropURL = "/api/tmdb/image?size=w1280&path=" + sr.BackdropPath
			}
		}

		items = append(items, item)
	}

	if s.deps.TMDBClient != nil && len(uncached) > 0 {
		s.deps.TMDBClient.SyncBatch(uncached)
	}

	scoreItem := func(item vodItem) int {
		score := 0
		if item.PosterURL != "" {
			score += 5
		}
		if item.Overview != "" {
			score += 3
		}
		if item.Rating > 0 {
			score += 2
		}
		if item.Year != "" {
			score += 1
		}
		return score
	}

	if vodType == "movie" || vodType == "" {
		deduped := make(map[string]int)
		var merged []vodItem
		for _, item := range items {
			if item.VODType == "series" {
				merged = append(merged, item)
				continue
			}
			key := strings.ToLower(item.Name)
			if idx, exists := deduped[key]; exists {
				existing := &merged[idx]
				if scoreItem(item) > scoreItem(*existing) {
					existing.Alternates = append(existing.Alternates, vodAlt{ID: existing.ID, Name: existing.Name, Group: existing.Group, SourceID: existing.SourceID})
					item.Alternates = existing.Alternates
					merged[idx] = item
				} else {
					existing.Alternates = append(existing.Alternates, vodAlt{ID: item.ID, Name: item.Name, Group: item.Group, SourceID: item.SourceID})
				}
			} else {
				deduped[key] = len(merged)
				merged = append(merged, item)
			}
		}
		items = merged
	}

	if items == nil {
		items = []vodItem{}
	}

	cached := 0
	for _, item := range items {
		if item.PosterURL != "" {
			cached++
		}
	}

	var syncStatus *tmdb.SyncStatus
	if s.deps.TMDBClient != nil {
		st := s.deps.TMDBClient.Status()
		syncStatus = &st
	}

	result := map[string]any{
		"items":  items,
		"total":  len(items),
		"cached": cached,
		"sync":   syncStatus,
	}

	s.vodCache.Store(cacheKey, &vodCacheEntry{data: result, createdAt: time.Now()})

	httputil.RespondJSON(w, http.StatusOK, result)
}

func (s *Server) handleTMDBSyncStatus(w http.ResponseWriter, r *http.Request) {
	if s.deps.TMDBClient == nil {
		httputil.RespondJSON(w, http.StatusOK, tmdb.SyncStatus{})
		return
	}
	httputil.RespondJSON(w, http.StatusOK, s.deps.TMDBClient.Status())
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
