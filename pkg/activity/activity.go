package activity

import (
	"sync"
	"time"
)

type Viewer struct {
	SessionID  string    `json:"session_id"`
	StreamID   string    `json:"stream_id"`
	StreamName string    `json:"stream_name"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	ClientName string    `json:"client_name"`
	Delivery   string    `json:"delivery"`
	StartedAt  time.Time `json:"started_at"`
	RemoteAddr string    `json:"remote_addr"`
}

type Service struct {
	viewers map[string]*Viewer
	mu      sync.RWMutex
}

func New() *Service {
	return &Service{
		viewers: make(map[string]*Viewer),
	}
}

func (s *Service) Add(v *Viewer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.viewers[v.SessionID] = v
}

func (s *Service) Remove(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.viewers, sessionID)
}

func (s *Service) List() []*Viewer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Viewer, 0, len(s.viewers))
	for _, v := range s.viewers {
		result = append(result, v)
	}
	return result
}

func (s *Service) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.viewers)
}
