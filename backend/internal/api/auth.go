package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"palpanel/internal/appconfig"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

const principalKey = "principal"

type Principal struct {
	Name string `json:"name"`
	Role Role   `json:"role"`
}

type Permission string

const (
	PermRead          Permission = "read"
	PermServerControl Permission = "server:control"
	PermConfigWrite   Permission = "config:write"
	PermBackupWrite   Permission = "backup:write"
	PermModsWrite     Permission = "mods:write"
	PermPlayersWrite  Permission = "players:write"
	PermSecurityWrite Permission = "security:write"
	PermAuditRead     Permission = "audit:read"
)

var rolePermissions = map[Role]map[Permission]bool{
	RoleAdmin: {
		PermRead: true, PermServerControl: true, PermConfigWrite: true, PermBackupWrite: true,
		PermModsWrite: true, PermPlayersWrite: true, PermSecurityWrite: true, PermAuditRead: true,
	},
	RoleOperator: {
		PermRead: true, PermServerControl: true, PermConfigWrite: true, PermBackupWrite: true,
		PermModsWrite: true, PermPlayersWrite: true,
	},
	RoleViewer: {
		PermRead: true,
	},
}

func Auth(cfg appconfig.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.RequireAuth {
			c.Set(principalKey, Principal{Name: "local", Role: RoleAdmin})
			c.Next()
			return
		}
		if cfg.PanelToken == "" {
			fail(c, http.StatusUnauthorized, "auth_not_configured", "panel token is not configured")
			c.Abort()
			return
		}
		header := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			fail(c, http.StatusUnauthorized, "missing_token", "Authorization header must be Bearer token")
			c.Abort()
			return
		}
		got := strings.TrimSpace(strings.TrimPrefix(header, prefix))
		principal, ok := matchToken(cfg, got)
		if !ok {
			fail(c, http.StatusUnauthorized, "invalid_token", "invalid bearer token")
			c.Abort()
			return
		}
		c.Set(principalKey, principal)
		c.Next()
	}
}

func Require(permission Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal := CurrentPrincipal(c)
		if rolePermissions[principal.Role][permission] {
			c.Next()
			return
		}
		fail(c, http.StatusForbidden, "permission_denied", "permission denied")
		c.Abort()
	}
}

func CurrentPrincipal(c *gin.Context) Principal {
	if value, ok := c.Get(principalKey); ok {
		if principal, ok := value.(Principal); ok {
			return principal
		}
	}
	return Principal{Name: "unknown", Role: RoleViewer}
}

func matchToken(cfg appconfig.Config, got string) (Principal, bool) {
	for _, candidate := range []struct {
		token string
		role  Role
		name  string
	}{
		{cfg.PanelToken, RoleAdmin, "admin"},
		{cfg.OperatorToken, RoleOperator, "operator"},
		{cfg.ViewerToken, RoleViewer, "viewer"},
	} {
		if candidate.token == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(candidate.token)) == 1 {
			return Principal{Name: candidate.name, Role: candidate.role}, true
		}
	}
	return Principal{}, false
}
