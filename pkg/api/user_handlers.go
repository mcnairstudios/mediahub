package api

import (
	"errors"
	"net/http"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/middleware"
)

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "user ID required")
		return
	}

	var req struct {
		Username string    `json:"username"`
		Role     auth.Role `json:"role"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.deps.AuthService.UpdateUser(r.Context(), id, req.Username, req.Role)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, auth.ErrUsernameExists) {
			httputil.RespondError(w, http.StatusConflict, "username already exists")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.RespondJSON(w, http.StatusOK, user)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "user ID required")
		return
	}

	if err := s.deps.AuthService.DeleteUser(r.Context(), id); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "user ID required")
		return
	}

	currentUser := middleware.UserFromContext(r.Context())
	if currentUser == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if !currentUser.IsAdmin && currentUser.ID != id {
		httputil.RespondError(w, http.StatusForbidden, "can only change your own password")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password == "" {
		httputil.RespondError(w, http.StatusBadRequest, "password required")
		return
	}

	if err := s.deps.AuthService.ChangePassword(r.Context(), id, req.Password); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "user not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
