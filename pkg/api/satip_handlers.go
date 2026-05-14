package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/source/satip/scan"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
)

var (
	satipScanMu     sync.RWMutex
	satipScanStatus = make(map[string]source.RefreshStatus)
	satipScanEvents = &scanBus{subs: make(map[string][]chan []byte)}
)

// scanBus is a per-source-ID pub/sub for SSE scan events.
type scanBus struct {
	mu   sync.RWMutex
	subs map[string][]chan []byte
}

func (b *scanBus) subscribe(id string) chan []byte {
	ch := make(chan []byte, 64)
	b.mu.Lock()
	b.subs[id] = append(b.subs[id], ch)
	b.mu.Unlock()
	return ch
}

func (b *scanBus) unsubscribe(id string, ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subs[id]
	for i, s := range subs {
		if s == ch {
			b.subs[id] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	close(ch)
}

func (b *scanBus) publish(id, eventType string, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	msg := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, payload))

	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs[id] {
		select {
		case ch <- msg:
		default: // drop if subscriber is slow
		}
	}
}

func (s *Server) handleCreateSatIPSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string `json:"name"`
		Host            string `json:"host"`
		HTTPPort        int    `json:"http_port"`
		TransmitterFile string `json:"transmitter_file"`
		IsEnabled       *bool  `json:"is_enabled"`
		MaxStreams      int    `json:"max_streams"`
		SourceProfileID string `json:"source_profile_id"`
		EPGSourceID     string `json:"epg_source_id"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, errInvalidBody)
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
		Type:      string(source.TypeSATIP),
		Name:      req.Name,
		IsEnabled: enabled,
		Config: map[string]string{
			"host":              req.Host,
			"http_port":         fmt.Sprintf("%d", httpPort),
			"transmitter_file":  req.TransmitterFile,
			"max_streams":       fmt.Sprintf("%d", req.MaxStreams),
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
	if existing == nil || existing.Type != string(source.TypeSATIP) {
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	var req struct {
		Name            *string `json:"name"`
		Host            *string `json:"host"`
		HTTPPort        *int    `json:"http_port"`
		TransmitterFile *string `json:"transmitter_file"`
		IsEnabled       *bool   `json:"is_enabled"`
		MaxStreams      *int    `json:"max_streams"`
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

func (s *Server) handleDeleteSatIPSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(source.TypeSATIP), id); err != nil {
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
	if err != nil || sc == nil || sc.Type != string(source.TypeSATIP) {
		httputil.RespondError(w, http.StatusNotFound, errSourceNotFound)
		return
	}

	satipScanMu.Lock()
	if st, ok := satipScanStatus[id]; ok && st.State == source.StateScanning {
		satipScanMu.Unlock()
		httputil.RespondError(w, http.StatusConflict, "scan already in progress")
		return
	}
	startStatus := source.RefreshStatus{State: source.StateScanning, Message: "Starting scan..."}
	satipScanStatus[id] = startStatus
	satipScanMu.Unlock()
	satipScanEvents.publish(id, "status", startStatus)

	go func() {
		src, err := s.deps.SourceReg.Create(context.Background(), source.TypeSATIP, id)
		if err != nil {
			errStatus := source.RefreshStatus{State: source.StateError, Message: err.Error()}
			satipScanMu.Lock()
			satipScanStatus[id] = errStatus
			satipScanMu.Unlock()
			satipScanEvents.publish(id, "error", errStatus)
			return
		}

		type progressSetter interface {
			SetScanProgress(fn func(done, total, channels int))
		}
		servicesSoFar := 0
		if ps, ok := src.(progressSetter); ok {
			ps.SetScanProgress(func(done, total, channels int) {
				servicesSoFar += channels
				status := source.RefreshStatus{
					State:    "scanning",
					Message:  fmt.Sprintf("Scanning mux %d/%d", done, total),
					Total:    total,
					Progress: done,
				}
				satipScanMu.Lock()
				satipScanStatus[id] = status
				satipScanMu.Unlock()
				satipScanEvents.publish(id, "progress", map[string]interface{}{
					"done":            done,
					"total":           total,
					"services_so_far": servicesSoFar,
				})
			})
		}

		if err := src.Refresh(context.Background()); err != nil {
			errStatus := source.RefreshStatus{State: source.StateError, Message: err.Error()}
			satipScanMu.Lock()
			satipScanStatus[id] = errStatus
			satipScanMu.Unlock()
			satipScanEvents.publish(id, "error", errStatus)
			log.Printf("satip scan failed for %s: %v", id, err)
			return
		}

		info := src.Info(context.Background())
		doneStatus := source.RefreshStatus{
			State:   source.StateDone,
			Message: fmt.Sprintf("Scan complete. %d streams found.", info.StreamCount),
		}
		satipScanMu.Lock()
		satipScanStatus[id] = doneStatus
		satipScanMu.Unlock()
		satipScanEvents.publish(id, "summary", map[string]interface{}{
			"state":          "done",
			"total_services": info.StreamCount,
			"message":        doneStatus.Message,
		})
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
		httputil.RespondJSON(w, http.StatusOK, source.RefreshStatus{State: source.StateIdle})
		return
	}

	httputil.RespondJSON(w, http.StatusOK, status)
}

func (s *Server) handleSatIPScanEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send current status immediately (for reconnects / late joiners)
	satipScanMu.RLock()
	current, exists := satipScanStatus[id]
	satipScanMu.RUnlock()
	if exists {
		payload, _ := json.Marshal(current)
		fmt.Fprintf(w, "event: status\ndata: %s\n\n", payload)
		flusher.Flush()
		// If scan already finished, send and close
		if current.State == source.StateDone || current.State == source.StateError {
			return
		}
	}

	ch := satipScanEvents.subscribe(id)
	defer satipScanEvents.unsubscribe(id, ch)

	for {
		select {
		case msg, open := <-ch:
			if !open {
				return
			}
			w.Write(msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func dvbTablesDir() string {
	return scan.DVBTablesDir
}

func (s *Server) handleListTransmitters(w http.ResponseWriter, r *http.Request) {
	system := r.URL.Query().Get("system")
	if system == "" {
		httputil.RespondError(w, http.StatusBadRequest, "system parameter required (e.g. dvb-t, dvb-s, dvb-c)")
		return
	}

	dir := filepath.Join(dvbTablesDir(), system)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			httputil.RespondJSON(w, http.StatusOK, []struct{}{})
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list transmitters: "+err.Error())
		return
	}

	type entry struct {
		Name string `json:"name"`
		File string `json:"file"`
	}
	var result []entry
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		result = append(result, entry{Name: n, File: system + "/" + n})
	}
	if result == nil {
		result = []entry{}
	}
	httputil.RespondJSON(w, http.StatusOK, result)
}

func (s *Server) handleListDVBSystems(w http.ResponseWriter, r *http.Request) {
	dir := dvbTablesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		httputil.RespondJSON(w, http.StatusOK, []string{})
		return
	}
	var systems []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			systems = append(systems, e.Name())
		}
	}
	sort.Strings(systems)
	httputil.RespondJSON(w, http.StatusOK, systems)
}

func (s *Server) handleSatIPClear(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	if err := s.deps.StreamStore.DeleteBySource(r.Context(), string(source.TypeSATIP), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to clear streams")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
