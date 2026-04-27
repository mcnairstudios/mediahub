package orchestrator

import (
	"context"
	"fmt"

	"github.com/mcnairstudios/mediahub/pkg/source"
)

type RefreshDeps struct {
	SourceReg *source.Registry
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
	var errs []error
	for _, st := range deps.SourceReg.Types() {
		src, err := deps.SourceReg.Create(ctx, st, "")
		if err != nil {
			errs = append(errs, fmt.Errorf("create source type %s: %w", st, err))
			continue
		}
		if err := src.Refresh(ctx); err != nil {
			errs = append(errs, fmt.Errorf("refresh source type %s: %w", st, err))
		}
	}
	return errs
}
