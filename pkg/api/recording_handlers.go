package api

import (
	"net/http"
	"os"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/activity"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/middleware"
	"github.com/mcnairstudios/mediahub/pkg/orchestrator"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/recording"
)

func (s *Server) handleGetRecording(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "recording ID required")
		return
	}

	rec, err := s.deps.RecordingStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get recording")
		return
	}
	if rec == nil {
		httputil.RespondError(w, http.StatusNotFound, "recording not found")
		return
	}

	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !user.IsAdmin && rec.UserID != user.ID {
		httputil.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	resp := recordingResponse(rec)

	if rec.FilePath != "" {
		if fi, err := os.Stat(rec.FilePath); err == nil {
			resp["file_size"] = fi.Size()
			resp["file_exists"] = true
		} else {
			resp["file_exists"] = false
		}
	}

	httputil.RespondJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePlayRecording(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "recording ID required")
		return
	}

	rec, err := s.deps.RecordingStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get recording")
		return
	}
	if rec == nil {
		httputil.RespondError(w, http.StatusNotFound, "recording not found")
		return
	}

	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !user.IsAdmin && rec.UserID != user.ID {
		httputil.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	if rec.Status != recording.StatusCompleted {
		httputil.RespondError(w, http.StatusConflict, "recording is not completed")
		return
	}

	if rec.FilePath == "" {
		httputil.RespondError(w, http.StatusNotFound, "recording has no file path")
		return
	}

	if _, err := os.Stat(rec.FilePath); os.IsNotExist(err) {
		httputil.RespondError(w, http.StatusNotFound, "recording file not found on disk")
		return
	}

	deps := orchestrator.PlaybackDeps{
		StreamStore:   s.deps.StreamStore,
		SettingsStore: s.deps.SettingsStore,
		SessionMgr:    s.deps.SessionMgr,
		Detector:      s.deps.Detector,
		OutputReg:     s.deps.OutputReg,
		Strategy:      s.deps.Strategy,
		UserAgent:     s.deps.UserAgent,
	}

	title := rec.Title
	if title == "" {
		title = rec.StreamName
	}

	result, err := orchestrator.PlayRecording(r.Context(), deps, rec.ID, rec.FilePath, title)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.deps.Activity != nil {
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
		"session_id":   result.Session.ID,
		"recording_id": rec.ID,
		"is_new":       result.IsNew,
		"delivery":     result.Delivery,
		"is_live":      false,
	}

	if result.ProbeInfo != nil && result.ProbeInfo.DurationMs > 0 {
		resp["duration_ms"] = result.ProbeInfo.DurationMs
	}

	base := "/api/recordings/completed/" + rec.ID + "/play"
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

func (s *Server) handleStopRecordingPlayback(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "recording ID required")
		return
	}

	sessionKey := "rec:" + id
	if s.deps.Activity != nil {
		if sess := s.deps.SessionMgr.Get(sessionKey); sess != nil {
			s.deps.Activity.Remove(sess.ID)
		}
	}

	deps := orchestrator.PlaybackDeps{
		SessionMgr: s.deps.SessionMgr,
	}
	orchestrator.StopRecordingPlayback(deps, id)

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStreamRecording(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "recording ID required")
		return
	}

	rec, err := s.deps.RecordingStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get recording")
		return
	}
	if rec == nil {
		httputil.RespondError(w, http.StatusNotFound, "recording not found")
		return
	}

	user := middleware.UserFromContext(r.Context())
	if user == nil {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !user.IsAdmin && rec.UserID != user.ID {
		httputil.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	if rec.FilePath == "" {
		httputil.RespondError(w, http.StatusNotFound, "recording has no file")
		return
	}

	http.ServeFile(w, r, rec.FilePath)
}

func (s *Server) handleDeleteRecording(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "recording ID required")
		return
	}

	rec, err := s.deps.RecordingStore.Get(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get recording")
		return
	}
	if rec == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if rec.FilePath != "" {
		os.Remove(rec.FilePath)
	}

	if err := s.deps.RecordingStore.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to delete recording")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSeekRecordingPlayback(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, "recording ID required")
		return
	}

	var req struct {
		PositionMs int64 `json:"position_ms"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sessionKey := "rec:" + id
	deps := orchestrator.PlaybackDeps{
		SessionMgr: s.deps.SessionMgr,
	}
	if err := orchestrator.Seek(deps, sessionKey, req.PositionMs); err != nil {
		httputil.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRecordingPlaybackServe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	rest := r.PathValue("path")
	if rest == "" {
		http.NotFound(w, r)
		return
	}

	sessionKey := "rec:" + id
	sess := s.deps.SessionMgr.Get(sessionKey)
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

func recordingResponse(rec *recording.Recording) map[string]any {
	resp := map[string]any{
		"id":          rec.ID,
		"stream_id":   rec.StreamID,
		"stream_name": rec.StreamName,
		"channel_id":  rec.ChannelID,
		"channel_name": rec.ChannelName,
		"title":       rec.Title,
		"user_id":     rec.UserID,
		"status":      rec.Status,
		"file_path":   rec.FilePath,
		"file_size":   rec.FileSize,
		"container":   rec.Container,
		"video_codec": rec.VideoCodec,
		"audio_codec": rec.AudioCodec,
	}
	if !rec.StartedAt.IsZero() {
		resp["started_at"] = rec.StartedAt.Format(time.RFC3339)
	}
	if !rec.StoppedAt.IsZero() {
		resp["stopped_at"] = rec.StoppedAt.Format(time.RFC3339)
	}
	if !rec.ScheduledStart.IsZero() {
		resp["scheduled_start"] = rec.ScheduledStart.Format(time.RFC3339)
	}
	if !rec.ScheduledStop.IsZero() {
		resp["scheduled_stop"] = rec.ScheduledStop.Format(time.RFC3339)
	}
	if !rec.StartedAt.IsZero() && !rec.StoppedAt.IsZero() {
		resp["duration_sec"] = int(rec.StoppedAt.Sub(rec.StartedAt).Seconds())
	}
	return resp
}
