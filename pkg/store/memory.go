package store

import (
	"context"
	"sync"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

type MemoryStreamStore struct {
	streams map[string]media.Stream
	mu      sync.RWMutex
}

func NewMemoryStreamStore() *MemoryStreamStore {
	return &MemoryStreamStore{
		streams: make(map[string]media.Stream),
	}
}

func (s *MemoryStreamStore) Get(_ context.Context, id string) (*media.Stream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stream, ok := s.streams[id]
	if !ok {
		return nil, nil
	}
	return &stream, nil
}

func (s *MemoryStreamStore) List(_ context.Context) ([]media.Stream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]media.Stream, 0, len(s.streams))
	for _, stream := range s.streams {
		result = append(result, stream)
	}
	return result, nil
}

func (s *MemoryStreamStore) ListBySource(_ context.Context, sourceType, sourceID string) ([]media.Stream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []media.Stream
	for _, stream := range s.streams {
		if stream.SourceType == sourceType && stream.SourceID == sourceID {
			result = append(result, stream)
		}
	}
	return result, nil
}

func (s *MemoryStreamStore) BulkUpsert(_ context.Context, streams []media.Stream) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, stream := range streams {
		s.streams[stream.ID] = stream
	}
	return nil
}

func (s *MemoryStreamStore) DeleteBySource(_ context.Context, sourceType, sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, stream := range s.streams {
		if stream.SourceType == sourceType && stream.SourceID == sourceID {
			delete(s.streams, id)
		}
	}
	return nil
}

func (s *MemoryStreamStore) DeleteStaleBySource(_ context.Context, sourceType, sourceID string, keepIDs []string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	keep := make(map[string]struct{}, len(keepIDs))
	for _, id := range keepIDs {
		keep[id] = struct{}{}
	}

	var deleted []string
	for id, stream := range s.streams {
		if stream.SourceType != sourceType || stream.SourceID != sourceID {
			continue
		}
		if _, ok := keep[id]; !ok {
			delete(s.streams, id)
			deleted = append(deleted, id)
		}
	}
	return deleted, nil
}

func (s *MemoryStreamStore) Save() error {
	return nil
}

type MemorySettingsStore struct {
	settings map[string]string
	mu       sync.RWMutex
}

func NewMemorySettingsStore() *MemorySettingsStore {
	return &MemorySettingsStore{
		settings: make(map[string]string),
	}
}

func (s *MemorySettingsStore) Get(_ context.Context, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.settings[key], nil
}

func (s *MemorySettingsStore) Set(_ context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.settings[key] = value
	return nil
}

func (s *MemorySettingsStore) List(_ context.Context) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string, len(s.settings))
	for k, v := range s.settings {
		result[k] = v
	}
	return result, nil
}
