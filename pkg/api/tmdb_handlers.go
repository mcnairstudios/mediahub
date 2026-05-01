package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
			blob, err := s.deps.TMDBStore.GetBlob(tmdbID)
			if err == nil && blob != nil {
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

	blob, err := s.deps.TMDBStore.GetBlob(tmdbID)
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
			streams, err = s.deps.StreamStore.ListBySource(r.Context(), sc.Type, sourceID)
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
			streams, err = s.deps.StreamStore.ListBySource(r.Context(), sc.Type, sourceID)
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
		ID      string `json:"id"`
		Name    string `json:"name"`
		TMDBID  string `json:"tmdb_id,omitempty"`
		VODType string `json:"vod_type"`
		Group   string `json:"group,omitempty"`
		Series  string `json:"series,omitempty"`
		Season  int    `json:"season,omitempty"`
		Episode int    `json:"episode,omitempty"`
		Year    string `json:"year,omitempty"`
	}

	type vodAlt struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Group    string `json:"group,omitempty"`
		SourceID string `json:"source_id,omitempty"`
	}

	type vodItem struct {
		ID             string   `json:"id"`
		Name           string   `json:"name"`
		TMDBID         string   `json:"tmdb_id,omitempty"`
		PosterURL      string   `json:"poster_url,omitempty"`
		Rating         float64  `json:"rating,omitempty"`
		Year           string   `json:"year,omitempty"`
		Genres         []string `json:"genres,omitempty"`
		Certification  string   `json:"certification,omitempty"`
		VODType        string   `json:"vod_type"`
		Group          string   `json:"group,omitempty"`
		Series         string   `json:"series,omitempty"`
		CollectionName string   `json:"collection_name,omitempty"`
		CollectionID   int      `json:"collection_id,omitempty"`
		Season         int      `json:"season,omitempty"`
		SeasonName     string   `json:"vod_season_name,omitempty"`
		Episode        int      `json:"episode,omitempty"`
		EpisodeName    string   `json:"episode_name,omitempty"`
		SourceID       string   `json:"source_id,omitempty"`
		SourceType     string   `json:"source_type,omitempty"`
		Tags           []string `json:"tags,omitempty"`
		Alternates     []vodAlt `json:"alternates,omitempty"`
	}

	fields := r.URL.Query().Get("fields")
	if fields == "slim" {
		var slim []slimItem
		genres := make(map[string]bool)
		decades := make(map[string]bool)
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
			}
			slim = append(slim, si)
			if st.Year != "" && len(st.Year) == 4 {
				decades[st.Year[:3]+"0s"] = true
			}
		}
		if slim == nil {
			slim = []slimItem{}
		}
		decadeList := make([]string, 0, len(decades))
		for d := range decades {
			decadeList = append(decadeList, d)
		}
		genreList := make([]string, 0, len(genres))
		for g := range genres {
			genreList = append(genreList, g)
		}
		httputil.RespondJSON(w, http.StatusOK, map[string]any{
			"items":   slim,
			"total":   len(slim),
			"decades": decadeList,
			"genres":  genreList,
		})
		return
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

