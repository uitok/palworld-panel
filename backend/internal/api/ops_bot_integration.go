package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"palpanel/internal/db"
	"palpanel/internal/id"
)

type astrBotControlRequest struct {
	ActorQQID string `json:"actor_qq_id"`
	GroupID   string `json:"group_id"`
	Action    string `json:"action"`
	WaitTime  int    `json:"waittime"`
	Message   string `json:"message"`
}

// serverSafeStop is registered by the shared route table at
// POST /api/server/safe-stop with server:control permission.
func (s Server) serverSafeStop(c *gin.Context) {
	var req struct {
		WaitTime int    `json:"waittime"`
		Message  string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	job, err := s.server.SafeStop(c.Request.Context(), req.WaitTime, req.Message, s.requestPalworldShutdown)
	if err != nil {
		fail(c, http.StatusBadRequest, "safe_stop_failed", err.Error())
		return
	}
	s.invalidateServerCaches()
	accepted(c, job)
}

// astrBotServerStatus exposes only the bounded operational data needed by QQ
// commands. The route is protected by astrBotSignatureAuth, not browser auth.
func (s Server) astrBotServerStatus(c *gin.Context) {
	status, err := s.server.Status(c.Request.Context())
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "server_status_unavailable", err.Error())
		return
	}
	result := gin.H{
		"server": gin.H{
			"container":    gin.H{"exists": status.Container.Exists, "status": status.Container.Status},
			"runtime_mode": status.RuntimeMode, "pending_restart": status.PendingRestart, "setup_step": status.SetupStep,
		},
		"online_count": 0, "online_players": []gin.H{},
	}
	if response, restErr := s.palworldREST().Do(c.Request.Context(), http.MethodGet, "players", nil); restErr == nil {
		players := normalizeAstrBotPlayers(response.Body)
		result["online_count"] = len(players)
		result["online_players"] = players
	} else {
		result["players_available"] = false
	}
	if response, restErr := s.palworldREST().Do(c.Request.Context(), http.MethodGet, "info", nil); restErr == nil {
		result["info"] = normalizeAstrBotInfo(response.Body)
	}
	ok(c, result)
}

// astrBotServerControl accepts a small action allow-list. Administrator
// membership is enforced by the AstrBot plugin; HMAC authentication prevents
// callers outside that trusted plugin from reaching this endpoint.
func (s Server) astrBotServerControl(c *gin.Context) {
	var req astrBotControlRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ActorQQID) == "" {
		fail(c, http.StatusBadRequest, "astrbot_control_invalid", "actor_qq_id and action are required")
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if req.WaitTime == 0 {
		req.WaitTime = 60
	}
	var payload any
	var actionErr error
	switch req.Action {
	case "start":
		actionErr = s.server.Start(c.Request.Context())
		payload = gin.H{"status": "started"}
	case "safe_stop":
		payload, actionErr = s.server.SafeStop(c.Request.Context(), req.WaitTime, req.Message, s.requestPalworldShutdown)
	case "safe_restart":
		payload, actionErr = s.server.SafeRestart(c.Request.Context(), req.WaitTime, req.Message, s.requestPalworldShutdown)
	case "force_stop":
		actionErr = s.server.Stop(c.Request.Context())
		payload = gin.H{"status": "stopped"}
	default:
		fail(c, http.StatusBadRequest, "astrbot_control_action_invalid", "unsupported server control action")
		return
	}
	if actionErr != nil {
		s.auditAstrBotControl(req, c.ClientIP(), "failed", "operation_failed")
		fail(c, http.StatusBadRequest, "astrbot_control_failed", "server control operation failed")
		return
	}
	s.invalidateServerCaches()
	s.auditAstrBotControl(req, c.ClientIP(), "success", "accepted")
	if req.Action == "safe_stop" || req.Action == "safe_restart" {
		accepted(c, payload)
		return
	}
	ok(c, payload)
}

func normalizeAstrBotInfo(body any) gin.H {
	root, ok := body.(map[string]any)
	if !ok {
		return gin.H{}
	}
	return gin.H{
		"server_name": firstAstrBotString(root, "servername", "server_name", "name"),
		"version":     firstAstrBotString(root, "version"),
	}
}

func (s Server) requestPalworldShutdown(ctx context.Context, wait int, message string) error {
	client := s.palworldREST()
	var failures []error
	if _, err := client.Do(ctx, http.MethodPost, "save", nil); err != nil {
		failures = append(failures, fmt.Errorf("save world: %w", err))
	}
	if _, err := client.Do(ctx, http.MethodPost, "shutdown", gin.H{"waittime": wait, "message": message}); err != nil {
		failures = append(failures, fmt.Errorf("request shutdown: %w", err))
	}
	return errors.Join(failures...)
}

func (s Server) auditAstrBotControl(req astrBotControlRequest, ip, status, message string) {
	_ = s.store.CreateAuditLog(context.Background(), db.AuditLog{
		ID:      id.New("audit"),
		Actor:   "qq:" + strings.TrimSpace(req.ActorQQID),
		Role:    "astrbot_admin",
		Action:  "ASTRBOT server/control/" + req.Action,
		Target:  strings.TrimSpace(req.GroupID),
		Status:  status,
		Message: message,
		IP:      ip,
	})
}

func normalizeAstrBotPlayers(body any) []gin.H {
	root, ok := body.(map[string]any)
	if !ok {
		return []gin.H{}
	}
	raw, ok := root["players"].([]any)
	if !ok {
		return []gin.H{}
	}
	players := make([]gin.H, 0, len(raw))
	for _, item := range raw {
		player, ok := item.(map[string]any)
		if !ok {
			continue
		}
		players = append(players, gin.H{
			"name":  firstAstrBotString(player, "name", "nickname"),
			"level": player["level"],
		})
	}
	return players
}

func firstAstrBotString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
