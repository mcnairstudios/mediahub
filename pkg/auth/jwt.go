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
	UserID          string    `json:"user_id"`
	Username        string    `json:"username"`
	Email           string    `json:"email,omitempty"`
	Role            Role      `json:"role"`
	Type            tokenType `json:"type"`
	ChannelGroupIDs []string  `json:"channel_group_ids,omitempty"`
}

type JWTService struct {
	store      UserStore
	invites    InviteStore
	apiKeys    APIKeyStore
	secret     []byte
	tokenTTL   time.Duration
	refreshTTL time.Duration
}

func NewJWTService(store UserStore, secret string) *JWTService {
	return &JWTService{
		store:      store,
		secret:     []byte(secret),
		tokenTTL:   24 * time.Hour,
		refreshTTL: 7 * 24 * time.Hour,
	}
}

func (s *JWTService) SetInviteStore(store InviteStore) {
	s.invites = store
}

func (s *JWTService) SetAPIKeyStore(store APIKeyStore) {
	s.apiKeys = store
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
		ID:              c.UserID,
		Username:        c.Username,
		Email:           c.Email,
		Role:            c.Role,
		IsAdmin:         c.Role == RoleAdmin,
		ChannelGroupIDs: c.ChannelGroupIDs,
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
		ID:              c.UserID,
		Username:        c.Username,
		Email:           c.Email,
		Role:            c.Role,
		IsAdmin:         c.Role == RoleAdmin,
		ChannelGroupIDs: c.ChannelGroupIDs,
	}

	return s.generateToken(user, tokenAccess, s.tokenTTL)
}

func (s *JWTService) CreateUser(ctx context.Context, username, password, email string, role Role) (*User, error) {
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
		Email:    email,
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
	users, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		u.IsAdmin = u.Role == RoleAdmin
	}
	return users, nil
}

func (s *JWTService) UpdateUser(ctx context.Context, id string, username, email string, role Role, channelGroupIDs []string) (*User, error) {
	user, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if username != "" {
		user.Username = username
	}
	user.Email = email
	if role != "" {
		user.Role = role
		user.IsAdmin = role == RoleAdmin
	}
	if channelGroupIDs != nil {
		user.ChannelGroupIDs = channelGroupIDs
	}

	if err := s.store.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
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

func (s *JWTService) GenerateAccessToken(user *User) (string, error) {
	return s.generateToken(user, tokenAccess, s.tokenTTL)
}

func (s *JWTService) GenerateRefreshToken(user *User) (string, error) {
	return s.generateToken(user, tokenRefresh, s.refreshTTL)
}

func (s *JWTService) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return s.store.GetByEmail(ctx, email)
}

func (s *JWTService) generateToken(user *User, typ tokenType, ttl time.Duration) (string, error) {
	now := time.Now()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		UserID:          user.ID,
		Username:        user.Username,
		Email:           user.Email,
		Role:            user.Role,
		Type:            typ,
		ChannelGroupIDs: user.ChannelGroupIDs,
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

func (s *JWTService) CreateInvite(_ context.Context, role Role, expiresIn time.Duration) (*Invite, error) {
	if s.invites == nil {
		return nil, fmt.Errorf("invite store not configured")
	}
	if expiresIn <= 0 {
		expiresIn = 24 * time.Hour
	}

	token, err := generateToken32()
	if err != nil {
		return nil, fmt.Errorf("generating invite token: %w", err)
	}

	now := time.Now()
	invite := &Invite{
		Token:     token,
		Role:      role,
		CreatedAt: now,
		ExpiresAt: now.Add(expiresIn),
	}

	if err := s.invites.Create(context.Background(), invite); err != nil {
		return nil, err
	}

	return invite, nil
}

func (s *JWTService) AcceptInvite(ctx context.Context, token, username, password string) (*User, error) {
	if s.invites == nil {
		return nil, fmt.Errorf("invite store not configured")
	}

	invite, err := s.invites.Get(ctx, token)
	if err != nil {
		return nil, ErrInviteNotFound
	}
	if invite.Used {
		return nil, ErrInviteUsed
	}
	if time.Now().After(invite.ExpiresAt) {
		return nil, ErrInviteExpired
	}

	user, err := s.CreateUser(ctx, username, password, "", invite.Role)
	if err != nil {
		return nil, err
	}

	invite.Used = true
	s.invites.Update(ctx, invite)

	return user, nil
}

func (s *JWTService) ListInvites(ctx context.Context) ([]*Invite, error) {
	if s.invites == nil {
		return nil, fmt.Errorf("invite store not configured")
	}
	return s.invites.List(ctx)
}

func (s *JWTService) DeleteInvite(ctx context.Context, token string) error {
	if s.invites == nil {
		return fmt.Errorf("invite store not configured")
	}
	return s.invites.Delete(ctx, token)
}

func (s *JWTService) CreateAPIKey(ctx context.Context, userID, name string) (*APIKey, error) {
	if s.apiKeys == nil {
		return nil, fmt.Errorf("api key store not configured")
	}

	if _, err := s.store.Get(ctx, userID); err != nil {
		return nil, err
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generating api key id: %w", err)
	}
	key, err := generateToken32()
	if err != nil {
		return nil, fmt.Errorf("generating api key: %w", err)
	}

	apiKey := &APIKey{
		ID:        id,
		Key:       key,
		UserID:    userID,
		Name:      name,
		CreatedAt: time.Now(),
	}

	if err := s.apiKeys.Create(ctx, apiKey); err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (s *JWTService) ValidateAPIKey(ctx context.Context, key string) (*User, error) {
	if s.apiKeys == nil {
		return nil, fmt.Errorf("api key store not configured")
	}

	apiKey, err := s.apiKeys.GetByKey(ctx, key)
	if err != nil {
		return nil, ErrAPIKeyNotFound
	}

	user, err := s.store.Get(ctx, apiKey.UserID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *JWTService) ListAPIKeys(ctx context.Context, userID string) ([]*APIKey, error) {
	if s.apiKeys == nil {
		return nil, fmt.Errorf("api key store not configured")
	}
	return s.apiKeys.ListByUser(ctx, userID)
}

func (s *JWTService) RevokeAPIKey(ctx context.Context, userID, keyID string) error {
	if s.apiKeys == nil {
		return fmt.Errorf("api key store not configured")
	}
	keys, err := s.apiKeys.ListByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if k.ID == keyID {
			return s.apiKeys.Delete(ctx, keyID)
		}
	}
	return ErrAPIKeyNotFound
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateToken32() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
