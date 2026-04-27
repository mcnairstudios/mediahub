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
	s.mux.Handle("GET /api/recordings", s.authenticated(s.handleListRecordings))

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
	s.mux.Handle("POST /api/sources/{sourceID}/refresh", s.adminOnly(s.handleRefreshSource))
	s.mux.Handle("GET /api/sources/{sourceID}/status", s.authenticated(s.handleSourceStatus))
	s.mux.Handle("GET /api/users", s.adminOnly(s.handleListUsers))
	s.mux.Handle("POST /api/users", s.adminOnly(s.handleCreateUser))

	s.mux.Handle("GET /api/wireguard/profiles", s.adminOnly(s.handleListWGProfiles))
	s.mux.Handle("POST /api/wireguard/profiles", s.adminOnly(s.handleCreateWGProfile))
	s.mux.Handle("PUT /api/wireguard/profiles/{id}", s.adminOnly(s.handleUpdateWGProfile))
	s.mux.Handle("DELETE /api/wireguard/profiles/{id}", s.adminOnly(s.handleDeleteWGProfile))
	s.mux.Handle("POST /api/wireguard/profiles/{id}/activate", s.adminOnly(s.handleActivateWGProfile))
	s.mux.Handle("POST /api/wireguard/profiles/{id}/test", s.adminOnly(s.handleTestWGProfile))
	s.mux.Handle("GET /api/wireguard/status", s.adminOnly(s.handleWGStatus))

	s.mux.Handle("GET /api/capabilities", s.authenticated(s.handleCapabilities))

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
