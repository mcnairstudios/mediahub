package orchestrator

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
)

type RefreshDeps struct {
	SourceReg         *source.Registry
	SourceConfigStore sourceconfig.Store
}

var (
	lastRefreshed   = make(map[string]time.Time)
	lastRefreshedMu sync.Mutex
)

func ResetRefreshTracking() {
	lastRefreshedMu.Lock()
	lastRefreshed = make(map[string]time.Time)
	lastRefreshedMu.Unlock()
}

var intervalDurations = map[string]time.Duration{
	"minute":  1 * time.Minute,
	"hourly":  1 * time.Hour,
	"daily":   24 * time.Hour,
	"weekly":  7 * 24 * time.Hour,
}

func RefreshSource(ctx context.Context, deps RefreshDeps, sourceType source.SourceType, sourceID string) error {
	src, err := deps.SourceReg.Create(ctx, sourceType, sourceID)
	if err != nil {
		return fmt.Errorf("create source: %w", err)
	}
	if err := src.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh source %s: %w", sourceID, err)
	}
	lastRefreshedMu.Lock()
	lastRefreshed[sourceID] = time.Now()
	lastRefreshedMu.Unlock()

	if deps.SourceConfigStore != nil {
		info := src.Info(ctx)
		sc, err := deps.SourceConfigStore.Get(ctx, sourceID)
		if err == nil && sc != nil {
			sc.Config["stream_count"] = fmt.Sprintf("%d", info.StreamCount)
			deps.SourceConfigStore.Update(ctx, sc)
		}
	}

	return nil
}

func RefreshAll(ctx context.Context, deps RefreshDeps) []error {
	if deps.SourceConfigStore == nil {
		return []error{fmt.Errorf("source config store not set")}
	}
	configs, err := deps.SourceConfigStore.List(ctx)
	if err != nil {
		return []error{fmt.Errorf("listing source configs: %w", err)}
	}
	var errs []error
	for _, cfg := range configs {
		if !cfg.IsEnabled {
			continue
		}
		interval := cfg.Config["refresh_interval"]
		if interval == "" || interval == "none" {
			continue
		}
		dur, ok := intervalDurations[interval]
		if !ok {
			continue
		}

		lastRefreshedMu.Lock()
		last, exists := lastRefreshed[cfg.ID]
		lastRefreshedMu.Unlock()

		if exists && time.Since(last) < dur {
			continue
		}

		log.Printf("source-refresh: refreshing %s (%s)", cfg.Name, cfg.Type)
		if err := RefreshSource(ctx, deps, source.SourceType(cfg.Type), cfg.ID); err != nil {
			log.Printf("source-refresh: failed %s (%s): %v", cfg.Name, cfg.Type, err)
			errs = append(errs, err)
		} else {
			log.Printf("source-refresh: completed %s (%s)", cfg.Name, cfg.Type)
		}
	}
	return errs
}
