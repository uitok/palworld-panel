package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/mods"
	"palpanel/internal/monitor"
	"palpanel/internal/palconfig"
	"palpanel/internal/paldefender"
	"palpanel/internal/palrest"
	"palpanel/internal/scheduler"
	"palpanel/internal/server"
)

func TestServerMetricsNormalizesPalworldRESTResponse(t *testing.T) {
	rest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api/metrics" {
			t.Fatalf("unexpected REST path %s", r.URL.Path)
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "admin" || password != "from-settings" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"serverfps":        58.5,
			"currentplayernum": 3,
			"maxplayernum":     32,
			"serverframetime":  17.2,
			"uptime":           90,
			"basecampnum":      7,
			"days":             12,
		})
	}))
	defer rest.Close()

	router, cleanup := newMetricsTestRouter(t, palrest.New(rest.URL+"/v1/api", "admin", ""))
	defer cleanup.Close()
	if err := palconfig.Write(cleanup.cfg.PalWorldSettingsPath(), palconfig.Settings{"AdminPassword": "from-settings"}); err != nil {
		t.Fatalf("writing palworld settings returned error: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/server/metrics", nil)
	authorizeTestRequest(req)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeMetricsResponse(t, rec)
	if payload["source"] != "palworld_rest" {
		t.Fatalf("source = %#v", payload["source"])
	}
	if payload["current_players"] != float64(3) || payload["max_players"] != float64(32) {
		t.Fatalf("unexpected players payload: %#v", payload)
	}
	if payload["server_fps"] != 58.5 || payload["frame_time"] != 17.2 {
		t.Fatalf("unexpected performance payload: %#v", payload)
	}
	if payload["active_bases"] != float64(7) || payload["days"] != float64(12) {
		t.Fatalf("unexpected 1.0 metrics payload: %#v", payload)
	}
}

func TestOfficialOneDotZeroRESTReadContracts(t *testing.T) {
	rest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responses := map[string]any{
			"/v1/api/info": map[string]any{
				"version": "v1.0.1.81201", "servername": "PalPanel", "description": "test", "worldguid": "world-guid",
			},
			"/v1/api/players": map[string]any{"players": []any{map[string]any{
				"name": "Player", "accountName": "account", "playerId": "player-id", "userId": "steam-id",
				"ip": "127.0.0.1", "ping": 3.14, "location_x": 1.0, "location_y": 2.0, "level": 3, "building_count": 4,
			}}},
			"/v1/api/settings": map[string]any{"Difficulty": "None", "RESTAPIEnabled": true, "RESTAPIPort": 8212},
			"/v1/api/game-data": map[string]any{
				"Time": "2026-06-17 13:00:40", "FPS": 91.71, "AverageFPS": 33.78, "ActorData": []any{map[string]any{"id": "actor"}},
			},
		}
		body, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer rest.Close()

	router, cleanup := newMetricsTestRouter(t, palrest.New(rest.URL+"/v1/api", "admin", ""))
	defer cleanup.Close()
	for _, path := range []string{"info", "players", "settings"} {
		rec := requestMetricsTestRoute(t, router, "/api/server/"+path)
		var envelope struct {
			OK   bool `json:"ok"`
			Data struct {
				Status int            `json:"status"`
				Body   map[string]any `json:"body"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil || !envelope.OK || envelope.Data.Status != http.StatusOK || envelope.Data.Body == nil {
			t.Fatalf("%s contract response = %s, error = %v", path, rec.Body.String(), err)
		}
	}

	rec := requestMetricsTestRoute(t, router, "/api/server/game-data")
	payload := decodeMetricsResponse(t, rec)
	if payload["Time"] != "2026-06-17 13:00:40" || payload["ActorData"] == nil {
		t.Fatalf("unexpected game-data payload: %#v", payload)
	}
}

func TestServerGameDataRejectsOversizedResponse(t *testing.T) {
	rest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ActorData":"this response is larger than the configured test limit"}`))
	}))
	defer rest.Close()

	router, cleanup := newMetricsTestRouterWithConfig(t, palrest.New(rest.URL+"/v1/api", "admin", ""), func(cfg *appconfig.Config) {
		cfg.PalworldGameDataMaxBytes = 16
		cfg.PalworldGameDataTimeoutMS = 500
	})
	defer cleanup.Close()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/server/game-data", nil)
	authorizeTestRequest(req)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "palworld_game_data_too_large") {
		t.Fatalf("expected bounded game-data failure, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestServerGameDataUsesDedicatedShortTimeout(t *testing.T) {
	rest := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer rest.Close()

	router, cleanup := newMetricsTestRouterWithConfig(t, palrest.New(rest.URL+"/v1/api", "admin", ""), func(cfg *appconfig.Config) {
		cfg.PalworldGameDataMaxBytes = 1024
		cfg.PalworldGameDataTimeoutMS = 20
	})
	defer cleanup.Close()
	started := time.Now()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/server/game-data", nil)
	authorizeTestRequest(req)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "palworld_game_data_failed") {
		t.Fatalf("expected timed out game-data failure, got %d: %s", rec.Code, rec.Body.String())
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("game-data request ignored dedicated timeout: %s", elapsed)
	}
}

func TestServerMetricsFallsBackToMonitorSampleWhenRESTUnavailable(t *testing.T) {
	rest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("closed before use")
	}))
	restURL := rest.URL
	rest.Close()

	router, cleanup := newMetricsTestRouter(t, palrest.New(restURL+"/v1/api", "admin", ""))
	defer cleanup.Close()
	store := cleanup.store
	if err := store.InsertMonitorSample(context.Background(), db.MonitorSample{
		ID:                "mon_1",
		CreatedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		CurrentPlayers:    2,
		MaxPlayers:        32,
		RESTHealthy:       false,
		UnavailableReason: "REST: EOF",
	}); err != nil {
		t.Fatalf("InsertMonitorSample returned error: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/server/metrics", nil)
	authorizeTestRequest(req)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 fallback, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeMetricsResponse(t, rec)
	if payload["source"] != "monitor_sample" {
		t.Fatalf("source = %#v", payload["source"])
	}
	if payload["current_players"] != float64(2) || payload["max_players"] != float64(32) {
		t.Fatalf("unexpected fallback players payload: %#v", payload)
	}
	if payload["error"] == "" {
		t.Fatalf("expected REST error in fallback payload: %#v", payload)
	}
}

type metricsTestCleanup struct {
	store *db.Store
	cfg   appconfig.Config
}

func (c metricsTestCleanup) Close() {
	_ = c.store.Close()
}

func newMetricsTestRouter(t *testing.T, restClient palrest.Client) (*gin.Engine, metricsTestCleanup) {
	return newMetricsTestRouterWithConfig(t, restClient, nil)
}

func newMetricsTestRouterWithConfig(t *testing.T, restClient palrest.Client, mutate func(*appconfig.Config)) (*gin.Engine, metricsTestCleanup) {
	t.Helper()
	root := t.TempDir()
	cfg := appconfig.Config{
		DataDir:         root,
		ServerDir:       filepath.Join(root, "server"),
		WinePrefixDir:   filepath.Join(root, "wineprefix"),
		ToolsDir:        filepath.Join(root, "tools"),
		SteamCMDDir:     filepath.Join(root, "tools", "steamcmd"),
		UploadsDir:      filepath.Join(root, "uploads"),
		BackupsDir:      filepath.Join(root, "backups"),
		LogsDir:         filepath.Join(root, "logs"),
		DBPath:          filepath.Join(root, "test.db"),
		RequireAuth:     true,
		DockerBinary:    "docker",
		DockerImage:     "test-image",
		DockerContainer: "test-container",
		GamePort:        8211,
		QueryPort:       27015,
		RESTPort:        8212,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs returned error: %v", err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
	provisionTestPrincipal(t, store, RoleAdmin)
	runner := docker.NewRunner(cfg)
	serverManager := server.NewManager(cfg, store, runner)
	monitorManager := monitor.New(cfg, store, serverManager, restClient)
	return NewRouter(
		cfg,
		store,
		serverManager,
		mods.NewManager(cfg, store, runner),
		paldefender.NewManager(cfg, store),
		restClient,
		monitorManager,
		scheduler.New(store, serverManager, restClient),
	), metricsTestCleanup{store: store, cfg: cfg}
}

func requestMetricsTestRoute(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	authorizeTestRequest(req)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s expected 200, got %d: %s", path, rec.Code, rec.Body.String())
	}
	return rec
}

func decodeMetricsResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("response not ok: %s", rec.Body.String())
	}
	return envelope.Data
}
