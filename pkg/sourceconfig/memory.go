package sourceconfig

import (
	"context"
	"errors"
	"sync"
)

var ErrNotFound = errors.New("source config not found")

type MemoryStore struct {
	mu      sync.RWMutex
	configs map[string]SourceConfig
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{configs: make(map[string]SourceConfig)}
}

func (m *MemoryStore) Get(_ context.Context, id string) (*SourceConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sc, ok := m.configs[id]
	if !ok {
		return nil, nil
	}
	return &sc, nil
}

func (m *MemoryStore) List(_ context.Context) ([]SourceConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SourceConfig, 0, len(m.configs))
	for _, sc := range m.configs {
		result = append(result, sc)
	}
	return result, nil
}

func (m *MemoryStore) ListByType(_ context.Context, sourceType string) ([]SourceConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []SourceConfig
	for _, sc := range m.configs {
		if sc.Type == sourceType {
			result = append(result, sc)
		}
	}
	if result == nil {
		result = []SourceConfig{}
	}
	return result, nil
}

func (m *MemoryStore) Create(_ context.Context, sc *SourceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.configs[sc.ID] = *sc
	return nil
}

func (m *MemoryStore) Update(_ context.Context, sc *SourceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.configs[sc.ID]; !ok {
		return ErrNotFound
	}
	m.configs[sc.ID] = *sc
	return nil
}

func (m *MemoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.configs, id)
	return nil
}
