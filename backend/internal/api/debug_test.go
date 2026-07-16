package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/debuglog"
)

func TestDebugLoggingStatusAndRuntimeToggle(t *testing.T) {
	root := t.TempDir()
	store, err := db.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	logger, err := debuglog.New(filepath.Join(root, "logs", "palpanel-debug.log"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()
	server := Server{cfg: appconfig.Config{LogsDir: filepath.Join(root, "logs"), DebugLogger: logger}, store: store}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/system/debug", server.debugLoggingStatus)
	router.PUT("/api/system/debug", server.putDebugLogging)

	update := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/system/debug", bytes.NewBufferString(`{"enabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(update, request)
	if update.Code != http.StatusOK {
		t.Fatalf("PUT status = %d: %s", update.Code, update.Body.String())
	}
	var payload struct {
		Data debuglog.Status `json:"data"`
	}
	if err := json.Unmarshal(update.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Data.Enabled || payload.Data.Path != logger.Path() {
		t.Fatalf("unexpected debug status: %#v", payload.Data)
	}
	if value, found, err := store.GetKV(t.Context(), debugLoggingKV); err != nil || !found || value != "true" {
		t.Fatalf("persisted state = %q, %v, %v", value, found, err)
	}

	status := httptest.NewRecorder()
	router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/system/debug", nil))
	if status.Code != http.StatusOK || !bytes.Contains(status.Body.Bytes(), []byte(`"enabled":true`)) {
		t.Fatalf("GET status = %d: %s", status.Code, status.Body.String())
	}
}
