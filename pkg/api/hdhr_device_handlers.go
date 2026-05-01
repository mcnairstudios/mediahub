package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"net/http"
	"sort"

	"github.com/google/uuid"
	"github.com/mcnairstudios/mediahub/pkg/frontend/hdhr"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

func (s *Server) handleListHDHRDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.deps.HDHRDeviceStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}
	if devices == nil {
		devices = []hdhr.Device{}
	}
	httputil.RespondJSON(w, http.StatusOK, devices)
}

func (s *Server) handleGetHDHRDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "device ID required")
		return
	}

	device, err := s.deps.HDHRDeviceStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get device")
		return
	}
	if device == nil {
		httputil.RespondError(w, http.StatusNotFound, "device not found")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, device)
}

func (s *Server) handleCreateHDHRDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Port        int      `json:"port"`
		GroupIDs    []string `json:"group_ids"`
		IsEnabled   *bool    `json:"is_enabled"`
		MaxChannels *int     `json:"max_channels"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name required")
		return
	}
	if req.Port <= 0 || req.Port > 65535 {
		httputil.RespondError(w, http.StatusBadRequest, "valid port required")
		return
	}

	if err := s.checkPortConflict(r.Context(), "", req.Port); err != nil {
		httputil.RespondError(w, http.StatusConflict, err.Error())
		return
	}

	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}

	maxChannels := hdhr.DefaultMaxChannels
	if req.MaxChannels != nil && *req.MaxChannels > 0 {
		maxChannels = *req.MaxChannels
	}

	device := &hdhr.Device{
		ID:          uuid.New().String(),
		Name:        req.Name,
		DeviceUUID:  generateDeviceUUID(),
		Port:        req.Port,
		GroupIDs:    req.GroupIDs,
		IsEnabled:   enabled,
		MaxChannels: maxChannels,
	}

	if err := s.deps.HDHRDeviceStore.Create(r.Context(), device); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create device")
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, device)
}

func (s *Server) handleUpdateHDHRDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "device ID required")
		return
	}

	existing, err := s.deps.HDHRDeviceStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get device")
		return
	}
	if existing == nil {
		httputil.RespondError(w, http.StatusNotFound, "device not found")
		return
	}

	var req struct {
		Name        *string  `json:"name"`
		Port        *int     `json:"port"`
		GroupIDs    []string `json:"group_ids"`
		IsEnabled   *bool    `json:"is_enabled"`
		MaxChannels *int     `json:"max_channels"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Port != nil {
		if *req.Port <= 0 || *req.Port > 65535 {
			httputil.RespondError(w, http.StatusBadRequest, "valid port required")
			return
		}
		if err := s.checkPortConflict(r.Context(), id, *req.Port); err != nil {
			httputil.RespondError(w, http.StatusConflict, err.Error())
			return
		}
		existing.Port = *req.Port
	}
	if req.GroupIDs != nil {
		existing.GroupIDs = req.GroupIDs
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}
	if req.MaxChannels != nil && *req.MaxChannels > 0 {
		existing.MaxChannels = *req.MaxChannels
	}

	if err := s.deps.HDHRDeviceStore.Update(r.Context(), existing); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update device")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteHDHRDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "device ID required")
		return
	}

	if err := s.deps.HDHRDeviceStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete device")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAutoSplitHDHRDevices(w http.ResponseWriter, r *http.Request) {
	channels, err := s.deps.ChannelStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}

	var enabled int
	for _, ch := range channels {
		if ch.IsEnabled {
			enabled++
		}
	}

	if enabled == 0 {
		httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "no enabled channels"})
		return
	}

	existing, err := s.deps.HDHRDeviceStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}

	devicesNeeded := int(math.Ceil(float64(enabled) / float64(hdhr.DefaultMaxChannels)))
	if devicesNeeded <= len(existing) {
		httputil.RespondJSON(w, http.StatusOK, map[string]any{
			"message":         "sufficient devices exist",
			"channels":        enabled,
			"devices_needed":  devicesNeeded,
			"devices_current": len(existing),
		})
		return
	}

	groups, err := s.deps.GroupStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list groups")
		return
	}

	channelsByGroup := make(map[string]int)
	for _, ch := range channels {
		if ch.IsEnabled {
			channelsByGroup[ch.GroupID]++
		}
	}

	type groupInfo struct {
		id    string
		count int
	}
	var sortedGroups []groupInfo
	for gid, count := range channelsByGroup {
		sortedGroups = append(sortedGroups, groupInfo{gid, count})
	}
	sort.Slice(sortedGroups, func(i, j int) bool {
		return sortedGroups[i].count > sortedGroups[j].count
	})

	_ = groups

	usedPorts := make(map[int]bool)
	for _, d := range existing {
		usedPorts[d.Port] = true
	}

	var deviceBuckets [][]string
	var currentBucket []string
	currentCount := 0
	for _, g := range sortedGroups {
		if currentCount+g.count > hdhr.DefaultMaxChannels && len(currentBucket) > 0 {
			deviceBuckets = append(deviceBuckets, currentBucket)
			currentBucket = nil
			currentCount = 0
		}
		currentBucket = append(currentBucket, g.id)
		currentCount += g.count
	}
	if len(currentBucket) > 0 {
		deviceBuckets = append(deviceBuckets, currentBucket)
	}

	nextPort := 5004
	var created []hdhr.Device
	for i, bucket := range deviceBuckets {
		if i < len(existing) {
			continue
		}

		for usedPorts[nextPort] {
			nextPort++
		}

		device := &hdhr.Device{
			ID:          uuid.New().String(),
			Name:        fmt.Sprintf("MediaHub HDHR %d", i+1),
			DeviceUUID:  generateDeviceUUID(),
			Port:        nextPort,
			GroupIDs:    bucket,
			IsEnabled:   true,
			MaxChannels: hdhr.DefaultMaxChannels,
		}

		if err := s.deps.HDHRDeviceStore.Create(r.Context(), device); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to create device")
			return
		}

		created = append(created, *device)
		usedPorts[nextPort] = true
		nextPort++
	}

	httputil.RespondJSON(w, http.StatusCreated, map[string]any{
		"message":         fmt.Sprintf("created %d devices", len(created)),
		"channels":        enabled,
		"devices_created": len(created),
		"devices":         created,
	})
}

func (s *Server) checkPortConflict(ctx context.Context, excludeID string, port int) error {
	devices, err := s.deps.HDHRDeviceStore.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to check port: %w", err)
	}
	for _, d := range devices {
		if d.ID != excludeID && d.Port == port {
			return fmt.Errorf("port %d already in use by device %q", port, d.Name)
		}
	}
	return nil
}

func generateDeviceUUID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%08X", b)
}
