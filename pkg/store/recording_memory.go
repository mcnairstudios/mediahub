package store

import (
	"context"
	"sync"

	"github.com/mcnairstudios/mediahub/pkg/recording"
)

type MemoryRecordingStore struct {
	recordings map[string]recording.Recording
	mu         sync.RWMutex
}

func NewMemoryRecordingStore() *MemoryRecordingStore {
	return &MemoryRecordingStore{
		recordings: make(map[string]recording.Recording),
	}
}

func (s *MemoryRecordingStore) Get(_ context.Context, id string) (*recording.Recording, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.recordings[id]
	if !ok {
		return nil, nil
	}
	return &r, nil
}

func (s *MemoryRecordingStore) List(_ context.Context, userID string, isAdmin bool) ([]recording.Recording, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []recording.Recording
	for _, r := range s.recordings {
		if isAdmin || r.UserID == userID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *MemoryRecordingStore) Create(_ context.Context, r *recording.Recording) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.recordings[r.ID] = *r
	return nil
}

func (s *MemoryRecordingStore) Update(_ context.Context, r *recording.Recording) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.recordings[r.ID] = *r
	return nil
}

func (s *MemoryRecordingStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.recordings, id)
	return nil
}

func (s *MemoryRecordingStore) ListByStatus(_ context.Context, status recording.Status) ([]recording.Recording, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []recording.Recording
	for _, r := range s.recordings {
		if r.Status == status {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *MemoryRecordingStore) ListScheduled(_ context.Context) ([]recording.Recording, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []recording.Recording
	for _, r := range s.recordings {
		if r.Status == recording.StatusScheduled {
			result = append(result, r)
		}
	}
	return result, nil
}
