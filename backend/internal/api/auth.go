package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func Auth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
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
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			fail(c, http.StatusUnauthorized, "invalid_token", "invalid bearer token")
			c.Abort()
			return
		}
		c.Next()
	}
}
