package tmdb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mcnairstudios/mediahub/pkg/cache"
)

type Cache struct {
	movies map[string]*Movie
	series map[string]*Series
	mu     sync.RWMutex
	dir    string
}

func New() *Cache {
	return &Cache{
		movies: make(map[string]*Movie),
		series: make(map[string]*Series),
	}
}

func NewPersistent(dir string) *Cache {
	os.MkdirAll(dir, 0755)
	c := &Cache{
		movies: make(map[string]*Movie),
		series: make(map[string]*Series),
		dir:    dir,
	}
	c.loadFromDisk()
	return c
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
		c.persistKey(key, "movie", v)
	case *Series:
		c.series[key] = v
		c.persistKey(key, "series", v)
	}
	return nil
}

func (c *Cache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.movies, key)
	delete(c.series, key)
	if c.dir != "" {
		os.Remove(filepath.Join(c.dir, sanitizeKey(key)+".json"))
	}
	return nil
}

func (c *Cache) Clear(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dir != "" {
		for k := range c.movies {
			os.Remove(filepath.Join(c.dir, sanitizeKey(k)+".json"))
		}
		for k := range c.series {
			os.Remove(filepath.Join(c.dir, sanitizeKey(k)+".json"))
		}
	}
	c.movies = make(map[string]*Movie)
	c.series = make(map[string]*Series)
	return nil
}

func (c *Cache) GetMovie(id string) (*Movie, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.movies[sanitizeKey(id)]
	return m, ok
}

func (c *Cache) GetSeries(id string) (*Series, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.series[sanitizeKey(id)]
	return s, ok
}

func (c *Cache) SetMovie(id string, m *Movie) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.movies[sanitizeKey(id)] = m
	c.persistKey(id, "movie", m)
}

func (c *Cache) SetSeries(id string, s *Series) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.series[sanitizeKey(id)] = s
	c.persistKey(id, "series", s)
}

func (c *Cache) persistKey(id, kind string, value any) {
	if c.dir == "" {
		return
	}
	wrapper := struct {
		Kind  string `json:"kind"`
		Value any    `json:"value"`
	}{Kind: kind, Value: value}
	raw, err := json.Marshal(wrapper)
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(c.dir, sanitizeKey(id)+".json"), raw, 0644)
}

func (c *Cache) loadFromDisk() {
	if c.dir == "" {
		return
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(c.dir, e.Name()))
		if err != nil {
			continue
		}
		var wrapper struct {
			Kind  string          `json:"kind"`
			Value json.RawMessage `json:"value"`
		}
		if json.Unmarshal(data, &wrapper) != nil {
			continue
		}
		key := strings.TrimSuffix(e.Name(), ".json")
		switch wrapper.Kind {
		case "movie":
			var m Movie
			if json.Unmarshal(wrapper.Value, &m) == nil {
				c.movies[key] = &m
			}
		case "series":
			var s Series
			if json.Unmarshal(wrapper.Value, &s) == nil {
				c.series[key] = &s
			}
		}
	}
}

func sanitizeKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:16])
}
