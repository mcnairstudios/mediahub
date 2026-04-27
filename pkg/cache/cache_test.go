package cache

import (
	"context"
	"sync"
	"testing"
)

type mockCache struct {
	mu    sync.RWMutex
	items map[string]any
}

func newMockCache() *mockCache {
	return &mockCache{items: make(map[string]any)}
}

func (m *mockCache) Type() CacheType { return CacheTMDB }

func (m *mockCache) Get(_ context.Context, key string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.items[key]
	return v, ok
}

func (m *mockCache) Set(_ context.Context, key string, value any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = value
	return nil
}

func (m *mockCache) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}

func (m *mockCache) Clear(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]any)
	return nil
}

func TestMockSatisfiesInterface(t *testing.T) {
	var _ Cache = newMockCache()
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	c := newMockCache()
	r.Register(c)

	got, ok := r.Get(CacheTMDB)
	if !ok {
		t.Fatal("expected to find registered cache")
	}
	if got.Type() != CacheTMDB {
		t.Fatalf("expected type %s, got %s", CacheTMDB, got.Type())
	}
}

func TestGetUnknownType(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get(CacheProbe)
	if ok {
		t.Fatal("expected false for unregistered cache type")
	}
}

func TestTypes(t *testing.T) {
	r := NewRegistry()

	probe := &typedCache{cacheType: CacheProbe}
	logo := &typedCache{cacheType: CacheLogo}
	r.Register(probe)
	r.Register(logo)

	types := r.Types()
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
	if types[0] != CacheLogo || types[1] != CacheProbe {
		t.Fatalf("expected sorted [logo probe], got %v", types)
	}
}

func TestSetGetDeleteCycle(t *testing.T) {
	ctx := context.Background()
	c := newMockCache()

	if err := c.Set(ctx, "movie:123", "The Matrix"); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	val, ok := c.Get(ctx, "movie:123")
	if !ok {
		t.Fatal("expected to find key after set")
	}
	if val != "The Matrix" {
		t.Fatalf("expected 'The Matrix', got %v", val)
	}

	if err := c.Delete(ctx, "movie:123"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, ok = c.Get(ctx, "movie:123")
	if ok {
		t.Fatal("expected key to be gone after delete")
	}
}

func TestClear(t *testing.T) {
	ctx := context.Background()
	c := newMockCache()

	_ = c.Set(ctx, "a", 1)
	_ = c.Set(ctx, "b", 2)

	if err := c.Clear(ctx); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	if _, ok := c.Get(ctx, "a"); ok {
		t.Fatal("expected cache to be empty after clear")
	}
	if _, ok := c.Get(ctx, "b"); ok {
		t.Fatal("expected cache to be empty after clear")
	}
}

type typedCache struct {
	mockCache
	cacheType CacheType
}

func (tc *typedCache) Type() CacheType { return tc.cacheType }
