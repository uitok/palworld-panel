package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

func TestWorkshopStatusUsesEmbeddedSteamAPIKeyWhenEnvUnset(t *testing.T) {
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
		WorkshopAppID:   "1623730",
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

	req := httptest.NewRequest(http.MethodGet, "/api/mods/workshop/status", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"configured":true`) || !strings.Contains(body, `"key_source":"embedded"`) {
		t.Fatalf("unexpected status response: %s", body)
	}
}

func TestWorkshopStatusDoesNotExposeSteamAPIKey(t *testing.T) {
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
		SteamWebAPIKey:  "secret-steam-key",
		WorkshopAppID:   "1623730",
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

	req := httptest.NewRequest(http.MethodGet, "/api/mods/workshop/status", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"configured":true`) ||
		!strings.Contains(rec.Body.String(), `"key_source":"env"`) ||
		strings.Contains(rec.Body.String(), "secret-steam-key") {
		t.Fatalf("unexpected status response: %s", rec.Body.String())
	}
}
