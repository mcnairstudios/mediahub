package api

import (
	"io/fs"
	"net/http"
	"strings"
)

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/refresh", s.handleRefreshToken)

	s.mux.Handle("GET /api/streams", s.authenticated(s.handleListStreams))
	s.mux.Handle("GET /api/channels", s.authenticated(s.handleListChannels))
	s.mux.Handle("GET /api/settings", s.authenticated(s.handleGetSettings))
	s.mux.Handle("PUT /api/settings", s.adminOnly(s.handleUpdateSettings))
	s.mux.Handle("GET /api/epg/sources", s.authenticated(s.handleListEPGSources))
	s.mux.Handle("POST /api/epg/sources", s.adminOnly(s.handleCreateEPGSource))
	s.mux.Handle("PUT /api/epg/sources/{id}", s.adminOnly(s.handleUpdateEPGSource))
	s.mux.Handle("DELETE /api/epg/sources/{id}", s.adminOnly(s.handleDeleteEPGSource))
	s.mux.Handle("POST /api/epg/sources/{id}/refresh", s.adminOnly(s.handleRefreshEPGSource))
	s.mux.Handle("GET /api/epg/now", s.authenticated(s.handleEPGNow))
	s.mux.Handle("GET /api/epg/programs", s.authenticated(s.handleEPGPrograms))
	s.mux.Handle("GET /api/dashboard/stats", s.authenticated(s.handleDashboardStats))
	s.mux.Handle("GET /api/recordings", s.authenticated(s.handleListRecordings))
	s.mux.Handle("GET /api/recordings/completed/{id}", s.authenticated(s.handleGetRecording))
	s.mux.Handle("POST /api/recordings/completed/{id}/play", s.authenticated(s.handlePlayRecording))
	s.mux.Handle("DELETE /api/recordings/completed/{id}/play", s.authenticated(s.handleStopRecordingPlayback))
	s.mux.Handle("POST /api/recordings/completed/{id}/seek", s.authenticated(s.handleSeekRecordingPlayback))
	s.mux.Handle("GET /api/recordings/completed/{id}/stream", s.authenticated(s.handleStreamRecording))
	s.mux.Handle("GET /api/recordings/completed/{id}/play/hls/{path...}", s.authenticated(s.handleRecordingPlaybackServe))
	s.mux.Handle("GET /api/recordings/completed/{id}/play/mse/{path...}", s.authenticated(s.handleRecordingPlaybackServe))
	s.mux.Handle("DELETE /api/recordings/completed/{id}", s.adminOnly(s.handleDeleteRecording))

	s.mux.Handle("POST /api/channels", s.adminOnly(s.handleCreateChannel))
	s.mux.Handle("PUT /api/channels/{id}", s.adminOnly(s.handleUpdateChannel))
	s.mux.Handle("DELETE /api/channels/{id}", s.adminOnly(s.handleDeleteChannel))
	s.mux.Handle("POST /api/channels/{id}/streams", s.adminOnly(s.handleAssignStreams))
	s.mux.Handle("GET /api/channel-groups", s.authenticated(s.handleListGroups))
	s.mux.Handle("POST /api/channel-groups", s.adminOnly(s.handleCreateGroup))
	s.mux.Handle("DELETE /api/channel-groups/{id}", s.adminOnly(s.handleDeleteGroup))

	s.mux.Handle("POST /api/play/{streamID}", s.authenticated(s.handleStartPlayback))
	s.mux.Handle("DELETE /api/play/{streamID}", s.authenticated(s.handleStopPlayback))
	s.mux.Handle("POST /api/play/{streamID}/seek", s.authenticated(s.handleSeek))
	s.mux.Handle("POST /api/play/{streamID}/record", s.authenticated(s.handleStartRecording))
	s.mux.Handle("DELETE /api/play/{streamID}/record", s.authenticated(s.handleStopRecording))
	s.mux.Handle("GET /api/play/{streamID}/hls/{path...}", s.authenticated(s.handlePlaybackServe))
	s.mux.Handle("GET /api/play/{streamID}/mse/{path...}", s.authenticated(s.handlePlaybackServe))

	s.mux.Handle("GET /api/sources", s.authenticated(s.handleListSources))
	s.mux.Handle("POST /api/sources/m3u", s.adminOnly(s.handleCreateM3USource))
	s.mux.Handle("PUT /api/sources/m3u/{id}", s.adminOnly(s.handleUpdateM3USource))
	s.mux.Handle("DELETE /api/sources/m3u/{id}", s.adminOnly(s.handleDeleteM3USource))
	s.mux.Handle("POST /api/sources/tvpstreams", s.adminOnly(s.handleCreateTVPStreamsSource))
	s.mux.Handle("PUT /api/sources/tvpstreams/{id}", s.adminOnly(s.handleUpdateTVPStreamsSource))
	s.mux.Handle("DELETE /api/sources/tvpstreams/{id}", s.adminOnly(s.handleDeleteTVPStreamsSource))
	s.mux.Handle("GET /api/sources/tvpstreams/{id}/tls", s.authenticated(s.handleTVPStreamsTLSStatus))
	s.mux.Handle("POST /api/sources/xtream", s.adminOnly(s.handleCreateXtreamSource))
	s.mux.Handle("PUT /api/sources/xtream/{id}", s.adminOnly(s.handleUpdateXtreamSource))
	s.mux.Handle("DELETE /api/sources/xtream/{id}", s.adminOnly(s.handleDeleteXtreamSource))
	s.mux.Handle("GET /api/sources/xtream/{id}/info", s.adminOnly(s.handleXtreamAccountInfo))
	s.mux.Handle("POST /api/sources/hdhr", s.adminOnly(s.handleCreateHDHRSource))
	s.mux.Handle("PUT /api/sources/hdhr/{id}", s.adminOnly(s.handleUpdateHDHRSource))
	s.mux.Handle("DELETE /api/sources/hdhr/{id}", s.adminOnly(s.handleDeleteHDHRSource))
	s.mux.Handle("POST /api/sources/hdhr/discover", s.adminOnly(s.handleHDHRDiscover))
	s.mux.Handle("POST /api/sources/hdhr/add-device", s.adminOnly(s.handleHDHRAddDevice))
	s.mux.Handle("GET /api/sources/hdhr/{id}/devices", s.adminOnly(s.handleHDHRDevices))
	s.mux.Handle("POST /api/sources/hdhr/{id}/scan", s.adminOnly(s.handleHDHRScan))
	s.mux.Handle("POST /api/sources/hdhr/{id}/retune", s.adminOnly(s.handleHDHRRetune))
	s.mux.Handle("GET /api/sources/hdhr/{id}/status", s.authenticated(s.handleHDHRRetuneStatus))
	s.mux.Handle("POST /api/sources/hdhr/{id}/clear", s.adminOnly(s.handleHDHRClear))
	s.mux.Handle("POST /api/sources/satip", s.adminOnly(s.handleCreateSatIPSource))
	s.mux.Handle("PUT /api/sources/satip/{id}", s.adminOnly(s.handleUpdateSatIPSource))
	s.mux.Handle("DELETE /api/sources/satip/{id}", s.adminOnly(s.handleDeleteSatIPSource))
	s.mux.Handle("POST /api/sources/satip/{id}/scan", s.adminOnly(s.handleSatIPScan))
	s.mux.Handle("GET /api/sources/satip/{id}/status", s.authenticated(s.handleSatIPScanStatus))
	s.mux.Handle("POST /api/sources/satip/{id}/clear", s.adminOnly(s.handleSatIPClear))
	s.mux.Handle("POST /api/sources/{sourceID}/refresh", s.adminOnly(s.handleRefreshSource))
	s.mux.Handle("GET /api/sources/{sourceID}/status", s.authenticated(s.handleSourceStatus))
	s.mux.Handle("GET /api/users", s.adminOnly(s.handleListUsers))
	s.mux.Handle("POST /api/users", s.adminOnly(s.handleCreateUser))
	s.mux.Handle("PUT /api/users/{id}", s.adminOnly(s.handleUpdateUser))
	s.mux.Handle("DELETE /api/users/{id}", s.adminOnly(s.handleDeleteUser))
	s.mux.Handle("PUT /api/users/{id}/password", s.authenticated(s.handleChangePassword))

	s.mux.Handle("GET /api/wireguard/profiles", s.adminOnly(s.handleListWGProfiles))
	s.mux.Handle("POST /api/wireguard/profiles", s.adminOnly(s.handleCreateWGProfile))
	s.mux.Handle("PUT /api/wireguard/profiles/{id}", s.adminOnly(s.handleUpdateWGProfile))
	s.mux.Handle("DELETE /api/wireguard/profiles/{id}", s.adminOnly(s.handleDeleteWGProfile))
	s.mux.Handle("POST /api/wireguard/profiles/{id}/activate", s.adminOnly(s.handleActivateWGProfile))
	s.mux.Handle("POST /api/wireguard/profiles/{id}/test", s.adminOnly(s.handleTestWGProfile))
	s.mux.Handle("GET /api/wireguard/status", s.adminOnly(s.handleWGStatus))

	s.mux.Handle("POST /api/recordings/schedule", s.authenticated(s.handleScheduleRecording))
	s.mux.Handle("GET /api/recordings/schedule", s.authenticated(s.handleListScheduledRecordings))
	s.mux.Handle("DELETE /api/recordings/schedule/{id}", s.authenticated(s.handleCancelScheduledRecording))

	s.mux.Handle("GET /api/favorites", s.authenticated(s.handleListFavorites))
	s.mux.Handle("POST /api/favorites", s.authenticated(s.handleAddFavorite))
	s.mux.Handle("DELETE /api/favorites/{streamID}", s.authenticated(s.handleRemoveFavorite))
	s.mux.Handle("GET /api/favorites/check/{streamID}", s.authenticated(s.handleCheckFavorite))

	s.mux.Handle("GET /api/streams/{id}/detail", s.authenticated(s.handleStreamDetail))
	s.mux.Handle("GET /api/tmdb/image", s.authenticated(s.handleTMDBImage))

	s.mux.Handle("GET /api/clients", s.authenticated(s.handleListClients))
	s.mux.Handle("GET /api/clients/{id}", s.authenticated(s.handleGetClient))
	s.mux.Handle("POST /api/clients", s.adminOnly(s.handleCreateClient))
	s.mux.Handle("PUT /api/clients/{id}", s.adminOnly(s.handleUpdateClient))
	s.mux.Handle("DELETE /api/clients/{id}", s.adminOnly(s.handleDeleteClient))

	s.mux.Handle("GET /api/capabilities", s.authenticated(s.handleCapabilities))
	s.mux.Handle("GET /api/activity", s.adminOnly(s.handleListActivity))

	s.mux.HandleFunc("GET /api/output/playlist.m3u", s.handleOutputM3U)
	s.mux.HandleFunc("GET /api/output/epg.xml", s.handleOutputEPG)
	s.mux.HandleFunc("GET /channel/{id}", s.handleChannelStream)

	s.mux.Handle("POST /api/probe", s.adminOnly(s.handleProbe))

	if s.deps.LogoCache != nil {
		s.mux.HandleFunc("GET /logo", s.deps.LogoCache.ServeHTTP)
	}

	if s.deps.StaticFS != nil {
		staticHandler := http.FileServerFS(s.deps.StaticFS)
		s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			path := r.URL.Path
			if path != "/" {
				if _, err := fs.Stat(s.deps.StaticFS, strings.TrimPrefix(path, "/")); err != nil {
					r.URL.Path = "/"
				}
			}
			staticHandler.ServeHTTP(w, r)
		})
	}
}

func (s *Server) authenticated(h http.HandlerFunc) http.Handler {
	return s.middleware.Authenticate(h)
}

func (s *Server) adminOnly(h http.HandlerFunc) http.Handler {
	return s.middleware.RequireAdmin(h)
}
