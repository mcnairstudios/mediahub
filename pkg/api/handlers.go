package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/activity"
	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/middleware"
	"github.com/mcnairstudios/mediahub/pkg/orchestrator"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/source"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		httputil.RespondError(w, http.StatusBadRequest, "username and password required")
		return
	}

	token, err := s.deps.AuthService.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		httputil.RespondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	resp := map[string]any{"access_token": token}

	user, valErr := s.deps.AuthService.ValidateToken(r.Context(), token)
	if valErr == nil && user != nil {
		if jwtSvc, ok := s.deps.AuthService.(*auth.JWTService); ok {
			if refreshToken, rtErr := jwtSvc.GenerateRefreshToken(user); rtErr == nil {
				resp["refresh_token"] = refreshToken
			}
		}
	}

	httputil.RespondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		httputil.RespondError(w, http.StatusBadRequest, "token required")
		return
	}

	newToken, err := s.deps.AuthService.RefreshToken(r.Context(), req.Token)
	if err != nil {
		httputil.RespondError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	resp := map[string]any{"access_token": newToken}

	user, valErr := s.deps.AuthService.ValidateToken(r.Context(), newToken)
	if valErr == nil && user != nil {
		if jwtSvc, ok := s.deps.AuthService.(*auth.JWTService); ok {
			if refreshToken, rtErr := jwtSvc.GenerateRefreshToken(user); rtErr == nil {
				resp["refresh_token"] = refreshToken
			}
		}
	}

	httputil.RespondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListStreams(w http.ResponseWriter, r *http.Request) {
	sourceType := r.URL.Query().Get("source_type")
	sourceID := r.URL.Query().Get("source_id")

	var streams []media.Stream
	var err error

	if sourceType != "" && sourceID != "" {
		streams, err = s.deps.StreamStore.ListBySource(r.Context(), sourceType, sourceID)
	} else {
		streams, err = s.deps.StreamStore.List(r.Context())
	}

	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list streams")
		return
	}
	if streams == nil {
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}
	httputil.RespondJSON(w, http.StatusOK, streams)
}

func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := s.deps.ChannelStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}
	if channels == nil {
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}
	httputil.RespondJSON(w, http.StatusOK, channels)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.deps.SettingsStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get settings")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, settings)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings map[string]string
	if err := httputil.DecodeJSON(r, &settings); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for key, value := range settings {
		if err := s.deps.SettingsStore.Set(r.Context(), key, value); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	httputil.RespondJSON(w, http.StatusOK, settings)
}

func (s *Server) handleListEPGSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.deps.EPGSourceStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list EPG sources")
		return
	}
	if sources == nil {
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}
	httputil.RespondJSON(w, http.StatusOK, sources)
}

func (s *Server) handleListRecordings(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	recordings, err := s.deps.RecordingStore.List(r.Context(), user.ID, user.IsAdmin)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list recordings")
		return
	}
	if recordings == nil {
		recordings = []recording.Recording{}
	}
	httputil.RespondJSON(w, http.StatusOK, recordings)
}

func (s *Server) handleStartPlayback(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("streamID")
	if streamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "stream ID required")
		return
	}

	deps := orchestrator.PlaybackDeps{
		StreamStore:       s.deps.StreamStore,
		SettingsStore:     s.deps.SettingsStore,
		SourceConfigStore: s.deps.SourceConfigStore,
		ConnRegistry:      s.deps.ConnRegistry,
		SessionMgr:        s.deps.SessionMgr,
		Detector:          s.deps.Detector,
		OutputReg:         s.deps.OutputReg,
		Strategy:          s.deps.Strategy,
	}

	headers := make(map[string]string)
	for key := range r.Header {
		headers[key] = r.Header.Get(key)
	}

	result, err := orchestrator.StartPlayback(r.Context(), deps, streamID, 0, headers)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httputil.RespondError(w, http.StatusNotFound, err.Error())
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if s.deps.Activity != nil {
		user := middleware.UserFromContext(r.Context())
		v := &activity.Viewer{
			SessionID:  result.Session.ID,
			StreamID:   result.Session.StreamID,
			StreamName: result.Session.StreamName,
			Delivery:   result.Delivery,
			StartedAt:  time.Now(),
			RemoteAddr: r.RemoteAddr,
		}
		if user != nil {
			v.UserID = user.ID
			v.Username = user.Username
		}
		s.deps.Activity.Add(v)
	}

	resp := map[string]any{
		"session_id": result.Session.ID,
		"stream_id":  result.Session.StreamID,
		"is_new":     result.IsNew,
		"delivery":   result.Delivery,
		"decision": map[string]any{
			"needs_transcode":       result.Decision.NeedsTranscode,
			"needs_audio_transcode": result.Decision.NeedsAudioTranscode,
			"video_codec":           result.Decision.VideoCodec,
			"audio_codec":           result.Decision.AudioCodec,
			"container":             result.Decision.Container,
		},
	}

	base := "/api/play/" + streamID
	switch result.Delivery {
	case "hls":
		resp["endpoints"] = map[string]string{
			"playlist": base + "/hls/playlist.m3u8",
		}
	case "mse":
		resp["endpoints"] = map[string]string{
			"video_init":    base + "/mse/video/init",
			"audio_init":    base + "/mse/audio/init",
			"video_segment": base + "/mse/video/segment",
			"audio_segment": base + "/mse/audio/segment",
		}
	}

	httputil.RespondJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePlaybackServe(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("streamID")
	if streamID == "" {
		http.NotFound(w, r)
		return
	}

	rest := r.PathValue("path")
	if rest == "" {
		http.NotFound(w, r)
		return
	}

	sess := s.deps.SessionMgr.Get(streamID)
	if sess == nil {
		http.NotFound(w, r)
		return
	}

	plugins := sess.FanOut.Plugins()
	for _, p := range plugins {
		sp, ok := p.(output.ServablePlugin)
		if !ok {
			continue
		}
		r.URL.Path = "/" + rest
		sp.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleStopPlayback(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("streamID")
	if streamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "stream ID required")
		return
	}

	if s.deps.Activity != nil {
		if sess := s.deps.SessionMgr.Get(streamID); sess != nil {
			s.deps.Activity.Remove(sess.ID)
		}
	}

	deps := orchestrator.PlaybackDeps{
		SessionMgr: s.deps.SessionMgr,
	}
	orchestrator.StopPlayback(deps, streamID)

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSeek(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("streamID")
	if streamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "stream ID required")
		return
	}

	var req struct {
		PositionMs int64 `json:"position_ms"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	deps := orchestrator.PlaybackDeps{
		SessionMgr: s.deps.SessionMgr,
	}
	if err := orchestrator.Seek(deps, streamID, req.PositionMs); err != nil {
		httputil.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStartRecording(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("streamID")
	if streamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "stream ID required")
		return
	}

	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	deps := orchestrator.RecordingDeps{
		SessionMgr:     s.deps.SessionMgr,
		RecordingStore: s.deps.RecordingStore,
		OutputReg:      s.deps.OutputReg,
	}

	if err := orchestrator.StartRecording(r.Context(), deps, streamID, req.Title, user.ID, false); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStopRecording(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("streamID")
	if streamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "stream ID required")
		return
	}

	deps := orchestrator.RecordingDeps{
		SessionMgr:     s.deps.SessionMgr,
		RecordingStore: s.deps.RecordingStore,
		OutputReg:      s.deps.OutputReg,
	}

	if err := orchestrator.StopRecording(r.Context(), deps, streamID); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRefreshSource(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("sourceID")
	if sourceID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source ID required")
		return
	}

	var sourceType string
	if s.deps.SourceConfigStore != nil {
		cfg, err := s.deps.SourceConfigStore.Get(r.Context(), sourceID)
		if err == nil && cfg != nil {
			sourceType = cfg.Type
		}
	}

	if sourceType == "" {
		httputil.RespondError(w, http.StatusNotFound, "source not found")
		return
	}

	deps := orchestrator.RefreshDeps{
		SourceReg: s.deps.SourceReg,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("source refresh PANIC: %s (%s): %v", sourceID, sourceType, r)
			}
		}()
		if err := orchestrator.RefreshSource(context.Background(), deps, source.SourceType(sourceType), sourceID); err != nil {
			log.Printf("source refresh failed: %s (%s): %v", sourceID, sourceType, err)
		} else {
			log.Printf("source refresh completed: %s (%s)", sourceID, sourceType)
		}
	}()

	httputil.RespondJSON(w, http.StatusAccepted, map[string]string{"status": "refreshing"})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.deps.AuthService.ListUsers(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	if users == nil {
		users = []*auth.User{}
	}
	httputil.RespondJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string    `json:"username"`
		Password string    `json:"password"`
		Role     auth.Role `json:"role"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		httputil.RespondError(w, http.StatusBadRequest, "username and password required")
		return
	}
	if req.Role == "" {
		req.Role = auth.RoleStandard
	}

	user, err := s.deps.AuthService.CreateUser(r.Context(), req.Username, req.Password, req.Role)
	if err != nil {
		httputil.RespondError(w, http.StatusConflict, err.Error())
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, user)
}

func (s *Server) handleListActivity(w http.ResponseWriter, r *http.Request) {
	if s.deps.Activity == nil {
		httputil.RespondJSON(w, http.StatusOK, []any{})
		return
	}

	viewers := s.deps.Activity.List()
	now := time.Now()
	result := make([]map[string]any, 0, len(viewers))
	for _, v := range viewers {
		result = append(result, map[string]any{
			"session_id":  v.SessionID,
			"stream_id":   v.StreamID,
			"stream_name": v.StreamName,
			"user_id":     v.UserID,
			"username":    v.Username,
			"client_name": v.ClientName,
			"delivery":    v.Delivery,
			"started_at":  v.StartedAt.Format(time.RFC3339),
			"duration":    now.Sub(v.StartedAt).Truncate(time.Second).String(),
			"remote_addr": v.RemoteAddr,
		})
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}
