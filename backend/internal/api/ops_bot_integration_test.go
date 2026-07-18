package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"palpanel/internal/appconfig"
	"palpanel/internal/astrbotclient"
	"palpanel/internal/communityservers"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/palrest"
	"palpanel/internal/server"
)

type astrBotCommunityFetcher struct{}

func (astrBotCommunityFetcher) Fetch(context.Context, communityservers.Query) ([]communityservers.Server, int, error) {
	return []communityservers.Server{{ID: "cn-1", Name: "中文服", Connect: "203.0.113.8:8211", Country: "CN", Status: "online"}}, 1, nil
}

func TestNormalizeAstrBotPlayersBoundsAndNormalizesFields(t *testing.T) {
	players := normalizeAstrBotPlayers(map[string]any{"players": []any{
		map[string]any{"name": "Alice", "playerId": "player-1", "userId": "steam-1", "level": float64(12)},
		"invalid",
		map[string]any{"nickname": "Bob", "player_id": "player-2", "user_id": "steam-2"},
	}})
	if len(players) != 2 {
		t.Fatalf("players = %#v", players)
	}
	if players[0]["name"] != "Alice" || players[0]["level"] != float64(12) || len(players[0]) != 2 {
		t.Fatalf("first player = %#v", players[0])
	}
	if players[1]["name"] != "Bob" || len(players[1]) != 2 {
		t.Fatalf("second player = %#v", players[1])
	}
}

func TestRequestPalworldShutdownAttemptsShutdownWhenSaveFails(t *testing.T) {
	var paths []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/save" {
			http.Error(w, "save unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	server := Server{palrest: palrest.New(upstream.URL, "", "")}
	if err := server.requestPalworldShutdown(t.Context(), 60, "maintenance"); err == nil {
		t.Fatal("save failure should be reported")
	}
	if !reflect.DeepEqual(paths, []string{"/save", "/shutdown"}) {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestFirstAstrBotStringSkipsEmptyValues(t *testing.T) {
	values := map[string]any{"first": " ", "second": "value"}
	if got := firstAstrBotString(values, "first", "second"); got != "value" {
		t.Fatalf("firstAstrBotString = %q", got)
	}
}

func TestNormalizeAstrBotInfoExposesOnlyBotFields(t *testing.T) {
	info := normalizeAstrBotInfo(map[string]any{
		"servername":  "PalPanel CN",
		"version":     "v1.0.0",
		"description": "private description",
		"worldguid":   "secret-world-guid",
	})
	if len(info) != 2 || info["server_name"] != "PalPanel CN" || info["version"] != "v1.0.0" {
		t.Fatalf("info = %#v", info)
	}
}

func TestAstrBotOperationalEndpointsRequireHMACRejectReplayAndAudit(t *testing.T) {
	router, store, secret, panelID := newAstrBotOpsTestRouter(t)

	status := signedAstrBotRequest(t, router, secret, panelID, "/api/integrations/astrbot/server-status", `{}`, "status-ok")
	if status.Code != http.StatusOK {
		t.Fatalf("server status = %d: %s", status.Code, status.Body.String())
	}
	if strings.Contains(status.Body.String(), "world-guid") || strings.Contains(status.Body.String(), "steam-1") || strings.Contains(status.Body.String(), "private description") {
		t.Fatalf("server status leaked internal fields: %s", status.Body.String())
	}
	if !strings.Contains(status.Body.String(), `"server_name":"PalPanel CN"`) || !strings.Contains(status.Body.String(), `"version":"v1.0.0"`) {
		t.Fatalf("server status missing bounded info: %s", status.Body.String())
	}

	community := signedAstrBotRequest(t, router, secret, panelID, "/api/integrations/astrbot/community-servers", `{"query":"中文"}`, "community-ok")
	if community.Code != http.StatusOK || !strings.Contains(community.Body.String(), "中文服") {
		t.Fatalf("community servers = %d: %s", community.Code, community.Body.String())
	}

	controlBody := `{"actor_qq_id":"10001","group_id":"20002","action":"force_stop"}`
	control := signedAstrBotRequest(t, router, secret, panelID, "/api/integrations/astrbot/server-control", controlBody, "control-ok")
	if control.Code != http.StatusOK {
		t.Fatalf("server control = %d: %s", control.Code, control.Body.String())
	}
	failedControl := signedAstrBotRequest(t, router, secret, panelID, "/api/integrations/astrbot/server-control", `{"actor_qq_id":"10001","group_id":"20002","action":"start"}`, "control-failed")
	if failedControl.Code != http.StatusBadRequest || !strings.Contains(failedControl.Body.String(), "server control operation failed") {
		t.Fatalf("failed server control = %d: %s", failedControl.Code, failedControl.Body.String())
	}
	audits, err := store.ListAuditLogs(t.Context(), 10)
	if err != nil || len(audits) < 2 {
		t.Fatalf("audit logs = %#v, %v", audits, err)
	}
	statuses := map[string]db.AuditLog{}
	for _, audit := range audits {
		statuses[audit.Status] = audit
	}
	if success := statuses["success"]; success.Actor != "qq:10001" || success.Target != "20002" || success.Message != "accepted" {
		t.Fatalf("success control audit = %#v", success)
	}
	if failed := statuses["failed"]; failed.Actor != "qq:10001" || failed.Target != "20002" || failed.Message != "operation_failed" {
		t.Fatalf("failed control audit = %#v", failed)
	}

	invalid := httptest.NewRecorder()
	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/integrations/astrbot/server-status", strings.NewReader(`{}`))
	invalidRequest.Header.Set("Content-Type", "application/json")
	invalidRequest.Header.Set("X-PalPanel-Id", panelID)
	invalidRequest.Header.Set("X-PalPanel-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	invalidRequest.Header.Set("X-PalPanel-Nonce", "invalid-signature")
	invalidRequest.Header.Set("X-PalPanel-Signature", "invalid")
	router.ServeHTTP(invalid, invalidRequest)
	if invalid.Code != http.StatusUnauthorized || !strings.Contains(invalid.Body.String(), "integration_signature_invalid") {
		t.Fatalf("invalid signature = %d: %s", invalid.Code, invalid.Body.String())
	}

	replayBody := `{"query":""}`
	first := signedAstrBotRequest(t, router, secret, panelID, "/api/integrations/astrbot/community-servers", replayBody, "replay-once")
	second := signedAstrBotRequest(t, router, secret, panelID, "/api/integrations/astrbot/community-servers", replayBody, "replay-once")
	if first.Code != http.StatusOK || second.Code != http.StatusUnauthorized || !strings.Contains(second.Body.String(), "integration_replay_rejected") {
		t.Fatalf("replay responses = %d/%d: %s", first.Code, second.Code, second.Body.String())
	}
}

func newAstrBotOpsTestRouter(t *testing.T) (*gin.Engine, *db.Store, string, string) {
	t.Helper()
	root := t.TempDir()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/players":
			_, _ = w.Write([]byte(`{"players":[{"name":"Alice","level":12,"userId":"steam-1","playerId":"player-1"}]}`))
		case "/info":
			_, _ = w.Write([]byte(`{"servername":"PalPanel CN","version":"v1.0.0","description":"private description","worldguid":"world-guid"}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	t.Cleanup(upstream.Close)
	secret := "astrbot-test-secret"
	panelID := "panel-test"
	cfg := appconfig.Config{
		DataDir: root, ServerDir: filepath.Join(root, "server"), WinePrefixDir: filepath.Join(root, "wine"),
		ToolsDir: filepath.Join(root, "tools"), SteamCMDDir: filepath.Join(root, "tools", "steamcmd"),
		UploadsDir: filepath.Join(root, "uploads"), BackupsDir: filepath.Join(root, "backups"), LogsDir: filepath.Join(root, "logs"),
		SaveIndexCacheDir: filepath.Join(root, "save-index"), DBPath: filepath.Join(root, "test.db"),
		DockerBinary: "docker", DockerImage: "test", DockerContainer: "test",
		GamePort: 8211, QueryPort: 27015, RESTPort: 8212, PalworldRESTBaseURL: upstream.URL,
		AstrBotSharedSecret: secret, AstrBotPanelID: panelID,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.PalWorldSettingsPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	manager := server.NewManager(cfg, store, docker.NewRunner(cfg))
	if err := manager.SetRuntimeMode(t.Context(), server.RuntimeWindowsSteamCMD); err != nil {
		t.Fatal(err)
	}
	community, err := communityservers.New(communityservers.Options{Fetcher: astrBotCommunityFetcher{}})
	if err != nil {
		t.Fatal(err)
	}
	s := Server{cfg: cfg, store: store, server: manager, palrest: palrest.New(upstream.URL, "admin", "secret"), community: community, astrbot: astrbotclient.New(cfg), cache: newTTLCache()}
	router := gin.New()
	integration := router.Group("/api/integrations/astrbot")
	integration.Use(s.astrBotSignatureAuth())
	integration.POST("/server-status", s.astrBotServerStatus)
	integration.POST("/server-control", s.astrBotServerControl)
	integration.POST("/community-servers", s.astrBotCommunityServers)
	return router, store, secret, panelID
}

func signedAstrBotRequest(t *testing.T, router http.Handler, secret, panelID, path, body, nonce string) *httptest.ResponseRecorder {
	t.Helper()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	payload := []byte(body)
	digest := sha256.Sum256(payload)
	canonical := strings.Join([]string{http.MethodPost, path, timestamp, nonce, hex.EncodeToString(digest[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-PalPanel-Id", panelID)
	request.Header.Set("X-PalPanel-Timestamp", timestamp)
	request.Header.Set("X-PalPanel-Nonce", nonce)
	request.Header.Set("X-PalPanel-Signature", hex.EncodeToString(mac.Sum(nil)))
	router.ServeHTTP(recorder, request)
	return recorder
}
