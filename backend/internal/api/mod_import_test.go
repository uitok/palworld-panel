package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
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
)

func TestModImportHTTPInspectAndStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	archive := apiModArchive(t)
	cfg := appconfig.Config{
		DataDir: root, ServerDir: filepath.Join(root, "server"), UploadsDir: filepath.Join(root, "uploads"),
		BackupsDir: filepath.Join(root, "backups"), LogsDir: filepath.Join(root, "logs"), DBPath: filepath.Join(root, "panel.db"),
		MaxUploadBytes: int64(len(archive)), WorkshopAppID: "1623730", DockerBinary: "false", DockerImage: "test",
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := Server{cfg: cfg, store: store, mods: mods.NewManager(cfg, store, docker.NewRunner(cfg))}
	router := gin.New()
	router.POST("/api/mods/import/inspect", server.inspectModImport)
	router.POST("/api/mods/import/inspect/:id/select", server.selectModImportCandidate)
	router.POST("/api/mods/import", server.startModImport)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "example.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(archive); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/mods/import/inspect", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("inspect response = %d: %s", recorder.Code, recorder.Body.String())
	}
	var inspectEnvelope struct {
		Data mods.ImportInspection `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &inspectEnvelope); err != nil {
		t.Fatal(err)
	}
	if len(inspectEnvelope.Data.Candidates) != 1 || inspectEnvelope.Data.Candidates[0].Action != "new" {
		t.Fatalf("inspection = %#v", inspectEnvelope.Data)
	}

	selectionBody, _ := json.Marshal(map[string]string{"candidate_id": inspectEnvelope.Data.SelectedCandidateID})
	request = httptest.NewRequest(http.MethodPost, "/api/mods/import/inspect/"+inspectEnvelope.Data.ID+"/select", bytes.NewReader(selectionBody))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("selection response = %d: %s", recorder.Code, recorder.Body.String())
	}

	requestBody, _ := json.Marshal(map[string]string{
		"inspection_id": inspectEnvelope.Data.ID,
		"candidate_id":  inspectEnvelope.Data.SelectedCandidateID,
	})
	request = httptest.NewRequest(http.MethodPost, "/api/mods/import", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("import response = %d: %s", recorder.Code, recorder.Body.String())
	}
	var jobEnvelope struct {
		Data db.Job `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &jobEnvelope); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		job, err := store.GetJob(context.Background(), jobEnvelope.Data.ID)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status == "completed" {
			break
		}
		if job.Status == "failed" || time.Now().After(deadline) {
			t.Fatalf("import job = %#v", job)
		}
		time.Sleep(10 * time.Millisecond)
	}
	items, err := store.ListMods(context.Background())
	if err != nil || len(items) != 1 || items[0].Enabled {
		t.Fatalf("imported mods = %#v, %v", items, err)
	}
}

func TestFailModImportMapsExpiredInspectionToGone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	failModImport(context, mods.ImportFailure{Code: "inspection_expired", Err: contextError("expired")})
	if recorder.Code != http.StatusGone || !bytes.Contains(recorder.Body.Bytes(), []byte(`"code":"inspection_expired"`)) {
		t.Fatalf("expired inspection response = %d: %s", recorder.Code, recorder.Body.String())
	}
}

type contextError string

func (e contextError) Error() string { return string(e) }

func TestModImportHTTPWorkshopInspection(t *testing.T) {
	root := t.TempDir()
	cfg := appconfig.Config{DataDir: root, ServerDir: filepath.Join(root, "server"), DBPath: filepath.Join(root, "panel.db"), MaxUploadBytes: 1 << 20}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := Server{cfg: cfg, mods: mods.NewManager(cfg, store, docker.NewRunner(cfg))}
	router := gin.New()
	router.POST("/api/mods/import/inspect", server.inspectModImport)
	request := httptest.NewRequest(http.MethodPost, "/api/mods/import/inspect", bytes.NewBufferString(`{"source":"https://steamcommunity.com/sharedfiles/filedetails/?id=123456789"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !bytes.Contains(recorder.Body.Bytes(), []byte(`"source_type":"workshop"`)) {
		t.Fatalf("Workshop inspection = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func apiModArchive(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	info, err := writer.Create("Example/Info.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := info.Write([]byte(`{"Name":"Example","PackageName":"ExamplePackage","Version":"1"}`)); err != nil {
		t.Fatal(err)
	}
	payload, err := writer.Create("Example/payload.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := payload.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}
