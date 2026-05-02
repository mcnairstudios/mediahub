package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Role      auth.Role `json:"role"`
		ExpiresIn string    `json:"expires_in"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Role == "" {
		req.Role = auth.RoleStandard
	}

	var dur time.Duration
	if req.ExpiresIn != "" {
		var err error
		dur, err = time.ParseDuration(req.ExpiresIn)
		if err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "invalid expires_in duration")
			return
		}
	}

	invite, err := s.deps.AuthService.CreateInvite(r.Context(), req.Role, dur)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, invite)
}

func (s *Server) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if req.Token == "" || req.Username == "" || req.Password == "" {
		httputil.RespondError(w, http.StatusBadRequest, "token, username, and password required")
		return
	}

	user, err := s.deps.AuthService.AcceptInvite(r.Context(), req.Token, req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInviteNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "invite not found")
			return
		}
		if errors.Is(err, auth.ErrInviteExpired) {
			httputil.RespondError(w, http.StatusGone, "invite expired")
			return
		}
		if errors.Is(err, auth.ErrInviteUsed) {
			httputil.RespondError(w, http.StatusConflict, "invite already used")
			return
		}
		if errors.Is(err, auth.ErrUsernameExists) {
			httputil.RespondError(w, http.StatusConflict, "username already exists")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, user)
}

func (s *Server) handleListInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := s.deps.AuthService.ListInvites(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if invites == nil {
		invites = []*auth.Invite{}
	}
	httputil.RespondJSON(w, http.StatusOK, invites)
}

func (s *Server) handleDeleteInvite(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		httputil.RespondError(w, http.StatusBadRequest, "token required")
		return
	}

	if err := s.deps.AuthService.DeleteInvite(r.Context(), token); err != nil {
		if errors.Is(err, auth.ErrInviteNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
