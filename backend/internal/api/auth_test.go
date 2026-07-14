package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"palpanel/internal/appconfig"
	panelauth "palpanel/internal/auth"
	"palpanel/internal/db"
)

func TestAuthenticationHTTPFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, err := db.Open(filepath.Join(t.TempDir(), "auth-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := panelauth.New(store)
	s := Server{cfg: appconfig.Config{RequireAuth: true}, store: store, auth: service, authLimiter: newAuthRateLimiter()}
	router := gin.New()
	public := router.Group("/api/auth")
	public.Use(SameOriginWrite())
	public.GET("/status", s.authStatus)
	public.POST("/register", s.registerAdmin)
	public.POST("/login", s.login)
	protected := router.Group("/api")
	protected.Use(Auth(s.cfg, service))
	protected.GET("/auth/me", s.authMe)
	protected.POST("/auth/logout", s.logout)
	protected.POST("/auth/api-keys", Require(PermSecurityWrite), s.createAPIKey)
	protected.DELETE("/auth/api-keys/:id", Require(PermSecurityWrite), s.revokeAPIKey)
	protected.POST("/write", func(c *gin.Context) { ok(c, gin.H{"written": true}) })

	status := authRequest(router, http.MethodGet, "/api/auth/status", "", nil, "")
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"initialized":false`) {
		t.Fatalf("unexpected initial status: %d %s", status.Code, status.Body.String())
	}
	crossSite := authRequest(router, http.MethodPost, "/api/auth/register", `{"username":"admin","password":"strong-password-123"}`, nil, "https://attacker.example")
	if crossSite.Code != http.StatusForbidden {
		t.Fatalf("cross-site registration status = %d", crossSite.Code)
	}

	registered := authRequest(router, http.MethodPost, "/api/auth/register", `{"username":"admin","password":"strong-password-123"}`, nil, "http://example.com")
	if registered.Code != http.StatusCreated {
		t.Fatalf("registration status = %d: %s", registered.Code, registered.Body.String())
	}
	cookies := registered.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("registration cookies = %#v", cookies)
	}
	sessionCookie := cookies[0]
	if sessionCookie.Name != panelauth.SessionCookieName || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode || sessionCookie.Path != "/" || sessionCookie.Secure {
		t.Fatalf("unexpected session cookie: %#v", sessionCookie)
	}

	authenticatedStatus := authRequest(router, http.MethodGet, "/api/auth/status", "", sessionCookie, "")
	if authenticatedStatus.Code != http.StatusOK || !strings.Contains(authenticatedStatus.Body.String(), `"authenticated":true`) {
		t.Fatalf("authenticated status = %d: %s", authenticatedStatus.Code, authenticatedStatus.Body.String())
	}
	blockedWrite := authRequest(router, http.MethodPost, "/api/write", "", sessionCookie, "https://attacker.example")
	if blockedWrite.Code != http.StatusForbidden {
		t.Fatalf("cross-site session write status = %d", blockedWrite.Code)
	}

	createdKey := authRequest(router, http.MethodPost, "/api/auth/api-keys", `{"name":"automation"}`, sessionCookie, "http://example.com")
	if createdKey.Code != http.StatusCreated {
		t.Fatalf("development key creation = %d: %s", createdKey.Code, createdKey.Body.String())
	}
	var keyEnvelope struct {
		Data struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createdKey.Body.Bytes(), &keyEnvelope); err != nil || !strings.HasPrefix(keyEnvelope.Data.Token, "ppk_") {
		t.Fatalf("development key response = %#v, %v", keyEnvelope, err)
	}
	keyWriteRequest := httptest.NewRequest(http.MethodPost, "/api/write", nil)
	keyWriteRequest.Header.Set("Authorization", "Bearer "+keyEnvelope.Data.Token)
	keyWriteRequest.Header.Set("Origin", "https://attacker.example")
	keyWrite := httptest.NewRecorder()
	router.ServeHTTP(keyWrite, keyWriteRequest)
	if keyWrite.Code != http.StatusOK {
		t.Fatalf("development key write status = %d: %s", keyWrite.Code, keyWrite.Body.String())
	}

	revoked := authRequest(router, http.MethodDelete, "/api/auth/api-keys/"+keyEnvelope.Data.ID, "", sessionCookie, "http://example.com")
	if revoked.Code != http.StatusOK {
		t.Fatalf("development key revocation = %d: %s", revoked.Code, revoked.Body.String())
	}
	revokedRequest := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	revokedRequest.Header.Set("Authorization", "Bearer "+keyEnvelope.Data.Token)
	revokedRecorder := httptest.NewRecorder()
	router.ServeHTTP(revokedRecorder, revokedRequest)
	if revokedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("revoked development key status = %d", revokedRecorder.Code)
	}

	logout := authRequest(router, http.MethodPost, "/api/auth/logout", "", sessionCookie, "http://example.com")
	if logout.Code != http.StatusOK || !strings.Contains(logout.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("logout response = %d %#v", logout.Code, logout.Header())
	}
	meAfterLogout := authRequest(router, http.MethodGet, "/api/auth/me", "", sessionCookie, "")
	if meAfterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("logged-out session status = %d", meAfterLogout.Code)
	}

	invalidLogin := authRequest(router, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"wrong"}`, nil, "http://example.com")
	if invalidLogin.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login status = %d", invalidLogin.Code)
	}
	validLogin := authRequest(router, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"strong-password-123"}`, nil, "http://example.com")
	if validLogin.Code != http.StatusOK || len(validLogin.Result().Cookies()) != 1 {
		t.Fatalf("valid login response = %d: %s", validLogin.Code, validLogin.Body.String())
	}
}

func TestSessionCookieSecurityAndAuthRateLimit(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "https://example.com/api/auth/login", nil)
	setSessionCookie(context, "session-token")
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("HTTPS session cookie = %#v", cookies)
	}

	limiter := newAuthRateLimiter()
	for attempt := 0; attempt < 10; attempt++ {
		if !limiter.allow("login:127.0.0.1") {
			t.Fatalf("attempt %d was unexpectedly limited", attempt)
		}
	}
	if limiter.allow("login:127.0.0.1") {
		t.Fatal("eleventh attempt should be rate limited")
	}
	limiter.clear("login:127.0.0.1")
	if !limiter.allow("login:127.0.0.1") {
		t.Fatal("successful authentication should clear the rate limit")
	}
}

func authRequest(router http.Handler, method, path, body string, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, err := db.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	provisionTestPrincipal(t, store, RoleAdmin)
	r := gin.New()
	r.Use(Auth(appconfig.Config{RequireAuth: true}, panelauth.New(store)))
	r.GET("/protected", func(c *gin.Context) {
		ok(c, gin.H{"passed": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	authorizeTestRequest(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRolePermissionsProtectWorldResetAndAIConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		role                       Role
		world, ai, translateStatus int
	}{
		{role: RoleAdmin, world: 204, ai: 204, translateStatus: 204},
		{role: RoleOperator, world: 403, ai: 403, translateStatus: 204},
		{role: RoleViewer, world: 403, ai: 403, translateStatus: 403},
	}
	for _, test := range tests {
		store, err := db.Open(filepath.Join(t.TempDir(), "auth.db"))
		if err != nil {
			t.Fatal(err)
		}
		provisionTestPrincipal(t, store, test.role)
		r := gin.New()
		r.Use(Auth(appconfig.Config{RequireAuth: true}, panelauth.New(store)))
		r.GET("/me", func(c *gin.Context) { ok(c, sessionPayload(CurrentPrincipal(c))) })
		r.POST("/world", Require(PermWorldReset), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		r.PUT("/ai", Require(PermAIConfig), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		r.POST("/translate", Require(PermModsWrite), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		for path, expected := range map[string]int{"/world": test.world, "/ai": test.ai, "/translate": test.translateStatus} {
			method := http.MethodPost
			if path == "/ai" {
				method = http.MethodPut
			}
			req := httptest.NewRequest(method, path, nil)
			authorizeTestRequest(req)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != expected {
				t.Errorf("role %s %s = %d, want %d: %s", test.role, path, rec.Code, expected, rec.Body.String())
			}
		}
		if test.role == RoleAdmin {
			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			authorizeTestRequest(req)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"role":"admin"`) || !strings.Contains(rec.Body.String(), `"world:reset"`) || strings.Contains(rec.Body.String(), testDevelopmentKey) {
				t.Fatalf("unexpected session response: %s", rec.Body.String())
			}
		}
		_ = store.Close()
	}
}
