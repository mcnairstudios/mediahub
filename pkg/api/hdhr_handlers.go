package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/source/hdhr"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
)

var (
	hdhrRetuneMu      sync.RWMutex
	hdhrRetuneStatus  = make(map[string]source.RefreshStatus)
)

func (s *Server) handleCreateHDHRSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		IsEnabled *bool  `json:"is_enabled"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name required")
		return
	}

	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}

	sc := &sourceconfig.SourceConfig{
		ID:        uuid.New().String(),
		Type:      "hdhr",
		Name:      req.Name,
		IsEnabled: enabled,
		Config:    map[string]string{},
	}

	if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateHDHRSource(w http.ResponseWriter, r *http.Request) {
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
	if existing == nil || existing.Type != "hdhr" {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	var req struct {
		Name      *string `json:"name"`
		IsEnabled *bool   `json:"is_enabled"`
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

	if err := s.deps.SourceConfigStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteHDHRSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), "hdhr", id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source streams")
		return
	}

	if err := s.deps.SourceConfigStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHDHRDiscover(w http.ResponseWriter, r *http.Request) {
	src, err := s.deps.SourceReg.Create(r.Context(), "hdhr", "")
	if err != nil {
		ips, discErr := hdhr.UDPDiscover()
		if discErr != nil {
			httputil.RespondError(w, http.StatusInternalServerError, discErr.Error())
			return
		}

		configs, _ := s.deps.SourceConfigStore.ListByType(r.Context(), "hdhr")
		existingHosts := make(map[string]bool)
		for _, cfg := range configs {
			if devicesJSON := cfg.Config["devices"]; devicesJSON != "" {
				var devices []hdhr.Device
				json.Unmarshal([]byte(devicesJSON), &devices)
				for _, d := range devices {
					existingHosts[d.Host] = true
				}
			}
		}

		var results []source.DiscoveredDevice
		for _, ip := range ips {
			results = append(results, source.DiscoveredDevice{
				Host:         ip,
				AlreadyAdded: existingHosts[ip],
			})
		}
		if results == nil {
			results = []source.DiscoveredDevice{}
		}
		httputil.RespondJSON(w, http.StatusOK, results)
		return
	}

	disc, ok := src.(source.Discoverable)
	if !ok {
		httputil.RespondJSON(w, http.StatusOK, []source.DiscoveredDevice{})
		return
	}

	devices, err := disc.Discover(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if devices == nil {
		devices = []source.DiscoveredDevice{}
	}
	httputil.RespondJSON(w, http.StatusOK, devices)
}

func (s *Server) handleHDHRAddDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host string `json:"host"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil || req.Host == "" {
		httputil.RespondError(w, http.StatusBadRequest, "host required")
		return
	}

	configs, err := s.deps.SourceConfigStore.ListByType(r.Context(), "hdhr")
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list sources")
		return
	}

	if len(configs) == 0 {
		sc := &sourceconfig.SourceConfig{
			ID:        uuid.New().String(),
			Type:      "hdhr",
			Name:      "HDHomeRun",
			IsEnabled: true,
			Config:    map[string]string{},
		}

		device := hdhr.Device{Host: req.Host}
		devicesJSON, _ := json.Marshal([]hdhr.Device{device})
		sc.Config["devices"] = string(devicesJSON)

		if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
			return
		}

		go func() {
			src, err := s.deps.SourceReg.Create(context.Background(), "hdhr", sc.ID)
			if err != nil {
				return
			}
			src.Refresh(context.Background())
		}()

		httputil.RespondJSON(w, http.StatusCreated, map[string]string{"message": "device added to new source"})
		return
	}

	sc := configs[0]
	var devices []hdhr.Device
	if devicesJSON := sc.Config["devices"]; devicesJSON != "" {
		json.Unmarshal([]byte(devicesJSON), &devices)
	}

	for _, d := range devices {
		if d.Host == req.Host {
			httputil.RespondError(w, http.StatusConflict, "device already added")
			return
		}
	}

	devices = append(devices, hdhr.Device{Host: req.Host})
	devicesJSON, _ := json.Marshal(devices)
	sc.Config["devices"] = string(devicesJSON)

	if err := s.deps.SourceConfigStore.Update(r.Context(), &sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	go func() {
		src, err := s.deps.SourceReg.Create(context.Background(), "hdhr", sc.ID)
		if err != nil {
			return
		}
		src.Refresh(context.Background())
	}()

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "device added"})
}

func (s *Server) handleHDHRDevices(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	sc, err := s.deps.SourceConfigStore.Get(r.Context(), id)
	if err != nil || sc == nil || sc.Type != "hdhr" {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	var devices []hdhr.Device
	if devicesJSON := sc.Config["devices"]; devicesJSON != "" {
		json.Unmarshal([]byte(devicesJSON), &devices)
	}
	if devices == nil {
		devices = []hdhr.Device{}
	}

	httputil.RespondJSON(w, http.StatusOK, devices)
}

func (s *Server) handleHDHRScan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	sc, err := s.deps.SourceConfigStore.Get(r.Context(), id)
	if err != nil || sc == nil || sc.Type != "hdhr" {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	go func() {
		src, err := s.deps.SourceReg.Create(context.Background(), "hdhr", id)
		if err != nil {
			log.Printf("hdhr scan: failed to create source: %v", err)
			return
		}
		if err := src.Refresh(context.Background()); err != nil {
			log.Printf("hdhr scan failed for %s: %v", id, err)
		} else {
			log.Printf("hdhr scan completed for %s", id)
		}
	}()

	httputil.RespondJSON(w, http.StatusAccepted, map[string]string{"message": "scan started"})
}

func (s *Server) handleHDHRRetune(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	sc, err := s.deps.SourceConfigStore.Get(r.Context(), id)
	if err != nil || sc == nil || sc.Type != "hdhr" {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	hdhrRetuneMu.Lock()
	hdhrRetuneStatus[id] = source.RefreshStatus{State: "scanning", Message: "Starting retune..."}
	hdhrRetuneMu.Unlock()

	go func() {
		src, err := s.deps.SourceReg.Create(context.Background(), "hdhr", id)
		if err != nil {
			hdhrRetuneMu.Lock()
			hdhrRetuneStatus[id] = source.RefreshStatus{State: "error", Message: err.Error()}
			hdhrRetuneMu.Unlock()
			return
		}

		retunable, ok := src.(source.Retunable)
		if !ok {
			hdhrRetuneMu.Lock()
			hdhrRetuneStatus[id] = source.RefreshStatus{State: "error", Message: "source does not support retune"}
			hdhrRetuneMu.Unlock()
			return
		}

		if err := retunable.Retune(context.Background()); err != nil {
			hdhrRetuneMu.Lock()
			hdhrRetuneStatus[id] = source.RefreshStatus{State: "error", Message: err.Error()}
			hdhrRetuneMu.Unlock()
			log.Printf("hdhr retune failed for %s: %v", id, err)
			return
		}

		info := src.Info(context.Background())
		hdhrRetuneMu.Lock()
		hdhrRetuneStatus[id] = source.RefreshStatus{
			State:   "done",
			Message: fmt.Sprintf("Retune complete. %d streams found.", info.StreamCount),
		}
		hdhrRetuneMu.Unlock()
		log.Printf("hdhr retune completed for %s: %d streams", id, info.StreamCount)
	}()

	httputil.RespondJSON(w, http.StatusAccepted, map[string]string{"message": "retune started"})
}

func (s *Server) handleHDHRRetuneStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	hdhrRetuneMu.RLock()
	status, ok := hdhrRetuneStatus[id]
	hdhrRetuneMu.RUnlock()

	if !ok {
		httputil.RespondJSON(w, http.StatusOK, source.RefreshStatus{State: "idle"})
		return
	}

	httputil.RespondJSON(w, http.StatusOK, status)
}

func (s *Server) handleHDHRClear(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), "hdhr", id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to clear streams")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func hdhrDevicesFromConfig(sc *sourceconfig.SourceConfig) []hdhr.Device {
	var devices []hdhr.Device
	if devicesJSON := sc.Config["devices"]; devicesJSON != "" {
		json.Unmarshal([]byte(devicesJSON), &devices)
	}
	return devices
}

func hdhrDeviceCount(sc *sourceconfig.SourceConfig) int {
	return len(hdhrDevicesFromConfig(sc))
}

func hdhrTunerCount(sc *sourceconfig.SourceConfig) int {
	total := 0
	for _, d := range hdhrDevicesFromConfig(sc) {
		total += d.TunerCount
	}
	return total
}

func hdhrMaxStreams(sc *sourceconfig.SourceConfig) int {
	if v := sc.Config["max_streams"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return hdhrTunerCount(sc)
}
