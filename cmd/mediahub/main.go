package main

import (
	"context"
	"log"
	"net/http"

	"github.com/mcnairstudios/mediahub/pkg/api"
	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/cache"
	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/config"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/session"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
	"github.com/mcnairstudios/mediahub/pkg/strategy"
)

func main() {
	cfg := config.Load()

	streamStore := store.NewMemoryStreamStore()
	settingsStore := store.NewMemorySettingsStore()
	channelStore := store.NewMemoryChannelStore()
	epgSourceStore := store.NewMemoryEPGSourceStore()
	recordingStore := store.NewMemoryRecordingStore()

	userStore := auth.NewMemoryUserStore()
	authService := auth.NewJWTService(userStore, "mediahub-secret-change-me")

	if _, err := authService.CreateUser(context.Background(), "admin", "admin", auth.RoleAdmin); err != nil {
		log.Fatalf("seeding admin user: %v", err)
	}

	sourceReg := source.NewRegistry()
	outputReg := output.NewRegistry()

	cacheReg := cache.NewRegistry()
	cacheReg.Register(tmdbcache.New())

	detector := client.NewDetector(nil)

	sessionMgr := session.NewManager(cfg.RecordDir)

	_ = cacheReg

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

	log.Printf("mediahub starting on %s", cfg.ListenAddr)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, server.Handler()))
}
