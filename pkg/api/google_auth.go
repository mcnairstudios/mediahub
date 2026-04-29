package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

type oauthState struct {
	createdAt time.Time
}

var (
	oauthStates   sync.Map
	stateLifetime = 5 * time.Minute
)

func (s *Server) handleGoogleAuth(w http.ResponseWriter, r *http.Request) {
	clientID, err := s.deps.SettingsStore.Get(r.Context(), "google_client_id")
	if err != nil || clientID == "" {
		httputil.RespondError(w, http.StatusNotFound, "google oauth not configured")
		return
	}

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}
	state := hex.EncodeToString(stateBytes)

	oauthStates.Store(state, oauthState{createdAt: time.Now()})

	go func() {
		time.Sleep(stateLifetime)
		oauthStates.Delete(state)
	}()

	baseURL, _ := s.deps.SettingsStore.Get(r.Context(), "base_url")
	if baseURL == "" && s.deps.Config != nil {
		baseURL = s.deps.Config.BaseURL
	}
	redirectURI := strings.TrimRight(baseURL, "/") + "/api/auth/google/callback"

	params := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
	}

	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
	httputil.RespondJSON(w, http.StatusOK, map[string]string{"url": authURL})
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		redirectWithError(w, r, "missing state or code")
		return
	}

	val, ok := oauthStates.LoadAndDelete(state)
	if !ok {
		redirectWithError(w, r, "invalid or expired state")
		return
	}
	st := val.(oauthState)
	if time.Since(st.createdAt) > stateLifetime {
		redirectWithError(w, r, "state expired")
		return
	}

	clientID, _ := s.deps.SettingsStore.Get(r.Context(), "google_client_id")
	clientSecret, _ := s.deps.SettingsStore.Get(r.Context(), "google_client_secret")
	if clientID == "" || clientSecret == "" {
		redirectWithError(w, r, "google oauth not configured")
		return
	}

	baseURL, _ := s.deps.SettingsStore.Get(r.Context(), "base_url")
	if baseURL == "" && s.deps.Config != nil {
		baseURL = s.deps.Config.BaseURL
	}
	redirectURI := strings.TrimRight(baseURL, "/") + "/api/auth/google/callback"

	tokenResp, err := exchangeGoogleCode(code, clientID, clientSecret, redirectURI)
	if err != nil {
		redirectWithError(w, r, "token exchange failed")
		return
	}

	email, err := extractEmailFromIDToken(tokenResp.IDToken)
	if err != nil || email == "" {
		redirectWithError(w, r, "failed to extract email from token")
		return
	}

	jwtSvc, ok := s.deps.AuthService.(*auth.JWTService)
	if !ok {
		redirectWithError(w, r, "internal error")
		return
	}

	user, err := jwtSvc.GetUserByEmail(r.Context(), email)
	if err != nil {
		redirectWithError(w, r, "no_account")
		return
	}

	accessToken, err := jwtSvc.GenerateAccessToken(user)
	if err != nil {
		redirectWithError(w, r, "failed to generate token")
		return
	}

	refreshToken, err := jwtSvc.GenerateRefreshToken(user)
	if err != nil {
		redirectWithError(w, r, "failed to generate refresh token")
		return
	}

	redirectURL := fmt.Sprintf("/#/login?token=%s&refresh=%s",
		url.QueryEscape(accessToken),
		url.QueryEscape(refreshToken))

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func redirectWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	http.Redirect(w, r, "/#/login?error="+url.QueryEscape(errMsg), http.StatusFound)
}

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

func exchangeGoogleCode(code, clientID, clientSecret, redirectURI string) (*googleTokenResponse, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}

	var tokenResp googleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	return &tokenResp, nil
}

func extractEmailFromIDToken(idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid id_token format")
	}

	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("decoding id_token payload: %w", err)
	}

	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return "", fmt.Errorf("parsing id_token claims: %w", err)
	}

	return claims.Email, nil
}
