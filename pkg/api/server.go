package api

import (
	"io/fs"
	"net/http"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/epg"
	"github.com/mcnairstudios/mediahub/pkg/middleware"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
)

type OrchestratorDeps struct {
	StreamStore       store.StreamStore
	ChannelStore      channel.Store
	SettingsStore     store.SettingsStore
	SourceConfigStore sourceconfig.Store
	SessionMgr        *session.Manager
	Detector          *client.Detector
	OutputReg         *output.Registry
	SourceReg         *source.Registry
	RecordingStore    recording.Store
	AuthService       auth.Service
	EPGSourceStore    epg.SourceStore
	Strategy          func(strategy.Input, strategy.Output) strategy.Decision
	StaticFS          fs.FS
}

type Server struct {
	mux        *http.ServeMux
	middleware *middleware.AuthMiddleware
	deps       OrchestratorDeps
}

func NewServer(deps OrchestratorDeps) *Server {
	s := &Server{
		mux:        http.NewServeMux(),
		middleware: middleware.NewAuthMiddleware(deps.AuthService),
		deps:       deps,
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}
