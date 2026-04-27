package output

import (
	"fmt"
	"sync"
)

// PluginFactory creates an OutputPlugin from the given configuration.
type PluginFactory func(cfg PluginConfig) (OutputPlugin, error)

// Registry holds plugin factories keyed by delivery mode. Concrete plugins
// register themselves at init time; the pipeline creates plugins by mode.
type Registry struct {
	factories map[DeliveryMode]PluginFactory
	mu        sync.RWMutex
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[DeliveryMode]PluginFactory),
	}
}

// Register adds or replaces the factory for the given delivery mode.
func (r *Registry) Register(mode DeliveryMode, factory PluginFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factories[mode] = factory
}

// Create instantiates a plugin for the given delivery mode using the
// registered factory. Returns an error if no factory is registered.
func (r *Registry) Create(mode DeliveryMode, cfg PluginConfig) (OutputPlugin, error) {
	r.mu.RLock()
	factory, ok := r.factories[mode]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no plugin registered for mode %q", mode)
	}
	return factory(cfg)
}

// Modes returns all registered delivery modes.
func (r *Registry) Modes() []DeliveryMode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modes := make([]DeliveryMode, 0, len(r.factories))
	for mode := range r.factories {
		modes = append(modes, mode)
	}
	return modes
}
