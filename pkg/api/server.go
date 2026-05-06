package api

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/activity"
	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/config"
	"github.com/mcnairstudios/mediahub/pkg/connectivity"
	"github.com/mcnairstudios/mediahub/pkg/connectivity/wg"
	"github.com/mcnairstudios/mediahub/pkg/epg"
	"github.com/mcnairstudios/mediahub/pkg/favorite"
	"github.com/mcnairstudios/mediahub/pkg/frontend/hdhr"
	"github.com/mcnairstudios/mediahub/pkg/logocache"
	"github.com/mcnairstudios/mediahub/pkg/middleware"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/sourceprofile"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
	"github.com/mcnairstudios/mediahub/pkg/tmdb"
	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
)

type OrchestratorDeps struct {
	StreamStore       store.StreamStore
	ChannelStore      channel.Store
	SettingsStore     store.SettingsStore
	SourceConfigStore sourceconfig.Store
	ConnRegistry      *connectivity.Registry
	SessionMgr        *session.Manager
	Detector          *client.Detector
	OutputReg         *output.Registry
	SourceReg         *source.Registry
	RecordingStore    recording.Store
	ClientStore       client.Store
	AuthService       auth.Service
	EPGSourceStore    epg.SourceStore
	ProgramStore      epg.ProgramStore
	GroupStore        channel.GroupStore
	Strategy          func(strategy.Input, strategy.Output) strategy.Decision
	WGService         *wg.Service
	FavoriteStore     favorite.Store
	LogoCache         *logocache.Cache
	Activity          *activity.Service
	TMDBClient        *tmdb.Client
	TMDBCache         *tmdbcache.Cache
	TMDBImages        *tmdb.ImageCache
	TMDBStore         *tmdb.Store
	TMDBImageServer   *tmdb.ImageServer
	SourceProfileStore sourceprofile.Store
	ProbeCache         store.ProbeCache
	HDHRDeviceStore    hdhr.DeviceStore
	Config            *config.Config
	StaticFS          fs.FS
	UserAgent         string
	PipelineRunner    func(*session.Session, session.PipelineConfig) (*session.PipelineResult, error)
	BypassHeader      string
	BypassSecret      string
	DBClearer         any
	PluginInteractor  PluginInteractor
}

// PluginInteractor dispatches interact calls to WASM plugins.
type PluginInteractor interface {
	Interact(ctx context.Context, pluginType string, actionJSON []byte) ([]byte, error)
}

type Server struct {
	mux        *http.ServeMux
	middleware *middleware.AuthMiddleware
	deps       OrchestratorDeps
	startedAt  time.Time
}

func NewServer(deps OrchestratorDeps) *Server {
	s := &Server{
		mux:        http.NewServeMux(),
		middleware: middleware.NewAuthMiddleware(deps.AuthService, deps.Activity),
		deps:       deps,
		startedAt:  time.Now().Round(0), // Round(0) strips monotonic reading — uptime uses wall clock, survives system sleep
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return corsMiddleware(s.mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
