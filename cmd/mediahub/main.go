package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/pprof"
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
	"github.com/mcnairstudios/mediahub/pkg/output/hls"
	"github.com/mcnairstudios/mediahub/pkg/output/mse"
	"github.com/mcnairstudios/mediahub/pkg/output/record"
	"github.com/mcnairstudios/mediahub/pkg/output/stream"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/sourceprofile"
	hdhrsource "github.com/mcnairstudios/mediahub/pkg/source/hdhr"
	m3usource "github.com/mcnairstudios/mediahub/pkg/source/m3u"
	satipsource "github.com/mcnairstudios/mediahub/pkg/source/satip"
	tvpstreamssource "github.com/mcnairstudios/mediahub/pkg/source/tvpstreams"
	xstreamsource "github.com/mcnairstudios/mediahub/pkg/source/xtream"
	"github.com/mcnairstudios/mediahub/pkg/orchestrator"
	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/scheduler"
	"github.com/mcnairstudios/mediahub/pkg/tmdb"
	boltstore "github.com/mcnairstudios/mediahub/pkg/store/bolt"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
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
			WGProfileID:  sc.Config["wg_profile_id"],
			UserAgent:    cfg.UserAgent,
			BypassHeader: cfg.BypassHeader,
			BypassSecret: cfg.BypassSecret,
			InitialETag:  sc.Config["etag"],
			StreamStore:  streamStore,
			OnRefreshDone: onRefreshDone,
		}
		m3uCfg.WGClient = resolveWGClient(wgService, m3uCfg.UseWireGuard, m3uCfg.WGProfileID)
		log.Printf("m3u factory: source=%s wg=%v wgProfile=%s wgClient=%v", sc.Name, m3uCfg.UseWireGuard, m3uCfg.WGProfileID, m3uCfg.WGClient != nil)
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
			InitialETag:     sc.Config["etag"],
			OnRefreshDone: onRefreshDone,
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
		wgProfileID := sc.Config["wg_profile_id"]
		tvpCfg.WGClient = resolveWGClient(wgService, tvpCfg.UseWireGuard, wgProfileID)
		log.Printf("tvpstreams factory: source=%s wg=%v wgProfile=%s wgClient=%v", sc.Name, tvpCfg.UseWireGuard, wgProfileID, tvpCfg.WGClient != nil)
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
			ID:            sc.ID,
			Name:          sc.Name,
			Server:        sc.Config["server"],
			Username:      sc.Config["username"],
			Password:      sc.Config["password"],
			IsEnabled:     sc.IsEnabled,
			UseWireGuard:  sc.Config["use_wireguard"] == "true",
			MaxStreams:    maxStreams,
			StreamStore:   streamStore,
			OnRefreshDone: onRefreshDone,
		}
		xtCfg.WGClient = resolveWGClient(wgService, xtCfg.UseWireGuard, sc.Config["wg_profile_id"])
		return xstreamsource.New(xtCfg), nil
	})
	sourceReg.Register("hdhr", func(ctx context.Context, sourceID string) (source.Source, error) {
		if sourceID == "" {
			return hdhrsource.New(hdhrsource.Config{
				StreamStore: streamStore,
			}), nil
		}
		sc, err := sourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		var devices []hdhrsource.Device
		if devicesJSON := sc.Config["devices"]; devicesJSON != "" {
			if jsonErr := json.Unmarshal([]byte(devicesJSON), &devices); jsonErr != nil {
				log.Printf("hdhr: failed to parse devices for %s: %v", sc.Name, jsonErr)
			}
		}
		hdhrCfg := hdhrsource.Config{
			ID:          sc.ID,
			Name:        sc.Name,
			IsEnabled:   sc.IsEnabled,
			Devices:     devices,
			StreamStore: streamStore,
		}
		return hdhrsource.New(hdhrCfg), nil
	})
	sourceReg.Register("satip", func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := sourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		httpPort := 8875
		if v := sc.Config["http_port"]; v != "" {
			if n, pErr := strconv.Atoi(v); pErr == nil {
				httpPort = n
			}
		}
		maxStreams := 0
		if v := sc.Config["max_streams"]; v != "" {
			if n, pErr := strconv.Atoi(v); pErr == nil {
				maxStreams = n
			}
		}
		diseqcSource := 0
		if ds := sc.Config["diseqc_source"]; ds != "" {
			fmt.Sscanf(ds, "%d", &diseqcSource)
		}
		satipCfg := satipsource.Config{
			ID:              sc.ID,
			Name:            sc.Name,
			Host:            sc.Config["host"],
			HTTPPort:        httpPort,
			IsEnabled:       sc.IsEnabled,
			MaxStreams:       maxStreams,
			TransmitterFile: sc.Config["transmitter_file"],
			DiSEqCSource:    diseqcSource,
			StreamStore:     streamStore,
		}
		return satipsource.New(satipCfg), nil
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

	var detectorClients []client.Client
	if storedClients, err := clientStore.List(ctx); err == nil {
		detectorClients = storedClients
	}
	detector := client.NewDetector(detectorClients)

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

	enqueueTMDBStreams := func() {
		streams, err := streamStore.List(ctx)
		if err != nil {
			log.Printf("tmdb enqueue: list streams: %v", err)
			return
		}
		enqueued := 0
		var toResolve []tmdb.StreamToResolve
		for _, st := range streams {
			if st.VODType == "" {
				continue
			}
			mediaType := "movie"
			if st.VODType == "series" || st.VODType == "episode" {
				mediaType = "series"
			}

			if st.TMDBID != "" {
				tmdbID := 0
				fmt.Sscanf(st.TMDBID, "%d", &tmdbID)
				if tmdbID <= 0 {
					continue
				}
				lookupName := st.SeriesName
				if lookupName == "" {
					lookupName = st.Name
				}
				if lookupName != "" {
					tmdbStore.SetName(lookupName, tmdbID, mediaType)
				}
				if has, _ := tmdbStore.HasBlobTyped(mediaType, tmdbID); has {
					continue
				}
				if err := tmdbStore.EnqueueMetadata(tmdb.QueueEntry{
					TMDBID:    tmdbID,
					MediaType: mediaType,
					Status:    "resolving",
					CreatedAt: time.Now().Unix(),
				}); err == nil {
					enqueued++
				}
			} else {
				lookupName := st.SeriesName
				if lookupName == "" {
					lookupName = st.Name
				}
				toResolve = append(toResolve, tmdb.StreamToResolve{
					StreamID:  st.ID,
					Name:      lookupName,
					Year:      st.Year,
					MediaType: mediaType,
				})
			}
		}
		if enqueued > 0 {
			log.Printf("tmdb enqueue: queued %d items for metadata resolution", enqueued)
		}

		if len(toResolve) > 0 {
			log.Printf("tmdb resolver: resolving %d streams without TMDB IDs", len(toResolve))
			resolved := metadataWorker.ResolveStreams(ctx, toResolve)
			if len(resolved) > 0 {
				streamMap := make(map[string]*media.Stream)
				for i := range streams {
					streamMap[streams[i].ID] = &streams[i]
				}
				var updated []media.Stream
				for _, r := range resolved {
					if st, ok := streamMap[r.StreamID]; ok {
						st.TMDBID = strconv.Itoa(r.TMDBID)
						updated = append(updated, *st)
					}
				}
				if len(updated) > 0 {
					if err := streamStore.BulkUpsert(ctx, updated); err != nil {
						log.Printf("tmdb resolver: update streams: %v", err)
					} else {
						log.Printf("tmdb resolver: updated %d streams with TMDB IDs", len(updated))
					}
				}
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

