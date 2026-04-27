package connectivity

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
)

var ErrUnknownPlugin = errors.New("unknown connectivity plugin")

type Plugin interface {
	Name() string
	ProxyURL(upstreamURL string) string
	HTTPClient() *http.Client
	IsConnected() bool
	Close() error
}

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	active  Plugin
}

func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

func (r *Registry) Register(p Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[p.Name()] = p
}

func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

func (r *Registry) Active() Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active
}

func (r *Registry) SetActive(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownPlugin, name)
	}
	r.active = p
	return nil
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}
