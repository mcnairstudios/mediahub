package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

func (s *Server) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string   `json:"name"`
		Number    int      `json:"number"`
		GroupID   string   `json:"group_id"`
		StreamIDs []string `json:"stream_ids"`
		LogoURL   string   `json:"logo_url"`
		TvgID     string   `json:"tvg_id"`
		IsEnabled *bool    `json:"is_enabled"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name required")
		return
	}

	if req.Number > 0 {
		if err := s.checkChannelNumberUnique(r.Context(), req.Number, ""); err != nil {
			httputil.RespondError(w, http.StatusConflict, err.Error())
			return
		}
	}

	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}

	ch := &channel.Channel{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Number:    req.Number,
		GroupID:   req.GroupID,
		StreamIDs: req.StreamIDs,
		LogoURL:   req.LogoURL,
		TvgID:     req.TvgID,
		IsEnabled: enabled,
	}

	if err := s.deps.ChannelStore.Create(r.Context(), ch); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create channel")
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, ch)
}

func (s *Server) handleUpdateChannel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "channel ID required")
		return
	}

	existing, err := s.deps.ChannelStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get channel")
		return
	}
	if existing == nil {
		httputil.RespondError(w, http.StatusNotFound, "channel not found")
		return
	}

	var req struct {
		Name      *string  `json:"name"`
		Number    *int     `json:"number"`
		GroupID   *string  `json:"group_id"`
		StreamIDs []string `json:"stream_ids"`
		LogoURL   *string  `json:"logo_url"`
		TvgID     *string  `json:"tvg_id"`
		IsEnabled *bool    `json:"is_enabled"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Number != nil {
		if *req.Number != existing.Number && *req.Number > 0 {
			if err := s.checkChannelNumberUnique(r.Context(), *req.Number, id); err != nil {
				httputil.RespondError(w, http.StatusConflict, err.Error())
				return
			}
		}
		existing.Number = *req.Number
	}
	if req.GroupID != nil {
		existing.GroupID = *req.GroupID
	}
	if req.StreamIDs != nil {
		existing.StreamIDs = req.StreamIDs
	}
	if req.LogoURL != nil {
		existing.LogoURL = *req.LogoURL
	}
	if req.TvgID != nil {
		existing.TvgID = *req.TvgID
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}

	if err := s.deps.ChannelStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update channel")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "channel ID required")
		return
	}

	if err := s.deps.ChannelStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete channel")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAssignStreams(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "channel ID required")
		return
	}

	existing, err := s.deps.ChannelStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get channel")
		return
	}
	if existing == nil {
		httputil.RespondError(w, http.StatusNotFound, "channel not found")
		return
	}

	var req struct {
		StreamIDs []string `json:"stream_ids"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.deps.ChannelStore.AssignStreams(r.Context(), id, req.StreamIDs); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to assign streams")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.deps.GroupStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list groups")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, groups)
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name required")
		return
	}

	g := &channel.Group{
		ID:   uuid.New().String(),
		Name: req.Name,
	}

	if err := s.deps.GroupStore.Create(r.Context(), g); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create group")
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, g)
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "group ID required")
		return
	}

	if err := s.deps.GroupStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete group")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) checkChannelNumberUnique(ctx context.Context, number int, excludeID string) error {
	channels, err := s.deps.ChannelStore.List(ctx)
	if err != nil {
		return err
	}
	for _, ch := range channels {
		if ch.Number == number && ch.ID != excludeID {
			return fmt.Errorf("channel number %d already in use", number)
		}
	}
	return nil
}
