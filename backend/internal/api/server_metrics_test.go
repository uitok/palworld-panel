package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	req.Header.Set("Authorization", "Bearer secret")
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
	req.Header.Set("Authorization", "Bearer secret")
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
		PanelToken:      "secret",
		RequireAuth:     true,
		DockerBinary:    "docker",
		DockerImage:     "test-image",
		DockerContainer: "test-container",
		GamePort:        8211,
		QueryPort:       27015,
		RESTPort:        8212,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs returned error: %v", err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
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
