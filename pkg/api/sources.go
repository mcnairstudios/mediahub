package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/m3u"
	"github.com/mcnairstudios/mediahub/pkg/media"
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

		if countStr := cfg.Config["stream_count"]; countStr != "" {
			if n, err := strconv.Atoi(countStr); err == nil {
				resp.StreamCount = n
			}
		}
		if resp.StreamCount == 0 && s.deps.StreamStore != nil {
			if n, err := s.deps.StreamStore.CountBySource(r.Context(), cfg.Type, cfg.ID); err == nil && n > 0 {
				resp.StreamCount = n
			}
		}

		src, err := s.deps.SourceReg.Create(r.Context(), source.SourceType(cfg.Type), cfg.ID)
		if err == nil {
			info := src.Info(r.Context())
			if info.LastRefreshed != nil {
				resp.LastRefreshed = info.LastRefreshed.Format("2006-01-02T15:04:05Z")
			}
			resp.LastError = info.LastError
		}
		if resp.LastRefreshed == "" {
			resp.LastRefreshed = cfg.Config["last_refreshed"]
		}

		result = append(result, resp)
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}

func (s *Server) handleCreateM3USource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string `json:"name"`
		URL             string `json:"url"`
		Username        string `json:"username"`
		Password        string `json:"password"`
		UseWireGuard    bool   `json:"use_wireguard"`
		WGProfileID     string `json:"wg_profile_id"`
		RefreshInterval string `json:"refresh_interval"`
		SourceProfileID string `json:"source_profile_id"`
		EPGSourceID     string `json:"epg_source_id"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Name == "" || req.URL == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name and url required")
		return
	}

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      string(source.TypeM3U),
		Name:      req.Name,
		IsEnabled: true,
		Config: map[string]string{
			"url":               req.URL,
			"username":          req.Username,
			"password":          req.Password,
			"use_wireguard":     boolStr(req.UseWireGuard),
			"wg_profile_id":     req.WGProfileID,
			"refresh_interval":  req.RefreshInterval,
			"source_profile_id": req.SourceProfileID,
			"epg_source_id":     req.EPGSourceID,
		},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	go func() {
		ctx := r.Context()
		src, err := s.deps.SourceReg.Create(ctx, source.TypeM3U, sc.ID)
		if err != nil {
			return
		}
		src.Refresh(ctx)
		s.AutoMatchStreamsToEPG(ctx, string(source.TypeM3U), sc.ID, sc.Config["epg_source_id"])
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
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	var req struct {
		Name            *string `json:"name"`
		URL             *string `json:"url"`
		Username        *string `json:"username"`
		Password        *string `json:"password"`
		IsEnabled       *bool   `json:"is_enabled"`
		UseWireGuard    *bool   `json:"use_wireguard"`
		WGProfileID     *string `json:"wg_profile_id"`
		RefreshInterval *string `json:"refresh_interval"`
		SourceProfileID *string `json:"source_profile_id"`
		EPGSourceID     *string `json:"epg_source_id"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
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
	if req.WGProfileID != nil {
		existing.Config["wg_profile_id"] = *req.WGProfileID
	}
	if req.RefreshInterval != nil {
		existing.Config["refresh_interval"] = *req.RefreshInterval
	}
	if req.SourceProfileID != nil {
		existing.Config["source_profile_id"] = *req.SourceProfileID
	}
	if req.EPGSourceID != nil {
		existing.Config["epg_source_id"] = *req.EPGSourceID
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

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(source.TypeM3U), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source streams")
		return
	}

	if err := s.deps.SourceConfigStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUploadM3U(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	existing, err := s.deps.SourceConfigStore.Get(r.Context(), id)
	if err != nil || existing == nil {
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	entries, err := m3u.Parse(file)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, fmt.Sprintf("parsing m3u: %v", err))
		return
	}

	go s.upsertM3UEntries(existing.ID, entries)

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"parsed": len(entries),
	})
}

func (s *Server) upsertM3UEntries(sourceID string, entries []m3u.Entry) {
	ctx := context.Background()

	seen := make(map[string]struct{}, len(entries))
	streams := make([]media.Stream, 0, len(entries))
	keepIDs := make([]string, 0, len(entries))

	for _, entry := range entries {
		h := sha256.Sum256([]byte(sourceID + ":" + entry.URL))
		id := fmt.Sprintf("%x", h[:16])
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		keepIDs = append(keepIDs, id)

		streams = append(streams, media.Stream{
			ID:         id,
			SourceType: string(source.TypeM3U),
			SourceID:   sourceID,
			Name:       entry.Name,
			URL:        entry.URL,
			Group:      entry.Group,
			TvgID:      entry.TvgID,
			TvgName:    entry.TvgName,
			TvgLogo:    entry.TvgLogo,
			IsActive:   true,
		})
	}

	if err := s.deps.StreamStore.BulkUpsert(ctx, streams); err != nil {
		log.Printf("m3u upload: upsert error: %v", err)
		return
	}

	deleted, err := s.deps.StreamStore.DeleteStaleBySource(ctx, string(source.TypeM3U), sourceID, keepIDs)
	if err != nil {
		log.Printf("m3u upload: delete stale error: %v", err)
		return
	}
	log.Printf("m3u upload: upserted %d streams, deleted %d stale for %s", len(streams), len(deleted), sourceID)
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
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	src, err := s.deps.SourceReg.Create(r.Context(), source.SourceType(cfg.Type), cfg.ID)
	if err != nil {
		httputil.RespondJSON(w, http.StatusOK, source.RefreshStatus{State: source.StateIdle})
		return
	}

	info := src.Info(r.Context())
	status := source.RefreshStatus{State: source.StateIdle}
	if info.LastError != "" {
		status.State = source.StateError
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
		WGProfileID     string `json:"wg_profile_id"`
		SourceProfileID string `json:"source_profile_id"`
		EPGSourceID     string `json:"epg_source_id"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Name == "" || req.URL == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name and url required")
		return
	}

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      string(source.TypeTVPStreams),
		Name:      req.Name,
		IsEnabled: true,
		Config: map[string]string{
			"url":               req.URL,
			"enrollment_token":  req.EnrollmentToken,
			"use_wireguard":     boolStr(req.UseWireGuard),
			"wg_profile_id":     req.WGProfileID,
			"source_profile_id": req.SourceProfileID,
			"epg_source_id":     req.EPGSourceID,
		},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	go func() {
		ctx := r.Context()
		src, err := s.deps.SourceReg.Create(ctx, source.TypeTVPStreams, sc.ID)
		if err != nil {
			return
		}
		src.Refresh(ctx)
		s.AutoMatchStreamsToEPG(ctx, string(source.TypeTVPStreams), sc.ID, sc.Config["epg_source_id"])
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
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	var req struct {
		Name            *string `json:"name"`
		URL             *string `json:"url"`
		EnrollmentToken *string `json:"enrollment_token"`
		IsEnabled       *bool   `json:"is_enabled"`
		UseWireGuard    *bool   `json:"use_wireguard"`
		WGProfileID     *string `json:"wg_profile_id"`
		SourceProfileID *string `json:"source_profile_id"`
		EPGSourceID     *string `json:"epg_source_id"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
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
	if req.WGProfileID != nil {
		existing.Config["wg_profile_id"] = *req.WGProfileID
	}
	if req.SourceProfileID != nil {
		existing.Config["source_profile_id"] = *req.SourceProfileID
	}
	if req.EPGSourceID != nil {
		existing.Config["epg_source_id"] = *req.EPGSourceID
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

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(source.TypeTVPStreams), id); err != nil {
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

	src, err := s.deps.SourceReg.Create(r.Context(), source.TypeTVPStreams, id)
	if err != nil {
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
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
		Name            string `json:"name"`
		Server          string `json:"server"`
		Username        string `json:"username"`
		Password        string `json:"password"`
		UseWireGuard    bool   `json:"use_wireguard"`
		WGProfileID     string `json:"wg_profile_id"`
		MaxStreams      int    `json:"max_streams"`
		RefreshInterval string `json:"refresh_interval"`
		SourceProfileID string `json:"source_profile_id"`
		EPGSourceID     string `json:"epg_source_id"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Name == "" || req.Server == "" || req.Username == "" || req.Password == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name, server, username, and password required")
		return
	}

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      string(source.TypeXtream),
		Name:      req.Name,
		IsEnabled: true,
		Config: map[string]string{
			"server":            req.Server,
			"username":          req.Username,
			"password":          req.Password,
			"use_wireguard":     boolStr(req.UseWireGuard),
			"wg_profile_id":     req.WGProfileID,
			"max_streams":       fmt.Sprintf("%d", req.MaxStreams),
			"refresh_interval":  req.RefreshInterval,
			"source_profile_id": req.SourceProfileID,
			"epg_source_id":     req.EPGSourceID,
		},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	go func() {
		ctx := r.Context()
		src, err := s.deps.SourceReg.Create(ctx, source.TypeXtream, sc.ID)
		if err != nil {
			return
		}
		src.Refresh(ctx)
		s.AutoMatchStreamsToEPG(ctx, string(source.TypeXtream), sc.ID, sc.Config["epg_source_id"])
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
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	var req struct {
		Name            *string `json:"name"`
		Server          *string `json:"server"`
		Username        *string `json:"username"`
		Password        *string `json:"password"`
		IsEnabled       *bool   `json:"is_enabled"`
		UseWireGuard    *bool   `json:"use_wireguard"`
		WGProfileID     *string `json:"wg_profile_id"`
		MaxStreams      *int    `json:"max_streams"`
		RefreshInterval *string `json:"refresh_interval"`
		SourceProfileID *string `json:"source_profile_id"`
		EPGSourceID     *string `json:"epg_source_id"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
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
	if req.WGProfileID != nil {
		existing.Config["wg_profile_id"] = *req.WGProfileID
	}
	if req.MaxStreams != nil {
		existing.Config["max_streams"] = fmt.Sprintf("%d", *req.MaxStreams)
	}
	if req.RefreshInterval != nil {
		existing.Config["refresh_interval"] = *req.RefreshInterval
	}
	if req.SourceProfileID != nil {
		existing.Config["source_profile_id"] = *req.SourceProfileID
	}
	if req.EPGSourceID != nil {
		existing.Config["epg_source_id"] = *req.EPGSourceID
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

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(source.TypeXtream), id); err != nil {
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
	if err != nil || sc == nil || sc.Type != string(source.TypeXtream) {
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	src, err := s.deps.SourceReg.Create(r.Context(), source.TypeXtream, id)
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

func (s *Server) handleCreateTrailersSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name required")
		return
	}

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      string(source.TypeTrailers),
		Name:      req.Name,
		IsEnabled: true,
		Config:    map[string]string{},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	go func() {
		ctx := r.Context()
		src, err := s.deps.SourceReg.Create(ctx, source.TypeTrailers, sc.ID)
		if err != nil {
			return
		}
		src.Refresh(ctx)
	}()

	httputil.RespondJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateTrailersSource(w http.ResponseWriter, r *http.Request) {
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
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	var req struct {
		Name      *string `json:"name"`
		IsEnabled *bool   `json:"is_enabled"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}

	if err := s.deps.SourceConfigStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteTrailersSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(source.TypeTrailers), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source streams")
		return
	}

	if err := s.deps.SourceConfigStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateDemoSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name required")
		return
	}

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      string(source.TypeDemo),
		Name:      req.Name,
		IsEnabled: true,
		Config:    map[string]string{},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	go func() {
		ctx := r.Context()
		src, err := s.deps.SourceReg.Create(ctx, source.TypeDemo, sc.ID)
		if err != nil {
			return
		}
		src.Refresh(ctx)
	}()

	httputil.RespondJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateDemoSource(w http.ResponseWriter, r *http.Request) {
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
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	var req struct {
		Name      *string `json:"name"`
		IsEnabled *bool   `json:"is_enabled"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}

	if err := s.deps.SourceConfigStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteDemoSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(source.TypeDemo), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source streams")
		return
	}

	if err := s.deps.SourceConfigStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateSpaceXSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name required")
		return
	}

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      string(source.TypeSpaceX),
		Name:      req.Name,
		IsEnabled: true,
		Config:    map[string]string{},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	go func() {
		ctx := r.Context()
		src, err := s.deps.SourceReg.Create(ctx, source.TypeSpaceX, sc.ID)
		if err != nil {
			return
		}
		src.Refresh(ctx)
	}()

	httputil.RespondJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateSpaceXSource(w http.ResponseWriter, r *http.Request) {
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
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	var req struct {
		Name      *string `json:"name"`
		IsEnabled *bool   `json:"is_enabled"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}

	if err := s.deps.SourceConfigStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteSpaceXSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(source.TypeSpaceX), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source streams")
		return
	}

	if err := s.deps.SourceConfigStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateRadioGardenSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Places    []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"places"`
		PlaceID   string `json:"place_id"`
		PlaceName string `json:"place_name"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name required")
		return
	}

	// Fall back to single place_id/place_name if places array is empty
	if len(req.Places) == 0 && req.PlaceID != "" {
		req.Places = append(req.Places, struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{ID: req.PlaceID, Name: req.PlaceName})
	}

	if len(req.Places) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "at least one place required")
		return
	}

	placesJSON, _ := json.Marshal(req.Places)

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      string(source.TypeRadioGarden),
		Name:      req.Name,
		IsEnabled: true,
		Config: map[string]string{
			"places": string(placesJSON),
		},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	go func() {
		ctx := r.Context()
		src, err := s.deps.SourceReg.Create(ctx, source.TypeRadioGarden, sc.ID)
		if err != nil {
			return
		}
		src.Refresh(ctx)
	}()

	httputil.RespondJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateRadioGardenSource(w http.ResponseWriter, r *http.Request) {
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
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	var req struct {
		Name      *string `json:"name"`
		IsEnabled *bool   `json:"is_enabled"`
		Places    []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"places"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}
	if req.Places != nil {
		placesJSON, _ := json.Marshal(req.Places)
		existing.Config["places"] = string(placesJSON)
		// Clean up old single-place keys
		delete(existing.Config, "place_id")
		delete(existing.Config, "place_name")
	}

	if err := s.deps.SourceConfigStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteRadioGardenSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(source.TypeRadioGarden), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source streams")
		return
	}

	if err := s.deps.SourceConfigStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRadioGardenPlaces(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		httputil.RespondError(w, http.StatusBadRequest, "missing q parameter")
		return
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://radio.garden/api/ara/content/places", nil)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to build request")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		httputil.RespondError(w, http.StatusBadGateway, "failed to fetch places")
		return
	}
	defer resp.Body.Close()

	var data struct {
		Data struct {
			List []struct {
				ID      string `json:"id"`
				Title   string `json:"title"`
				Country string `json:"country"`
				Size    int    `json:"size"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		httputil.RespondError(w, http.StatusBadGateway, "failed to decode places")
		return
	}

	query := strings.ToLower(q)
	var matches []map[string]any
	for _, p := range data.Data.List {
		if strings.Contains(strings.ToLower(p.Title), query) || strings.Contains(strings.ToLower(p.Country), query) {
			matches = append(matches, map[string]any{
				"id":      p.ID,
				"title":   p.Title,
				"country": p.Country,
				"size":    p.Size,
			})
			if len(matches) >= 20 {
				break
			}
		}
	}

	httputil.RespondJSON(w, http.StatusOK, matches)
}

func (s *Server) handleSourceHealth(w http.ResponseWriter, r *http.Request) {
	configs, err := s.deps.SourceConfigStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list sources")
		return
	}

	type sourceHealth struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Type          string `json:"type"`
		IsEnabled     bool   `json:"is_enabled"`
		StreamCount   int    `json:"stream_count"`
		LastRefreshed string `json:"last_refreshed,omitempty"`
		LastError     string `json:"last_error,omitempty"`
		Health        string `json:"health"`
	}

	healthy := 0
	stale := 0
	errCount := 0
	disabled := 0

	items := make([]sourceHealth, 0, len(configs))
	for _, cfg := range configs {
		sh := sourceHealth{
			ID:        cfg.ID,
			Name:      cfg.Name,
			Type:      cfg.Type,
			IsEnabled: cfg.IsEnabled,
		}

		// Stream count
		if cs := cfg.Config["stream_count"]; cs != "" {
			if n, serr := strconv.Atoi(cs); serr == nil {
				sh.StreamCount = n
			}
		}
		if sh.StreamCount == 0 && s.deps.StreamStore != nil {
			if n, cerr := s.deps.StreamStore.CountBySource(r.Context(), cfg.Type, cfg.ID); cerr == nil && n > 0 {
				sh.StreamCount = n
			}
		}

		// Health determination
		sh.Health = "healthy"
		if !cfg.IsEnabled {
			sh.Health = "disabled"
			disabled++
		} else {
			src, serr := s.deps.SourceReg.Create(r.Context(), source.SourceType(cfg.Type), cfg.ID)
			if serr == nil {
				info := src.Info(r.Context())
				if info.LastRefreshed != nil {
					sh.LastRefreshed = info.LastRefreshed.Format(time.RFC3339)
					if time.Since(*info.LastRefreshed) > time.Hour {
						sh.Health = "stale"
					}
				} else {
					sh.Health = "stale"
				}
				if info.LastError != "" {
					sh.LastError = info.LastError
					sh.Health = "error"
				}
			}
			if sh.LastRefreshed == "" {
				if lr := cfg.Config["last_refreshed"]; lr != "" {
					sh.LastRefreshed = lr
					if t, terr := time.Parse(time.RFC3339, lr); terr == nil && time.Since(t) > time.Hour {
						sh.Health = "stale"
					}
				} else {
					sh.Health = "stale"
				}
			}

			switch sh.Health {
			case "healthy":
				healthy++
			case "stale":
				stale++
			case "error":
				errCount++
			}
		}

		items = append(items, sh)
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"sources": items,
		"summary": map[string]any{
			"total":    len(configs),
			"healthy":  healthy,
			"stale":    stale,
			"error":    errCount,
			"disabled": disabled,
		},
	})
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
