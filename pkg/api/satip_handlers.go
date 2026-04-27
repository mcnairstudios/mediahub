package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
)

var (
	satipScanMu     sync.RWMutex
	satipScanStatus = make(map[string]source.RefreshStatus)
)

func (s *Server) handleCreateSatIPSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string `json:"name"`
		Host            string `json:"host"`
		HTTPPort        int    `json:"http_port"`
		TransmitterFile string `json:"transmitter_file"`
		IsEnabled       *bool  `json:"is_enabled"`
		MaxStreams      int    `json:"max_streams"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Host == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name and host required")
		return
	}

	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}

	httpPort := req.HTTPPort
	if httpPort == 0 {
		httpPort = 8875
	}

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      "satip",
		Name:      req.Name,
		IsEnabled: enabled,
		Config: map[string]string{
			"host":             req.Host,
			"http_port":        fmt.Sprintf("%d", httpPort),
			"transmitter_file": req.TransmitterFile,
			"max_streams":      fmt.Sprintf("%d", req.MaxStreams),
		},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateSatIPSource(w http.ResponseWriter, r *http.Request) {
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
	if existing == nil || existing.Type != "satip" {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	var req struct {
		Name            *string `json:"name"`
		Host            *string `json:"host"`
		HTTPPort        *int    `json:"http_port"`
		TransmitterFile *string `json:"transmitter_file"`
		IsEnabled       *bool   `json:"is_enabled"`
		MaxStreams      *int    `json:"max_streams"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}
	if req.Host != nil {
		existing.Config["host"] = *req.Host
	}
	if req.HTTPPort != nil {
		existing.Config["http_port"] = fmt.Sprintf("%d", *req.HTTPPort)
	}
	if req.TransmitterFile != nil {
		existing.Config["transmitter_file"] = *req.TransmitterFile
	}
	if req.MaxStreams != nil {
		existing.Config["max_streams"] = fmt.Sprintf("%d", *req.MaxStreams)
	}

	if err := s.deps.SourceConfigStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteSatIPSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), "satip", id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source streams")
		return
	}

	if err := s.deps.SourceConfigStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSatIPScan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	sc, err := s.deps.SourceConfigStore.Get(r.Context(), id)
	if err != nil || sc == nil || sc.Type != "satip" {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	satipScanMu.Lock()
	satipScanStatus[id] = source.RefreshStatus{State: "scanning", Message: "Starting scan..."}
	satipScanMu.Unlock()

	go func() {
		src, err := s.deps.SourceReg.Create(context.Background(), "satip", id)
		if err != nil {
			satipScanMu.Lock()
			satipScanStatus[id] = source.RefreshStatus{State: "error", Message: err.Error()}
			satipScanMu.Unlock()
			return
		}

		if err := src.Refresh(context.Background()); err != nil {
			satipScanMu.Lock()
			satipScanStatus[id] = source.RefreshStatus{State: "error", Message: err.Error()}
			satipScanMu.Unlock()
			log.Printf("satip scan failed for %s: %v", id, err)
			return
		}

		info := src.Info(context.Background())
		satipScanMu.Lock()
		satipScanStatus[id] = source.RefreshStatus{
			State:   "done",
			Message: fmt.Sprintf("Scan complete. %d streams found.", info.StreamCount),
		}
		satipScanMu.Unlock()
		log.Printf("satip scan completed for %s: %d streams", id, info.StreamCount)
	}()

	httputil.RespondJSON(w, http.StatusAccepted, map[string]string{"message": "scan started"})
}

func (s *Server) handleSatIPScanStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	satipScanMu.RLock()
	status, ok := satipScanStatus[id]
	satipScanMu.RUnlock()

	if !ok {
		httputil.RespondJSON(w, http.StatusOK, source.RefreshStatus{State: "idle"})
		return
	}

	httputil.RespondJSON(w, http.StatusOK, status)
}

func (s *Server) handleSatIPClear(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), "satip", id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to clear streams")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
