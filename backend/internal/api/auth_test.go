package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"palpanel/internal/appconfig"
)

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(appconfig.Config{PanelToken: "secret", RequireAuth: true}))
	r.GET("/protected", func(c *gin.Context) {
		ok(c, gin.H{"passed": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer secret")
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
	r := gin.New()
	r.Use(Auth(appconfig.Config{
		PanelToken: "admin-token", OperatorToken: "operator-token", ViewerToken: "viewer-token", RequireAuth: true,
	}))
	r.GET("/me", authMe)
	r.POST("/world", Require(PermWorldReset), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.PUT("/ai", Require(PermAIConfig), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.POST("/translate", Require(PermModsWrite), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	tests := []struct {
		token                      string
		world, ai, translateStatus int
	}{
		{token: "admin-token", world: 204, ai: 204, translateStatus: 204},
		{token: "operator-token", world: 403, ai: 403, translateStatus: 204},
		{token: "viewer-token", world: 403, ai: 403, translateStatus: 403},
	}
	for _, test := range tests {
		for path, expected := range map[string]int{"/world": test.world, "/ai": test.ai, "/translate": test.translateStatus} {
			method := http.MethodPost
			if path == "/ai" {
				method = http.MethodPut
			}
			req := httptest.NewRequest(method, path, nil)
			req.Header.Set("Authorization", "Bearer "+test.token)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != expected {
				t.Errorf("token %s %s = %d, want %d: %s", test.token, path, rec.Code, expected, rec.Body.String())
			}
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"role":"admin"`) || !strings.Contains(rec.Body.String(), `"world:reset"`) || strings.Contains(rec.Body.String(), "admin-token") {
		t.Fatalf("unexpected session response: %s", rec.Body.String())
	}
}
