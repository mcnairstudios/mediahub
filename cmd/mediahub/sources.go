package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"

	tmdbcache "github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/config"
	"github.com/mcnairstudios/mediahub/pkg/connectivity/wg"
	"github.com/mcnairstudios/mediahub/pkg/source"
	hdhrsource "github.com/mcnairstudios/mediahub/pkg/source/hdhr"
	m3usource "github.com/mcnairstudios/mediahub/pkg/source/m3u"
	satipsource "github.com/mcnairstudios/mediahub/pkg/source/satip"
	demosource "github.com/mcnairstudios/mediahub/pkg/source/demo"
	spacexsource "github.com/mcnairstudios/mediahub/pkg/source/spacex"
	trailerssource "github.com/mcnairstudios/mediahub/pkg/source/trailers"
	tvpstreamssource "github.com/mcnairstudios/mediahub/pkg/source/tvpstreams"
	xstreamsource "github.com/mcnairstudios/mediahub/pkg/source/xtream"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

type sourceDeps struct {
	SourceConfigStore sourceconfig.Store
	StreamStore       store.StreamStore
	SettingsStore     store.SettingsStore
	Config            *config.Config
	WGService         *wg.Service
	TMDBCache         *tmdbcache.Cache
	OnRefreshDone     func(string, string, int)
}

func registerSources(reg *source.Registry, deps sourceDeps) {
	reg.Register(source.TypeM3U, func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		m3uCfg := m3usource.Config{
			ID:            sc.ID,
			Name:          sc.Name,
			URL:           sc.Config["url"],
			IsEnabled:     sc.IsEnabled,
			UseWireGuard:  sc.Config["use_wireguard"] == "true",
			WGProfileID:   sc.Config["wg_profile_id"],
			UserAgent:     deps.Config.UserAgent,
			BypassHeader:  deps.Config.BypassHeader,
			BypassSecret:  deps.Config.BypassSecret,
			InitialETag:   sc.Config["etag"],
			StreamStore:   deps.StreamStore,
			OnRefreshDone: deps.OnRefreshDone,
		}
		m3uCfg.WGClient = resolveWGClient(deps.WGService, m3uCfg.UseWireGuard, m3uCfg.WGProfileID)
		return m3usource.New(m3uCfg), nil
	})

	reg.Register(source.TypeTVPStreams, func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
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
			DataDir:         deps.Config.DataDir,
			EnrollmentToken: sc.Config["enrollment_token"],
			TLSEnrolled:     sc.Config["tls_enrolled"] == "true",
			BypassHeader:    deps.Config.BypassHeader,
			BypassSecret:    deps.Config.BypassSecret,
			StreamStore:     deps.StreamStore,
			TMDBCache:       deps.TMDBCache,
			InitialETag:     sc.Config["etag"],
			OnRefreshDone:   deps.OnRefreshDone,
			OnEnrolled: func(sourceID string) error {
				scUpd, err := deps.SourceConfigStore.Get(ctx, sourceID)
				if err != nil || scUpd == nil {
					return err
				}
				scUpd.Config["tls_enrolled"] = "true"
				scUpd.Config["enrollment_token"] = ""
				return deps.SourceConfigStore.Update(ctx, scUpd)
			},
		}
		wgProfileID := sc.Config["wg_profile_id"]
		tvpCfg.WGClient = resolveWGClient(deps.WGService, tvpCfg.UseWireGuard, wgProfileID)
		return tvpstreamssource.New(tvpCfg), nil
	})

	reg.Register(source.TypeXtream, func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
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
			StreamStore:   deps.StreamStore,
			OnRefreshDone: deps.OnRefreshDone,
		}
		xtCfg.WGClient = resolveWGClient(deps.WGService, xtCfg.UseWireGuard, sc.Config["wg_profile_id"])
		return xstreamsource.New(xtCfg), nil
	})

	reg.Register(source.TypeHDHR, func(ctx context.Context, sourceID string) (source.Source, error) {
		if sourceID == "" {
			return hdhrsource.New(hdhrsource.Config{
				StreamStore: deps.StreamStore,
			}), nil
		}
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
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
			StreamStore: deps.StreamStore,
		}
		return hdhrsource.New(hdhrCfg), nil
	})

	reg.Register(source.TypeSATIP, func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
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
			StreamStore:     deps.StreamStore,
		}
		return satipsource.New(satipCfg), nil
	})

	reg.Register(source.TypeTrailers, func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		tmdbKey, _ := deps.SettingsStore.Get(ctx, "tmdb_api_key")
		tCfg := trailerssource.Config{
			ID:          sc.ID,
			Name:        sc.Name,
			IsEnabled:   sc.IsEnabled,
			TMDBKey:     tmdbKey,
			StreamStore: deps.StreamStore,
		}
		return trailerssource.New(tCfg), nil
	})

	reg.Register(source.TypeDemo, func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		demoCfg := demosource.Config{
			ID:          sc.ID,
			Name:        sc.Name,
			IsEnabled:   sc.IsEnabled,
			StreamStore: deps.StreamStore,
		}
		return demosource.New(demoCfg), nil
	})

	reg.Register(source.TypeSpaceX, func(ctx context.Context, sourceID string) (source.Source, error) {
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("get source config: %w", err)
		}
		if sc == nil {
			return nil, errors.New("source config not found")
		}
		sxCfg := spacexsource.Config{
			ID:          sc.ID,
			Name:        sc.Name,
			IsEnabled:   sc.IsEnabled,
			StreamStore: deps.StreamStore,
		}
		return spacexsource.New(sxCfg), nil
	})
}
