package source

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrUnknownSourceType is returned when a factory is requested for an
// unregistered source type.
var ErrUnknownSourceType = errors.New("unknown source type")

// Factory creates a Source instance for the given source ID.
// The implementation should read configuration from its own backing store.
type Factory func(ctx context.Context, sourceID string) (Source, error)

// Registry holds the mapping from source types to their factory functions.
// Source type plugins register themselves at startup; the session/service
// layer uses Create to instantiate sources by type.
type Registry struct {
	mu        sync.RWMutex
	factories map[SourceType]Factory
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[SourceType]Factory),
	}
}

// Register adds or replaces a factory for the given source type.
func (r *Registry) Register(st SourceType, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[st] = factory
}

// Create instantiates a Source using the registered factory for the given type.
// Returns ErrUnknownSourceType if no factory is registered.
func (r *Registry) Create(ctx context.Context, st SourceType, sourceID string) (Source, error) {
	r.mu.RLock()
	factory, ok := r.factories[st]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownSourceType, st)
	}
	return factory(ctx, sourceID)
}

// Types returns all registered source types in no particular order.
func (r *Registry) Types() []SourceType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]SourceType, 0, len(r.factories))
	for st := range r.factories {
		types = append(types, st)
	}
	return types
}
