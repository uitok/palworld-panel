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
	"palpanel/internal/paldefender"
	"palpanel/internal/palrest"
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
	router := NewRouter(
		cfg,
		store,
		server.NewManager(cfg, store, runner),
		mods.NewManager(cfg, store, runner),
		paldefender.NewManager(cfg, store),
		palrest.New("", "", ""),
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
}
