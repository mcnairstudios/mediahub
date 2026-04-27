package auth

import "context"

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleStandard Role = "standard"
	RoleJellyfin Role = "jellyfin"
)

type User struct {
	ID       string
	Username string
	IsAdmin  bool
	Role     Role
}

type Service interface {
	Login(ctx context.Context, username, password string) (token string, err error)
	ValidateToken(ctx context.Context, token string) (*User, error)
	RefreshToken(ctx context.Context, token string) (newToken string, err error)
	CreateUser(ctx context.Context, username, password string, role Role) (*User, error)
	ListUsers(ctx context.Context) ([]*User, error)
	UpdateUser(ctx context.Context, id string, username string, role Role) (*User, error)
	DeleteUser(ctx context.Context, id string) error
	ChangePassword(ctx context.Context, id, newPassword string) error
}

type UserStore interface {
	Get(ctx context.Context, id string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	List(ctx context.Context) ([]*User, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, id, hashedPassword string) error
}
