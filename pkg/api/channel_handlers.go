package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

func (s *Server) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string   `json:"name"`
		Number          int      `json:"number"`
		GroupID         string   `json:"group_id"`
		StreamIDs       []string `json:"stream_ids"`
		LogoURL         string   `json:"logo_url"`
		TvgID           string   `json:"tvg_id"`
		StreamProfileID string   `json:"stream_profile_id"`
		IsEnabled       *bool    `json:"is_enabled"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
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
		ID:              uuid.New().String(),
		Name:            req.Name,
		Number:          req.Number,
		GroupID:         req.GroupID,
		StreamIDs:       req.StreamIDs,
		LogoURL:         req.LogoURL,
		TvgID:           req.TvgID,
		StreamProfileID: req.StreamProfileID,
		IsEnabled:       enabled,
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
		Name            *string  `json:"name"`
		Number          *int     `json:"number"`
		GroupID         *string  `json:"group_id"`
		StreamIDs       []string `json:"stream_ids"`
		LogoURL         *string  `json:"logo_url"`
		TvgID           *string  `json:"tvg_id"`
		StreamProfileID *string  `json:"stream_profile_id"`
		IsEnabled       *bool    `json:"is_enabled"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
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
	if req.StreamProfileID != nil {
		existing.StreamProfileID = *req.StreamProfileID
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
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
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

	channels, chErr := s.deps.ChannelStore.List(r.Context())
	if chErr == nil {
		counts := make(map[string]int)
		for _, ch := range channels {
			if ch.GroupID != "" {
				counts[ch.GroupID]++
			}
		}
		for i := range groups {
			groups[i].ChannelCount = counts[groups[i].ID]
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].SortOrder != groups[j].SortOrder {
			return groups[i].SortOrder < groups[j].SortOrder
		}
		return groups[i].Name < groups[j].Name
	})

	httputil.RespondJSON(w, http.StatusOK, groups)
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "group ID required")
		return
	}

	existing, err := s.deps.GroupStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get group")
		return
	}
	if existing == nil {
		httputil.RespondError(w, http.StatusNotFound, "group not found")
		return
	}

	var req struct {
		Name      *string `json:"name"`
		SortOrder *int    `json:"sort_order"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.SortOrder != nil {
		existing.SortOrder = *req.SortOrder
	}

	if err := s.deps.GroupStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update group")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
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

func (s *Server) handleReorderGroups(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GroupIDs []string `json:"group_ids"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	for i, id := range req.GroupIDs {
		g, err := s.deps.GroupStore.Get(r.Context(), id)
		if err != nil || g == nil {
			continue
		}
		g.SortOrder = i
		s.deps.GroupStore.Update(r.Context(), g)
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{"updated": len(req.GroupIDs)})
}

func (s *Server) handleBulkAssignGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelIDs []string `json:"channel_ids"`
		GroupID    string   `json:"group_id"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if len(req.ChannelIDs) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "channel_ids required")
		return
	}

	updated := 0
	for _, id := range req.ChannelIDs {
		ch, err := s.deps.ChannelStore.Get(r.Context(), id)
		if err != nil || ch == nil {
			continue
		}
		ch.GroupID = req.GroupID
		if s.deps.ChannelStore.Update(r.Context(), ch) == nil {
			updated++
		}
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{"updated": updated})
}

func (s *Server) handleBatchUpdateChannels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelIDs []string `json:"channel_ids"`
		IsEnabled  *bool    `json:"is_enabled"`
		AutoNumber *bool    `json:"auto_number"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.AutoNumber != nil && *req.AutoNumber {
		channels, err := s.deps.ChannelStore.List(r.Context())
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to list channels")
			return
		}
		for i := range channels {
			channels[i].Number = i + 1
			if err := s.deps.ChannelStore.Update(r.Context(), &channels[i]); err != nil {
				httputil.RespondError(w, http.StatusInternalServerError, "failed to update channel")
				return
			}
		}
		httputil.RespondJSON(w, http.StatusOK, map[string]any{"updated": len(channels)})
		return
	}

	if len(req.ChannelIDs) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "channel_ids required")
		return
	}

	updated := 0
	for _, id := range req.ChannelIDs {
		ch, err := s.deps.ChannelStore.Get(r.Context(), id)
		if err != nil || ch == nil {
			continue
		}
		if req.IsEnabled != nil {
			ch.IsEnabled = *req.IsEnabled
		}
		if err := s.deps.ChannelStore.Update(r.Context(), ch); err != nil {
			continue
		}
		updated++
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{"updated": updated})
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
