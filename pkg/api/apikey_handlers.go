package api

import (
	"errors"
	"net/http"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/middleware"
)

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	apiKey, err := s.deps.AuthService.CreateAPIKey(r.Context(), user.ID, req.Name)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, apiKey)
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	keys, err := s.deps.AuthService.ListAPIKeys(r.Context(), user.ID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if keys == nil {
		keys = []*auth.APIKey{}
	}

	httputil.RespondJSON(w, http.StatusOK, keys)
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	keyID := r.PathValue("id")
	if keyID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "key ID required")
		return
	}

	if err := s.deps.AuthService.RevokeAPIKey(r.Context(), user.ID, keyID); err != nil {
		if errors.Is(err, auth.ErrAPIKeyNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
