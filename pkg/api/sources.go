package api

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
)

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	configs, err := s.deps.SourceConfigStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list sources")
		return
	}

	type sourceResponse struct {
		sourceconfig.SourceConfig
		StreamCount   int    `json:"stream_count"`
		LastRefreshed string `json:"last_refreshed,omitempty"`
		LastError     string `json:"last_error,omitempty"`
		RefreshState  string `json:"refresh_state,omitempty"`
	}

	result := make([]sourceResponse, 0, len(configs))
	for _, cfg := range configs {
		resp := sourceResponse{SourceConfig: cfg}

		streams, err := s.deps.StreamStore.ListBySource(r.Context(), cfg.Type, cfg.ID)
		if err == nil {
			resp.StreamCount = len(streams)
		}

		src, err := s.deps.SourceReg.Create(r.Context(), source.SourceType(cfg.Type), cfg.ID)
		if err == nil {
			info := src.Info(r.Context())
			if info.LastRefreshed != nil {
				resp.LastRefreshed = info.LastRefreshed.Format("2006-01-02T15:04:05Z")
			}
			resp.LastError = info.LastError
		}

		result = append(result, resp)
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}

func (s *Server) handleCreateM3USource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		URL          string `json:"url"`
		Username     string `json:"username"`
		Password     string `json:"password"`
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

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      "m3u",
		Name:      req.Name,
		IsEnabled: true,
		Config: map[string]string{
			"url":           req.URL,
			"username":      req.Username,
			"password":      req.Password,
			"use_wireguard": boolStr(req.UseWireGuard),
		},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	go func() {
		ctx := r.Context()
		src, err := s.deps.SourceReg.Create(ctx, "m3u", sc.ID)
		if err != nil {
			return
		}
		src.Refresh(ctx)
	}()

	httputil.RespondJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateM3USource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	existing, err := s.deps.SourceConfigStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get source")
		return
	}
	if existing == nil {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	var req struct {
		Name         *string `json:"name"`
		URL          *string `json:"url"`
		Username     *string `json:"username"`
		Password     *string `json:"password"`
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
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}
	if req.URL != nil {
		existing.Config["url"] = *req.URL
	}
	if req.Username != nil {
		existing.Config["username"] = *req.Username
	}
	if req.Password != nil {
		existing.Config["password"] = *req.Password
	}
	if req.UseWireGuard != nil {
		existing.Config["use_wireguard"] = boolStr(*req.UseWireGuard)
	}

	if err := s.deps.SourceConfigStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteM3USource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), "m3u", id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source streams")
		return
	}

	if err := s.deps.SourceConfigStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSourceStatus(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("sourceID")
	if sourceID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	cfg, err := s.deps.SourceConfigStore.Get(r.Context(), sourceID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get source")
		return
	}
	if cfg == nil {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	src, err := s.deps.SourceReg.Create(r.Context(), source.SourceType(cfg.Type), cfg.ID)
	if err != nil {
		httputil.RespondJSON(w, http.StatusOK, source.RefreshStatus{State: "idle"})
		return
	}

	info := src.Info(r.Context())
	status := source.RefreshStatus{State: "idle"}
	if info.LastError != "" {
		status.State = "error"
		status.Message = info.LastError
	}
	httputil.RespondJSON(w, http.StatusOK, status)
}

func (s *Server) handleCreateTVPStreamsSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string `json:"name"`
		URL             string `json:"url"`
		EnrollmentToken string `json:"enrollment_token"`
		UseWireGuard    bool   `json:"use_wireguard"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.URL == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name and url required")
		return
	}

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      "tvpstreams",
		Name:      req.Name,
		IsEnabled: true,
		Config: map[string]string{
			"url":              req.URL,
			"enrollment_token": req.EnrollmentToken,
			"use_wireguard":    boolStr(req.UseWireGuard),
		},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	go func() {
		ctx := r.Context()
		src, err := s.deps.SourceReg.Create(ctx, "tvpstreams", sc.ID)
		if err != nil {
			return
		}
		src.Refresh(ctx)
	}()

	httputil.RespondJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateTVPStreamsSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	existing, err := s.deps.SourceConfigStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get source")
		return
	}
	if existing == nil {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	var req struct {
		Name            *string `json:"name"`
		URL             *string `json:"url"`
		EnrollmentToken *string `json:"enrollment_token"`
		IsEnabled       *bool   `json:"is_enabled"`
		UseWireGuard    *bool   `json:"use_wireguard"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}
	if req.URL != nil {
		existing.Config["url"] = *req.URL
	}
	if req.EnrollmentToken != nil {
		existing.Config["enrollment_token"] = *req.EnrollmentToken
	}
	if req.UseWireGuard != nil {
		existing.Config["use_wireguard"] = boolStr(*req.UseWireGuard)
	}

	if err := s.deps.SourceConfigStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteTVPStreamsSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), "tvpstreams", id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source streams")
		return
	}

	if err := s.deps.SourceConfigStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTVPStreamsTLSStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	src, err := s.deps.SourceReg.Create(r.Context(), "tvpstreams", id)
	if err != nil {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	if tp, ok := src.(source.TLSProvider); ok {
		httputil.RespondJSON(w, http.StatusOK, tp.TLSInfo())
		return
	}

	httputil.RespondJSON(w, http.StatusOK, source.TLSStatus{})
}

func (s *Server) handleCreateXtreamSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		Server       string `json:"server"`
		Username     string `json:"username"`
		Password     string `json:"password"`
		UseWireGuard bool   `json:"use_wireguard"`
		MaxStreams   int    `json:"max_streams"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Server == "" || req.Username == "" || req.Password == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name, server, username, and password required")
		return
	}

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      "xtream",
		Name:      req.Name,
		IsEnabled: true,
		Config: map[string]string{
			"server":        req.Server,
			"username":      req.Username,
			"password":      req.Password,
			"use_wireguard": boolStr(req.UseWireGuard),
			"max_streams":   fmt.Sprintf("%d", req.MaxStreams),
		},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	go func() {
		ctx := r.Context()
		src, err := s.deps.SourceReg.Create(ctx, "xtream", sc.ID)
		if err != nil {
			return
		}
		src.Refresh(ctx)
	}()

	httputil.RespondJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateXtreamSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	existing, err := s.deps.SourceConfigStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get source")
		return
	}
	if existing == nil {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	var req struct {
		Name         *string `json:"name"`
		Server       *string `json:"server"`
		Username     *string `json:"username"`
		Password     *string `json:"password"`
		IsEnabled    *bool   `json:"is_enabled"`
		UseWireGuard *bool   `json:"use_wireguard"`
		MaxStreams   *int    `json:"max_streams"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}
	if req.Server != nil {
		existing.Config["server"] = *req.Server
	}
	if req.Username != nil {
		existing.Config["username"] = *req.Username
	}
	if req.Password != nil {
		existing.Config["password"] = *req.Password
	}
	if req.UseWireGuard != nil {
		existing.Config["use_wireguard"] = boolStr(*req.UseWireGuard)
	}
	if req.MaxStreams != nil {
		existing.Config["max_streams"] = fmt.Sprintf("%d", *req.MaxStreams)
	}

	if err := s.deps.SourceConfigStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteXtreamSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), "xtream", id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source streams")
		return
	}

	if err := s.deps.SourceConfigStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleXtreamAccountInfo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	sc, err := s.deps.SourceConfigStore.Get(r.Context(), id)
	if err != nil || sc == nil || sc.Type != "xtream" {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	src, err := s.deps.SourceReg.Create(r.Context(), "xtream", id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if provider, ok := src.(source.AccountInfoProvider); ok {
		info, err := provider.GetAccountInfo(r.Context())
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httputil.RespondJSON(w, http.StatusOK, info)
		return
	}

	httputil.RespondError(w, http.StatusNotImplemented, "source does not support account info")
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
