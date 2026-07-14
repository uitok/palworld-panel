package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"palpanel/internal/paldefender"
	"palpanel/internal/pallocalize"
)

const palDefenderGMRequestLimit = 256 << 10

func (s Server) palDefenderGMStatus(c *gin.Context) {
	ok(c, s.defender.GMStatus(c.Request.Context()))
}

func (s Server) palDefenderGMPlayers(c *gin.Context) {
	response, err := s.defender.RESTPlayers(c.Request.Context())
	if err != nil {
		failPalDefenderGM(c, err)
		return
	}
	ok(c, response)
}

func (s Server) palDefenderGMItems(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit < 1 || limit > 5000 {
		fail(c, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 5000")
		return
	}
	items := pallocalize.SearchItems(c.Query("q"), limit)
	ok(c, gin.H{"items": items, "returned": len(items)})
}

func (s Server) palDefenderGMInventory(c *gin.Context) {
	response, err := s.defender.RESTInventory(c.Request.Context(), c.Param("id"))
	if err != nil {
		failPalDefenderGM(c, err)
		return
	}
	ok(c, response)
}

func (s Server) palDefenderGMGiveItems(c *gin.Context) {
	var request paldefender.GiveItemsRequest
	if err := bindPalDefenderGMJSON(c, &request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	response, err := s.defender.RESTGiveItems(c.Request.Context(), c.Param("id"), request)
	if err != nil {
		failPalDefenderGM(c, err)
		return
	}
	ok(c, response)
}

func (s Server) palDefenderGMSendMessage(c *gin.Context) {
	var request paldefender.SendMessageRequest
	if err := bindPalDefenderGMJSON(c, &request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	response, err := s.defender.RESTSendMessage(c.Request.Context(), c.Param("id"), request)
	if err != nil {
		failPalDefenderGM(c, err)
		return
	}
	ok(c, response)
}

func (s Server) palDefenderGMBroadcast(c *gin.Context) {
	var request struct {
		Message string `json:"message" binding:"required"`
		Alert   bool   `json:"alert"`
	}
	if err := bindPalDefenderGMJSON(c, &request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	response, err := s.defender.RESTBroadcast(c.Request.Context(), request.Message, request.Alert)
	if err != nil {
		failPalDefenderGM(c, err)
		return
	}
	ok(c, response)
}

func (s Server) palDefenderGMKick(c *gin.Context) {
	s.palDefenderGMPunishment(c, s.defender.RESTKick)
}

func (s Server) palDefenderGMBan(c *gin.Context) {
	s.palDefenderGMPunishment(c, s.defender.RESTBan)
}

func (s Server) palDefenderGMUnban(c *gin.Context) {
	s.palDefenderGMPunishment(c, s.defender.RESTUnban)
}

func (s Server) palDefenderGMPunishment(c *gin.Context, action func(ctx context.Context, identifier string, request paldefender.PunishmentRequest) (paldefender.RESTActionResult, error)) {
	var request paldefender.PunishmentRequest
	if c.Request.ContentLength != 0 {
		if err := bindPalDefenderGMJSON(c, &request); err != nil {
			fail(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
	}
	response, err := action(c.Request.Context(), c.Param("id"), request)
	if err != nil {
		failPalDefenderGM(c, err)
		return
	}
	ok(c, response)
}

func failPalDefenderGM(c *gin.Context, err error) {
	if errors.Is(err, paldefender.ErrRESTTokenMissing) {
		fail(c, http.StatusConflict, "paldefender_rest_not_configured", err.Error())
		return
	}
	var restErr *paldefender.RESTError
	if errors.As(err, &restErr) {
		status := restErr.Status
		if status == http.StatusUnauthorized || status == http.StatusForbidden || status >= http.StatusInternalServerError {
			status = http.StatusBadGateway
		}
		if status < http.StatusBadRequest || status >= 600 {
			status = http.StatusBadGateway
		}
		code := normalizePalDefenderErrorCode(restErr.Code)
		if code == "" {
			code = "rest_failed"
		}
		fail(c, status, "paldefender_"+code, restErr.Message)
		return
	}
	if errors.Is(err, paldefender.ErrInvalidRESTRequest) {
		fail(c, http.StatusBadRequest, "paldefender_invalid_request", err.Error())
		return
	}
	fail(c, http.StatusBadGateway, "paldefender_rest_unavailable", err.Error())
}

func bindPalDefenderGMJSON(c *gin.Context, value any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, palDefenderGMRequestLimit)
	return c.ShouldBindJSON(value)
}

func normalizePalDefenderErrorCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	var normalized strings.Builder
	for _, char := range code {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			normalized.WriteRune(char)
		case char == '-' || char == '_' || char == ' ':
			if normalized.Len() > 0 {
				normalized.WriteByte('_')
			}
		}
	}
	return strings.Trim(normalized.String(), "_")
}
