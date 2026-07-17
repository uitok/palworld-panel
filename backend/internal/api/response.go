package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ok(c *gin.Context, data any) {
	succeed(c, http.StatusOK, data)
}

func accepted(c *gin.Context, data any) {
	succeed(c, http.StatusAccepted, data)
}

func created(c *gin.Context, data any) {
	succeed(c, http.StatusCreated, data)
}

func succeed(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"ok": true, "data": data})
}

func fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"ok": false, "error": gin.H{"code": code, "message": message}})
}
