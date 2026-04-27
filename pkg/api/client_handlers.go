package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sort"

	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

func (s *Server) handleListClients(w http.ResponseWriter, r *http.Request) {
	if s.deps.ClientStore == nil {
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}

	clients, err := s.deps.ClientStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list clients")
		return
	}
	if clients == nil {
		clients = []client.Client{}
	}

	sort.Slice(clients, func(i, j int) bool {
		return clients[i].Priority > clients[j].Priority
	})

	httputil.RespondJSON(w, http.StatusOK, clients)
}

func (s *Server) handleGetClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "client ID required")
		return
	}

	c, err := s.deps.ClientStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get client")
		return
	}
	if c == nil {
		httputil.RespondError(w, http.StatusNotFound, "client not found")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, c)
}

func (s *Server) handleUpdateClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "client ID required")
		return
	}

	existing, err := s.deps.ClientStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get client")
		return
	}
	if existing == nil {
		httputil.RespondError(w, http.StatusNotFound, "client not found")
		return
	}

	var req struct {
		Name       *string            `json:"name"`
		Priority   *int               `json:"priority"`
		ListenPort *int               `json:"listen_port"`
		IsEnabled  *bool              `json:"is_enabled"`
		MatchRules *[]client.MatchRule `json:"match_rules"`
		Profile    *client.Profile    `json:"profile"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil && !existing.IsSystem {
		existing.Name = *req.Name
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
	if req.ListenPort != nil {
		existing.ListenPort = *req.ListenPort
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}
	if req.MatchRules != nil && !existing.IsSystem {
		existing.MatchRules = *req.MatchRules
	}
	if req.Profile != nil {
		existing.Profile = *req.Profile
	}

	if err := s.deps.ClientStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update client")
		return
	}

	s.rebuildDetector(r.Context())

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	var c client.Client
	if err := httputil.DecodeJSON(r, &c); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if c.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name required")
		return
	}

	c.ID = generateClientID()
	c.IsSystem = false

	if err := s.deps.ClientStore.Create(r.Context(), &c); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create client")
		return
	}

	s.rebuildDetector(r.Context())

	httputil.RespondJSON(w, http.StatusCreated, &c)
}

func (s *Server) handleDeleteClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "client ID required")
		return
	}

	existing, err := s.deps.ClientStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get client")
		return
	}
	if existing == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if existing.IsSystem {
		httputil.RespondError(w, http.StatusForbidden, "cannot delete system client")
		return
	}

	if err := s.deps.ClientStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete client")
		return
	}

	s.rebuildDetector(r.Context())

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) rebuildDetector(ctx context.Context) {
	if s.deps.ClientStore == nil {
		return
	}
	clients, err := s.deps.ClientStore.List(ctx)
	if err != nil {
		return
	}
	s.deps.Detector = client.NewDetector(clients)
}

func generateClientID() string {
	b := make([]byte, 16)
	_, _ = cryptoRandRead(b)
	return hexEncode(b)
}

var cryptoRandRead = func(b []byte) (int, error) {
	return rand.Read(b)
}

var hexEncode = func(b []byte) string {
	return hex.EncodeToString(b)
}
