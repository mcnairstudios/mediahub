package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/mcnairstudios/mediahub/pkg/activity"
	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

type contextKey string

const UserContextKey contextKey = "user"

type AuthMiddleware struct {
	authService     auth.Service
	activityService *activity.Service
}

func NewAuthMiddleware(authService auth.Service, activityService *activity.Service) *AuthMiddleware {
	return &AuthMiddleware{authService: authService, activityService: activityService}
}

func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. X-API-Key header (API keys)
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			user, err := m.authService.ValidateAPIKey(r.Context(), apiKey)
			if err != nil {
				httputil.RespondError(w, http.StatusUnauthorized, "invalid api key")
				return
			}
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Try to extract a JWT token from multiple sources, in priority order:
		// 2. Authorization: Bearer header
		// 3. mediahub_token cookie
		// 4. api_key query parameter
		// 5. token query parameter
		token := ""
		if header := r.Header.Get("Authorization"); header != "" {
			if t, ok := parseBearerToken(header); ok {
				token = t
			}
		}
		if token == "" {
			if c, err := r.Cookie("mediahub_token"); err == nil && c.Value != "" {
				token = c.Value
			}
		}
		if token == "" {
			if t := r.URL.Query().Get("api_key"); t != "" {
				token = t
			}
		}
		if token == "" {
			if t := r.URL.Query().Get("token"); t != "" {
				token = t
			}
		}

		if token == "" {
			httputil.RespondError(w, http.StatusUnauthorized, "missing authorization")
			return
		}

		user, err := m.authService.ValidateToken(r.Context(), token)
		if err != nil {
			httputil.RespondError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		if m.activityService != nil {
			m.activityService.TouchUser(user.ID, user.Username, "Dashboard", r.RemoteAddr, r.UserAgent())
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return m.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || !user.IsAdmin {
			httputil.RespondError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func UserFromContext(ctx context.Context) *auth.User {
	user, _ := ctx.Value(UserContextKey).(*auth.User)
	return user
}

func parseBearerToken(header string) (string, bool) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}
