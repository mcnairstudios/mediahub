# auth -- Interfaces

## Service

Authentication and user management contract.

```go
type Service interface {
    Login(ctx context.Context, username, password string) (token string, err error)
    ValidateToken(ctx context.Context, token string) (*User, error)
    RefreshToken(ctx context.Context, token string) (newToken string, err error)
    CreateUser(ctx context.Context, username, password string, role Role) (*User, error)
    ListUsers(ctx context.Context) ([]*User, error)
    DeleteUser(ctx context.Context, id string) error
    ChangePassword(ctx context.Context, id, newPassword string) error
}
```

| Method | Description |
|--------|-------------|
| `Login` | Authenticate by username/password, return a JWT access token |
| `ValidateToken` | Parse and validate a JWT, return the authenticated user |
| `RefreshToken` | Exchange a refresh token for a new access token |
| `CreateUser` | Create a new user with the given role |
| `ListUsers` | Return all users |
| `DeleteUser` | Delete a user by ID |
| `ChangePassword` | Update a user's password |

Implemented by: `JWTService`

---

## UserStore

Persistence layer for user records and password hashes.

```go
type UserStore interface {
    Get(ctx context.Context, id string) (*User, error)
    GetByUsername(ctx context.Context, username string) (*User, error)
    List(ctx context.Context) ([]*User, error)
    Create(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
    UpdatePassword(ctx context.Context, id, hashedPassword string) error
}
```

| Method | Description |
|--------|-------------|
| `Get` | Retrieve a user by ID |
| `GetByUsername` | Retrieve a user by username |
| `List` | Return all users |
| `Create` | Persist a new user |
| `Delete` | Remove a user by ID |
| `UpdatePassword` | Update the stored password hash for a user |

Implemented by: `MemoryUserStore`
