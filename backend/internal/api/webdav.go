package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"palpanel/internal/server"
)

func (s Server) getWebDAVConfig(c *gin.Context) {
	config, err := s.server.WebDAVConfig(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "webdav_config_read_failed", err.Error())
		return
	}
	ok(c, config)
}

func (s Server) putWebDAVConfig(c *gin.Context) {
	var update server.WebDAVConfigUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	config, err := s.server.UpdateWebDAVConfig(c.Request.Context(), update)
	if err != nil {
		fail(c, http.StatusBadRequest, "webdav_config_write_failed", err.Error())
		return
	}
	ok(c, config)
}

func (s Server) testWebDAVConfig(c *gin.Context) {
	var update server.WebDAVConfigUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := s.server.TestWebDAV(c.Request.Context(), update); err != nil {
		fail(c, http.StatusBadGateway, "webdav_test_failed", err.Error())
		return
	}
	ok(c, gin.H{"connected": true})
}

func (s Server) uploadBackupToWebDAV(c *gin.Context) {
	job, err := s.server.UploadBackupToWebDAV(c.Request.Context(), c.Param("name"))
	if err != nil {
		fail(c, http.StatusBadRequest, "webdav_upload_failed", err.Error())
		return
	}
	accepted(c, job)
}
