package activity

import (
	"sync"
	"time"
)

const sessionTimeout = 20 * time.Minute

type Viewer struct {
	SessionID   string    `json:"session_id"`
	StreamID    string    `json:"stream_id"`
	StreamName  string    `json:"stream_name"`
	ChannelID   string    `json:"channel_id,omitempty"`
	ChannelName string    `json:"channel_name,omitempty"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	ClientName  string    `json:"client_name"`
	Delivery    string    `json:"delivery"`
	StartedAt   time.Time `json:"started_at"`
	RemoteAddr  string    `json:"remote_addr"`
	VideoCodec  string    `json:"video_codec,omitempty"`
	AudioCodec  string    `json:"audio_codec,omitempty"`
	Resolution  string    `json:"resolution,omitempty"`
	Transcoding bool      `json:"transcoding"`
}

type UserSession struct {
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	Source     string    `json:"source"`
	RemoteAddr string    `json:"remote_addr"`
	UserAgent  string    `json:"user_agent"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
}

type Service struct {
	viewers  map[string]*Viewer
	sessions map[string]*UserSession
	mu       sync.RWMutex
}

func New() *Service {
	return &Service{
		viewers:  make(map[string]*Viewer),
		sessions: make(map[string]*UserSession),
	}
}

func (s *Service) TouchUser(userID, username, source, remoteAddr, userAgent string) {
	key := userID + ":" + source
	s.mu.Lock()
	now := time.Now()
	if sess, ok := s.sessions[key]; ok {
		sess.LastSeen = now
		sess.RemoteAddr = remoteAddr
		sess.UserAgent = userAgent
	} else {
		s.sessions[key] = &UserSession{
			UserID:     userID,
			Username:   username,
			Source:     source,
			RemoteAddr: remoteAddr,
			UserAgent:  userAgent,
			FirstSeen:  now,
			LastSeen:   now,
		}
	}
	s.mu.Unlock()
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

func (s *Service) RecentUsers() []*UserSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	result := make([]*UserSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		if now.Sub(sess.LastSeen) <= sessionTimeout {
			result = append(result, sess)
		}
	}
	return result
}

func (s *Service) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.viewers)
}
