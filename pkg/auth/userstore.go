package auth

import (
	"context"
	"errors"
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
