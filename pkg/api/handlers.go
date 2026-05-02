package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/mcnairstudios/mediahub/pkg/activity"
	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/channel"
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
	vodType := r.URL.Query().Get("vod_type")
	fields := r.URL.Query().Get("fields")

	var streams []media.Stream
	var err error

	if sourceType != "" && sourceID != "" && vodType != "" {
		streams, err = s.deps.StreamStore.ListBySourceAndType(r.Context(), sourceType, sourceID, vodType)
	} else if sourceType != "" && sourceID != "" {
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

	if fields == "slim" {
		httputil.RespondJSON(w, http.StatusOK, media.ToSlimStreams(streams))
		return
	}

	s.resolveStreamLogos(streams)
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
	channels = s.filterChannelsByUser(r, channels)
	httputil.RespondJSON(w, http.StatusOK, channels)
}

var apiSettableKeys = map[string]bool{
	"base_url":               true,
	"default_hwaccel":        true,
	"recording_video_codec":  true,
	"default_decode_hwaccel": true,
	"encoder_h264":           true,
	"encoder_h265":           true,
	"encoder_av1":            true,
	"decoder_h264":           true,
	"decoder_h265":           true,
	"decoder_av1":            true,
	"decoder_mpeg2":          true,
	"delivery":               true,
	"dlna_enabled":           true,
	"jellyfin_enabled":       true,
	"debug_enabled":          true,
	"tmdb_api_key":           true,
	"max_bit_depth":          true,
	"default_max_bit_depth":  true,
	"epg_channel_meta":       true,
	"google_client_id":       true,
	"google_client_secret":   true,
	"audio_language":         true,
	"subtitle_language":      true,
	"subprocess_transcode":   true,
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	all, err := s.deps.SettingsStore.List(r.Context())
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to get settings")
		return
	}
	filtered := make(map[string]string, len(apiSettableKeys))
	for k, v := range all {
		if apiSettableKeys[k] {
			filtered[k] = v
		}
	}
	httputil.RespondJSON(w, http.StatusOK, filtered)
}

var validHWAccelValues = map[string]bool{
	"":              true,
	"none":          true,
	"vaapi":         true,
	"qsv":           true,
	"nvenc":          true,
	"videotoolbox":  true,
}

var validVideoCodecValues = map[string]bool{
	"":     true,
	"h264": true,
	"h265": true,
	"hevc": true,
	"av1":  true,
	"copy": true,
}

var isoLangPattern = regexp.MustCompile(`^[a-zA-Z]{2,3}$`)

func validateSettingValue(key, value string) error {
	switch key {
	case "default_hwaccel", "default_decode_hwaccel":
		if !validHWAccelValues[value] {
			return fmt.Errorf("%s must be one of: none, vaapi, qsv, nvenc, videotoolbox, or empty", key)
		}
	case "default_max_bit_depth":
		if value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				return fmt.Errorf("default_max_bit_depth must be a positive integer")
			}
		}
	case "recording_video_codec":
		if !validVideoCodecValues[value] {
			return fmt.Errorf("recording_video_codec must be one of: h264, h265, hevc, av1, copy, or empty")
		}
	case "audio_language", "subtitle_language":
		if value != "" && !isoLangPattern.MatchString(value) {
			return fmt.Errorf("%s must be a 2-3 letter ISO language code", key)
		}
	}
	return nil
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings map[string]string
	if err := httputil.DecodeJSON(r, &settings); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for key, value := range settings {
		if !apiSettableKeys[key] {
			httputil.RespondError(w, http.StatusBadRequest, "unknown setting: "+key)
			return
		}
		if err := validateSettingValue(key, value); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.deps.SettingsStore.Set(r.Context(), key, value); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	if v, ok := settings["debug_enabled"]; ok {
		if v == "true" || v == "1" {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			log.Println("debug mode enabled via settings: log level set to debug")
		} else {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
			log.Println("debug mode disabled via settings: log level set to info")
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

	deps := s.playbackDeps()

	if profileName := r.URL.Query().Get("profile"); profileName != "" {
		if s.deps.ClientStore != nil {
			clients, _ := s.deps.ClientStore.List(r.Context())
			for _, c := range clients {
				if c.Name == profileName {
					deps.ClientOverrideID = c.ID
					break
				}
			}
		}
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
	if result.ProbeInfo != nil {
		probeMap := map[string]any{}
		if result.ProbeInfo.Video != nil {
			probeMap["video"] = map[string]any{
				"codec":      result.ProbeInfo.Video.Codec,
				"width":      result.ProbeInfo.Video.Width,
				"height":     result.ProbeInfo.Video.Height,
				"interlaced": result.ProbeInfo.Video.Interlaced,
				"bit_depth":  result.ProbeInfo.Video.BitDepth,
			}
		}
		if len(result.ProbeInfo.AudioTracks) > 0 {
			a := result.ProbeInfo.AudioTracks[0]
			probeMap["audio"] = map[string]any{
				"codec":       a.Codec,
				"channels":    a.Channels,
				"sample_rate": a.SampleRate,
				"language":    a.Language,
			}
		}
		if len(result.ProbeInfo.SubTracks) > 0 {
			subs := make([]map[string]any, 0, len(result.ProbeInfo.SubTracks))
			for _, st := range result.ProbeInfo.SubTracks {
				subs = append(subs, map[string]any{
					"index":    st.Index,
					"codec":    st.Codec,
					"language": st.Language,
				})
			}
			probeMap["subtitles"] = subs
		}
		resp["probe_info"] = probeMap
	}

	base := "/api/play/" + streamID
	switch result.Delivery {
	case "hls":
		endpoints := map[string]string{
			"playlist": base + "/hls/playlist.m3u8",
		}
		if sess := s.deps.SessionMgr.Get(streamID); sess != nil && sess.Subtitles != nil {
			endpoints["subtitles"] = base + "/subtitles"
		}
		resp["endpoints"] = endpoints
	case "mse":
		endpoints := map[string]string{
			"video_init":    base + "/mse/video/init",
			"audio_init":    base + "/mse/audio/init",
			"video_segment": base + "/mse/video/segment",
			"audio_segment": base + "/mse/audio/segment",
		}
		if sess := s.deps.SessionMgr.Get(streamID); sess != nil && sess.Subtitles != nil {
			endpoints["subtitles"] = base + "/subtitles"
		}
		resp["endpoints"] = endpoints
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

func (s *Server) handleSubtitles(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("streamID")
	if streamID == "" {
		http.NotFound(w, r)
		return
	}

	sess := s.deps.SessionMgr.Get(streamID)
	if sess == nil {
		http.NotFound(w, r)
		return
	}

	if sess.Subtitles == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Write(sess.Subtitles.WebVTT())
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

	deps := s.recordingDeps()

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

	deps := s.recordingDeps()

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
		SourceReg:         s.deps.SourceReg,
		SourceConfigStore: s.deps.SourceConfigStore,
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
		Email    string    `json:"email"`
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

	user, err := s.deps.AuthService.CreateUser(r.Context(), req.Username, req.Password, req.Email, req.Role)
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

func (s *Server) recordingDeps() orchestrator.RecordingDeps {
	deps := orchestrator.RecordingDeps{
		SessionMgr:     s.deps.SessionMgr,
		RecordingStore: s.deps.RecordingStore,
		OutputReg:      s.deps.OutputReg,
	}
	if s.deps.Config != nil {
		deps.RecordDir = s.deps.Config.RecordDir
	}
	return deps
}

func (s *Server) playbackDeps() orchestrator.PlaybackDeps {
	return orchestrator.PlaybackDeps{
		StreamStore:       s.deps.StreamStore,
		SettingsStore:     s.deps.SettingsStore,
		SourceConfigStore: s.deps.SourceConfigStore,
		ConnRegistry:      s.deps.ConnRegistry,
		WGService:         s.deps.WGService,
		SessionMgr:        s.deps.SessionMgr,
		Detector:          s.deps.Detector,
		ClientStore:       s.deps.ClientStore,
		OutputReg:         s.deps.OutputReg,
		Strategy:          s.deps.Strategy,
		ProbeCache:        s.deps.ProbeCache,
		UserAgent:         s.deps.UserAgent,
	}
}

func (s *Server) filterChannelsByUser(r *http.Request, channels []channel.Channel) []channel.Channel {
	user := middleware.UserFromContext(r.Context())
	if user == nil || user.IsAdmin || len(user.ChannelGroupIDs) == 0 {
		return channels
	}
	allowed := make(map[string]bool, len(user.ChannelGroupIDs))
	for _, gid := range user.ChannelGroupIDs {
		allowed[gid] = true
	}
	filtered := make([]channel.Channel, 0, len(channels))
	for _, ch := range channels {
		if allowed[ch.GroupID] {
			filtered = append(filtered, ch)
		}
	}
	return filtered
}


func (s *Server) resolveStreamLogos(streams []media.Stream) {
	if s.deps.LogoCache == nil {
		return
	}
	for i := range streams {
		if streams[i].TvgLogo != "" {
			streams[i].TvgLogo = s.deps.LogoCache.Resolve(streams[i].TvgLogo)
		}
	}
}
