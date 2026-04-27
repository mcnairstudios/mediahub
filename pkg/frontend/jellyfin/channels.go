package jellyfin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) liveTvInfo(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{
		"Services": []any{}, "IsEnabled": true, "EnabledUsers": []string{},
	})
}

func (s *Server) liveTvChannels(w http.ResponseWriter, r *http.Request) {
	if s.channels == nil {
		s.respondJSON(w, http.StatusOK, emptyResult())
		return
	}

	channels, err := s.channels.List(r.Context())
	if err != nil {
		s.respondJSON(w, http.StatusOK, emptyResult())
		return
	}

	var items []BaseItemDto
	for i, ch := range channels {
		item := s.newLiveTvChannelItem(&ch, i)
		if s.programs != nil {
			if p, err := s.programs.NowPlaying(r.Context(), ch.TvgID); err == nil && p != nil {
				item.CurrentProgram = &BaseItemDto{
					Name:     p.Title,
					Overview: p.Description,
					ID:       fmt.Sprintf("prog_%s_%d", stripDashes(ch.ID), p.StartTime.Unix()),
					Type:     "LiveTvProgram",
				}
				if !p.StartTime.IsZero() {
					item.CurrentProgram.PremiereDate = p.StartTime.Format(time.RFC3339)
				}
				if !p.StartTime.IsZero() && !p.EndTime.IsZero() {
					item.CurrentProgram.RunTimeTicks = durationToTicks(p.EndTime.Sub(p.StartTime))
				}
			}
		}
		items = append(items, item)
	}
	if items == nil {
		items = []BaseItemDto{}
	}

	s.respondJSON(w, http.StatusOK, BaseItemDtoQueryResult{Items: items, TotalRecordCount: len(items)})
}

func (s *Server) liveTvPrograms(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	isAiring := q.Get("isAiring") == "true"
	hasAired := q.Get("hasAired")
	isMovie := q.Get("isMovie")
	isSeries := q.Get("isSeries")
	isNews := q.Get("isNews")
	isKids := q.Get("isKids")
	isSports := q.Get("isSports")
	limit := 50
	if l := q.Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	if s.channels == nil || s.programs == nil {
		s.respondJSON(w, http.StatusOK, emptyResult())
		return
	}

	channels, _ := s.channels.List(r.Context())
	now := time.Now().UTC()

	var items []BaseItemDto
	for _, ch := range channels {
		programs, err := s.programs.Range(r.Context(), ch.TvgID, now.Add(-1*time.Hour), now.Add(24*time.Hour))
		if err != nil {
			continue
		}
		for _, p := range programs {
			if isAiring && !(now.After(p.StartTime) && now.Before(p.EndTime)) {
				continue
			}
			if hasAired == "false" && p.EndTime.Before(now) {
				continue
			}

			cat := strings.ToLower(strings.Join(p.Categories, " "))
			if isMovie == "true" && !strings.Contains(cat, "movie") && !strings.Contains(cat, "film") {
				continue
			}
			if isSeries == "true" && !strings.Contains(cat, "series") && !strings.Contains(cat, "drama") && !strings.Contains(cat, "soap") {
				continue
			}
			if isNews == "true" && !strings.Contains(cat, "news") {
				continue
			}
			if isKids == "true" && !strings.Contains(cat, "kid") && !strings.Contains(cat, "child") && !strings.Contains(cat, "cartoon") && !strings.Contains(cat, "animation") {
				continue
			}
			if isSports == "true" && !strings.Contains(cat, "sport") {
				continue
			}

			chID := stripDashes(ch.ID)
			item := BaseItemDto{
				Name:          p.Title,
				ServerID:      s.serverID,
				ID:            fmt.Sprintf("prog_%s_%d", chID, p.StartTime.Unix()),
				Type:          "LiveTvProgram",
				Overview:      p.Description,
				ParentID:      chID,
				ChannelNumber: ch.Name,
				ImageTags:     map[string]string{},
			}
			if ch.LogoURL != "" {
				item.ChannelPrimaryImageTag = "logo"
			}
			if !p.StartTime.IsZero() {
				item.PremiereDate = p.StartTime.Format(time.RFC3339)
			}
			if !p.StartTime.IsZero() && !p.EndTime.IsZero() {
				item.RunTimeTicks = durationToTicks(p.EndTime.Sub(p.StartTime))
			}

			items = append(items, item)
			if len(items) >= limit {
				break
			}
		}
		if len(items) >= limit {
			break
		}
	}

	if items == nil {
		items = []BaseItemDto{}
	}
	s.respondJSON(w, http.StatusOK, BaseItemDtoQueryResult{Items: items, TotalRecordCount: len(items)})
}

func (s *Server) liveTvGuideInfo(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	s.respondJSON(w, http.StatusOK, map[string]any{
		"StartDate": now.Format(time.RFC3339),
		"EndDate":   now.Add(7 * 24 * time.Hour).Format(time.RFC3339),
	})
}

func (s *Server) groupChannels(w http.ResponseWriter, r *http.Request, groupID string) {
	if s.channels == nil {
		s.respondJSON(w, http.StatusOK, emptyResult())
		return
	}

	channels, err := s.channels.List(r.Context())
	if err != nil {
		s.respondJSON(w, http.StatusOK, emptyResult())
		return
	}

	var items []BaseItemDto
	for _, ch := range channels {
		if ch.GroupID != groupID {
			continue
		}
		item := s.newChannelItem(&ch)
		if s.programs != nil {
			if p, err := s.programs.NowPlaying(r.Context(), ch.TvgID); err == nil && p != nil {
				item.Overview = p.Title + " -- " + p.Description
			}
		}
		items = append(items, item)
	}
	if items == nil {
		items = []BaseItemDto{}
	}
	s.paginateAndRespond(w, r, items)
}
