package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"palpanel/internal/aitranslation"
)

func (s Server) getAITranslationConfig(c *gin.Context) {
	config, err := s.ai.Config(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "ai_config_read_failed", err.Error())
		return
	}
	ok(c, config)
}

func (s Server) putAITranslationConfig(c *gin.Context) {
	var request aitranslation.ConfigUpdate
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	config, err := s.ai.UpdateConfig(c.Request.Context(), request)
	if err != nil {
		failAITranslation(c, err, "ai_config_write_failed")
		return
	}
	ok(c, config)
}

func (s Server) testAITranslationConfig(c *gin.Context) {
	var request aitranslation.ConfigUpdate
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	result, err := s.ai.Test(c.Request.Context(), request)
	if err != nil {
		failAITranslation(c, err, "ai_test_failed")
		return
	}
	ok(c, result)
}

func (s Server) translateWorkshopMod(c *gin.Context) {
	if !s.requireWorkshopLogin(c) {
		return
	}
	var request struct {
		Force bool `json:"force"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	item, err := s.mods.WorkshopDetail(c.Request.Context(), c.Param("id"))
	if err != nil {
		failWorkshop(c, err)
		return
	}
	translation, err := s.ai.Translate(c.Request.Context(), item.ID, item.Summary, request.Force)
	if err != nil {
		failAITranslation(c, err, "ai_translation_failed")
		return
	}
	ok(c, translation)
}

func failAITranslation(c *gin.Context, err error, fallbackCode string) {
	var serviceErr *aitranslation.ServiceError
	if errors.As(err, &serviceErr) {
		fail(c, serviceErr.Status, serviceErr.Code, serviceErr.Message)
		return
	}
	fail(c, http.StatusInternalServerError, fallbackCode, err.Error())
}
