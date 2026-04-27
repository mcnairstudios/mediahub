package store

import (
	"context"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/epg"
)

type MemoryEPGSourceStore struct {
	sources map[string]epg.Source
	mu      sync.RWMutex
}

func NewMemoryEPGSourceStore() *MemoryEPGSourceStore {
	return &MemoryEPGSourceStore{
		sources: make(map[string]epg.Source),
	}
}

func (s *MemoryEPGSourceStore) Get(_ context.Context, id string) (*epg.Source, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	src, ok := s.sources[id]
	if !ok {
		return nil, nil
	}
	return &src, nil
}

func (s *MemoryEPGSourceStore) List(_ context.Context) ([]epg.Source, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]epg.Source, 0, len(s.sources))
	for _, src := range s.sources {
		result = append(result, src)
	}
	return result, nil
}

func (s *MemoryEPGSourceStore) Create(_ context.Context, src *epg.Source) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sources[src.ID] = *src
	return nil
}

func (s *MemoryEPGSourceStore) Update(_ context.Context, src *epg.Source) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sources[src.ID] = *src
	return nil
}

func (s *MemoryEPGSourceStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sources, id)
	return nil
}

type MemoryProgramStore struct {
	programs []epg.Program
	mu       sync.RWMutex
}

func NewMemoryProgramStore() *MemoryProgramStore {
	return &MemoryProgramStore{}
}

func (s *MemoryProgramStore) NowPlaying(_ context.Context, channelID string) (*epg.Program, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	for _, p := range s.programs {
		if p.ChannelID == channelID && !now.Before(p.StartTime) && now.Before(p.EndTime) {
			return &p, nil
		}
	}
	return nil, nil
}

func (s *MemoryProgramStore) Range(_ context.Context, channelID string, start, end time.Time) ([]epg.Program, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []epg.Program
	for _, p := range s.programs {
		if p.ChannelID != channelID {
			continue
		}
		if p.StartTime.Before(end) && p.EndTime.After(start) {
			result = append(result, p)
		}
	}
	return result, nil
}

func (s *MemoryProgramStore) ListAll(_ context.Context) ([]epg.Program, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]epg.Program, len(s.programs))
	copy(result, s.programs)
	return result, nil
}

func (s *MemoryProgramStore) BulkInsert(_ context.Context, programs []epg.Program) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.programs = append(s.programs, programs...)
	return nil
}

func (s *MemoryProgramStore) DeleteBySource(_ context.Context, sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var kept []epg.Program
	for _, p := range s.programs {
		if p.ChannelID != sourceID {
			kept = append(kept, p)
		}
	}
	s.programs = kept
	return nil
}
