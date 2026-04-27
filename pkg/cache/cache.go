package cache

import (
	"context"
	"sort"
	"sync"
)

type CacheType string

const (
	CacheTMDB  CacheType = "tmdb"
	CacheProbe CacheType = "probe"
	CacheLogo  CacheType = "logo"
)

type Cache interface {
	Type() CacheType
	Get(ctx context.Context, key string) (any, bool)
	Set(ctx context.Context, key string, value any) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

type Registry struct {
	mu     sync.RWMutex
	caches map[CacheType]Cache
}

func NewRegistry() *Registry {
	return &Registry{
		caches: make(map[CacheType]Cache),
	}
}

func (r *Registry) Register(c Cache) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.caches[c.Type()] = c
}

func (r *Registry) Get(cacheType CacheType) (Cache, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.caches[cacheType]
	return c, ok
}

func (r *Registry) Types() []CacheType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]CacheType, 0, len(r.caches))
	for t := range r.caches {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool {
		return types[i] < types[j]
	})
	return types
}
