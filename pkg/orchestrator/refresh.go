package orchestrator

import (
	"context"
	"fmt"
	"log"

	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
)

type RefreshDeps struct {
	SourceReg         *source.Registry
	SourceConfigStore sourceconfig.Store
}

func RefreshSource(ctx context.Context, deps RefreshDeps, sourceType source.SourceType, sourceID string) error {
	src, err := deps.SourceReg.Create(ctx, sourceType, sourceID)
	if err != nil {
		return fmt.Errorf("create source: %w", err)
	}
	if err := src.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh source %s: %w", sourceID, err)
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
