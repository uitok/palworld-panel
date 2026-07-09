package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

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

func TestPlayerAccessEmptyListsReturnArrays(t *testing.T) {
	router, cleanup := newPlayerTestRouter(t, palrest.New("", "admin", ""))
	defer cleanup.Close()

	for _, path := range []string{"/api/players/bans", "/api/players/whitelist"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer secret")
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s expected 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
		var envelope struct {
			OK   bool            `json:"ok"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("response is not JSON: %v", err)
		}
		if !envelope.OK {
			t.Fatalf("response not ok: %s", rec.Body.String())
		}
		if string(envelope.Data) != "[]" {
			t.Fatalf("%s data = %s, want []", path, envelope.Data)
		}
	}
}

func TestPlayerActionsUseDynamicPalworldRESTFromSettings(t *testing.T) {
	var paths []string
	rest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "admin" || password != "from-settings" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		paths = append(paths, r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer rest.Close()

	router, cleanup := newPlayerTestRouter(t, palrest.New("http://127.0.0.1:1/v1/api", "admin", "static-password"))
	defer cleanup.Close()

	restURL, err := url.Parse(rest.URL)
	if err != nil {
		t.Fatalf("parse rest URL: %v", err)
	}
	if err := palconfig.Write(cleanup.cfg.PalWorldSettingsPath(), palconfig.Settings{
		"RESTAPIPort":   restURL.Port(),
		"AdminPassword": "from-settings",
	}); err != nil {
		t.Fatalf("writing palworld settings returned error: %v", err)
	}

	postJSON(t, router, "/api/players/76561198000000001/kick", map[string]any{"message": "test kick"})
	postJSON(t, router, "/api/players/76561198000000001/ban", map[string]any{"nickname": "Tester", "reason": "test ban"})
	bans, err := cleanup.store.ListPlayerAccess(context.Background(), "ban")
	if err != nil {
		t.Fatalf("ListPlayerAccess returned error: %v", err)
	}
	if len(bans) != 1 || bans[0].SteamID != "76561198000000001" || bans[0].Nickname != "Tester" || bans[0].Reason != "test ban" {
		t.Fatalf("unexpected ban record: %#v", bans)
	}

	postJSON(t, router, "/api/players/76561198000000001/unban", map[string]any{"userid": "ignored"})
	bans, err = cleanup.store.ListPlayerAccess(context.Background(), "ban")
	if err != nil {
		t.Fatalf("ListPlayerAccess after unban returned error: %v", err)
	}
	if len(bans) != 0 {
		t.Fatalf("expected ban record removed, got %#v", bans)
	}

	want := []string{"/v1/api/kick", "/v1/api/ban", "/v1/api/unban"}
	if len(paths) != len(want) {
		t.Fatalf("REST paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("REST paths = %#v, want %#v", paths, want)
		}
	}
}

type playerTestCleanup struct {
	store *db.Store
	cfg   appconfig.Config
}

func (c playerTestCleanup) Close() {
	_ = c.store.Close()
}

func newPlayerTestRouter(t *testing.T, restClient palrest.Client) (*gin.Engine, playerTestCleanup) {
	t.Helper()
	root := t.TempDir()
	cfg := appconfig.Config{
		DataDir:             root,
		ServerDir:           filepath.Join(root, "server"),
		WinePrefixDir:       filepath.Join(root, "wineprefix"),
		ToolsDir:            filepath.Join(root, "tools"),
		SteamCMDDir:         filepath.Join(root, "tools", "steamcmd"),
		UploadsDir:          filepath.Join(root, "uploads"),
		BackupsDir:          filepath.Join(root, "backups"),
		LogsDir:             filepath.Join(root, "logs"),
		DBPath:              filepath.Join(root, "test.db"),
		PanelToken:          "secret",
		RequireAuth:         true,
		DockerBinary:        "docker",
		DockerImage:         "test-image",
		DockerContainer:     "test-container",
		GamePort:            8211,
		QueryPort:           27015,
		RESTPort:            8212,
		PalworldRESTBaseURL: restClient.BaseURL,
		PalworldRESTUser:    restClient.User,
		PalworldRESTPass:    restClient.Password,
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
	), playerTestCleanup{store: store, cfg: cfg}
}

func postJSON(t *testing.T, router *gin.Engine, path string, payload map[string]any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s expected 200, got %d: %s", path, rec.Code, rec.Body.String())
	}
}
