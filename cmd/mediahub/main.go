package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"encoding/json"

	"github.com/mcnairstudios/mediahub/pkg/activity"
	"github.com/mcnairstudios/mediahub/pkg/api"
	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/defaults"
	"github.com/mcnairstudios/mediahub/pkg/logocache"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/cache"
	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/config"
	"github.com/mcnairstudios/mediahub/pkg/connectivity"
	"github.com/mcnairstudios/mediahub/pkg/connectivity/wg"
	"github.com/mcnairstudios/mediahub/pkg/epg"
	"github.com/mcnairstudios/mediahub/pkg/frontend/dlna"
	"github.com/mcnairstudios/mediahub/pkg/frontend/hdhr"
	"github.com/mcnairstudios/mediahub/pkg/frontend/jellyfin"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/sourceprofile"
	"github.com/mcnairstudios/mediahub/pkg/orchestrator"
	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/scheduler"
	"github.com/mcnairstudios/mediahub/pkg/tmdb"
	boltstore "github.com/mcnairstudios/mediahub/pkg/store/bolt"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
	"github.com/mcnairstudios/mediahub/pkg/wasm"
	"github.com/mcnairstudios/mediahub/pkg/source/wasmsource"
	"github.com/mcnairstudios/mediahub/web"
)

func main() {
	cfg := config.Load()

	if cfg.DLNAPort == 0 {
		cfg.DLNAPort = extractPort(cfg.ListenAddr)
	}

	logLevel := zerolog.InfoLevel
	if v := os.Getenv("MEDIAHUB_LOG_LEVEL"); v != "" {
		if l, err := zerolog.ParseLevel(v); err == nil {
			logLevel = l
		}
	}
	zerolog.SetGlobalLevel(logLevel)
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
	clientStore := db.ClientStore()
	sourceProfileStore := db.SourceProfileStore()
	probeCache := db.ProbeCacheStore()
	hdhrDeviceStore := db.HDHRDeviceStore()

	inviteStore := db.InviteStore()
	apiKeyStore := db.APIKeyStore()

	jwtSecret := os.Getenv("MEDIAHUB_JWT_SECRET")
	if jwtSecret == "" {
		secretFile := filepath.Join(cfg.DataDir, "jwt_secret")
		data, err := os.ReadFile(secretFile)
		if err == nil && len(data) > 0 {
			jwtSecret = string(data)
		} else {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				log.Fatalf("generating JWT secret: %v", err)
			}
			jwtSecret = hex.EncodeToString(b)
			if err := os.WriteFile(secretFile, []byte(jwtSecret), 0600); err != nil {
				log.Fatalf("persisting JWT secret to %s: %v", secretFile, err)
			}
			log.Println("generated and persisted new JWT secret")
		}
	}
	authService := auth.NewJWTService(userStore, jwtSecret)
	authService.SetInviteStore(inviteStore)
	authService.SetAPIKeyStore(apiKeyStore)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	users, err := authService.ListUsers(ctx)
	if err != nil {
		log.Fatalf("listing users: %v", err)
	}
	if len(users) == 0 {
		if _, err := authService.CreateUser(ctx, "admin", "admin", "", auth.RoleAdmin); err != nil {
			log.Fatalf("seeding admin user: %v", err)
		}
		log.Println("seeded default admin user (admin/admin)")
	}

	settingsDefs, err := defaults.LoadSettings(cfg.DataDir)
	if err != nil {
		log.Fatalf("loading default settings: %v", err)
	}
	for k, v := range settingsDefs {
		existing, _ := settingsStore.Get(ctx, k)
		if existing == "" {
			settingsStore.Set(ctx, k, v)
		}
	}

	clientDefs, err := defaults.LoadClients(cfg.DataDir)
	if err != nil {
		log.Fatalf("loading default clients: %v", err)
	}
	if err := client.SeedDefaults(ctx, clientStore, clientDefs); err != nil {
		log.Printf("warning: failed to seed default clients: %v", err)
	}

	sourceProfileDefs, err := defaults.LoadSourceProfiles(cfg.DataDir)
	if err != nil {
		log.Fatalf("loading default source profiles: %v", err)
	}
	if err := sourceprofile.SeedDefaults(ctx, sourceProfileStore, sourceProfileDefs); err != nil {
		log.Printf("warning: failed to seed default source profiles: %v", err)
	}

	autoNumberChannels(ctx, channelStore)

	wgService := wg.NewService(settingsStore, wg.PluginConfig{
		UserAgent:    cfg.UserAgent,
		BypassHeader: cfg.BypassHeader,
		BypassSecret: cfg.BypassSecret,
	})

	tmdbCache := tmdbcache.NewPersistent(filepath.Join(cfg.DataDir, "tmdb_cache"))

	onRefreshDone := makeOnRefreshDone(ctx, sourceConfigStore)

	sourceReg := source.NewRegistry()
	registerSources(sourceReg, sourceDeps{
		SourceConfigStore: sourceConfigStore,
		StreamStore:       streamStore,
		SettingsStore:     settingsStore,
		Config:            cfg,
		WGService:         wgService,
		TMDBCache:         tmdbCache,
		OnRefreshDone:     onRefreshDone,
	})

	// Load WASM plugins.
	pluginsDir := cfg.PluginsDir
	if pluginsDir == "" {
		pluginsDir = filepath.Join(cfg.DataDir, "plugins")
	}
	os.MkdirAll(pluginsDir, 0755)

	wasmKVStore, err := wasm.NewBoltKVStore(db.BoltDB())
	if err != nil {
		log.Printf("wasm: failed to create KV store: %v", err)
	}
	wasmHost := wasm.NewHost(nil, wasmKVStore)
	wasmPlugins, err := wasmHost.LoadDir(ctx, pluginsDir)
	if err != nil {
		log.Printf("wasm: failed to load plugins dir: %v", err)
	}
	for _, wp := range wasmPlugins {
		wpCopy := wp
		sourceReg.RegisterPlugin(source.PluginRegistration{
			Descriptor: wpCopy.Descriptor,
			Factory: func(fCtx context.Context, sourceID string) (source.Source, error) {
				sc, fErr := sourceConfigStore.Get(fCtx, sourceID)
				if fErr != nil {
					return nil, fmt.Errorf("get source config: %w", fErr)
				}
				if sc == nil {
					return nil, fmt.Errorf("source config not found")
				}
				configJSON, _ := json.Marshal(sc.Config)
				return wasmsource.New(wasmsource.Config{
					ID:            sc.ID,
					Name:          sc.Name,
					IsEnabled:     sc.IsEnabled,
					Plugin:        wpCopy,
					ConfigJSON:    configJSON,
					StreamStore:   streamStore,
					OnRefreshDone: onRefreshDone,
				}), nil
			},
		})
	}
	if len(wasmPlugins) > 0 {
		log.Printf("wasm: loaded %d plugin(s)", len(wasmPlugins))
	}

	outputReg := output.NewRegistry()
	registerOutputs(outputReg)

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

	detector := client.NewDetectorWithStore(clientStore)

	sessionTmpDir := filepath.Join(os.TempDir(), "mediahub-sessions")
	os.MkdirAll(sessionTmpDir, 0755)
	sessionMgr := session.NewManager(sessionTmpDir)

	sched := scheduler.New(db.BoltDB(), func(name string, err error) {
		log.Printf("scheduler %s error: %v", name, err)
	})

	var epgRefreshFn orchestrator.EPGRefreshFunc
	refreshDeps := orchestrator.RefreshDeps{
		SourceReg:         sourceReg,
		SourceConfigStore: sourceConfigStore,
		EPGSourceStore:    epgSourceStore,
		EPGRefreshFn: func(ctx context.Context, src *epg.Source) {
			if epgRefreshFn != nil {
				epgRefreshFn(ctx, src)
			}
		},
	}
	sched.Every("source-refresh", "@every 1m", func(ctx context.Context) error {
		errs := orchestrator.RefreshAll(ctx, refreshDeps)
		if len(errs) > 0 {
			return fmt.Errorf("source refresh: %d errors, last: %w", len(errs), errs[len(errs)-1])
		}
		return nil
	})

	sched.Every("wg-health", "@every 30s", func(ctx context.Context) error {
		if wgService.ActivePlugin() == nil {
			return nil
		}
		if err := wgService.HealthCheck(ctx); err != nil {
			log.Printf("wireguard: health check failed: %v — attempting failover", err)
			name, fErr := wgService.Failover(ctx)
			if fErr != nil {
				return fmt.Errorf("wireguard failover failed: %w", fErr)
			}
			log.Printf("wireguard: failover succeeded — now using %s", name)
			if plugin := wgService.ActivePlugin(); plugin != nil {
				connReg.Register(plugin)
				connReg.SetActive("wireguard")
			}
		}
		return nil
	})

	recDeps := orchestrator.RecordingDeps{
		SessionMgr:     sessionMgr,
		RecordingStore: recordingStore,
		OutputReg:      outputReg,
		RecordDir:      cfg.RecordDir,
	}
	sched.Every("recording-check", "@every 30s", func(ctx context.Context) error {
		now := time.Now()

		scheduled, err := recordingStore.ListScheduled(ctx)
		if err != nil {
			return fmt.Errorf("list scheduled: %w", err)
		}
		for i := range scheduled {
			rec := &scheduled[i]
			if !rec.ScheduledStart.IsZero() && !rec.ScheduledStart.After(now) {
				if err := orchestrator.StartRecording(ctx, recDeps, rec.StreamID, rec.Title, "system", false); err != nil {
					rec.Status = recording.StatusFailed
					recordingStore.Update(ctx, rec)
					continue
				}
				rec.Status = recording.StatusRecording
				rec.StartedAt = now
				recordingStore.Update(ctx, rec)
			}
		}

		active, err := recordingStore.ListByStatus(ctx, recording.StatusRecording)
		if err != nil {
			return fmt.Errorf("list active: %w", err)
		}
		for i := range active {
			rec := &active[i]
			if !rec.ScheduledStop.IsZero() && !rec.ScheduledStop.After(now) {
				if err := orchestrator.StopRecording(ctx, recDeps, rec.StreamID); err != nil {
					rec.Status = recording.StatusFailed
					recordingStore.Update(ctx, rec)
					continue
				}
				rec.Status = recording.StatusCompleted
				rec.StoppedAt = now
				recordingStore.Update(ctx, rec)
			}
		}
		return nil
	})
	orchestrator.RecoverRecordings(ctx, recDeps)

	logoCache := logocache.New(filepath.Join(cfg.DataDir, "logocache"))

	staticFS, _ := fs.Sub(web.Assets, "dist")

	activityService := activity.New()

	tmdbClient := tmdb.NewClient(func() string {
		key, _ := settingsStore.Get(ctx, "tmdb_api_key")
		return key
	}, tmdbCache)
	tmdbImages := tmdb.NewImageCache(filepath.Join(cfg.DataDir, "tmdb_images"))

	tmdbImageDir := filepath.Join(cfg.DataDir, "tmdb_images")
	tmdbStore, err := tmdb.NewStore(db.BoltDB())
	if err != nil {
		log.Fatalf("tmdb store: %v", err)
	}
	tmdbImageServer := tmdb.NewImageServer(tmdbImageDir)

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
		ClientStore:       clientStore,
		AuthService:       authService,
		EPGSourceStore:    epgSourceStore,
		ProgramStore:      programStore,
		GroupStore:        groupStore,
		Strategy:          strategy.Resolve,
		FavoriteStore:     favoriteStore,
		WGService:         wgService,
		LogoCache:         logoCache,
		Activity:          activityService,
		TMDBClient:        tmdbClient,
		TMDBCache:         tmdbCache,
		TMDBImages:        tmdbImages,
		TMDBStore:         tmdbStore,
		TMDBImageServer:   tmdbImageServer,
		SourceProfileStore: sourceProfileStore,
		ProbeCache:         probeCache,
		HDHRDeviceStore:    hdhrDeviceStore,
		Config:            cfg,
		StaticFS:          staticFS,
		UserAgent:         cfg.UserAgent,
		BypassHeader:      cfg.BypassHeader,
		BypassSecret:      cfg.BypassSecret,
		DBClearer:         db,
		PluginInteractor:  &wasmInteractorAdapter{host: wasmHost},
	})

	epgRefreshFn = apiServer.RefreshEPGSource

	mainMux := http.NewServeMux()

	debugEnabled, _ := settingsStore.Get(ctx, "debug_enabled")
	if debugEnabled == "true" || debugEnabled == "1" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		mainMux.HandleFunc("GET /debug/pprof/", pprof.Index)
		mainMux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
		mainMux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
		mainMux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
		mainMux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
		log.Printf("debug mode enabled: pprof endpoints registered, log level set to debug")
	}

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
	dlnaServer.SetAuthenticator(&dlnaAuthAdapter{auth: authService})
	dlnaServer.SetEPG(&dlnaEPGAdapter{programs: programStore})
	dlnaServer.RegisterRoutes(mainMux)

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

	jellyfinPlayback := &jellyfinPlaybackAdapter{
		deps: orchestrator.PlaybackDeps{
			StreamStore:        streamStore,
			SettingsStore:      settingsStore,
			SourceConfigStore:  sourceConfigStore,
			SourceProfileStore: sourceProfileStore,
			ConnRegistry:       connReg,
			WGService:          wgService,
			SessionMgr:         sessionMgr,
			Detector:           detector,
			ClientStore:        clientStore,
			OutputReg:          outputReg,
			Strategy:           strategy.Resolve,
			ProbeCache:         probeCache,
			UserAgent:          cfg.UserAgent,
		},
	}
	jellyfinServer := jellyfin.NewServer(jellyfin.ServerDeps{
		ServerName: "MediaHub",
		StateDir:   cfg.DataDir,
		Auth:       authService,
		Channels:   channelStore,
		Groups:     groupStore,
		Streams:    streamStore,
		Programs:   programStore,
		Favorites:  favoriteStore,
		TMDBCache:  tmdbCache,
		TMDBClient: tmdbClient,
		ImageCache: tmdbImages,
		LogoCache:  logoCache,
		SessionMgr: sessionMgr,
		Playback:   jellyfinPlayback,
		Log:        zlog,
	})
	jellyfinEnabled := true
	if val, err := settingsStore.Get(ctx, "jellyfin_enabled"); err == nil && (val == "false" || val == "0") {
		jellyfinEnabled = false
	}
	if jellyfinEnabled {
		go func() {
			addr := fmt.Sprintf(":%d", cfg.JellyfinPort)
			log.Printf("jellyfin emulation on %s", addr)
			if err := jellyfinServer.ListenAndServe(addr); err != nil {
				errCh <- fmt.Errorf("jellyfin server: %w", err)
			}
		}()
	} else {
		log.Printf("jellyfin emulation disabled via settings")
	}

	hdhrDiscovery := hdhr.NewDiscoveryResponder(cfg.BaseURL, zlog)
	go func() {
		log.Printf("hdhr discovery responder starting (UDP 65001)")
		hdhrDiscovery.Run(ctx)
	}()

	hdhrSsdp := hdhr.NewSSDPAdvertiser(cfg.BaseURL, 30*time.Second, zlog)
	go func() {
		log.Printf("hdhr SSDP advertiser starting")
		hdhrSsdp.Run(ctx)
	}()

	hdhrDeviceMgr := hdhr.NewDeviceManager(hdhrDeviceStore, channelStore, cfg.BaseURL, zlog)
	go func() {
		log.Printf("hdhr device manager starting")
		hdhrDeviceMgr.Run(ctx)
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

	metadataWorker := tmdb.NewMetadataWorker(tmdbStore, func() string {
		key, _ := settingsStore.Get(ctx, "tmdb_api_key")
		return key
	}, tmdbImageDir)
	go func() {
		log.Printf("tmdb metadata worker starting")
		metadataWorker.Run(ctx)
	}()

	imageWorker := tmdb.NewImageWorker(tmdbStore, tmdbImageDir)
	go func() {
		log.Printf("tmdb image worker starting")
		imageWorker.Run(ctx)
	}()

	var tmdbRunning atomic.Bool
	enqueueTMDBStreams := func() {
		if !tmdbRunning.CompareAndSwap(false, true) {
			return
		}
		defer tmdbRunning.Store(false)

		// Reconcile the pending index on first run to catch any streams
		// that were added before the index existed.
		if count, err := streamStore.TMDBPendingCount(ctx); err == nil && count == 0 {
			added, err := streamStore.TMDBPendingReconcile(ctx)
			if err != nil {
				log.Printf("tmdb: reconcile pending index: %v", err)
			} else if added > 0 {
				log.Printf("tmdb: reconciled %d pending streams into index", added)
			}
		}

		total, _ := streamStore.TMDBPendingCount(ctx)
		if total == 0 {
			return
		}
		log.Printf("tmdb resolver: %d streams pending TMDB resolution", total)

		// Process one batch of 100 per invocation. The scheduler will
		// call us again in 1 minute for the next batch.
		batch, err := streamStore.TMDBPendingBatch(ctx, 100)
		if err != nil || len(batch) == 0 {
			return
		}

		var toResolve []tmdb.StreamToResolve
		for _, p := range batch {
			toResolve = append(toResolve, tmdb.StreamToResolve{
				StreamID:  p.StreamID,
				Name:      p.Name,
				Year:      p.Year,
				MediaType: p.MediaType,
			})
		}

		resolved := metadataWorker.ResolveStreams(ctx, toResolve)
		if len(resolved) > 0 {
			var updated []media.Stream
			for _, r := range resolved {
				st, err := streamStore.Get(ctx, r.StreamID)
				if err != nil || st == nil {
					streamStore.TMDBPendingRemove(ctx, r.StreamID)
					continue
				}
				st.TMDBID = strconv.Itoa(r.TMDBID)
				updated = append(updated, *st)
			}
			if len(updated) > 0 {
				if err := streamStore.BulkUpsert(ctx, updated); err != nil {
					log.Printf("tmdb resolver: update streams: %v", err)
				}
			}
		}

		// Remove resolved and failed entries from the pending index
		for _, p := range batch {
			found := false
			for _, r := range resolved {
				if r.StreamID == p.StreamID {
					found = true
					break
				}
			}
			if !found {
				// Unresolvable — remove from pending to avoid infinite retries
				streamStore.TMDBPendingRemove(ctx, p.StreamID)
			}
		}
	}
	go enqueueTMDBStreams()

	sched.Every("tmdb-enqueue", "@every 1m", func(_ context.Context) error {
		enqueueTMDBStreams()
		return nil
	})

	sched.Start(ctx)

	select {
	case sig := <-sigCh:
		log.Printf("received %s, shutting down...", sig)
	case err := <-errCh:
		log.Printf("server error: %v", err)
	}

	cancel()
	sched.Stop()
	sessionMgr.StopAll()
	wasmHost.Close(context.Background())
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
			TvgID:   ch.TvgID,
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
		TvgID:   ch.TvgID,
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

type dlnaAuthAdapter struct {
	auth *auth.JWTService
}

func (a *dlnaAuthAdapter) AuthenticateBasic(ctx context.Context, username, password string) (*dlna.DLNAUser, error) {
	token, err := a.auth.Login(ctx, username, password)
	if err != nil {
		return nil, err
	}
	user, err := a.auth.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}
	return &dlna.DLNAUser{
		IsAdmin:         user.IsAdmin,
		ChannelGroupIDs: user.ChannelGroupIDs,
	}, nil
}

type dlnaEPGAdapter struct {
	programs epg.ProgramStore
}

func (a *dlnaEPGAdapter) NowPlaying(ctx context.Context, tvgID string) (*dlna.NowPlayingInfo, error) {
	p, err := a.programs.NowPlaying(ctx, tvgID)
	if err != nil || p == nil {
		return nil, err
	}
	return &dlna.NowPlayingInfo{Title: p.Title}, nil
}

func resolveWGClient(wgService *wg.Service, useWireGuard bool, profileID string) *http.Client {
	if !useWireGuard || wgService == nil {
		return nil
	}
	if profileID != "" {
		return wgService.HTTPClientForProfile(profileID)
	}
	if p := wgService.ActivePlugin(); p != nil {
		return p.HTTPClient()
	}
	return nil
}

func makeOnRefreshDone(ctx context.Context, store sourceconfig.Store) func(string, string, int) {
	return func(sourceID, etag string, streamCount int) {
		sc, err := store.Get(ctx, sourceID)
		if err != nil || sc == nil {
			return
		}
		sc.Config["etag"] = etag
		sc.Config["stream_count"] = fmt.Sprintf("%d", streamCount)
		sc.Config["last_refreshed"] = time.Now().Format(time.RFC3339)
		store.Update(ctx, sc)
	}
}

type jellyfinPlaybackAdapter struct {
	deps orchestrator.PlaybackDeps
}

func (a *jellyfinPlaybackAdapter) StartPlayback(streamID string, port int, headers map[string]string) error {
	ctx := context.Background()
	_, err := orchestrator.StartPlayback(ctx, a.deps, streamID, port, headers)
	return err
}

func autoNumberChannels(ctx context.Context, store channel.Store) {
	channels, err := store.List(ctx)
	if err != nil {
		log.Printf("auto-number channels: list: %v", err)
		return
	}

	maxNumber := 0
	for _, ch := range channels {
		if ch.Number > maxNumber {
			maxNumber = ch.Number
		}
	}

	next := maxNumber + 1
	numbered := 0
	for i := range channels {
		if channels[i].Number == 0 {
			channels[i].Number = next
			next++
			if err := store.Update(ctx, &channels[i]); err != nil {
				log.Printf("auto-number channels: update %s: %v", channels[i].ID, err)
				continue
			}
			numbered++
		}
	}
	if numbered > 0 {
		log.Printf("auto-numbered %d channels (starting at %d)", numbered, maxNumber+1)
	}
}

type wasmInteractorAdapter struct {
	host *wasm.WASMHost
}

func (a *wasmInteractorAdapter) Interact(ctx context.Context, pluginType string, actionJSON []byte) ([]byte, error) {
	p := a.host.Plugin(pluginType)
	if p == nil {
		return nil, fmt.Errorf("no WASM plugin loaded for type %q", pluginType)
	}
	return p.CallInteract(ctx, actionJSON)
}

