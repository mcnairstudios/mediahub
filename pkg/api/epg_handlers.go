package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/epg"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/xmltv"
)

func (s *Server) handleCreateEPGSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string `json:"name"`
		URL             string `json:"url"`
		UseWireGuard    bool   `json:"use_wireguard"`
		RefreshInterval string `json:"refresh_interval"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Name == "" || req.URL == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name and url required")
		return
	}

	refreshInterval := req.RefreshInterval
	if refreshInterval == "" {
		refreshInterval = "daily"
	}

	src := &epg.Source{
		ID:              uuid.New().String(),
		Name:            req.Name,
		URL:             req.URL,
		IsEnabled:       true,
		UseWireGuard:    req.UseWireGuard,
		RefreshInterval: refreshInterval,
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
		Name            *string `json:"name"`
		URL             *string `json:"url"`
		IsEnabled       *bool   `json:"is_enabled"`
		UseWireGuard    *bool   `json:"use_wireguard"`
		RefreshInterval *string `json:"refresh_interval"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
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
	if req.RefreshInterval != nil {
		existing.RefreshInterval = *req.RefreshInterval
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
				SourceID:    src.ID,
				ChannelID:   p.ChannelID,
				Title:       p.Title,
				Subtitle:    p.Subtitle,
				Description: p.Description,
				StartTime:   p.Start,
				EndTime:     p.Stop,
				Categories:  p.Categories,
				Rating:      p.Rating,
				EpisodeNum:  p.EpisodeNum,
				SeriesID:    p.SeriesID,
				IsNew:       p.IsNew,
			})
		}

		batchSize := 5000
		for i := 0; i < len(programs); i += batchSize {
			end := i + batchSize
			if end > len(programs) {
				end = len(programs)
			}
			if err := s.deps.ProgramStore.BulkInsert(ctx, programs[i:end]); err != nil {
				log.Printf("epg refresh: BulkInsert failed for source %s batch %d-%d: %v", src.ID, i, end, err)
			}
		}
	}

	iconMap := make(map[string]string, len(channels))
	nameMap := make(map[string]string, len(channels))
	for _, ch := range channels {
		if ch.Icon != "" {
			iconMap[ch.ID] = ch.Icon
		}
		if ch.DisplayName != "" {
			nameMap[ch.ID] = ch.DisplayName
		}
	}
	if len(iconMap) > 0 || len(nameMap) > 0 {
		combined := map[string]map[string]string{"icons": iconMap, "names": nameMap}
		if data, err := json.Marshal(combined); err == nil {
			s.deps.SettingsStore.Set(ctx, "epg_channel_meta", string(data))
		}
	}

	s.assignEPGLogos(ctx, iconMap, nameMap)

	now := time.Now()
	src.LastRefreshed = &now
	src.ChannelCount = len(channels)
	src.ProgramCount = len(programmes)
	src.ETag = result.ETag
	src.LastError = ""
	s.deps.EPGSourceStore.Update(ctx, src)
}

func (s *Server) handleEPGGuide(w http.ResponseWriter, r *http.Request) {
	if s.deps.ProgramStore == nil {
		httputil.RespondJSON(w, http.StatusOK, map[string]any{
			"start":    time.Now().Truncate(30 * time.Minute),
			"stop":     time.Now().Truncate(30 * time.Minute).Add(6 * time.Hour),
			"programs": map[string]any{},
		})
		return
	}

	hours := 6
	if hs := r.URL.Query().Get("hours"); hs != "" {
		if parsed, err := strconv.Atoi(hs); err == nil && parsed > 0 && parsed <= 48 {
			hours = parsed
		}
	}

	var start time.Time
	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = parsed.Truncate(30 * time.Minute)
		}
	}
	if start.IsZero() {
		start = time.Now().Truncate(30 * time.Minute)
	}
	stop := start.Add(time.Duration(hours) * time.Hour)

	allPrograms, err := s.deps.ProgramStore.ListAll(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list guide programs")
		return
	}

	type guideProgram struct {
		ChannelID   string    `json:"channel_id"`
		Title       string    `json:"title"`
		Description string    `json:"description,omitempty"`
		Start       time.Time `json:"start"`
		Stop        time.Time `json:"stop"`
		Categories  []string  `json:"categories,omitempty"`
		SeriesID    string    `json:"series_id,omitempty"`
		EpisodeNum  string    `json:"episode_num,omitempty"`
	}

	grouped := make(map[string][]guideProgram)
	for _, p := range allPrograms {
		if p.StartTime.Before(stop) && p.EndTime.After(start) {
			grouped[p.ChannelID] = append(grouped[p.ChannelID], guideProgram{
				ChannelID:   p.ChannelID,
				Title:       p.Title,
				Description: p.Description,
				Start:       p.StartTime,
				Stop:        p.EndTime,
				Categories:  p.Categories,
				SeriesID:    p.SeriesID,
				EpisodeNum:  p.EpisodeNum,
			})
		}
	}

	result := map[string]any{
		"start":    start,
		"stop":     stop,
		"programs": grouped,
	}

	if metaStr, err := s.deps.SettingsStore.Get(r.Context(), "epg_channel_meta"); err == nil && metaStr != "" {
		var meta map[string]map[string]string
		if json.Unmarshal([]byte(metaStr), &meta) == nil {
			if icons, ok := meta["icons"]; ok {
				result["channel_icons"] = icons
			}
			if names, ok := meta["names"]; ok {
				result["channel_names"] = names
			}
		}
	}

	httputil.RespondJSON(w, http.StatusOK, result)
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
	channels = s.filterChannelsByUser(r, channels)

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
		NextTitle   string `json:"next_title,omitempty"`
		NextStart   string `json:"next_start,omitempty"`
		NextEnd     string `json:"next_end,omitempty"`
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
		entry := nowEntry{
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
		}
		nextProgs, err := s.deps.ProgramStore.Range(r.Context(), tvgID, prog.EndTime, prog.EndTime.Add(4*time.Hour))
		if err == nil && len(nextProgs) > 0 {
			entry.NextTitle = nextProgs[0].Title
			entry.NextStart = nextProgs[0].StartTime.Format(time.RFC3339)
			entry.NextEnd = nextProgs[0].EndTime.Format(time.RFC3339)
		}
		result = append(result, entry)
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

	if sourceID := r.URL.Query().Get("source_id"); sourceID != "" {
		programs, err := s.deps.ProgramStore.ListAll(r.Context())
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to list programs")
			return
		}
		if programs == nil {
			httputil.RespondJSON(w, http.StatusOK, []any{})
			return
		}
		now := time.Now()
		var current []epg.Program
		for _, p := range programs {
			if p.SourceID == sourceID && p.StartTime.Before(now) && p.EndTime.After(now) {
				current = append(current, p)
			}
		}
		if current == nil {
			current = []epg.Program{}
		}
		httputil.RespondJSON(w, http.StatusOK, current)
		return
	}

	channelID := r.URL.Query().Get("channel_id")
	if channelID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "channel_id or source_id required")
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
			if cs := cfg.Config["stream_count"]; cs != "" {
				count, _ = strconv.Atoi(cs)
			}
			if count == 0 && s.deps.StreamStore != nil {
				if n, cerr := s.deps.StreamStore.CountBySource(r.Context(), cfg.Type, cfg.ID); cerr == nil && n > 0 {
					count = n
				}
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
		channels = s.filterChannelsByUser(r, channels)
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
		var lastRefreshed *time.Time
		for _, src := range epgSources {
			totalPrograms += src.ProgramCount
			totalEPGChannels += src.ChannelCount
			if src.LastError != "" {
				epgErrors++
			}
			if src.LastRefreshed != nil {
				if lastRefreshed == nil || src.LastRefreshed.After(*lastRefreshed) {
					lastRefreshed = src.LastRefreshed
				}
			}
		}
		epgStats := map[string]any{
			"source_count":  len(epgSources),
			"channel_count": totalEPGChannels,
			"program_count": totalPrograms,
			"error_count":   epgErrors,
		}
		if lastRefreshed != nil {
			epgStats["last_refreshed"] = lastRefreshed.Format(time.RFC3339)
		}
		stats["epg"] = epgStats
	}

	recordings, err := s.deps.RecordingStore.List(r.Context(), "", true)
	if err == nil {
		active := 0
		pending := 0
		completed := 0
		scheduled := 0
		failed := 0
		cancelled := 0
		for _, rec := range recordings {
			switch rec.Status {
			case "recording":
				active++
			case "pending":
				pending++
			case "completed":
				completed++
			case "scheduled":
				scheduled++
			case "failed":
				failed++
			case "cancelled":
				cancelled++
			}
		}
		stats["recordings"] = map[string]any{
			"active":    active,
			"pending":   pending,
			"completed": completed,
			"scheduled": scheduled,
			"failed":    failed,
			"cancelled": cancelled,
			"total":     len(recordings),
		}
	}

	if s.deps.WGService != nil {
		plugin := s.deps.WGService.ActivePlugin()
		stats["wireguard"] = map[string]any{
			"connected": plugin != nil,
		}
	}

	uptime := time.Since(s.startedAt)
	stats["uptime_seconds"] = int(uptime.Seconds())
	stats["started_at"] = s.startedAt.Format(time.RFC3339)

	httputil.RespondJSON(w, http.StatusOK, stats)
}

func (s *Server) RefreshEPGSource(ctx context.Context, src *epg.Source) {
	s.refreshEPGSource(ctx, src)
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

func (s *Server) assignEPGLogos(ctx context.Context, iconMap, nameMap map[string]string) {
	if len(iconMap) == 0 || s.deps.ChannelStore == nil {
		return
	}

	channels, err := s.deps.ChannelStore.List(ctx)
	if err != nil {
		return
	}

	nameToIcon := make(map[string]string, len(nameMap))
	for epgID, name := range nameMap {
		if icon, ok := iconMap[epgID]; ok {
			nameToIcon[strings.ToLower(name)] = icon
		}
	}

	for i := range channels {
		ch := &channels[i]
		if ch.LogoURL != "" {
			continue
		}

		var logoURL string
		if ch.TvgID != "" {
			logoURL = iconMap[ch.TvgID]
		}
		if logoURL == "" {
			logoURL = nameToIcon[strings.ToLower(ch.Name)]
		}
		if logoURL == "" {
			continue
		}

		if s.deps.LogoCache != nil {
			logoURL = s.deps.LogoCache.Resolve(logoURL)
		}
		ch.LogoURL = logoURL
		s.deps.ChannelStore.Update(ctx, ch)
	}
}

func (s *Server) handleListLogos(w http.ResponseWriter, r *http.Request) {
	channels, err := s.deps.ChannelStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}

	var epgMeta map[string]map[string]string
	if metaStr, err := s.deps.SettingsStore.Get(r.Context(), "epg_channel_meta"); err == nil && metaStr != "" {
		if err := json.Unmarshal([]byte(metaStr), &epgMeta); err != nil {
			log.Printf("list logos: failed to unmarshal epg_channel_meta: %v", err)
		}
	}

	type logoEntry struct {
		ChannelID   string `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		Number      int    `json:"number"`
		LogoURL     string `json:"logo_url"`
		TvgID       string `json:"tvg_id,omitempty"`
		Source      string `json:"source"`
	}

	var entries []logoEntry
	for _, ch := range channels {
		source := "none"
		if ch.LogoURL != "" {
			source = "manual"
			if epgMeta != nil {
				if icons, ok := epgMeta["icons"]; ok {
					if ch.TvgID != "" {
						if epgIcon, ok := icons[ch.TvgID]; ok && strings.Contains(ch.LogoURL, "url=") {
							_ = epgIcon
							source = "epg"
						}
					}
				}
			}
		}
		entries = append(entries, logoEntry{
			ChannelID:   ch.ID,
			ChannelName: ch.Name,
			Number:      ch.Number,
			LogoURL:     ch.LogoURL,
			TvgID:       ch.TvgID,
			Source:       source,
		})
	}
	if entries == nil {
		entries = []logoEntry{}
	}

	httputil.RespondJSON(w, http.StatusOK, entries)
}

func (s *Server) handleRefreshLogosFromEPG(w http.ResponseWriter, r *http.Request) {
	var epgMeta map[string]map[string]string
	metaStr, err := s.deps.SettingsStore.Get(r.Context(), "epg_channel_meta")
	if err != nil || metaStr == "" {
		httputil.RespondJSON(w, http.StatusOK, map[string]int{"updated": 0})
		return
	}
	if json.Unmarshal([]byte(metaStr), &epgMeta) != nil {
		httputil.RespondJSON(w, http.StatusOK, map[string]int{"updated": 0})
		return
	}

	iconMap := epgMeta["icons"]
	nameMap := epgMeta["names"]
	if len(iconMap) == 0 {
		httputil.RespondJSON(w, http.StatusOK, map[string]int{"updated": 0})
		return
	}

	channels, err := s.deps.ChannelStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}

	nameToIcon := make(map[string]string, len(nameMap))
	for epgID, name := range nameMap {
		if icon, ok := iconMap[epgID]; ok {
			nameToIcon[strings.ToLower(name)] = icon
		}
	}

	updated := 0
	for i := range channels {
		ch := &channels[i]
		var logoURL string
		if ch.TvgID != "" {
			logoURL = iconMap[ch.TvgID]
		}
		if logoURL == "" {
			logoURL = nameToIcon[strings.ToLower(ch.Name)]
		}
		if logoURL == "" {
			continue
		}
		if s.deps.LogoCache != nil {
			logoURL = s.deps.LogoCache.Resolve(logoURL)
		}
		if ch.LogoURL == logoURL {
			continue
		}
		ch.LogoURL = logoURL
		if s.deps.ChannelStore.Update(r.Context(), ch) == nil {
			updated++
		}
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]int{"updated": updated})
}

func (s *Server) handleUpdateChannelLogo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "channel ID required")
		return
	}

	ch, err := s.deps.ChannelStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get channel")
		return
	}
	if ch == nil {
		httputil.RespondError(w, http.StatusNotFound, "channel not found")
		return
	}

	var req struct {
		LogoURL string `json:"logo_url"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if s.deps.LogoCache != nil && req.LogoURL != "" {
		req.LogoURL = s.deps.LogoCache.Resolve(req.LogoURL)
	}
	ch.LogoURL = req.LogoURL

	if err := s.deps.ChannelStore.Update(r.Context(), ch); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update channel")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, ch)
}

func (s *Server) handleListEPGChannelIDs(w http.ResponseWriter, r *http.Request) {
	if s.deps.ProgramStore == nil {
		httputil.RespondJSON(w, http.StatusOK, []string{})
		return
	}

	ids, err := s.deps.ProgramStore.ListChannelIDs(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list EPG channel IDs")
		return
	}
	if ids == nil {
		ids = []string{}
	}

	var epgMeta map[string]map[string]string
	if metaStr, _ := s.deps.SettingsStore.Get(r.Context(), "epg_channel_meta"); metaStr != "" {
		json.Unmarshal([]byte(metaStr), &epgMeta)
	}

	type epgChannelInfo struct {
		ID   string `json:"id"`
		Name string `json:"name,omitempty"`
		Icon string `json:"icon,omitempty"`
	}

	result := make([]epgChannelInfo, 0, len(ids))
	for _, id := range ids {
		info := epgChannelInfo{ID: id}
		if epgMeta != nil {
			if names, ok := epgMeta["names"]; ok {
				info.Name = names[id]
			}
			if icons, ok := epgMeta["icons"]; ok {
				info.Icon = icons[id]
			}
		}
		result = append(result, info)
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}

func (s *Server) handleAutoMatchEPG(w http.ResponseWriter, r *http.Request) {
	channels, err := s.deps.ChannelStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}

	if s.deps.ProgramStore == nil {
		httputil.RespondJSON(w, http.StatusOK, map[string]int{"matched": 0})
		return
	}

	epgIDs, err := s.deps.ProgramStore.ListChannelIDs(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list EPG channels")
		return
	}

	var epgMeta map[string]map[string]string
	if metaStr, _ := s.deps.SettingsStore.Get(r.Context(), "epg_channel_meta"); metaStr != "" {
		json.Unmarshal([]byte(metaStr), &epgMeta)
	}

	epgNames := make(map[string]string, len(epgIDs))
	if epgMeta != nil {
		if names, ok := epgMeta["names"]; ok {
			for id, name := range names {
				epgNames[id] = name
			}
		}
	}

	normalizedEPG := make(map[string]string, len(epgIDs))
	for _, id := range epgIDs {
		name := epgNames[id]
		if name != "" {
			normalizedEPG[normalizeChannelName(name)] = id
		}
		normalizedEPG[normalizeChannelName(id)] = id
	}

	matched := 0
	for i := range channels {
		ch := &channels[i]
		if ch.TvgID != "" {
			continue
		}

		for _, epgID := range epgIDs {
			if strings.EqualFold(ch.Name, epgID) {
				ch.TvgID = epgID
				s.deps.ChannelStore.Update(r.Context(), ch)
				matched++
				break
			}
		}
		if ch.TvgID != "" {
			continue
		}

		normalized := normalizeChannelName(ch.Name)
		if epgID, ok := normalizedEPG[normalized]; ok {
			ch.TvgID = epgID
			s.deps.ChannelStore.Update(r.Context(), ch)
			matched++
		}
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]int{"matched": matched})
}

func normalizeChannelName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	for _, suffix := range []string{" hd", " sd", " fhd", " uhd", " 4k", " +1", " (hd)", " (sd)"} {
		s = strings.TrimSuffix(s, suffix)
	}
	return strings.TrimSpace(s)
}

func (s *Server) AutoMatchStreamsToEPG(ctx context.Context, sourceType, sourceID, epgSourceID string) int {
	if epgSourceID == "" || s.deps.StreamStore == nil || s.deps.ProgramStore == nil {
		return 0
	}

	streams, err := s.deps.StreamStore.ListBySource(ctx, sourceType, sourceID)
	if err != nil || len(streams) == 0 {
		return 0
	}

	epgIDs, err := s.deps.ProgramStore.ListChannelIDs(ctx)
	if err != nil || len(epgIDs) == 0 {
		return 0
	}

	var epgMeta map[string]map[string]string
	if metaStr, _ := s.deps.SettingsStore.Get(ctx, "epg_channel_meta"); metaStr != "" {
		json.Unmarshal([]byte(metaStr), &epgMeta)
	}

	epgNames := make(map[string]string, len(epgIDs))
	if epgMeta != nil {
		if names, ok := epgMeta["names"]; ok {
			for id, name := range names {
				epgNames[id] = name
			}
		}
	}

	normalizedEPG := make(map[string]string, len(epgIDs))
	for _, id := range epgIDs {
		name := epgNames[id]
		if name != "" {
			normalizedEPG[normalizeChannelName(name)] = id
		}
		normalizedEPG[normalizeChannelName(id)] = id
	}

	matched := 0
	var updated []media.Stream
	for _, st := range streams {
		if st.TvgID != "" {
			continue
		}

		var matchedID string
		for _, epgID := range epgIDs {
			if strings.EqualFold(st.Name, epgID) {
				matchedID = epgID
				break
			}
		}
		if matchedID == "" {
			for _, epgID := range epgIDs {
				name := epgNames[epgID]
				if name != "" && strings.EqualFold(st.Name, name) {
					matchedID = epgID
					break
				}
			}
		}
		if matchedID == "" {
			normalized := normalizeChannelName(st.Name)
			if epgID, ok := normalizedEPG[normalized]; ok {
				matchedID = epgID
			}
		}

		if matchedID != "" {
			st.TvgID = matchedID
			updated = append(updated, st)
			matched++
		}
	}

	if len(updated) > 0 {
		s.deps.StreamStore.BulkUpsert(ctx, updated)
	}

	log.Printf("epg auto-match: matched %d/%d streams from source %s to EPG", matched, len(streams), sourceID)
	return matched
}
