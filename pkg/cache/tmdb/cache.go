package tmdb

import (
	"context"
	"sync"

	"github.com/mcnairstudios/mediahub/pkg/cache"
)

type Cache struct {
	movies map[string]*Movie
	series map[string]*Series
	mu     sync.RWMutex
}

func New() *Cache {
	return &Cache{
		movies: make(map[string]*Movie),
		series: make(map[string]*Series),
	}
}

func (c *Cache) Type() cache.CacheType {
	return cache.CacheTMDB
}

func (c *Cache) Get(_ context.Context, key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if m, ok := c.movies[key]; ok {
		return m, true
	}
	if s, ok := c.series[key]; ok {
		return s, true
	}
	return nil, false
}

func (c *Cache) Set(_ context.Context, key string, value any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch v := value.(type) {
	case *Movie:
		c.movies[key] = v
	case *Series:
		c.series[key] = v
	}
	return nil
}

func (c *Cache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.movies, key)
	delete(c.series, key)
	return nil
}

func (c *Cache) Clear(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.movies = make(map[string]*Movie)
	c.series = make(map[string]*Series)
	return nil
}

func (c *Cache) GetMovie(id string) (*Movie, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.movies[id]
	return m, ok
}

func (c *Cache) GetSeries(id string) (*Series, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.series[id]
	return s, ok
}

func (c *Cache) SetMovie(id string, m *Movie) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.movies[id] = m
}

func (c *Cache) SetSeries(id string, s *Series) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.series[id] = s
}
