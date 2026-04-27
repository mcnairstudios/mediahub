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

	result, err := httputil.FetchConditional(ctx, client, src.URL, src.ETag, "MediaHub/1.0", extraHeaders)
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
