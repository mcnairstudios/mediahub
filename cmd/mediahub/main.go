package main

import (
	"context"
	"encoding/json"
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
	"github.com/mcnairstudios/mediahub/pkg/sourceprofile"
	hdhrsource "github.com/mcnairstudios/mediahub/pkg/source/hdhr"
	m3usource "github.com/mcnairstudios/mediahub/pkg/source/m3u"
	satipsource "github.com/mcnairstudios/mediahub/pkg/source/satip"
	tvpstreamssource "github.com/mcnairstudios/mediahub/pkg/source/tvpstreams"
	xstreamsource "github.com/mcnairstudios/mediahub/pkg/source/xtream"
	"github.com/mcnairstudios/mediahub/pkg/orchestrator"
	recscheduler "github.com/mcnairstudios/mediahub/pkg/scheduler"
	"github.com/mcnairstudios/mediahub/pkg/tmdb"
	boltstore "github.com/mcnairstudios/mediahub/pkg/store/bolt"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
	"github.com/mcnairstudios/mediahub/pkg/worker"
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

	authService := auth.NewJWTService(userStore, "mediahub-secret-change-me")

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

	wgService := wg.NewService(settingsStore, wg.PluginConfig{
		UserAgent:    cfg.UserAgent,
		BypassHeader: cfg.BypassHeader,
		BypassSecret: cfg.BypassSecret,
	})

	tmdbCache := tmdbcache.NewPersistent(filepath.Join(cfg.DataDir, "tmdb_cache"))

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
			InitialETag:  sc.Config["etag"],
			StreamStore:  streamStore,
			OnRefreshDone: func(sourceID, etag string, streamCount int) {
				scUpd, err := sourceConfigStore.Get(ctx, sourceID)
				if err != nil || scUpd == nil {
					return
				}
				scUpd.Config["etag"] = etag
				scUpd.Config["stream_count"] = fmt.Sprintf("%d", streamCount)
				scUpd.Config["last_refreshed"] = time.Now().Format(time.RFC3339)
				sourceConfigStore.Update(ctx, scUpd)
			},
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
			InitialETag:     sc.Config["etag"],
			OnRefreshDone: func(sourceID, etag string, streamCount int) {
				scUpd, err := sourceConfigStore.Get(ctx, sourceID)
				if err != nil || scUpd == nil {
					return
				}
				scUpd.Config["etag"] = etag
				scUpd.Config["stream_count"] = fmt.Sprintf("%d", streamCount)
				scUpd.Config["last_refreshed"] = time.Now().Format(time.RFC3339)
				sourceConfigStore.Update(ctx, scUpd)
			},
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

	sessionMgr := session.NewManager(cfg.RecordDir)

	scheduler := worker.NewScheduler(func(name string, err error) {
		log.Printf("worker %s error: %v", name, err)
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
	scheduler.Add(worker.Job{
		Name:     "source-refresh",
		Interval: 1 * time.Minute,
		Fn: func(ctx context.Context) error {
			errs := orchestrator.RefreshAll(ctx, refreshDeps)
			if len(errs) > 0 {
				return fmt.Errorf("source refresh: %d errors, last: %w", len(errs), errs[len(errs)-1])
			}
			return nil
		},
	})

	recScheduler := recscheduler.New(recordingStore)
	recDeps := orchestrator.RecordingDeps{
		SessionMgr:     sessionMgr,
		RecordingStore: recordingStore,
		OutputReg:      outputReg,
		RecordDir:      cfg.RecordDir,
	}
	recScheduler.SetStartFunc(func(streamID, title string) error {
		return orchestrator.StartRecording(ctx, recDeps, streamID, title, "system", false)
	})
	recScheduler.SetStopFunc(func(streamID string) error {
		return orchestrator.StopRecording(ctx, recDeps, streamID)
	})
	orchestrator.RecoverRecordings(ctx, recDeps)
	recScheduler.Start(ctx)

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
		Config:            cfg,
		StaticFS:          staticFS,
		UserAgent:         cfg.UserAgent,
		BypassHeader:      cfg.BypassHeader,
		BypassSecret:      cfg.BypassSecret,
		DBClearer:         db,
	})

	epgRefreshFn = apiServer.RefreshEPGSource

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

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				enqueueTMDBStreams()
			}
		}
	}()

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

