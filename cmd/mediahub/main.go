package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/api"
	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/cache"
	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/config"
	"github.com/mcnairstudios/mediahub/pkg/connectivity"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/output/hls"
	"github.com/mcnairstudios/mediahub/pkg/output/mse"
	"github.com/mcnairstudios/mediahub/pkg/output/record"
	"github.com/mcnairstudios/mediahub/pkg/output/stream"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/source"
	boltstore "github.com/mcnairstudios/mediahub/pkg/store/bolt"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
	"github.com/mcnairstudios/mediahub/pkg/worker"
)

func main() {
	cfg := config.Load()

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
	epgSourceStore := db.EPGSourceStore()
	recordingStore := db.RecordingStore()
	userStore := db.UserStore()

	authService := auth.NewJWTService(userStore, "mediahub-secret-change-me")

	ctx := context.Background()
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

	sourceReg := source.NewRegistry()
	sourceReg.Register("m3u", func(_ context.Context, _ string) (source.Source, error) {
		return nil, errors.New("m3u sources are created via API with their config")
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
	_ = connReg

	cacheReg := cache.NewRegistry()
	cacheReg.Register(tmdbcache.New())

	detector := client.NewDetector(nil)

	sessionMgr := session.NewManager(cfg.RecordDir)

	scheduler := worker.NewScheduler(func(name string, err error) {
		log.Printf("worker %s error: %v", name, err)
	})

	scheduler.Add(worker.Job{
		Name:     "source-refresh",
		Interval: 6 * time.Hour,
		Fn: func(ctx context.Context) error {
			log.Println("source refresh: not yet wired to source instances")
			return nil
		},
	})

	server := api.NewServer(api.OrchestratorDeps{
		StreamStore:    streamStore,
		ChannelStore:   channelStore,
		SettingsStore:  settingsStore,
		SessionMgr:     sessionMgr,
		Detector:       detector,
		OutputReg:      outputReg,
		SourceReg:      sourceReg,
		RecordingStore: recordingStore,
		AuthService:    authService,
		EPGSourceStore: epgSourceStore,
		Strategy:       strategy.Resolve,
	})

	scheduler.Start(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("mediahub starting on %s (data: %s)", cfg.ListenAddr, cfg.DataDir)

	errCh := make(chan error, 1)
	go func() {
		errCh <- http.ListenAndServe(cfg.ListenAddr, server.Handler())
	}()

	select {
	case sig := <-sigCh:
		log.Printf("received %s, shutting down...", sig)
	case err := <-errCh:
		log.Printf("server error: %v", err)
	}

	scheduler.Stop()
	sessionMgr.StopAll()

	fmt.Println("mediahub stopped")
}
