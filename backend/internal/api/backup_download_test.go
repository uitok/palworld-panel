package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/server"
)

func TestDownloadBackupStreamsAttachmentMetadataAndContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	cfg := appconfig.Config{
		RuntimeRoot: root,
		DataDir:     root,
		ServerDir:   filepath.Join(root, "server"),
		BackupsDir:  filepath.Join(root, "backups"),
		DBPath:      filepath.Join(root, "palpanel.db"),
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	const name = "palpanel manual.zip"
	content := []byte("streamed-backup-content")
	if err := os.WriteFile(filepath.Join(cfg.BackupsDir, name), content, 0o600); err != nil {
		t.Fatal(err)
	}

	handler := Server{server: server.NewManager(cfg, store, docker.NewRunner(cfg))}
	router := gin.New()
	router.GET("/backups/:name/download", handler.downloadBackup)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/backups/palpanel%20manual.zip/download", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Length"); got != "23" {
		t.Fatalf("Content-Length = %q", got)
	}
	if disposition := recorder.Header().Get("Content-Disposition"); !strings.Contains(disposition, "attachment") || !strings.Contains(disposition, "palpanel manual.zip") {
		t.Fatalf("Content-Disposition = %q", disposition)
	}
	if got := recorder.Body.Bytes(); string(got) != string(content) {
		t.Fatalf("body = %q", got)
	}
}
