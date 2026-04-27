package store

import (
	"context"
	"sync"

	"github.com/mcnairstudios/mediahub/pkg/channel"
)

type MemoryChannelStore struct {
	channels map[string]channel.Channel
	mu       sync.RWMutex
}

func NewMemoryChannelStore() *MemoryChannelStore {
	return &MemoryChannelStore{
		channels: make(map[string]channel.Channel),
	}
}

func (s *MemoryChannelStore) Get(_ context.Context, id string) (*channel.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.channels[id]
	if !ok {
		return nil, nil
	}
	return &ch, nil
}

func (s *MemoryChannelStore) List(_ context.Context) ([]channel.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]channel.Channel, 0, len(s.channels))
	for _, ch := range s.channels {
		result = append(result, ch)
	}
	return result, nil
}

func (s *MemoryChannelStore) Create(_ context.Context, ch *channel.Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.channels[ch.ID] = *ch
	return nil
}

func (s *MemoryChannelStore) Update(_ context.Context, ch *channel.Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.channels[ch.ID] = *ch
	return nil
}

func (s *MemoryChannelStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.channels, id)
	return nil
}

func (s *MemoryChannelStore) AssignStreams(_ context.Context, channelID string, streamIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, ok := s.channels[channelID]
	if !ok {
		return nil
	}
	ch.StreamIDs = make([]string, len(streamIDs))
	copy(ch.StreamIDs, streamIDs)
	s.channels[channelID] = ch
	return nil
}

func (s *MemoryChannelStore) RemoveStreamMappings(_ context.Context, streamIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	remove := make(map[string]struct{}, len(streamIDs))
	for _, id := range streamIDs {
		remove[id] = struct{}{}
	}

	for chID, ch := range s.channels {
		var filtered []string
		for _, sid := range ch.StreamIDs {
			if _, ok := remove[sid]; !ok {
				filtered = append(filtered, sid)
			}
		}
		ch.StreamIDs = filtered
		s.channels[chID] = ch
	}
	return nil
}

type MemoryGroupStore struct {
	groups map[string]channel.Group
	mu     sync.RWMutex
}

func NewMemoryGroupStore() *MemoryGroupStore {
	return &MemoryGroupStore{
		groups: make(map[string]channel.Group),
	}
}

func (s *MemoryGroupStore) List(_ context.Context) ([]channel.Group, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]channel.Group, 0, len(s.groups))
	for _, g := range s.groups {
		result = append(result, g)
	}
	return result, nil
}

func (s *MemoryGroupStore) Create(_ context.Context, g *channel.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.groups[g.ID] = *g
	return nil
}

func (s *MemoryGroupStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.groups, id)
	return nil
}
