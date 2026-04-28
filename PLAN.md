# Google OAuth / SSO Login

## Goal

Users with an email address on their mediahub account can log in via Google OAuth. Once set up, they never need to enter a password — just click "Sign in with Google". The existing username/password login stays as a fallback.

## Prerequisites

- Google Cloud Console: create OAuth 2.0 client ID (Web application type)
- Authorized redirect URI: `{base_url}/api/auth/google/callback`
- Two settings needed: `google_client_id` and `google_client_secret`

## Backend Changes

### 1. Add email field to User

File: `pkg/auth/auth.go`

```go
type User struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email,omitempty"`
    IsAdmin  bool   `json:"is_admin"`
    Role     Role   `json:"role"`
}
```

Update `pkg/auth/jwt.go`:
- `CreateUser` accepts optional email
- `UpdateUser` accepts optional email
- New method: `GetByEmail(ctx, email) (*User, error)` on UserStore interface
- JWT claims include email

Update `pkg/store/bolt/users.go`:
- Implement `GetByEmail` — scan users for matching email
- Store email in user record

Update `pkg/api/handlers.go`:
- User create/update endpoints accept `email` field
- Users list returns email

### 2. Add Google OAuth endpoints

File: `pkg/api/auth_handlers.go` (new file or add to existing)

**`GET /api/auth/google`** (public)
- Reads `google_client_id` from settings
- If not configured, returns 404
- Builds Google OAuth URL: `https://accounts.google.com/o/oauth2/v2/auth?client_id=...&redirect_uri=...&response_type=code&scope=openid email profile&state={random}`
- Stores state in a short-lived map (5 min TTL) to prevent CSRF
- Returns `{"url": "https://accounts.google.com/o/oauth2/v2/auth?..."}`

**`GET /api/auth/google/callback`** (public)
- Receives `?code=...&state=...` from Google
- Validates state against stored map
- Exchanges code for tokens: POST `https://oauth2.googleapis.com/token` with client_id, client_secret, code, redirect_uri, grant_type=authorization_code
- Decodes the ID token (JWT) to get email, name, picture
- Looks up mediahub user by email (`GetByEmail`)
- If found: issue mediahub JWT access token + refresh token
- If not found: return error page "No account with this email. Ask an admin to add your email to your account."
- Redirects to `/#/login?token={jwt}` so the frontend can store it

### 3. Add settings

Add to `apiSettableKeys` in handlers.go:
- `google_client_id`
- `google_client_secret`

Add to settings UI:
- Google OAuth section with client ID and client secret fields
- Help text: "Create credentials at console.cloud.google.com. Redirect URI: {base_url}/api/auth/google/callback"

### 4. Add routes

File: `pkg/api/routes.go`

```go
s.mux.HandleFunc("GET /api/auth/google", s.handleGoogleAuth)
s.mux.HandleFunc("GET /api/auth/google/callback", s.handleGoogleCallback)
```

Both are public (no auth middleware).

## Frontend Changes

### 5. Login page — "Sign in with Google" button

File: `web/dist/app.js`

In the login form renderer:
- Check if Google OAuth is configured: `GET /api/auth/google` — if 404, don't show button
- If configured, show a "Sign in with Google" button below the password form
- On click: `window.location.href = data.url` (full redirect to Google)
- On return: check for `?token=` in URL hash, store it, navigate to dashboard

### 6. User management — email field

- Add email input to user create/edit forms
- Show email in users list table
- Settings page: Google OAuth configuration section

### 7. Auto-login on callback

When the app loads, check URL for `#/login?token=...`:
- Extract token
- Store in localStorage
- Decode JWT to get user info
- Navigate to dashboard
- Clean URL

## Implementation Order

1. Add email to User struct + store + JWT claims
2. Add Google OAuth endpoints (auth + callback)
3. Add settings (client_id, client_secret)
4. Frontend: Google button on login + auto-login on callback
5. Frontend: email field on user CRUD
6. Frontend: Google OAuth config in settings
7. Test end-to-end

## Dependencies

- No new Go modules needed — Google OAuth is just HTTP calls + JWT decode
- `golang.org/x/oauth2` could be used but raw HTTP is simpler and avoids a dependency
- Google ID token is a standard JWT — decode with the existing JWT library

## Security Notes

- State parameter prevents CSRF on the OAuth flow
- ID token must be validated (signature check against Google's JWKS, or just use the token endpoint which returns a verified token)
- Client secret must never be exposed to the frontend
- Refresh tokens from Google are NOT stored — we issue our own mediahub JWTs
- Email matching is case-insensitive
- Admin must explicitly add email to a user account before OAuth works — no auto-registration
