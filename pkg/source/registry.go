package source

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

var ErrUnknownSourceType = errors.New("unknown source type")

// DefaultRegistry is a package-level registry for self-registering plugins.
var DefaultRegistry = NewRegistry()

type Factory func(ctx context.Context, sourceID string) (Source, error)

type Registry struct {
	mu        sync.RWMutex
	factories map[SourceType]Factory
	plugins   map[SourceType]*PluginRegistration
}

func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[SourceType]Factory),
		plugins:   make(map[SourceType]*PluginRegistration),
	}
}

func (r *Registry) Register(st SourceType, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[st] = factory
}

// RegisterPlugin stores a plugin registration, including its descriptor and
// optional factory/routes/frontend JS. If the registration includes a factory,
// it is also stored in the factories map.
func (r *Registry) RegisterPlugin(reg PluginRegistration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[reg.Descriptor.Type] = &reg
	if reg.Factory != nil {
		r.factories[reg.Descriptor.Type] = reg.Factory
	}
}

// Plugin returns the registration for the given source type, or nil.
func (r *Registry) Plugin(st SourceType) *PluginRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[st]
}

// Plugins returns all registered plugin registrations, sorted by type for
// deterministic output.
func (r *Registry) Plugins() []*PluginRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*PluginRegistration, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Descriptor.Type < result[j].Descriptor.Type
	})
	return result
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
