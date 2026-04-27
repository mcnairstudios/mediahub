package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/mcnairstudios/mediahub/pkg/activity"
	"github.com/mcnairstudios/mediahub/pkg/api"
	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/logocache"
	"github.com/mcnairstudios/mediahub/pkg/cache"
	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/config"
	"github.com/mcnairstudios/mediahub/pkg/connectivity"
	"github.com/mcnairstudios/mediahub/pkg/connectivity/wg"
	"github.com/mcnairstudios/mediahub/pkg/frontend/dlna"
	"github.com/mcnairstudios/mediahub/pkg/frontend/hdhr"
	"github.com/mcnairstudios/mediahub/pkg/frontend/jellyfin"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/output/hls"
	"github.com/mcnairstudios/mediahub/pkg/output/mse"
	"github.com/mcnairstudios/mediahub/pkg/output/record"
	"github.com/mcnairstudios/mediahub/pkg/output/stream"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/source"
	m3usource "github.com/mcnairstudios/mediahub/pkg/source/m3u"
	tvpstreamssource "github.com/mcnairstudios/mediahub/pkg/source/tvpstreams"
	xstreamsource "github.com/mcnairstudios/mediahub/pkg/source/xtream"
	"github.com/mcnairstudios/mediahub/pkg/orchestrator"
	recscheduler "github.com/mcnairstudios/mediahub/pkg/scheduler"
	"github.com/mcnairstudios/mediahub/pkg/store"
	boltstore "github.com/mcnairstudios/mediahub/pkg/store/bolt"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
	"github.com/mcnairstudios/mediahub/pkg/worker"
	"github.com/mcnairstudios/mediahub/web"
)

func main() {
	cfg := config.Load()

	zlog := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().Timestamp().Logger()

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatalf("creating data directory %s: %v", cfg.DataDir, err)
	}

	db, err := boltstore.Open(filepath.Join(cfg.DataDir, "mediahub.db"))
	if err != nil {
		log.Fatalf("opening bolt database: %v", err)
	}
	defer db.Close()

	streamStore := db.StreamStore()
	settingsStore := db.SettingsStore()
	channelStore := db.ChannelStore()
	groupStore := db.GroupStore()
	epgSourceStore := db.EPGSourceStore()
	programStore := db.ProgramStore()
	recordingStore := db.RecordingStore()
	userStore := db.UserStore()
	sourceConfigStore := db.SourceConfigStore()
	favoriteStore := db.FavoriteStore()

	authService := auth.NewJWTService(userStore, "mediahub-secret-change-me")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	users, err := authService.ListUsers(ctx)
	if err != nil {
		log.Fatalf("listing users: %v", err)
	}
	if len(users) == 0 {
		if _, err := authService.CreateUser(ctx, "admin", "admin", auth.RoleAdmin); err != nil {
			log.Fatalf("seeding admin user: %v", err)
		}
		log.Println("seeded default admin user (admin/admin)")
	}

	seedDefaults(ctx, settingsStore)

	wgService := wg.NewService(settingsStore)

	tmdbCache := tmdbcache.New()

	sourceReg := source.NewRegistry()
	sourceReg.Register("m3u", func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := sourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		m3uCfg := m3usource.Config{
			ID:           sc.ID,
			Name:         sc.Name,
			URL:          sc.Config["url"],
			IsEnabled:    sc.IsEnabled,
			UseWireGuard: sc.Config["use_wireguard"] == "true",
			UserAgent:    cfg.UserAgent,
			BypassHeader: cfg.BypassHeader,
			BypassSecret: cfg.BypassSecret,
			StreamStore:  streamStore,
		}
		if m3uCfg.UseWireGuard && wgService != nil {
			if p := wgService.ActivePlugin(); p != nil {
				m3uCfg.WGClient = p.HTTPClient()
			}
		}
		log.Printf("m3u factory: source=%s wg=%v wgClient=%v", sc.Name, m3uCfg.UseWireGuard, m3uCfg.WGClient != nil)
		return m3usource.New(m3uCfg), nil
	})
	sourceReg.Register("tvpstreams", func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := sourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		tvpCfg := tvpstreamssource.Config{
			ID:              sc.ID,
			Name:            sc.Name,
			URL:             sc.Config["url"],
			IsEnabled:       sc.IsEnabled,
			UseWireGuard:    sc.Config["use_wireguard"] == "true",
			DataDir:         cfg.DataDir,
			EnrollmentToken: sc.Config["enrollment_token"],
			TLSEnrolled:     sc.Config["tls_enrolled"] == "true",
			BypassHeader:    cfg.BypassHeader,
			BypassSecret:    cfg.BypassSecret,
			StreamStore:     streamStore,
			TMDBCache:       tmdbCache,
			OnEnrolled: func(sourceID string) error {
				scUpd, err := sourceConfigStore.Get(ctx, sourceID)
				if err != nil || scUpd == nil {
					return err
				}
				scUpd.Config["tls_enrolled"] = "true"
				scUpd.Config["enrollment_token"] = ""
				return sourceConfigStore.Update(ctx, scUpd)
			},
		}
		if tvpCfg.UseWireGuard && wgService != nil {
			if p := wgService.ActivePlugin(); p != nil {
				tvpCfg.WGClient = p.HTTPClient()
			}
		}
		log.Printf("tvpstreams factory: source=%s wg=%v wgClient=%v", sc.Name, tvpCfg.UseWireGuard, tvpCfg.WGClient != nil)
		return tvpstreamssource.New(tvpCfg), nil
	})
	sourceReg.Register("xtream", func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := sourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		maxStreams := 0
		if v := sc.Config["max_streams"]; v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				maxStreams = n
			}
		}
		xtCfg := xstreamsource.Config{
			ID:           sc.ID,
			Name:         sc.Name,
			Server:       sc.Config["server"],
			Username:     sc.Config["username"],
			Password:     sc.Config["password"],
			IsEnabled:    sc.IsEnabled,
			UseWireGuard: sc.Config["use_wireguard"] == "true",
			MaxStreams:   maxStreams,
			StreamStore:  streamStore,
		}
		if xtCfg.UseWireGuard && wgService != nil {
			if p := wgService.ActivePlugin(); p != nil {
				xtCfg.WGClient = p.HTTPClient()
			}
		}
		return xstreamsource.New(xtCfg), nil
	})
	sourceReg.Register("hdhr", func(_ context.Context, _ string) (source.Source, error) {
		return nil, errors.New("hdhr sources are created via API with their config")
	})
	sourceReg.Register("satip", func(_ context.Context, _ string) (source.Source, error) {
		return nil, errors.New("satip sources are created via API with their config")
	})

	outputReg := output.NewRegistry()
	outputReg.Register(output.DeliveryMSE, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return mse.New(cfg)
	})
	outputReg.Register(output.DeliveryHLS, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return hls.New(cfg)
	})
	outputReg.Register(output.DeliveryStream, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return stream.New(cfg)
	})
	outputReg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return record.New(cfg)
	})

	connReg := connectivity.NewRegistry()

	if err := wgService.RestoreActive(ctx); err != nil {
		log.Printf("wireguard: failed to restore active profile: %v", err)
	} else if plugin := wgService.ActivePlugin(); plugin != nil {
		connReg.Register(plugin)
		connReg.SetActive("wireguard")
		log.Printf("wireguard: restored active tunnel (proxy port %d)", plugin.Port())
	}

	cacheReg := cache.NewRegistry()
	cacheReg.Register(tmdbCache)

	detector := client.NewDetector(nil)

	sessionMgr := session.NewManager(cfg.RecordDir)

	scheduler := worker.NewScheduler(func(name string, err error) {
		log.Printf("worker %s error: %v", name, err)
	})

	refreshDeps := orchestrator.RefreshDeps{
		SourceReg:         sourceReg,
		SourceConfigStore: sourceConfigStore,
	}
	scheduler.Add(worker.Job{
		Name:     "source-refresh",
		Interval: 6 * time.Hour,
		Fn: func(ctx context.Context) error {
			errs := orchestrator.RefreshAll(ctx, refreshDeps)
			if len(errs) > 0 {
				return fmt.Errorf("source refresh: %d errors, last: %w", len(errs), errs[len(errs)-1])
			}
			return nil
		},
	})

	var epgRefreshFn func(ctx context.Context) error
	scheduler.Add(worker.Job{
		Name:     "epg-refresh",
		Interval: 24 * time.Hour,
		Fn: func(ctx context.Context) error {
			if epgRefreshFn != nil {
				return epgRefreshFn(ctx)
			}
			return nil
		},
	})

	recScheduler := recscheduler.New(recordingStore)
	recDeps := orchestrator.RecordingDeps{
		SessionMgr:     sessionMgr,
		RecordingStore: recordingStore,
		OutputReg:      outputReg,
	}
	recScheduler.SetStartFunc(func(streamID, title string) error {
		return orchestrator.StartRecording(ctx, recDeps, streamID, title, "system", false)
	})
	recScheduler.SetStopFunc(func(streamID string) error {
		return orchestrator.StopRecording(ctx, recDeps, streamID)
	})
	recScheduler.Start(ctx)

	logoCache := logocache.New(filepath.Join(cfg.DataDir, "logocache"))

	staticFS, _ := fs.Sub(web.Assets, "dist")

	activityService := activity.New()

	apiServer := api.NewServer(api.OrchestratorDeps{
		StreamStore:       streamStore,
		ChannelStore:      channelStore,
		SettingsStore:     settingsStore,
		SourceConfigStore: sourceConfigStore,
		ConnRegistry:      connReg,
		SessionMgr:        sessionMgr,
		Detector:          detector,
		OutputReg:         outputReg,
		SourceReg:         sourceReg,
		RecordingStore:    recordingStore,
		AuthService:       authService,
		EPGSourceStore:    epgSourceStore,
		ProgramStore:      programStore,
		GroupStore:        groupStore,
		Strategy:          strategy.Resolve,
		FavoriteStore:     favoriteStore,
		WGService:         wgService,
		LogoCache:         logoCache,
		Activity:          activityService,
		StaticFS:          staticFS,
		UserAgent:         cfg.UserAgent,
		BypassHeader:      cfg.BypassHeader,
		BypassSecret:      cfg.BypassSecret,
	})

	epgRefreshFn = apiServer.RefreshAllEPGSources

	mainMux := http.NewServeMux()

	apiHandler := apiServer.Handler()
	mainMux.Handle("/", apiHandler)

	hdhrServer := hdhr.NewServer(channelStore, cfg)
	hdhrHandler := hdhrServer.Handler()
	mainMux.HandleFunc("GET /discover.json", func(w http.ResponseWriter, r *http.Request) {
		hdhrHandler.ServeHTTP(w, r)
	})
	mainMux.HandleFunc("GET /lineup_status.json", func(w http.ResponseWriter, r *http.Request) {
		hdhrHandler.ServeHTTP(w, r)
	})
	mainMux.HandleFunc("GET /lineup.json", func(w http.ResponseWriter, r *http.Request) {
		hdhrHandler.ServeHTTP(w, r)
	})
	mainMux.HandleFunc("GET /lineup.xml", func(w http.ResponseWriter, r *http.Request) {
		hdhrHandler.ServeHTTP(w, r)
	})
	mainMux.HandleFunc("GET /device.xml", func(w http.ResponseWriter, r *http.Request) {
		hdhrHandler.ServeHTTP(w, r)
	})

	dlnaChannels := &dlnaChannelAdapter{channels: channelStore, groups: groupStore}
	dlnaSettings := &dlnaSettingsAdapter{settings: settingsStore, enabled: cfg.DLNAEnabled}
	dlnaServer := dlna.NewServer(dlnaChannels, dlnaSettings, cfg.BaseURL, cfg.DLNAPort, zlog)
	dlnaServer.RegisterRoutes(mainMux)

	scheduler.Start(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	apiPort := extractPort(cfg.ListenAddr)
	log.Printf("mediahub API on %s (data: %s)", cfg.ListenAddr, cfg.DataDir)

	errCh := make(chan error, 3)

	go func() {
		if err := http.ListenAndServe(cfg.ListenAddr, mainMux); err != nil {
			errCh <- fmt.Errorf("api server: %w", err)
		}
	}()

	jellyfinServer := jellyfin.NewServer(jellyfin.ServerDeps{
		ServerName: "MediaHub",
		StateDir:   cfg.DataDir,
		Auth:       authService,
		Channels:   channelStore,
		Groups:     groupStore,
		Streams:    streamStore,
		Programs:   programStore,
		TMDBCache:  tmdbCache,
		Log:        zlog,
	})
	go func() {
		addr := fmt.Sprintf(":%d", cfg.JellyfinPort)
		log.Printf("jellyfin emulation on %s", addr)
		if err := jellyfinServer.ListenAndServe(addr); err != nil {
			errCh <- fmt.Errorf("jellyfin server: %w", err)
		}
	}()

	hdhrDiscovery := hdhr.NewDiscoveryResponder(cfg.BaseURL, zlog)
	go func() {
		log.Printf("hdhr discovery responder starting (UDP 65001)")
		hdhrDiscovery.Run(ctx)
	}()

	if cfg.DLNAEnabled {
		ssdp := dlna.NewSSDPAdvertiser(dlnaServer, cfg.BaseURL, cfg.DLNAPort, 30*time.Second, zlog)
		go func() {
			log.Printf("dlna SSDP advertiser starting (port %d)", apiPort)
			ssdp.Run(ctx)
		}()
	}

	log.Printf("hdhr endpoints on %s (/discover.json, /lineup.json, /device.xml)", cfg.ListenAddr)
	log.Printf("dlna endpoints on %s (/dlna/*)", cfg.ListenAddr)

	select {
	case sig := <-sigCh:
		log.Printf("received %s, shutting down...", sig)
	case err := <-errCh:
		log.Printf("server error: %v", err)
	}

	cancel()
	recScheduler.Stop()
	scheduler.Stop()
	sessionMgr.StopAll()
	wgService.Close()

	fmt.Println("mediahub stopped")
}

func extractPort(listenAddr string) int {
	for i := len(listenAddr) - 1; i >= 0; i-- {
		if listenAddr[i] == ':' {
			if port, err := strconv.Atoi(listenAddr[i+1:]); err == nil {
				return port
			}
		}
	}
	return 8080
}

type dlnaChannelAdapter struct {
	channels channel.Store
	groups   channel.GroupStore
}

func (a *dlnaChannelAdapter) ListChannels(ctx context.Context) ([]dlna.ChannelItem, error) {
	channels, err := a.channels.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]dlna.ChannelItem, 0, len(channels))
	for _, ch := range channels {
		if !ch.IsEnabled {
			continue
		}
		items = append(items, dlna.ChannelItem{
			ID:      ch.ID,
			Name:    ch.Name,
			LogoURL: ch.LogoURL,
			GroupID: ch.GroupID,
		})
	}
	return items, nil
}

func (a *dlnaChannelAdapter) GetChannel(ctx context.Context, id string) (*dlna.ChannelItem, error) {
	ch, err := a.channels.Get(ctx, id)
	if err != nil || ch == nil {
		return nil, err
	}
	return &dlna.ChannelItem{
		ID:      ch.ID,
		Name:    ch.Name,
		LogoURL: ch.LogoURL,
		GroupID: ch.GroupID,
	}, nil
}

func (a *dlnaChannelAdapter) ListGroups(ctx context.Context) ([]dlna.GroupItem, error) {
	groups, err := a.groups.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]dlna.GroupItem, 0, len(groups))
	for _, g := range groups {
		items = append(items, dlna.GroupItem{
			ID:   g.ID,
			Name: g.Name,
		})
	}
	return items, nil
}

type dlnaSettingsAdapter struct {
	settings interface {
		Get(ctx context.Context, key string) (string, error)
	}
	enabled bool
}

func (a *dlnaSettingsAdapter) IsEnabled(ctx context.Context) bool {
	if !a.enabled {
		return false
	}
	val, err := a.settings.Get(ctx, "dlna_enabled")
	if err != nil || val == "" {
		return true
	}
	return val != "false" && val != "0"
}

func seedDefaults(ctx context.Context, s store.SettingsStore) {
	defaults := map[string]string{
		"default_hwaccel":        "none",
		"default_video_codec":    "copy",
		"default_decode_hwaccel": "",
		"default_max_bit_depth":  "",
		"encoder_h264":           "",
		"encoder_h265":           "",
		"encoder_av1":            "",
		"decoder_h264":           "",
		"decoder_h265":           "",
		"dlna_enabled":           "true",
		"delivery":               "hls",
		"container":              "mp4",
	}
	for k, v := range defaults {
		existing, _ := s.Get(ctx, k)
		if existing == "" {
			s.Set(ctx, k, v)
		}
	}
}
