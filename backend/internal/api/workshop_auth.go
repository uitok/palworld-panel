package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"palpanel/internal/mods"
	"palpanel/internal/steamcmd"
)

const maxSteamAuthRequestBytes = 4 << 10

type workshopAuthRequest struct {
	AccountName string `json:"account_name"`
}

func (s Server) workshopAuthStatus(c *gin.Context) {
	status, err := s.mods.WorkshopAuthStatus(c.Request.Context())
	if err != nil {
		failSteamAuth(c, "status", err)
		return
	}
	ok(c, status)
}

func (s Server) startWorkshopAuth(c *gin.Context) {
	if !requireLoopbackSteamAuth(c) {
		return
	}
	request, valid := decodeWorkshopAuthRequest(c)
	if !valid {
		return
	}
	status, err := s.mods.StartWorkshopLogin(c.Request.Context(), request.AccountName)
	if err != nil {
		failSteamAuth(c, "start", err)
		return
	}
	ok(c, status)
}

func (s Server) verifyWorkshopAuth(c *gin.Context) {
	if !requireLoopbackSteamAuth(c) {
		return
	}
	request, valid := decodeWorkshopAuthRequest(c)
	if !valid {
		return
	}
	status, err := s.mods.VerifyWorkshopLogin(c.Request.Context(), request.AccountName)
	if err != nil {
		failSteamAuth(c, "verify", err)
		return
	}
	ok(c, status)
}

func requireLoopbackSteamAuth(c *gin.Context) bool {
	remote := strings.TrimSpace(c.Request.RemoteAddr)
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip != nil && ip.IsLoopback() {
		return true
	}
	fail(c, http.StatusForbidden, "steam_login_local_only", "Steam login operations are available only from the server host")
	return false
}

func (s Server) requireWorkshopLogin(c *gin.Context) bool {
	_, err := s.mods.RequireWorkshopLogin(c.Request.Context())
	if err == nil {
		return true
	}
	if errors.Is(err, steamcmd.ErrLoginRequired) {
		fail(c, http.StatusUnauthorized, "steam_login_required", steamcmd.ErrLoginRequired.Error())
		return false
	}
	failSteamAuth(c, "verify", err)
	return false
}

func decodeWorkshopAuthRequest(c *gin.Context) (workshopAuthRequest, bool) {
	var request workshopAuthRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSteamAuthRequestBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		fail(c, http.StatusBadRequest, "invalid_steam_auth_request", "request must contain only an optional account_name string")
		return workshopAuthRequest{}, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		fail(c, http.StatusBadRequest, "invalid_steam_auth_request", "request must contain one JSON object")
		return workshopAuthRequest{}, false
	}
	return request, true
}

func failSteamAuth(c *gin.Context, operation string, err error) {
	switch {
	case errors.Is(err, steamcmd.ErrInvalidAccountName):
		fail(c, http.StatusBadRequest, "invalid_steam_account", err.Error())
	case errors.Is(err, mods.ErrSteamAccountRequired):
		fail(c, http.StatusBadRequest, "steam_account_required", mods.ErrSteamAccountRequired.Error())
	case errors.Is(err, steamcmd.ErrInteractiveLogin):
		fail(c, http.StatusConflict, "steam_login_unsupported", steamcmd.ErrInteractiveLogin.Error())
	case errors.Is(err, context.Canceled):
		fail(c, http.StatusRequestTimeout, "steam_login_cancelled", "Steam login operation was cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		fail(c, http.StatusGatewayTimeout, "steam_login_timeout", "Steam login verification timed out")
	default:
		fail(c, http.StatusBadGateway, "steam_login_"+operation+"_failed", "Steam login operation failed")
	}
}
