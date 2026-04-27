package api

import (
	"net/http"

	"github.com/mcnairstudios/mediahub/pkg/connectivity/wg"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

func (s *Server) handleListWGProfiles(w http.ResponseWriter, r *http.Request) {
	if s.deps.WGService == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "wireguard not configured")
		return
	}

	profiles, err := s.deps.WGService.ListProfiles(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list profiles")
		return
	}
	if profiles == nil {
		profiles = []wg.ProfileResponse{}
	}
	httputil.RespondJSON(w, http.StatusOK, profiles)
}

func (s *Server) handleCreateWGProfile(w http.ResponseWriter, r *http.Request) {
	if s.deps.WGService == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "wireguard not configured")
		return
	}

	var req struct {
		Name       string `json:"name"`
		PrivateKey string `json:"private_key"`
		Endpoint   string `json:"endpoint"`
		PublicKey  string `json:"public_key"`
		AllowedIPs string `json:"allowed_ips"`
		DNS        string `json:"dns"`
		Address    string `json:"address"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	profile, err := s.deps.WGService.CreateProfile(r.Context(), wg.TunnelConfig{
		Name:       req.Name,
		PrivateKey: req.PrivateKey,
		Endpoint:   req.Endpoint,
		PublicKey:  req.PublicKey,
		AllowedIPs: req.AllowedIPs,
		DNS:        req.DNS,
		Address:    req.Address,
	})
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, profile)
}

func (s *Server) handleUpdateWGProfile(w http.ResponseWriter, r *http.Request) {
	if s.deps.WGService == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "wireguard not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "profile ID required")
		return
	}

	var req struct {
		Name       string `json:"name"`
		PrivateKey string `json:"private_key"`
		Endpoint   string `json:"endpoint"`
		PublicKey  string `json:"public_key"`
		AllowedIPs string `json:"allowed_ips"`
		DNS        string `json:"dns"`
		Address    string `json:"address"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	profile, err := s.deps.WGService.UpdateProfile(r.Context(), id, wg.TunnelConfig{
		Name:       req.Name,
		PrivateKey: req.PrivateKey,
		Endpoint:   req.Endpoint,
		PublicKey:  req.PublicKey,
		AllowedIPs: req.AllowedIPs,
		DNS:        req.DNS,
		Address:    req.Address,
	})
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.RespondJSON(w, http.StatusOK, profile)
}

func (s *Server) handleDeleteWGProfile(w http.ResponseWriter, r *http.Request) {
	if s.deps.WGService == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "wireguard not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "profile ID required")
		return
	}

	if err := s.deps.WGService.DeleteProfile(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleActivateWGProfile(w http.ResponseWriter, r *http.Request) {
	if s.deps.WGService == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "wireguard not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "profile ID required")
		return
	}

	if err := s.deps.WGService.Activate(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"status": "activated"})
}

func (s *Server) handleTestWGProfile(w http.ResponseWriter, r *http.Request) {
	if s.deps.WGService == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "wireguard not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "profile ID required")
		return
	}

	result := s.deps.WGService.TestProfile(r.Context(), id)
	httputil.RespondJSON(w, http.StatusOK, result)
}

func (s *Server) handleWGStatus(w http.ResponseWriter, r *http.Request) {
	if s.deps.WGService == nil {
		httputil.RespondJSON(w, http.StatusOK, wg.StatusResponse{Connected: false})
		return
	}

	httputil.RespondJSON(w, http.StatusOK, s.deps.WGService.Status())
}
