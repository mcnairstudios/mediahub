package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

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
		Name            string `json:"name"`
		IsEnabled       *bool  `json:"is_enabled"`
		SourceProfileID string `json:"source_profile_id"`
		EPGSourceID     string `json:"epg_source_id"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
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
		Type:      string(source.TypeHDHR),
		Name:      req.Name,
		IsEnabled: enabled,
		Config: map[string]string{
			"source_profile_id": req.SourceProfileID,
			"epg_source_id":     req.EPGSourceID,
		},
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
	if existing == nil || existing.Type != string(source.TypeHDHR) {
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	var req struct {
		Name            *string `json:"name"`
		IsEnabled       *bool   `json:"is_enabled"`
		SourceProfileID *string `json:"source_profile_id"`
		EPGSourceID     *string `json:"epg_source_id"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}
	if req.SourceProfileID != nil {
		existing.Config["source_profile_id"] = *req.SourceProfileID
	}
	if req.EPGSourceID != nil {
		existing.Config["epg_source_id"] = *req.EPGSourceID
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

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(source.TypeHDHR), id); err != nil {
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
	src, err := s.deps.SourceReg.Create(r.Context(), source.TypeHDHR, "")
	if err != nil {
		ips, discErr := hdhr.UDPDiscover()
		if discErr != nil {
			httputil.RespondError(w, http.StatusInternalServerError, discErr.Error())
			return
		}

		configs, _ := s.deps.SourceConfigStore.ListByType(r.Context(), string(source.TypeHDHR))
		existingHosts := make(map[string]bool)
		for _, cfg := range configs {
			if devicesJSON := cfg.Config["devices"]; devicesJSON != "" {
				var devices []hdhr.Device
				if err := json.Unmarshal([]byte(devicesJSON), &devices); err != nil {
					log.Printf("hdhr discover: failed to unmarshal devices for source %s: %v", cfg.ID, err)
					continue
				}
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

	configs, err := s.deps.SourceConfigStore.ListByType(r.Context(), string(source.TypeHDHR))
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list sources")
		return
	}

	if len(configs) == 0 {
		sc := &sourceconfig.SourceConfig{
			ID:        uuid.New().String(),
			Type:      string(source.TypeHDHR),
			Name:      "HDHomeRun",
			IsEnabled: true,
			Config:    map[string]string{},
		}

		device := hdhr.Device{Host: req.Host}
		devicesJSON, err := json.Marshal([]hdhr.Device{device})
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to marshal device")
			return
		}
		sc.Config["devices"] = string(devicesJSON)

		if err := s.deps.SourceConfigStore.Create(r.Context(), sc); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to create source")
			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			src, err := s.deps.SourceReg.Create(ctx, source.TypeHDHR, sc.ID)
			if err != nil {
				log.Printf("hdhr add device: failed to create source %s: %v", sc.ID, err)
				return
			}
			if err := src.Refresh(ctx); err != nil {
				log.Printf("hdhr add device: refresh failed for %s: %v", sc.ID, err)
			}
		}()

		httputil.RespondJSON(w, http.StatusCreated, map[string]string{"message": "device added to new source"})
		return
	}

	sc := configs[0]
	var devices []hdhr.Device
	if devicesJSON := sc.Config["devices"]; devicesJSON != "" {
		if err := json.Unmarshal([]byte(devicesJSON), &devices); err != nil {
			log.Printf("hdhr add device: failed to unmarshal devices for source %s: %v", sc.ID, err)
			httputil.RespondError(w, http.StatusInternalServerError, "failed to parse existing devices")
			return
		}
	}

	for _, d := range devices {
		if d.Host == req.Host {
			httputil.RespondError(w, http.StatusConflict, "device already added")
			return
		}
	}

	devices = append(devices, hdhr.Device{Host: req.Host})
	devicesJSON, err := json.Marshal(devices)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to marshal devices")
		return
	}
	sc.Config["devices"] = string(devicesJSON)

	if err := s.deps.SourceConfigStore.Update(r.Context(), &sc); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update source")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		src, err := s.deps.SourceReg.Create(ctx, source.TypeHDHR, sc.ID)
		if err != nil {
			log.Printf("hdhr add device: failed to create source %s: %v", sc.ID, err)
			return
		}
		if err := src.Refresh(ctx); err != nil {
			log.Printf("hdhr add device: refresh failed for %s: %v", sc.ID, err)
		}
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
	if err != nil || sc == nil || sc.Type != string(source.TypeHDHR) {
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	var devices []hdhr.Device
	if devicesJSON := sc.Config["devices"]; devicesJSON != "" {
		if err := json.Unmarshal([]byte(devicesJSON), &devices); err != nil {
			log.Printf("hdhr devices: failed to unmarshal devices for source %s: %v", id, err)
			httputil.RespondError(w, http.StatusInternalServerError, "failed to parse devices")
			return
		}
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
	if err != nil || sc == nil || sc.Type != string(source.TypeHDHR) {
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		src, err := s.deps.SourceReg.Create(ctx, source.TypeHDHR, id)
		if err != nil {
			log.Printf("hdhr scan: failed to create source: %v", err)
			return
		}
		if err := src.Refresh(ctx); err != nil {
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
	if err != nil || sc == nil || sc.Type != string(source.TypeHDHR) {
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	hdhrRetuneMu.Lock()
	hdhrRetuneStatus[id] = source.RefreshStatus{State: source.StateScanning, Message: "Starting retune..."}
	hdhrRetuneMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		src, err := s.deps.SourceReg.Create(ctx, source.TypeHDHR, id)
		if err != nil {
			hdhrRetuneMu.Lock()
			hdhrRetuneStatus[id] = source.RefreshStatus{State: source.StateError, Message: err.Error()}
			hdhrRetuneMu.Unlock()
			return
		}

		retunable, ok := src.(source.Retunable)
		if !ok {
			hdhrRetuneMu.Lock()
			hdhrRetuneStatus[id] = source.RefreshStatus{State: source.StateError, Message: "source does not support retune"}
			hdhrRetuneMu.Unlock()
			return
		}

		if err := retunable.Retune(ctx); err != nil {
			hdhrRetuneMu.Lock()
			hdhrRetuneStatus[id] = source.RefreshStatus{State: source.StateError, Message: err.Error()}
			hdhrRetuneMu.Unlock()
			log.Printf("hdhr retune failed for %s: %v", id, err)
			return
		}

		info := src.Info(ctx)
		hdhrRetuneMu.Lock()
		hdhrRetuneStatus[id] = source.RefreshStatus{
			State:   source.StateDone,
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
		httputil.RespondJSON(w, http.StatusOK, source.RefreshStatus{State: source.StateIdle})
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

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(source.TypeHDHR), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to clear streams")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func hdhrDevicesFromConfig(sc *sourceconfig.SourceConfig) []hdhr.Device {
	var devices []hdhr.Device
	if devicesJSON := sc.Config["devices"]; devicesJSON != "" {
		if err := json.Unmarshal([]byte(devicesJSON), &devices); err != nil {
			log.Printf("hdhr: failed to unmarshal devices for source %s: %v", sc.ID, err)
		}
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
