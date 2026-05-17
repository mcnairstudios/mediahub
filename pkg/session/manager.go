package session

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/output"
)

const idleTimeout = 30 * time.Second

var errSessionNotFound = errors.New("session not found")

type Manager struct {
	sessions  map[string]*Session
	mu        sync.RWMutex
	outputDir string
	stopCh    chan struct{}
}

func NewManager(outputDir string) *Manager {
	m := &Manager{
		sessions:  make(map[string]*Session),
		outputDir: outputDir,
		stopCh:    make(chan struct{}),
	}
	go m.reapIdle()
	return m
}

// reapIdle periodically checks for sessions with no viewer activity
// and stops them to free resources.
func (m *Manager) reapIdle() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.mu.RLock()
			var idle []string
			for id, s := range m.sessions {
				if s.IdleDuration() > idleTimeout && !s.IsRecorded() {
					idle = append(idle, id)
				}
			}
			m.mu.RUnlock()

			for _, id := range idle {
				m.mu.Lock()
				s, ok := m.sessions[id]
				if ok && s.IdleDuration() > idleTimeout {
					delete(m.sessions, id)
					log.Printf("session idle timeout: %s (%s)", id, s.StreamName)
					go s.Stop()
				}
				m.mu.Unlock()
			}
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) GetOrCreate(_ context.Context, streamID, streamURL, streamName string) (*Session, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[streamID]; ok {
		if s.IsFinished() {
			delete(m.sessions, streamID)
			s.Stop()
		} else {
			return s, false, nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := newSession(ctx, cancel, streamID, streamURL, streamName, m.outputDir)
	m.sessions[streamID] = s
	return s, true, nil
}

func (m *Manager) Get(streamID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[streamID]
}

func (m *Manager) Stop(streamID string) {
	m.mu.Lock()
	s, ok := m.sessions[streamID]
	if ok {
		delete(m.sessions, streamID)
	}
	m.mu.Unlock()

	if ok {
		s.Stop()
	}
}

func (m *Manager) StopAll() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}

	m.mu.Lock()
	sessions := m.sessions
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	for _, s := range sessions {
		s.Stop()
	}
}

func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	return list
}

func (m *Manager) AddPlugin(streamID string, plugin output.OutputPlugin) error {
	m.mu.RLock()
	s, ok := m.sessions[streamID]
	m.mu.RUnlock()

	if !ok {
		return errSessionNotFound
	}
	s.FanOut.Add(plugin)
	return nil
}

func (m *Manager) RemovePlugin(streamID string, mode output.DeliveryMode) error {
	m.mu.RLock()
	s, ok := m.sessions[streamID]
	m.mu.RUnlock()

	if !ok {
		return errSessionNotFound
	}
	s.FanOut.Remove(mode)
	return nil
}
