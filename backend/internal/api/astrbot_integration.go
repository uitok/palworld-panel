package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"palpanel/internal/astrbotclient"
	"palpanel/internal/breeding"
	"palpanel/internal/db"
	"palpanel/internal/id"
	"palpanel/internal/paldefender"
)

const breedSessionCookie = "palpanel_breed_session"
const breedPrincipalKey = "breed_principal"

type breedPrincipal struct {
	Subject   string `json:"subject"`
	QQID      string `json:"qq_id"`
	PlayerUID string `json:"player_uid"`
	Nickname  string `json:"nickname,omitempty"`
	Balance   *int   `json:"balance,omitempty"`
}

var astrBotNonces sync.Map

func (s Server) exchangeBreedSession(c *gin.Context) {
	var input struct {
		Ticket string `json:"ticket" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, http.StatusBadRequest, "ticket_required", "a one-time AstrBot ticket is required")
		return
	}
	identity, err := s.astrbot.ExchangeTicket(c.Request.Context(), input.Ticket)
	if err != nil || identity.PlayerUID == "" || identity.QQID == "" {
		fail(c, http.StatusUnauthorized, "ticket_invalid", "the AstrBot ticket is invalid or expired")
		return
	}
	token := randomToken(32)
	hash := sha256.Sum256([]byte(token))
	expires := time.Now().UTC().Add(12 * time.Hour)
	if err := s.store.CreateBreedSession(c.Request.Context(), db.BreedSession{
		ID: id.New("breed_session"), Subject: "qq:" + identity.QQID, TokenHash: hex.EncodeToString(hash[:]),
		PlayerUID: identity.PlayerUID, ExpiresAt: expires.Format(time.RFC3339Nano),
	}); err != nil {
		fail(c, http.StatusInternalServerError, "breed_session_failed", err.Error())
		return
	}
	secureCookie := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	http.SetCookie(c.Writer, &http.Cookie{Name: breedSessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secureCookie, Expires: expires})
	balance := identity.Balance
	ok(c, breedPrincipal{Subject: "qq:" + identity.QQID, QQID: identity.QQID, PlayerUID: identity.PlayerUID, Nickname: identity.Nickname, Balance: &balance})
}

func (s Server) breedSessionAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(breedSessionCookie)
		if err != nil {
			fail(c, http.StatusUnauthorized, "breed_auth_required", "open the one-time link from the QQ bot")
			c.Abort()
			return
		}
		hash := sha256.Sum256([]byte(cookie))
		session, err := s.store.GetBreedSession(c.Request.Context(), hex.EncodeToString(hash[:]), time.Now())
		if err != nil {
			fail(c, http.StatusUnauthorized, "breed_session_expired", "the breeding session expired")
			c.Abort()
			return
		}
		qqID := strings.TrimPrefix(session.Subject, "qq:")
		c.Set(breedPrincipalKey, breedPrincipal{Subject: session.Subject, QQID: qqID, PlayerUID: session.PlayerUID})
		c.Next()
	}
}

func currentBreedPrincipal(c *gin.Context) breedPrincipal {
	if value, ok := c.Get(breedPrincipalKey); ok {
		if principal, valid := value.(breedPrincipal); valid {
			return principal
		}
	}
	return breedPrincipal{}
}

func (s Server) breedSessionMe(c *gin.Context) { ok(c, currentBreedPrincipal(c)) }

func (s Server) breedCatalog(c *gin.Context) {
	raw, err := s.breeding.Catalog(c.Request.Context())
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "palcalc_unavailable", err.Error())
		return
	}
	c.Data(http.StatusOK, "application/json", raw)
}

func (s Server) breedSubmitJob(c *gin.Context) {
	principal := currentBreedPrincipal(c)
	var input breeding.SubmitInput
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, http.StatusBadRequest, "breeding_request_invalid", err.Error())
		return
	}
	input.OwnerPlayerUID = principal.PlayerUID
	referenceID := id.New("charge")
	reservation, balance, err := s.astrbot.Reserve(c.Request.Context(), principal.QQID, referenceID, 0)
	if err != nil {
		fail(c, http.StatusPaymentRequired, "insufficient_points", "积分不足，无法开始配种计算")
		return
	}
	billing := &breeding.Billing{ReservationID: reservation, Settle: s.astrbot.Settle}
	job, err := s.breeding.Submit(c.Request.Context(), principal.Subject, input, billing)
	if err != nil {
		_ = s.astrbot.Settle(c.Request.Context(), reservation, false)
		fail(c, http.StatusConflict, "breeding_submit_failed", err.Error())
		return
	}
	accepted(c, gin.H{"job": job, "balance": balance})
}

func (s Server) breedHistory(c *gin.Context) {
	items, err := s.breeding.History(c.Request.Context(), currentBreedPrincipal(c).Subject, 30)
	if err != nil {
		fail(c, http.StatusInternalServerError, "breeding_history_failed", err.Error())
		return
	}
	for position := range items {
		items[position].RequestJSON, items[position].ResultJSON = "", ""
	}
	ok(c, items)
}

func (s Server) breedJob(c *gin.Context) {
	if !s.ownsBreedJob(c) {
		return
	}
	job, err := s.store.GetJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "job_not_found", "job was not found")
		return
	}
	ok(c, job)
}

func (s Server) breedJobResult(c *gin.Context) {
	if !s.ownsBreedJob(c) {
		return
	}
	item, raw, err := s.breeding.Result(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "breeding_result_not_found", "result was not found")
		return
	}
	if item.Status != "completed" {
		ok(c, gin.H{"job_id": item.JobID, "status": item.Status})
		return
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		fail(c, http.StatusInternalServerError, "breeding_result_invalid", err.Error())
		return
	}
	ok(c, gin.H{"job_id": item.JobID, "status": item.Status, "fingerprint": item.Fingerprint, "stale": s.breedingResultStale(c.Request.Context(), item.Fingerprint), "result": result})
}

func (s Server) breedControlJob(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.ownsBreedJob(c) {
			return
		}
		raw, err := s.breeding.Control(c.Request.Context(), c.Param("id"), action)
		if err != nil {
			fail(c, http.StatusBadGateway, "breeding_control_failed", err.Error())
			return
		}
		var value any
		_ = json.Unmarshal(raw, &value)
		ok(c, value)
	}
}

func (s Server) ownsBreedJob(c *gin.Context) bool {
	item, err := s.store.GetBreedingResultByJob(c.Request.Context(), c.Param("id"))
	if errorsIsNoRows(err) {
		fail(c, http.StatusNotFound, "job_not_found", "job was not found")
		return false
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "job_lookup_failed", err.Error())
		return false
	}
	if item.Subject != currentBreedPrincipal(c).Subject {
		fail(c, http.StatusForbidden, "permission_denied", "permission denied")
		return false
	}
	return true
}

func errorsIsNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

func (s Server) astrBotSignatureAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.astrbot.Enabled() {
			fail(c, http.StatusServiceUnavailable, "astrbot_disabled", "AstrBot integration is not configured")
			c.Abort()
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if err != nil {
			fail(c, http.StatusBadRequest, "request_read_failed", err.Error())
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		nonce := c.GetHeader("X-PalPanel-Nonce")
		if c.GetHeader("X-PalPanel-Id") != s.cfg.AstrBotPanelID || !astrbotclient.Verify(s.cfg.AstrBotSharedSecret, c.Request.Method, c.Request.URL.Path, c.GetHeader("X-PalPanel-Timestamp"), nonce, c.GetHeader("X-PalPanel-Signature"), body) {
			fail(c, http.StatusUnauthorized, "integration_signature_invalid", "invalid integration signature")
			c.Abort()
			return
		}
		now := time.Now()
		astrBotNonces.Range(func(key, value any) bool {
			if seen, valid := value.(time.Time); !valid || now.Sub(seen) > 2*time.Minute {
				astrBotNonces.Delete(key)
			}
			return true
		})
		if _, loaded := astrBotNonces.LoadOrStore(nonce, now); loaded {
			fail(c, http.StatusUnauthorized, "integration_replay_rejected", "integration nonce was already used")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s Server) astrBotBindingChallenge(c *gin.Context) {
	var input struct {
		PlayerUID string `json:"player_uid"`
		Nickname  string `json:"nickname"`
		Message   string `json:"message"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Message) == "" {
		fail(c, http.StatusBadRequest, "challenge_invalid", "player_uid and message are required")
		return
	}
	players, err := s.defender.RESTPlayers(c.Request.Context())
	if err != nil {
		failPalDefenderGM(c, err)
		return
	}
	var matched *paldefender.RESTPlayer
	for position := range players.Players {
		player := &players.Players[position]
		if strings.EqualFold(player.PlayerUID, input.PlayerUID) || strings.EqualFold(player.UserID, input.PlayerUID) || strings.EqualFold(player.Name, input.Nickname) {
			matched = player
			break
		}
	}
	if matched == nil {
		fail(c, http.StatusConflict, "player_not_online", "the player is not online")
		return
	}
	identifier := matched.UserID
	if identifier == "" {
		identifier = matched.PlayerUID
	}
	result, err := s.defender.RESTSendMessage(c.Request.Context(), identifier, paldefender.SendMessageRequest{SendType: "Chat", Message: input.Message})
	if err != nil {
		failPalDefenderGM(c, err)
		return
	}
	ok(c, result)
}

func (s Server) astrBotQuickSolve(c *gin.Context) {
	var input struct {
		QQID      string   `json:"qq_id"`
		PlayerUID string   `json:"player_uid"`
		Target    string   `json:"target"`
		Passives  []string `json:"passives"`
		JobID     string   `json:"job_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.QQID) == "" {
		fail(c, http.StatusBadRequest, "quick_solve_invalid", "qq_id is required")
		return
	}
	subject := "qq:" + input.QQID
	if input.JobID != "" {
		item, err := s.store.GetBreedingResultByJob(c.Request.Context(), input.JobID)
		if err != nil || item.Subject != subject {
			fail(c, http.StatusNotFound, "quick_solve_not_found", "quick solve was not found")
			return
		}
		job, err := s.store.GetJob(c.Request.Context(), input.JobID)
		if err != nil {
			fail(c, http.StatusNotFound, "quick_solve_not_found", "quick solve was not found")
			return
		}
		response := gin.H{"job": job}
		if item.Status == "completed" && item.ResultJSON != "" {
			var result any
			if json.Unmarshal([]byte(item.ResultJSON), &result) == nil {
				response["result"] = result
			}
		}
		ok(c, response)
		return
	}
	if strings.TrimSpace(input.PlayerUID) == "" || strings.TrimSpace(input.Target) == "" {
		fail(c, http.StatusBadRequest, "quick_solve_invalid", "player_uid and target are required")
		return
	}
	referenceID := id.New("quick_charge")
	reservation, balance, err := s.astrbot.Reserve(c.Request.Context(), input.QQID, referenceID, 0)
	if err != nil {
		fail(c, http.StatusPaymentRequired, "insufficient_points", "积分不足，无法开始配种计算")
		return
	}
	job, err := s.breeding.Submit(c.Request.Context(), subject, breeding.SubmitInput{
		OwnerPlayerUID: input.PlayerUID,
		Target:         breeding.Target{PalID: input.Target, Gender: "wildcard", RequiredPassives: input.Passives},
		Settings:       breeding.Settings{MaxBreedingSteps: 6, MaxSolverIterations: 20, MaxWildPals: 1, MaxInputIrrelevantPassives: 2, MaxBredIrrelevantPassives: 1},
		GameSettings:   breeding.GameSettings{BreedingTimeSeconds: 300, MassiveEggIncubationMinutes: 120, MultipleBreedingFarms: true, MultipleIncubators: true},
		ResultLimit:    5,
	}, &breeding.Billing{ReservationID: reservation, Settle: s.astrbot.Settle})
	if err != nil {
		_ = s.astrbot.Settle(c.Request.Context(), reservation, false)
		fail(c, http.StatusConflict, "quick_solve_submit_failed", err.Error())
		return
	}
	accepted(c, gin.H{"job": job, "balance": balance})
}

func (s Server) runAstrBotCatalogSync() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		_ = s.syncAstrBotCatalog(ctx)
		cancel()
		<-ticker.C
	}
}

func (s Server) triggerAstrBotCatalogSync() {
	if !s.astrbot.Enabled() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_ = s.syncAstrBotCatalog(ctx)
	}()
}

func (s Server) syncAstrBotCatalog(ctx context.Context) error {
	index, _, err := s.saveIndex.Current(ctx)
	if err != nil {
		return err
	}
	online := map[string]bool{}
	if live, liveErr := s.defender.RESTPlayers(ctx); liveErr == nil {
		for _, player := range live.Players {
			online[strings.ToLower(player.PlayerUID)] = true
			online[strings.ToLower(player.UserID)] = true
		}
	}
	players := make([]map[string]any, 0, len(index.Players))
	for _, player := range index.Players {
		players = append(players, map[string]any{"player_uid": player.PlayerUID, "nickname": player.Nickname, "online": online[strings.ToLower(player.PlayerUID)]})
	}
	return s.astrbot.SyncCatalog(ctx, index.Snapshot.Fingerprint, players)
}

func randomToken(size int) string {
	value := make([]byte, size)
	_, _ = rand.Read(value)
	return base64.RawURLEncoding.EncodeToString(value)
}
