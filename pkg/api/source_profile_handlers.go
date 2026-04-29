package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/sourceprofile"
)

func (s *Server) handleListSourceProfiles(w http.ResponseWriter, r *http.Request) {
	if s.deps.SourceProfileStore == nil {
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}

	profiles, err := s.deps.SourceProfileStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list source profiles")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, profiles)
}

func (s *Server) handleGetSourceProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source profile ID required")
		return
	}

	p, err := s.deps.SourceProfileStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get source profile")
		return
	}
	if p == nil {
		httputil.RespondError(w, http.StatusNotFound, "source profile not found")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, p)
}

func (s *Server) handleCreateSourceProfile(w http.ResponseWriter, r *http.Request) {
	var p sourceprofile.Profile
	if err := httputil.DecodeJSON(r, &p); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if p.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name required")
		return
	}

	p.ID = generateSourceProfileID()

	if err := s.deps.SourceProfileStore.Create(r.Context(), &p); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source profile")
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, &p)
}

func (s *Server) handleUpdateSourceProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source profile ID required")
		return
	}

	existing, err := s.deps.SourceProfileStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get source profile")
		return
	}
	if existing == nil {
		httputil.RespondError(w, http.StatusNotFound, "source profile not found")
		return
	}

	var req sourceprofile.Profile
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.ID = id
	if err := s.deps.SourceProfileStore.Update(r.Context(), &req); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source profile")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, &req)
}

func (s *Server) handleDeleteSourceProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source profile ID required")
		return
	}

	existing, err := s.deps.SourceProfileStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get source profile")
		return
	}
	if existing == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := s.deps.SourceProfileStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source profile")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func generateSourceProfileID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
