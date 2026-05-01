package store

import (
	"context"
	"sync"

	"github.com/mcnairstudios/mediahub/pkg/frontend/hdhr"
)

type MemoryHDHRDeviceStore struct {
	devices map[string]hdhr.Device
	mu      sync.RWMutex
}

func NewMemoryHDHRDeviceStore() *MemoryHDHRDeviceStore {
	return &MemoryHDHRDeviceStore{
		devices: make(map[string]hdhr.Device),
	}
}

func (s *MemoryHDHRDeviceStore) Get(_ context.Context, id string) (*hdhr.Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.devices[id]
	if !ok {
		return nil, nil
	}
	return &d, nil
}

func (s *MemoryHDHRDeviceStore) List(_ context.Context) ([]hdhr.Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]hdhr.Device, 0, len(s.devices))
	for _, d := range s.devices {
		result = append(result, d)
	}
	return result, nil
}

func (s *MemoryHDHRDeviceStore) Create(_ context.Context, d *hdhr.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.devices[d.ID] = *d
	return nil
}

func (s *MemoryHDHRDeviceStore) Update(_ context.Context, d *hdhr.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.devices[d.ID] = *d
	return nil
}

func (s *MemoryHDHRDeviceStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.devices, id)
	return nil
}
