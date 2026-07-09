package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/mods"
	"palpanel/internal/monitor"
	"palpanel/internal/paldefender"
	"palpanel/internal/palrest"
	"palpanel/internal/scheduler"
	"palpanel/internal/server"
)

func TestNewContractRoutes(t *testing.T) {
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
		ViewerToken:     "viewer",
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
	defer store.Close()
	runner := docker.NewRunner(cfg)
	serverManager := server.NewManager(cfg, store, runner)
	restClient := palrest.New("", "", "")
	router := NewRouter(
		cfg,
		store,
		serverManager,
		mods.NewManager(cfg, store, runner),
		paldefender.NewManager(cfg, store),
		restClient,
		monitor.New(cfg, store, serverManager, restClient),
		scheduler.New(store, serverManager, restClient),
	)

	for _, path := range []string{
		"/api/config/palworld/schema",
		"/api/server/startup",
		"/api/server/runtime",
		"/api/security/paldefender/status",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s expected 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}

	routes := map[string]bool{}
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"POST /api/players/:id/kick",
		"POST /api/players/:id/ban",
		"POST /api/players/:id/unban",
		"POST /api/server/force-stop",
		"GET /api/server/version",
		"POST /api/server/version/check",
		"POST /api/server/update-if-needed",
		"GET /api/server/host",
		"GET /api/server/docker/plan",
		"POST /api/server/docker/install",
		"GET /api/server/docker/mirrors/plan",
		"POST /api/server/docker/mirrors/configure",
		"GET /api/monitor/snapshot",
		"GET /api/monitor/history",
		"GET /api/schedules",
		"POST /api/schedules",
		"PUT /api/schedules/:id",
		"DELETE /api/schedules/:id",
		"POST /api/schedules/:id/run",
		"GET /api/alerts",
		"POST /api/alerts/:id/ack",
		"GET /api/mods/workshop/status",
		"GET /api/mods/workshop/search",
		"GET /api/mods/workshop/:id",
		"POST /api/backups/:name/restore",
		"GET /api/backups/:name/download",
		"DELETE /api/backups/:name",
		"POST /api/backups/:name/verify",
	} {
		if !routes[want] {
			t.Fatalf("missing route %s", want)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/server/docker/install", nil)
	req.Header.Set("Authorization", "Bearer viewer")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer docker install expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/server/docker/mirrors/configure", nil)
	req.Header.Set("Authorization", "Bearer viewer")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer docker mirror configure expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}
