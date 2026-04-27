package api

import (
	"net/http"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/middleware"
	"github.com/mcnairstudios/mediahub/pkg/orchestrator"
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

	httputil.RespondJSON(w, http.StatusOK, map[string]any{"access_token": token})
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

	httputil.RespondJSON(w, http.StatusOK, map[string]any{"access_token": newToken})
}

func (s *Server) handleListStreams(w http.ResponseWriter, r *http.Request) {
	streams, err := s.deps.StreamStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list streams")
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
	httputil.RespondJSON(w, http.StatusOK, recordings)
}

func (s *Server) handleStartPlayback(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("streamID")
	if streamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "stream ID required")
		return
	}

	deps := orchestrator.PlaybackDeps{
		StreamStore: s.deps.StreamStore,
		SessionMgr:  s.deps.SessionMgr,
		Detector:    s.deps.Detector,
		OutputReg:   s.deps.OutputReg,
		Strategy:    s.deps.Strategy,
	}

	headers := make(map[string]string)
	for key := range r.Header {
		headers[key] = r.Header.Get(key)
	}

	result, err := orchestrator.StartPlayback(r.Context(), deps, streamID, 0, headers)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"session_id": result.Session.ID,
		"stream_id":  result.Session.StreamID,
		"is_new":     result.IsNew,
		"decision": map[string]any{
			"needs_transcode":       result.Decision.NeedsTranscode,
			"needs_audio_transcode": result.Decision.NeedsAudioTranscode,
			"video_codec":           result.Decision.VideoCodec,
			"audio_codec":           result.Decision.AudioCodec,
			"container":             result.Decision.Container,
		},
	})
}

func (s *Server) handleStopPlayback(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("streamID")
	if streamID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "stream ID required")
		return
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

	var req struct {
		SourceType string `json:"source_type"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	deps := orchestrator.RefreshDeps{
		SourceReg: s.deps.SourceReg,
	}

	if err := orchestrator.RefreshSource(r.Context(), deps, source.SourceType(req.SourceType), sourceID); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.deps.AuthService.ListUsers(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list users")
		return
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
