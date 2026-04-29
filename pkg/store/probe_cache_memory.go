package store

import (
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

const memoryCacheTTL = 24 * time.Hour

type memoryCacheEntry struct {
	result   *media.ProbeResult
	storedAt time.Time
}

type MemoryProbeCache struct {
	entries map[string]memoryCacheEntry
	mu      sync.RWMutex
}

func NewMemoryProbeCache() *MemoryProbeCache {
	return &MemoryProbeCache{
		entries: make(map[string]memoryCacheEntry),
	}
}

func (c *MemoryProbeCache) Get(url string) (*media.ProbeResult, error) {
	c.mu.RLock()
	entry, ok := c.entries[url]
	c.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	if time.Since(entry.storedAt) > memoryCacheTTL {
		c.mu.Lock()
		delete(c.entries, url)
		c.mu.Unlock()
		return nil, nil
	}

	return entry.result, nil
}

func (c *MemoryProbeCache) Set(url string, result *media.ProbeResult) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[url] = memoryCacheEntry{
		result:   result,
		storedAt: time.Now(),
	}
	return nil
}

func (c *MemoryProbeCache) Delete(url string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, url)
	return nil
}
