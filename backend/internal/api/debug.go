package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const debugLoggingKV = "debug_logging_enabled"

type debugLoggingUpdate struct {
	Enabled bool `json:"enabled"`
}

func (s Server) debugLoggingStatus(c *gin.Context) {
	if s.cfg.DebugLogger == nil {
		ok(c, gin.H{
			"enabled":   false,
			"path":      s.cfg.DebugLogPath(),
			"size":      0,
			"max_bytes": 20 * 1024 * 1024,
			"max_files": 5,
		})
		return
	}
	ok(c, s.cfg.DebugLogger.Status())
}

func (s Server) putDebugLogging(c *gin.Context) {
	if s.cfg.DebugLogger == nil {
		fail(c, http.StatusServiceUnavailable, "debug_logging_unavailable", "debug logging is unavailable")
		return
	}
	var request debugLoggingUpdate
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	previous := s.cfg.DebugLogger.Enabled()
	if err := s.cfg.DebugLogger.SetEnabled(request.Enabled); err != nil {
		fail(c, http.StatusInternalServerError, "debug_logging_update_failed", err.Error())
		return
	}
	if err := s.store.SetKV(c.Request.Context(), debugLoggingKV, boolString(request.Enabled)); err != nil {
		_ = s.cfg.DebugLogger.SetEnabled(previous)
		fail(c, http.StatusInternalServerError, "debug_logging_update_failed", err.Error())
		return
	}
	s.cfg.DebugLogger.Printf("debug setting changed enabled=%t", request.Enabled)
	ok(c, s.cfg.DebugLogger.Status())
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
