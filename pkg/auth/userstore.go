package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUsernameExists    = errors.New("username already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired      = errors.New("token expired")
	ErrTokenInvalid      = errors.New("token invalid")
)

type storedUser struct {
	User         User
	PasswordHash string
}

type MemoryUserStore struct {
	users map[string]storedUser
	mu    sync.RWMutex
}

func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{
		users: make(map[string]storedUser),
	}
}

func (s *MemoryUserStore) Get(_ context.Context, id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	su, ok := s.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	u := su.User
	return &u, nil
}

func (s *MemoryUserStore) GetByUsername(_ context.Context, username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, su := range s.users {
		if su.User.Username == username {
			u := su.User
			return &u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (s *MemoryUserStore) GetByEmail(_ context.Context, email string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(email)
	for _, su := range s.users {
		if strings.ToLower(su.User.Email) == lower {
			u := su.User
			return &u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (s *MemoryUserStore) List(_ context.Context) ([]*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*User, 0, len(s.users))
	for _, su := range s.users {
		u := su.User
		result = append(result, &u)
	}
	return result, nil
}

func (s *MemoryUserStore) Create(_ context.Context, user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, su := range s.users {
		if su.User.Username == user.Username {
			return ErrUsernameExists
		}
	}

	s.users[user.ID] = storedUser{User: *user}
	return nil
}

func (s *MemoryUserStore) Update(_ context.Context, user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	su, ok := s.users[user.ID]
	if !ok {
		return ErrUserNotFound
	}

	for _, existing := range s.users {
		if existing.User.Username == user.Username && existing.User.ID != user.ID {
			return ErrUsernameExists
		}
	}

	su.User = *user
	s.users[user.ID] = su
	return nil
}

func (s *MemoryUserStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		return ErrUserNotFound
	}
	delete(s.users, id)
	return nil
}

func (s *MemoryUserStore) UpdatePassword(_ context.Context, id, hashedPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	su, ok := s.users[id]
	if !ok {
		return ErrUserNotFound
	}
	su.PasswordHash = hashedPassword
	s.users[id] = su
	return nil
}

func (s *MemoryUserStore) GetPasswordHash(_ context.Context, id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	su, ok := s.users[id]
	if !ok {
		return "", ErrUserNotFound
	}
	return su.PasswordHash, nil
}

type MemoryInviteStore struct {
	invites map[string]*Invite
	mu      sync.RWMutex
}

func NewMemoryInviteStore() *MemoryInviteStore {
	return &MemoryInviteStore{invites: make(map[string]*Invite)}
}

func (s *MemoryInviteStore) Create(_ context.Context, invite *Invite) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *invite
	s.invites[invite.Token] = &cp
	return nil
}

func (s *MemoryInviteStore) Get(_ context.Context, token string) (*Invite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inv, ok := s.invites[token]
	if !ok {
		return nil, ErrInviteNotFound
	}
	cp := *inv
	return &cp, nil
}

func (s *MemoryInviteStore) List(_ context.Context) ([]*Invite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Invite, 0, len(s.invites))
	for _, inv := range s.invites {
		cp := *inv
		result = append(result, &cp)
	}
	return result, nil
}

func (s *MemoryInviteStore) Update(_ context.Context, invite *Invite) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.invites[invite.Token]; !ok {
		return ErrInviteNotFound
	}
	cp := *invite
	s.invites[invite.Token] = &cp
	return nil
}

func (s *MemoryInviteStore) Delete(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.invites[token]; !ok {
		return ErrInviteNotFound
	}
	delete(s.invites, token)
	return nil
}

type MemoryAPIKeyStore struct {
	keys map[string]*APIKey
	mu   sync.RWMutex
}

func NewMemoryAPIKeyStore() *MemoryAPIKeyStore {
	return &MemoryAPIKeyStore{keys: make(map[string]*APIKey)}
}

func (s *MemoryAPIKeyStore) Create(_ context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *key
	s.keys[key.ID] = &cp
	return nil
}

func (s *MemoryAPIKeyStore) GetByKey(_ context.Context, key string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.keys {
		if k.Key == key {
			cp := *k
			return &cp, nil
		}
	}
	return nil, ErrAPIKeyNotFound
}

func (s *MemoryAPIKeyStore) ListByUser(_ context.Context, userID string) ([]*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*APIKey
	for _, k := range s.keys {
		if k.UserID == userID {
			cp := *k
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *MemoryAPIKeyStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.keys[id]; !ok {
		return ErrAPIKeyNotFound
	}
	delete(s.keys, id)
	return nil
}

var _ InviteStore = (*MemoryInviteStore)(nil)
var _ APIKeyStore = (*MemoryAPIKeyStore)(nil)
