package source

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrUnknownSourceType = errors.New("unknown source type")

type Factory func(ctx context.Context, sourceID string) (Source, error)

type Registry struct {
	mu        sync.RWMutex
	factories map[SourceType]Factory
}

func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[SourceType]Factory),
	}
}

func (r *Registry) Register(st SourceType, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[st] = factory
}

func (r *Registry) Create(ctx context.Context, st SourceType, sourceID string) (Source, error) {
	r.mu.RLock()
	factory, ok := r.factories[st]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownSourceType, st)
	}
	return factory(ctx, sourceID)
}

func (r *Registry) Types() []SourceType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]SourceType, 0, len(r.factories))
	for st := range r.factories {
		types = append(types, st)
	}
	return types
}
