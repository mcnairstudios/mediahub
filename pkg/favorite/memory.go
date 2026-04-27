package favorite

import (
	"context"
	"sync"
	"time"
)

type MemoryStore struct {
	favorites map[string]map[string]Favorite
	mu        sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		favorites: make(map[string]map[string]Favorite),
	}
}

func (s *MemoryStore) List(_ context.Context, userID string) ([]Favorite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userFavs, ok := s.favorites[userID]
	if !ok {
		return nil, nil
	}

	result := make([]Favorite, 0, len(userFavs))
	for _, f := range userFavs {
		result = append(result, f)
	}
	return result, nil
}

func (s *MemoryStore) Add(_ context.Context, userID, streamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.favorites[userID] == nil {
		s.favorites[userID] = make(map[string]Favorite)
	}

	if _, exists := s.favorites[userID][streamID]; exists {
		return nil
	}

	s.favorites[userID][streamID] = Favorite{
		StreamID: streamID,
		UserID:   userID,
		AddedAt:  time.Now(),
	}
	return nil
}

func (s *MemoryStore) Remove(_ context.Context, userID, streamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userFavs, ok := s.favorites[userID]; ok {
		delete(userFavs, streamID)
	}
	return nil
}

func (s *MemoryStore) IsFavorite(_ context.Context, userID, streamID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userFavs, ok := s.favorites[userID]
	if !ok {
		return false, nil
	}
	_, exists := userFavs[streamID]
	return exists, nil
}
