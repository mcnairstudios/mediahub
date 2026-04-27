# middleware -- Public API

No interfaces defined. `AuthMiddleware` is a concrete struct wrapping `auth.Service`.

## AuthMiddleware

HTTP middleware for JWT authentication and authorization.

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewAuthMiddleware` | `(authService auth.Service) *AuthMiddleware` | Create middleware backed by the given auth service |
| `Authenticate` | `(next http.Handler) http.Handler` | Require a valid Bearer token; inject `*auth.User` into context |
| `RequireAdmin` | `(next http.Handler) http.Handler` | Authenticate + require `IsAdmin == true` |

## UserFromContext (package-level)

```go
func UserFromContext(ctx context.Context) *auth.User
```

Extract the authenticated user from the request context. Returns nil if unauthenticated.
