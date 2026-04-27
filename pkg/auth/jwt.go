package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type tokenType string

const (
	tokenAccess  tokenType = "access"
	tokenRefresh tokenType = "refresh"
)

type claims struct {
	jwt.RegisteredClaims
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Role     Role      `json:"role"`
	Type     tokenType `json:"type"`
}

type JWTService struct {
	store      UserStore
	secret     []byte
	tokenTTL   time.Duration
	refreshTTL time.Duration
}

func NewJWTService(store UserStore, secret string) *JWTService {
	return &JWTService{
		store:      store,
		secret:     []byte(secret),
		tokenTTL:   15 * time.Minute,
		refreshTTL: 7 * 24 * time.Hour,
	}
}

func (s *JWTService) Login(ctx context.Context, username, password string) (string, error) {
	user, err := s.store.GetByUsername(ctx, username)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	type passwordHasher interface {
		GetPasswordHash(ctx context.Context, id string) (string, error)
	}

	ph, ok := s.store.(passwordHasher)
	if !ok {
		return "", fmt.Errorf("store does not support password retrieval")
	}

	hash, err := ph.GetPasswordHash(ctx, user.ID)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	return s.generateToken(user, tokenAccess, s.tokenTTL)
}

func (s *JWTService) ValidateToken(_ context.Context, tokenString string) (*User, error) {
	c, err := s.parseToken(tokenString)
	if err != nil {
		return nil, err
	}

	if c.Type != tokenAccess {
		return nil, ErrTokenInvalid
	}

	return &User{
		ID:       c.UserID,
		Username: c.Username,
		Role:     c.Role,
		IsAdmin:  c.Role == RoleAdmin,
	}, nil
}

func (s *JWTService) RefreshToken(_ context.Context, tokenString string) (string, error) {
	c, err := s.parseToken(tokenString)
	if err != nil {
		return "", err
	}

	if c.Type != tokenRefresh {
		return "", ErrTokenInvalid
	}

	user := &User{
		ID:       c.UserID,
		Username: c.Username,
		Role:     c.Role,
		IsAdmin:  c.Role == RoleAdmin,
	}

	return s.generateToken(user, tokenAccess, s.tokenTTL)
}

func (s *JWTService) CreateUser(ctx context.Context, username, password string, role Role) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generating id: %w", err)
	}

	user := &User{
		ID:       id,
		Username: username,
		Role:     role,
		IsAdmin:  role == RoleAdmin,
	}

	if err := s.store.Create(ctx, user); err != nil {
		return nil, err
	}

	if err := s.store.UpdatePassword(ctx, user.ID, string(hash)); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *JWTService) ListUsers(ctx context.Context) ([]*User, error) {
	return s.store.List(ctx)
}

func (s *JWTService) DeleteUser(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}

func (s *JWTService) ChangePassword(ctx context.Context, id, newPassword string) error {
	if _, err := s.store.Get(ctx, id); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	return s.store.UpdatePassword(ctx, id, string(hash))
}

func (s *JWTService) GenerateRefreshToken(user *User) (string, error) {
	return s.generateToken(user, tokenRefresh, s.refreshTTL)
}

func (s *JWTService) generateToken(user *User, typ tokenType, ttl time.Duration) (string, error) {
	now := time.Now()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     typ,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString(s.secret)
}

func (s *JWTService) parseToken(tokenString string) (*claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return c, nil
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
