# middleware

HTTP middleware for mediahub.

## AuthMiddleware

Validates JWT Bearer tokens on incoming requests and sets the authenticated user on the request context.

### Usage

```go
authMW := middleware.NewAuthMiddleware(authService)

// Require authentication
router.With(authMW.Authenticate).Get("/api/channels", channelHandler.List)

// Require admin role
router.With(authMW.RequireAdmin).Post("/api/users", userHandler.Create)

// Read user in a handler
user := middleware.UserFromContext(r.Context())
```

### Behavior

- `Authenticate` extracts the Bearer token from the Authorization header, validates it via `auth.Service.ValidateToken`, and places the `*auth.User` on the request context. Returns 401 for missing or invalid tokens.
- `RequireAdmin` chains `Authenticate` then checks `user.IsAdmin`. Returns 403 if the user is not an admin.
- `UserFromContext` retrieves the user from context, returning nil if not set.

### Dependencies

- `pkg/auth` — `Service` interface and `User` type
- `pkg/httputil` — `RespondError` for JSON error responses
