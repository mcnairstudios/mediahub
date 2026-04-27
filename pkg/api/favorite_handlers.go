package api

import (
	"net/http"

	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/middleware"
)

func (s *Server) handleListFavorites(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if s.deps.FavoriteStore == nil {
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}

	favs, err := s.deps.FavoriteStore.List(r.Context(), user.ID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list favorites")
		return
	}
	if favs == nil {
		favs = nil
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}

	httputil.RespondJSON(w, http.StatusOK, favs)
}

func (s *Server) handleAddFavorite(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if s.deps.FavoriteStore == nil {
		httputil.RespondError(w, http.StatusInternalServerError, "favorites not configured")
		return
	}

	var req struct {
		StreamID string `json:"stream_id"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.StreamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "stream_id required")
		return
	}

	if err := s.deps.FavoriteStore.Add(r.Context(), user.ID, req.StreamID); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to add favorite")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveFavorite(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if s.deps.FavoriteStore == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	streamID := r.PathValue("streamID")
	if streamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "stream ID required")
		return
	}

	if err := s.deps.FavoriteStore.Remove(r.Context(), user.ID, streamID); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to remove favorite")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCheckFavorite(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if s.deps.FavoriteStore == nil {
		httputil.RespondJSON(w, http.StatusOK, map[string]bool{"is_favorite": false})
		return
	}

	streamID := r.PathValue("streamID")
	if streamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "stream ID required")
		return
	}

	isFav, err := s.deps.FavoriteStore.IsFavorite(r.Context(), user.ID, streamID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to check favorite")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]bool{"is_favorite": isFav})
}
