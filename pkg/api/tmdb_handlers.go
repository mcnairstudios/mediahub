package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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

	if s.deps.TMDBStore != nil && stream.TMDBID != "" {
		tmdbID, err := strconv.Atoi(stream.TMDBID)
		if err == nil && tmdbID > 0 {
			mt := "movie"
			isSeries := stream.VODType == "series" || stream.VODType == "episode" || stream.Season > 0
			if isSeries {
				mt = "series"
			}
			blob, err := s.deps.TMDBStore.GetBlobTyped(mt, tmdbID)
			if err == nil && blob != nil {
				if isSeries && stream.Season > 0 && stream.Episode > 0 {
					var sb tmdb.SeriesBlob
					if json.Unmarshal(blob, &sb) == nil {
						result["series_name"] = sb.Name
						result["series_overview"] = sb.Overview
						result["series_rating"] = sb.Rating
						result["series_year"] = sb.Year
						result["series_genres"] = sb.Genres
						result["series_poster_url"] = fmt.Sprintf("/api/tmdb/i/%d/poster.jpg", tmdbID)
						result["series_backdrop_url"] = fmt.Sprintf("/api/tmdb/i/%d/backdrop.jpg", tmdbID)
						for _, sn := range sb.Seasons {
							if sn.SeasonNumber != stream.Season {
								continue
							}
							result["season_name"] = sn.Name
							for _, ep := range sn.Episodes {
								if ep.EpisodeNumber != stream.Episode {
									continue
								}
								if ep.Name != "" {
									result["episode_name"] = ep.Name
								}
								if ep.Overview != "" {
									result["episode_overview"] = ep.Overview
								}
								if ep.AirDate != "" {
									result["episode_air_date"] = ep.AirDate
								}
								if ep.Runtime > 0 {
									result["episode_runtime"] = ep.Runtime
								}
								result["still_url"] = fmt.Sprintf("/api/tmdb/i/%s/s%de%d.jpg", stream.TMDBID, stream.Season, stream.Episode)
								break
							}
							break
						}
						httputil.RespondJSON(w, http.StatusOK, result)
						return
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(blob)
				return
			}
		}
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}

func (s *Server) handleTMDBDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("tmdbID")
	tmdbID, err := strconv.Atoi(idStr)
	if err != nil || tmdbID <= 0 {
		httputil.RespondError(w, http.StatusBadRequest, "invalid TMDB ID")
		return
	}

	if s.deps.TMDBStore == nil {
		httputil.RespondError(w, http.StatusNotFound, "TMDB store not configured")
		return
	}

	mt := r.URL.Query().Get("type")
	if mt == "" {
		mt = "movie"
	}
	blob, err := s.deps.TMDBStore.GetBlobTyped(mt, tmdbID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "store error")
		return
	}
	if blob == nil {
		httputil.RespondError(w, http.StatusNotFound, "no metadata for this TMDB ID")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(blob)
}

func (s *Server) handleTMDBQueue(w http.ResponseWriter, r *http.Request) {
	var metaCount, imageCount int
	if s.deps.TMDBStore != nil {
		metaCount, _ = s.deps.TMDBStore.QueueCount()
		imageCount, _ = s.deps.TMDBStore.ImageQueueCount()
	}
	httputil.RespondJSON(w, http.StatusOK, map[string]int{
		"metadata": metaCount,
		"images":   imageCount,
	})
}

func (s *Server) handleVODCategories(w http.ResponseWriter, r *http.Request) {
	vodType := r.URL.Query().Get("type")
	sourceID := r.URL.Query().Get("source_id")

	var streams []media.Stream
	var err error
	if sourceID != "" && s.deps.SourceConfigStore != nil {
		if sc, scErr := s.deps.SourceConfigStore.Get(r.Context(), sourceID); scErr == nil && sc != nil {
			if vodType != "" && vodType != "series" {
				streams, err = s.deps.StreamStore.ListBySourceAndType(r.Context(), sc.Type, sourceID, vodType)
			} else {
				streams, err = s.deps.StreamStore.ListBySource(r.Context(), sc.Type, sourceID)
			}
		}
	}
	if streams == nil && err == nil {
		streams, err = s.deps.StreamStore.List(r.Context())
	}
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list streams")
		return
	}

	counts := make(map[string]int)
	for _, st := range streams {
		if st.VODType == "" {
			continue
		}
		if vodType != "" {
			if vodType == "series" {
				if st.VODType != "series" && st.VODType != "episode" {
					continue
				}
			} else if st.VODType != vodType {
				continue
			}
		}
		g := st.Group
		if g == "" {
			g = "Uncategorized"
		}
		counts[g]++
	}

	type cat struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	var cats []cat
	for name, count := range counts {
		cats = append(cats, cat{Name: name, Count: count})
	}
	if cats == nil {
		cats = []cat{}
	}

	httputil.RespondJSON(w, http.StatusOK, cats)
}

func (s *Server) handleVODLibrary(w http.ResponseWriter, r *http.Request) {
	vodType := r.URL.Query().Get("type")
	sourceID := r.URL.Query().Get("source_id")
	groupFilter := r.URL.Query().Get("group")

	var streams []media.Stream
	var err error
	if sourceID != "" && s.deps.SourceConfigStore != nil {
		if sc, scErr := s.deps.SourceConfigStore.Get(r.Context(), sourceID); scErr == nil && sc != nil {
			if vodType != "" && vodType != "series" {
				streams, err = s.deps.StreamStore.ListBySourceAndType(r.Context(), sc.Type, sourceID, vodType)
			} else {
				streams, err = s.deps.StreamStore.ListBySource(r.Context(), sc.Type, sourceID)
			}
		}
	}
	if streams == nil && err == nil {
		streams, err = s.deps.StreamStore.List(r.Context())
	}
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list streams")
		return
	}

	var filtered []media.Stream
	for _, st := range streams {
		if st.VODType == "" {
			continue
		}
		if vodType != "" {
			if vodType == "series" {
				if st.VODType != "series" && st.VODType != "episode" {
					continue
				}
			} else if st.VODType != vodType {
				continue
			}
		}
		if vodType == "series" && st.Season == 0 && st.Episode == 0 && st.IsLocal {
			continue
		}
		if groupFilter != "" && st.Group != groupFilter {
			continue
		}
		filtered = append(filtered, st)
	}

	type slimItem struct {
		ID             string   `json:"id"`
		Name           string   `json:"name"`
		TMDBID         string   `json:"tmdb_id,omitempty"`
		VODType        string   `json:"vod_type"`
		Group          string   `json:"group,omitempty"`
		Series         string   `json:"series,omitempty"`
		Season         int      `json:"season,omitempty"`
		Episode        int      `json:"episode,omitempty"`
		Year           string   `json:"year,omitempty"`
		Tags           []string `json:"tags,omitempty"`
		Genres         []string `json:"genres,omitempty"`
		Certification  string   `json:"certification,omitempty"`
		CollectionName string   `json:"collection_name,omitempty"`
	}

	type vodAlt struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Group    string `json:"group,omitempty"`
		SourceID string `json:"source_id,omitempty"`
	}

	type vodItem struct {
		ID              string   `json:"id"`
		Name            string   `json:"name"`
		TMDBID          string   `json:"tmdb_id,omitempty"`
		PosterURL       string   `json:"poster_url,omitempty"`
		Rating          float64  `json:"rating,omitempty"`
		Year            string   `json:"year,omitempty"`
		Genres          []string `json:"genres,omitempty"`
		Certification   string   `json:"certification,omitempty"`
		VODType         string   `json:"vod_type"`
		Group           string   `json:"group,omitempty"`
		Series          string   `json:"series,omitempty"`
		CollectionName  string   `json:"collection_name,omitempty"`
		CollectionID    int      `json:"collection_id,omitempty"`
		Season          int      `json:"season,omitempty"`
		SeasonName      string   `json:"vod_season_name,omitempty"`
		Episode         int      `json:"episode,omitempty"`
		EpisodeName     string   `json:"episode_name,omitempty"`
		EpisodeOverview string   `json:"episode_overview,omitempty"`
		StillURL        string   `json:"still_url,omitempty"`
		SourceID        string   `json:"source_id,omitempty"`
		SourceType      string   `json:"source_type,omitempty"`
		Tags            []string `json:"tags,omitempty"`
		Alternates      []vodAlt `json:"alternates,omitempty"`
	}

	fields := r.URL.Query().Get("fields")
	if fields == "slim" {
		var slim []slimItem
		genreSet := make(map[string]bool)
		decadeSet := make(map[string]bool)
		certSet := make(map[string]bool)
		tagSet := make(map[string]bool)

		tmdbCache := make(map[string][]byte)
		if s.deps.TMDBStore != nil {
			for _, st := range filtered {
				if st.TMDBID == "" {
					continue
				}
				tmdbID, err := strconv.Atoi(st.TMDBID)
				if err != nil || tmdbID <= 0 {
					continue
				}
				mt := "movie"
				if st.VODType == "series" || st.VODType == "episode" || st.Season > 0 {
					mt = "series"
				}
				key := mt + ":" + st.TMDBID
				if _, exists := tmdbCache[key]; !exists {
					blob, err := s.deps.TMDBStore.GetBlobTyped(mt, tmdbID)
					if err == nil && blob != nil {
						tmdbCache[key] = blob
					}
				}
			}
		}

		for _, st := range filtered {
			si := slimItem{
				ID:      st.ID,
				Name:    st.Name,
				TMDBID:  st.TMDBID,
				VODType: st.VODType,
				Group:   st.Group,
				Series:  st.SeriesName,
				Season:  st.Season,
				Episode: st.Episode,
				Year:    st.Year,
				Tags:    st.Tags,
			}

			if st.CollectionName != "" {
				si.CollectionName = st.CollectionName
			}

			if st.TMDBID != "" {
				mt := "movie"
				if st.VODType == "series" || st.VODType == "episode" || st.Season > 0 {
					mt = "series"
				}
				key := mt + ":" + st.TMDBID
				if blob, ok := tmdbCache[key]; ok {
					if mt == "movie" {
						var movie tmdbcache.Movie
						if json.Unmarshal(blob, &movie) == nil {
							si.Genres = movie.Genres
							si.Certification = movie.Certification
							if movie.CollectionName != "" && si.CollectionName == "" {
								si.CollectionName = movie.CollectionName
							}
						}
					} else {
						var series tmdbcache.Series
						if json.Unmarshal(blob, &series) == nil {
							si.Genres = series.Genres
						}
					}
				}
			}

			for _, g := range si.Genres {
				genreSet[g] = true
			}
			if si.Certification != "" {
				certSet[si.Certification] = true
			}
			for _, t := range si.Tags {
				tagSet[t] = true
			}
			if st.Year != "" && len(st.Year) == 4 {
				decadeSet[st.Year[:3]+"0s"] = true
			}

			slim = append(slim, si)
		}
		if slim == nil {
			slim = []slimItem{}
		}
		decadeList := make([]string, 0, len(decadeSet))
		for d := range decadeSet {
			decadeList = append(decadeList, d)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(decadeList)))
		genreList := make([]string, 0, len(genreSet))
		for g := range genreSet {
			genreList = append(genreList, g)
		}
		sort.Strings(genreList)
		certList := make([]string, 0, len(certSet))
		for c := range certSet {
			certList = append(certList, c)
		}
		sort.Strings(certList)
		tagList := make([]string, 0, len(tagSet))
		for t := range tagSet {
			tagList = append(tagList, t)
		}
		sort.Strings(tagList)

		httputil.RespondJSON(w, http.StatusOK, map[string]any{
			"items":          slim,
			"total":          len(slim),
			"decades":        decadeList,
			"genres":         genreList,
			"certifications": certList,
			"tags":           tagList,
		})
		return
	}

	tmdbSeriesCache := make(map[string]*tmdb.SeriesBlob)
	if s.deps.TMDBStore != nil {
		for _, st := range filtered {
			if st.TMDBID == "" || (st.VODType != "series" && st.Season == 0) {
				continue
			}
			tmdbID, err := strconv.Atoi(st.TMDBID)
			if err != nil || tmdbID <= 0 {
				continue
			}
			key := st.TMDBID
			if _, exists := tmdbSeriesCache[key]; exists {
				continue
			}
			blob, err := s.deps.TMDBStore.GetBlobTyped("series", tmdbID)
			if err != nil || blob == nil {
				continue
			}
			var sb tmdb.SeriesBlob
			if json.Unmarshal(blob, &sb) == nil {
				tmdbSeriesCache[key] = &sb
			}
		}
	}

	var items []vodItem
	for _, st := range filtered {
		item := vodItem{
			ID:          st.ID,
			Name:        st.Name,
			TMDBID:      st.TMDBID,
			VODType:     st.VODType,
			Group:       st.Group,
			Series:      st.SeriesName,
			Season:      st.Season,
			SeasonName:  st.SeasonName,
			Episode:     st.Episode,
			EpisodeName: st.EpisodeName,
			SourceID:    st.SourceID,
			SourceType:  st.SourceType,
			Year:        st.Year,
			Tags:        st.Tags,
		}

		if st.CollectionName != "" {
			item.CollectionName = st.CollectionName
		}
		if st.CollectionID != "" {
			cid, _ := strconv.Atoi(st.CollectionID)
			item.CollectionID = cid
		}

		if st.TMDBID != "" {
			tmdbID, err := strconv.Atoi(st.TMDBID)
			if err == nil && tmdbID > 0 {
				item.PosterURL = fmt.Sprintf("/api/tmdb/i/%d/poster.jpg", tmdbID)
			}
		}

		if sb, ok := tmdbSeriesCache[st.TMDBID]; ok && st.Season > 0 && st.Episode > 0 {
			for _, sn := range sb.Seasons {
				if sn.SeasonNumber != st.Season {
					continue
				}
				for _, ep := range sn.Episodes {
					if ep.EpisodeNumber != st.Episode {
						continue
					}
					if ep.Name != "" {
						item.EpisodeName = ep.Name
					}
					if ep.Overview != "" {
						item.EpisodeOverview = ep.Overview
					}
					if st.TMDBID != "" {
						item.StillURL = fmt.Sprintf("/api/tmdb/i/%s/s%de%d.jpg", st.TMDBID, st.Season, st.Episode)
					}
					break
				}
				break
			}
		}

		items = append(items, item)
	}

	scoreItem := func(item vodItem) int {
		score := 0
		if item.PosterURL != "" {
			score += 5
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

	var queue map[string]int
	if s.deps.TMDBStore != nil {
		metaCount, _ := s.deps.TMDBStore.QueueCount()
		imageCount, _ := s.deps.TMDBStore.ImageQueueCount()
		queue = map[string]int{"metadata": metaCount, "images": imageCount}
	}

	result := map[string]any{
		"items": items,
		"total": len(items),
		"queue": queue,
	}

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

func (s *Server) handleTMDBResync(w http.ResponseWriter, r *http.Request) {
	if s.deps.TMDBStore == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "TMDB store not configured")
		return
	}

	if err := s.deps.TMDBStore.ClearAllBlobs(); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to clear blobs")
		return
	}
	if err := s.deps.TMDBStore.ClearImageQueue(); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to clear image queue")
		return
	}

	streams, err := s.deps.StreamStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list streams")
		return
	}

	enqueued := 0
	seen := make(map[string]bool)
	for _, st := range streams {
		if st.TMDBID == "" {
			continue
		}
		tmdbID, err := strconv.Atoi(st.TMDBID)
		if err != nil || tmdbID <= 0 {
			continue
		}
		mt := "movie"
		if st.VODType == "series" || st.VODType == "episode" || st.Season > 0 {
			mt = "series"
		}
		key := fmt.Sprintf("%s:%d", mt, tmdbID)
		if seen[key] {
			continue
		}
		seen[key] = true
		s.deps.TMDBStore.EnqueueMetadata(tmdb.QueueEntry{
			TMDBID:    tmdbID,
			MediaType: mt,
			Status:    "resolving",
			CreatedAt: now().Unix(),
		})
		enqueued++
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]int{"enqueued": enqueued})
}

func (s *Server) handleTMDBRecent(w http.ResponseWriter, r *http.Request) {
	if s.deps.TMDBStore == nil {
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}

	entries, err := s.deps.TMDBStore.ListRecentBlobs(50)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list recent entries")
		return
	}
	if entries == nil {
		entries = []tmdb.RecentBlobEntry{}
	}
	httputil.RespondJSON(w, http.StatusOK, entries)
}

var now = time.Now

