package api

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/epg"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/xmltv"
)

func (s *Server) handleCreateEPGSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		URL          string `json:"url"`
		UseWireGuard bool   `json:"use_wireguard"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.URL == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name and url required")
		return
	}

	src := &epg.Source{
		ID:           uuid.New().String(),
		Name:         req.Name,
		URL:          req.URL,
		IsEnabled:    true,
		UseWireGuard: req.UseWireGuard,
	}

	if err := s.deps.EPGSourceStore.Create(r.Context(), src); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create EPG source")
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, src)
}

func (s *Server) handleUpdateEPGSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "EPG source ID required")
		return
	}

	existing, err := s.deps.EPGSourceStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get EPG source")
		return
	}
	if existing == nil {
		httputil.RespondError(w, http.StatusNotFound, "EPG source not found")
		return
	}

	var req struct {
		Name         *string `json:"name"`
		URL          *string `json:"url"`
		IsEnabled    *bool   `json:"is_enabled"`
		UseWireGuard *bool   `json:"use_wireguard"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.URL != nil {
		existing.URL = *req.URL
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}
	if req.UseWireGuard != nil {
		existing.UseWireGuard = *req.UseWireGuard
	}

	if err := s.deps.EPGSourceStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update EPG source")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteEPGSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "EPG source ID required")
		return
	}

	if s.deps.ProgramStore != nil {
		if err := s.deps.ProgramStore.DeleteBySource(r.Context(), id); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to delete EPG programs")
			return
		}
	}

	if err := s.deps.EPGSourceStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete EPG source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRefreshEPGSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "EPG source ID required")
		return
	}

	src, err := s.deps.EPGSourceStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get EPG source")
		return
	}
	if src == nil {
		httputil.RespondError(w, http.StatusNotFound, "EPG source not found")
		return
	}

	go s.refreshEPGSource(context.Background(), src)

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) refreshEPGSource(ctx context.Context, src *epg.Source) {
	client := http.DefaultClient
	if src.UseWireGuard && s.deps.WGService != nil {
		if p := s.deps.WGService.ActivePlugin(); p != nil {
			client = p.HTTPClient()
		}
	}

	var extraHeaders map[string]string
	if s.deps.BypassHeader != "" && s.deps.BypassSecret != "" {
		extraHeaders = map[string]string{s.deps.BypassHeader: s.deps.BypassSecret}
	}

	userAgent := s.deps.UserAgent
	if userAgent == "" {
		userAgent = "MediaHub/1.0"
	}
	result, err := httputil.FetchConditional(ctx, client, src.URL, src.ETag, userAgent, extraHeaders)
	if err != nil {
		now := time.Now()
		src.LastRefreshed = &now
		src.LastError = err.Error()
		s.deps.EPGSourceStore.Update(ctx, src)
		return
	}

	if !result.Changed {
		now := time.Now()
		src.LastRefreshed = &now
		src.LastError = ""
		s.deps.EPGSourceStore.Update(ctx, src)
		return
	}
	defer result.Body.Close()

	channels, programmes, err := xmltv.Parse(result.Body)
	if err != nil {
		now := time.Now()
		src.LastRefreshed = &now
		src.LastError = err.Error()
		s.deps.EPGSourceStore.Update(ctx, src)
		return
	}

	if s.deps.ProgramStore != nil {
		s.deps.ProgramStore.DeleteBySource(ctx, src.ID)

		programs := make([]epg.Program, 0, len(programmes))
		for _, p := range programmes {
			programs = append(programs, epg.Program{
				ChannelID:   p.ChannelID,
				Title:       p.Title,
				Subtitle:    p.Subtitle,
				Description: p.Description,
				StartTime:   p.Start,
				EndTime:     p.Stop,
				Categories:  p.Categories,
				Rating:      p.Rating,
				EpisodeNum:  p.EpisodeNum,
				IsNew:       p.IsNew,
			})
		}

		batchSize := 5000
		for i := 0; i < len(programs); i += batchSize {
			end := i + batchSize
			if end > len(programs) {
				end = len(programs)
			}
			s.deps.ProgramStore.BulkInsert(ctx, programs[i:end])
		}
	}

	now := time.Now()
	src.LastRefreshed = &now
	src.ChannelCount = len(channels)
	src.ProgramCount = len(programmes)
	src.ETag = result.ETag
	src.LastError = ""
	s.deps.EPGSourceStore.Update(ctx, src)
}

func (s *Server) handleEPGNow(w http.ResponseWriter, r *http.Request) {
	if s.deps.ProgramStore == nil {
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}

	channels, err := s.deps.ChannelStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}

	type nowEntry struct {
		ChannelID   string `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		Title       string `json:"title"`
		Subtitle    string `json:"subtitle,omitempty"`
		Description string `json:"description,omitempty"`
		StartTime   string `json:"start_time"`
		EndTime     string `json:"end_time"`
		Categories  []string `json:"categories,omitempty"`
		Rating      string `json:"rating,omitempty"`
		Progress    float64 `json:"progress"`
	}

	now := time.Now()
	var result []nowEntry
	for _, ch := range channels {
		if !ch.IsEnabled {
			continue
		}
		tvgID := ch.TvgID
		if tvgID == "" {
			tvgID = ch.ID
		}
		prog, err := s.deps.ProgramStore.NowPlaying(r.Context(), tvgID)
		if err != nil || prog == nil {
			continue
		}
		dur := prog.EndTime.Sub(prog.StartTime).Seconds()
		elapsed := now.Sub(prog.StartTime).Seconds()
		progress := 0.0
		if dur > 0 {
			progress = elapsed / dur
			if progress > 1 {
				progress = 1
			}
		}
		result = append(result, nowEntry{
			ChannelID:   ch.ID,
			ChannelName: ch.Name,
			Title:       prog.Title,
			Subtitle:    prog.Subtitle,
			Description: prog.Description,
			StartTime:   prog.StartTime.Format(time.RFC3339),
			EndTime:     prog.EndTime.Format(time.RFC3339),
			Categories:  prog.Categories,
			Rating:      prog.Rating,
			Progress:    progress,
		})
	}

	if result == nil {
		result = []nowEntry{}
	}
	httputil.RespondJSON(w, http.StatusOK, result)
}

func (s *Server) handleEPGPrograms(w http.ResponseWriter, r *http.Request) {
	if s.deps.ProgramStore == nil {
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}

	channelID := r.URL.Query().Get("channel_id")
	if channelID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "channel_id required")
		return
	}

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "invalid start time (use RFC3339)")
			return
		}
	} else {
		start = time.Now().Add(-1 * time.Hour)
	}

	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "invalid end time (use RFC3339)")
			return
		}
	} else {
		end = time.Now().Add(6 * time.Hour)
	}

	ch, err := s.deps.ChannelStore.Get(r.Context(), channelID)
	if err != nil || ch == nil {
		httputil.RespondError(w, http.StatusNotFound, "channel not found")
		return
	}

	tvgID := ch.TvgID
	if tvgID == "" {
		tvgID = ch.ID
	}

	programs, err := s.deps.ProgramStore.Range(r.Context(), tvgID, start, end)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to query programs")
		return
	}
	if programs == nil {
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}
	httputil.RespondJSON(w, http.StatusOK, programs)
}

func (s *Server) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]any{}

	configs, err := s.deps.SourceConfigStore.List(r.Context())
	if err == nil {
		totalStreams := 0
		sourceBreakdown := make([]map[string]any, 0, len(configs))
		for _, cfg := range configs {
			count := 0
			streams, sErr := s.deps.StreamStore.ListBySource(r.Context(), cfg.Type, cfg.ID)
			if sErr == nil {
				count = len(streams)
			}
			totalStreams += count
			sourceBreakdown = append(sourceBreakdown, map[string]any{
				"id":           cfg.ID,
				"name":         cfg.Name,
				"type":         cfg.Type,
				"is_enabled":   cfg.IsEnabled,
				"stream_count": count,
			})
		}
		stats["total_streams"] = totalStreams
		stats["sources"] = sourceBreakdown
	}

	channels, err := s.deps.ChannelStore.List(r.Context())
	if err == nil {
		stats["total_channels"] = len(channels)
	}

	if s.deps.Activity != nil {
		stats["active_sessions"] = len(s.deps.Activity.List())
	}

	epgSources, err := s.deps.EPGSourceStore.List(r.Context())
	if err == nil {
		totalPrograms := 0
		totalEPGChannels := 0
		epgErrors := 0
		for _, src := range epgSources {
			totalPrograms += src.ProgramCount
			totalEPGChannels += src.ChannelCount
			if src.LastError != "" {
				epgErrors++
			}
		}
		stats["epg"] = map[string]any{
			"source_count":  len(epgSources),
			"channel_count": totalEPGChannels,
			"program_count": totalPrograms,
			"error_count":   epgErrors,
		}
	}

	recordings, err := s.deps.RecordingStore.List(r.Context(), "", true)
	if err == nil {
		active := 0
		completed := 0
		scheduled := 0
		for _, rec := range recordings {
			switch rec.Status {
			case "recording":
				active++
			case "completed":
				completed++
			case "scheduled":
				scheduled++
			}
		}
		stats["recordings"] = map[string]any{
			"active":    active,
			"completed": completed,
			"scheduled": scheduled,
			"total":     len(recordings),
		}
	}

	if s.deps.WGService != nil {
		plugin := s.deps.WGService.ActivePlugin()
		stats["wireguard"] = map[string]any{
			"connected": plugin != nil,
		}
	}

	httputil.RespondJSON(w, http.StatusOK, stats)
}

func (s *Server) RefreshAllEPGSources(ctx context.Context) error {
	sources, err := s.deps.EPGSourceStore.List(ctx)
	if err != nil {
		return err
	}
	for i := range sources {
		if sources[i].IsEnabled {
			s.refreshEPGSource(ctx, &sources[i])
		}
	}
	return nil
}
