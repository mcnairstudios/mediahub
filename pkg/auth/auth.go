package auth

import (
	"context"
	"errors"
	"time"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleStandard Role = "standard"
	RoleJellyfin Role = "jellyfin"
)

type User struct {
	ID              string   `json:"id"`
	Username        string   `json:"username"`
	Email           string   `json:"email,omitempty"`
	IsAdmin         bool     `json:"is_admin"`
	Role            Role     `json:"role"`
	ChannelGroupIDs []string `json:"channel_group_ids,omitempty"`
}

type Invite struct {
	Token     string    `json:"token"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
}

type APIKey struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	ErrInviteNotFound = errors.New("invite not found")
	ErrInviteExpired  = errors.New("invite expired")
	ErrInviteUsed     = errors.New("invite already used")
	ErrAPIKeyNotFound = errors.New("api key not found")
)

type Service interface {
	Login(ctx context.Context, username, password string) (token string, err error)
	ValidateToken(ctx context.Context, token string) (*User, error)
	RefreshToken(ctx context.Context, token string) (newToken string, err error)
	CreateUser(ctx context.Context, username, password, email string, role Role) (*User, error)
	ListUsers(ctx context.Context) ([]*User, error)
	UpdateUser(ctx context.Context, id string, username, email string, role Role, channelGroupIDs []string) (*User, error)
	DeleteUser(ctx context.Context, id string) error
	ChangePassword(ctx context.Context, id, newPassword string) error

	CreateInvite(ctx context.Context, role Role, expiresIn time.Duration) (*Invite, error)
	AcceptInvite(ctx context.Context, token, username, password string) (*User, error)
	ListInvites(ctx context.Context) ([]*Invite, error)
	DeleteInvite(ctx context.Context, token string) error

	CreateAPIKey(ctx context.Context, userID, name string) (*APIKey, error)
	ValidateAPIKey(ctx context.Context, key string) (*User, error)
	ListAPIKeys(ctx context.Context, userID string) ([]*APIKey, error)
	RevokeAPIKey(ctx context.Context, userID, keyID string) error
}

type UserStore interface {
	Get(ctx context.Context, id string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context) ([]*User, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, id, hashedPassword string) error
}

type InviteStore interface {
	Create(ctx context.Context, invite *Invite) error
	Get(ctx context.Context, token string) (*Invite, error)
	List(ctx context.Context) ([]*Invite, error)
	Update(ctx context.Context, invite *Invite) error
	Delete(ctx context.Context, token string) error
}

type APIKeyStore interface {
	Create(ctx context.Context, key *APIKey) error
	GetByKey(ctx context.Context, key string) (*APIKey, error)
	ListByUser(ctx context.Context, userID string) ([]*APIKey, error)
	Delete(ctx context.Context, id string) error
}
