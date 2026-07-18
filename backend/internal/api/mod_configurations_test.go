package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/mods"
)

func TestModConfigurationHandlersListReadAndWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	cfg := appconfig.Config{
		RuntimeRoot: root, DataDir: filepath.Join(root, "data"), ServerDir: filepath.Join(root, "server"),
		UploadsDir: filepath.Join(root, "uploads"), BackupsDir: filepath.Join(root, "backups"),
		DBPath: filepath.Join(root, "test.db"),
	}
	modRoot := filepath.Join(cfg.ServerDir, "Mods", "Workshop", "mod_one")
	if err := os.MkdirAll(modRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modRoot, "Config.json"), []byte(`{"Enabled":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.UpsertMod(t.Context(), db.Mod{ID: "mod_one", Name: "One", PackageName: "One", Path: modRoot}); err != nil {
		t.Fatal(err)
	}
	server := Server{cfg: cfg, store: store, mods: mods.NewManager(cfg, store, docker.NewRunner(cfg))}
	router := gin.New()
	router.GET("/api/mods/:id/files", server.listModConfigFiles)
	router.GET("/api/mods/:id/files/:file", server.getModConfigFile)
	router.PUT("/api/mods/:id/files/:file", server.updateModConfigFile)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/mods/mod_one/files", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", response.Code, response.Body.String())
	}
	var list struct {
		Data []mods.ConfigFile `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil || len(list.Data) != 1 {
		t.Fatalf("list response = %s, %v", response.Body.String(), err)
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/mods/mod_one/files/"+list.Data[0].ID, nil))
	var document struct {
		Data mods.ConfigDocument `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	body := bytes.NewBufferString(`{"content":"{\"Enabled\":false}","revision":"` + document.Data.File.Revision + `"}`)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/mods/mod_one/files/"+list.Data[0].ID, body))
	if response.Code != http.StatusOK {
		t.Fatalf("write status = %d, body = %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil || document.Data.Content != `{"Enabled":false}` {
		t.Fatalf("write response = %s, %v", response.Body.String(), err)
	}
}
