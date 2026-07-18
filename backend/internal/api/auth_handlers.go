package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	panelauth "palpanel/internal/auth"
	"palpanel/internal/db"
)

type authRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	now      func() time.Time
}

func newAuthRateLimiter() *authRateLimiter {
	return &authRateLimiter{attempts: map[string][]time.Time{}, now: time.Now}
}

func (l *authRateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	cutoff := now.Add(-5 * time.Minute)
	recent := l.attempts[key][:0]
	for _, attempt := range l.attempts[key] {
		if attempt.After(cutoff) {
			recent = append(recent, attempt)
		}
	}
	if len(recent) >= 10 {
		l.attempts[key] = recent
		return false
	}
	l.attempts[key] = append(recent, now)
	return true
}

func (l *authRateLimiter) clear(key string) {
	l.mu.Lock()
	delete(l.attempts, key)
	l.mu.Unlock()
}

func (s Server) authStatus(c *gin.Context) {
	initialized, err := s.auth.Initialized(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "auth_status_failed", "could not read authentication status")
		return
	}
	data := gin.H{"initialized": initialized, "authenticated": false}
	if !s.cfg.RequireAuth {
		data["authenticated"] = true
		data["user"] = gin.H{"name": "local", "role": RoleAdmin, "permissions": PermissionsForRole(RoleAdmin)}
		ok(c, data)
		return
	}
	if initialized {
		principal, authErr := authenticateRequest(c, s.auth)
		if authErr == nil {
			data["authenticated"] = true
			data["user"] = sessionPayload(principal)
		} else if !isAuthenticationMiss(authErr) {
			fail(c, http.StatusInternalServerError, "auth_status_failed", "could not validate authentication state")
			return
		}
	}
	ok(c, data)
}

func (s Server) registerAdmin(c *gin.Context) {
	key := "register:" + c.ClientIP()
	if !s.authLimiter.allow(key) {
		fail(c, http.StatusTooManyRequests, "rate_limited", "too many registration attempts; try again later")
		return
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", "username and password are required")
		return
	}
	user, token, err := s.auth.Register(c.Request.Context(), request.Username, request.Password)
	if errors.Is(err, db.ErrAlreadyInitialized) {
		fail(c, http.StatusConflict, "already_initialized", "the first administrator has already been registered")
		return
	}
	if errors.Is(err, panelauth.ErrInvalidUsername) || errors.Is(err, panelauth.ErrInvalidPassword) {
		fail(c, http.StatusUnprocessableEntity, "invalid_credentials", err.Error())
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "registration_failed", "administrator registration failed")
		return
	}
	s.authLimiter.clear(key)
	setSessionCookie(c, token)
	created(c, sessionPayload(Principal{UserID: user.ID, Name: user.Username, Role: Role(user.Role), Credential: panelauth.CredentialSession}))
}

func (s Server) login(c *gin.Context) {
	key := "login:" + c.ClientIP()
	if !s.authLimiter.allow(key) {
		fail(c, http.StatusTooManyRequests, "rate_limited", "too many login attempts; try again later")
		return
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", "username and password are required")
		return
	}
	user, token, err := s.auth.Login(c.Request.Context(), request.Username, request.Password)
	if errors.Is(err, panelauth.ErrInvalidLogin) || errors.Is(err, panelauth.ErrUserDisabled) {
		fail(c, http.StatusUnauthorized, "invalid_login", "invalid username or password")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "login_failed", "login failed")
		return
	}
	s.authLimiter.clear(key)
	setSessionCookie(c, token)
	ok(c, sessionPayload(Principal{UserID: user.ID, Name: user.Username, Role: Role(user.Role), Credential: panelauth.CredentialSession}))
}

func (s Server) logout(c *gin.Context) {
	if token, err := c.Cookie(panelauth.SessionCookieName); err == nil {
		_ = s.auth.Logout(c.Request.Context(), token)
	}
	clearSessionCookie(c)
	ok(c, gin.H{"logged_out": true})
}

func (s Server) authMe(c *gin.Context) {
	ok(c, sessionPayload(CurrentPrincipal(c)))
}

func (s Server) changePassword(c *gin.Context) {
	principal := CurrentPrincipal(c)
	if principal.Credential != panelauth.CredentialSession {
		fail(c, http.StatusForbidden, "password_change_session_required", "password changes require an authenticated browser session")
		return
	}
	key := "password-change:" + principal.UserID
	if !s.authLimiter.allow(key) {
		fail(c, http.StatusTooManyRequests, "rate_limited", "too many password change attempts; try again later")
		return
	}
	var request struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", "current_password and new_password are required")
		return
	}
	err := s.auth.ChangePassword(c.Request.Context(), principal.Name, request.CurrentPassword, request.NewPassword)
	if errors.Is(err, panelauth.ErrInvalidCurrentPassword) {
		fail(c, http.StatusUnauthorized, "current_password_invalid", "current password is incorrect")
		return
	}
	if errors.Is(err, panelauth.ErrInvalidPassword) {
		fail(c, http.StatusUnprocessableEntity, "invalid_password", err.Error())
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "password_change_failed", "could not change the administrator password")
		return
	}
	s.authLimiter.clear(key)
	clearSessionCookie(c)
	ok(c, gin.H{"password_changed": true, "sessions_revoked": true, "api_keys_revoked": true})
}

func (s Server) listAPIKeys(c *gin.Context) {
	principal := CurrentPrincipal(c)
	items, err := s.auth.ListAPIKeys(c.Request.Context(), principal.UserID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "api_keys_list_failed", "could not list development keys")
		return
	}
	ok(c, items)
}

func (s Server) createAPIKey(c *gin.Context) {
	var request struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", "development key name is required")
		return
	}
	key, err := s.auth.CreateAPIKey(c.Request.Context(), CurrentPrincipal(c).UserID, request.Name)
	if errors.Is(err, panelauth.ErrInvalidKeyName) {
		fail(c, http.StatusUnprocessableEntity, "invalid_key_name", err.Error())
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "api_key_create_failed", "could not create development key")
		return
	}
	created(c, key)
}

func (s Server) revokeAPIKey(c *gin.Context) {
	err := s.auth.RevokeAPIKey(c.Request.Context(), CurrentPrincipal(c).UserID, strings.TrimSpace(c.Param("id")))
	if errors.Is(err, sql.ErrNoRows) {
		fail(c, http.StatusNotFound, "api_key_not_found", "development key not found or already revoked")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "api_key_revoke_failed", "could not revoke development key")
		return
	}
	ok(c, gin.H{"revoked": true})
}

func sessionPayload(principal Principal) gin.H {
	return gin.H{
		"name":        principal.Name,
		"role":        principal.Role,
		"permissions": PermissionsForRole(principal.Role),
	}
}

func setSessionCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(panelauth.SessionCookieName, token, int(panelauth.SessionLifetime.Seconds()), "/", "", requestIsHTTPS(c.Request), true)
}

func clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(panelauth.SessionCookieName, "", -1, "/", "", requestIsHTTPS(c.Request), true)
}

func requestIsHTTPS(request *http.Request) bool {
	return request.TLS != nil || strings.EqualFold(strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")), "https")
}
