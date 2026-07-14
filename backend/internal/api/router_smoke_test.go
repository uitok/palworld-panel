package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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

func TestAuthenticatedReadRoutesReturnStructuredResponses(t *testing.T) {
	router := newSmokeRouter(t)
	paths := []string{
		"/api/auth/me",
		"/api/jobs",
		"/api/jobs/missing",
		"/api/audit-logs",
		"/api/alerts",
		"/api/schedules",
		"/api/server/status",
		"/api/server/prerequisites",
		"/api/server/host",
		"/api/server/runtime",
		"/api/server/logs?tail=5",
		"/api/server/world",
		"/api/server/version",
		"/api/server/startup",
		"/api/monitor/snapshot",
		"/api/monitor/history?limit=5",
		"/api/backups",
		"/api/config/palworld",
		"/api/config/palworld/schema",
		"/api/mods",
		"/api/mods/workshop/status",
		"/api/ai/translation/config",
		"/api/security/paldefender/status",
		"/api/security/paldefender/config",
		"/api/server/info",
		"/api/server/players",
		"/api/server/settings",
		"/api/server/metrics",
		"/api/server/game-data",
		"/api/save/index/status",
		"/api/players",
		"/api/players/missing",
		"/api/players/missing/inventory",
		"/api/guilds",
		"/api/guilds/missing",
		"/api/bases",
		"/api/bases/missing",
		"/api/bases/missing/storage",
		"/api/pals",
		"/api/pals/missing",
		"/api/map/entities",
		"/api/players/bans",
		"/api/players/whitelist",
	}
	for _, path := range paths {
		t.Run(strings.TrimPrefix(path, "/api/"), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			authorizeTestRequest(request)
			router.ServeHTTP(recorder, request)
			if recorder.Code == http.StatusNotFound && strings.Contains(recorder.Body.String(), "api route not found") {
				t.Fatalf("route was not registered: %s", recorder.Body.String())
			}
			if recorder.Code == http.StatusMethodNotAllowed || recorder.Code >= 600 {
				t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
			}
			contentType := recorder.Header().Get("Content-Type")
			if !strings.HasPrefix(contentType, "application/json") {
				t.Fatalf("Content-Type = %q, body=%s", contentType, recorder.Body.String())
			}
		})
	}
}

func TestWriteRoutesValidateBadInput(t *testing.T) {
	router := newSmokeRouter(t)
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/schedules", "{"},
		{http.MethodPut, "/api/schedules/missing", "{"},
		{http.MethodDelete, "/api/schedules/missing", ""},
		{http.MethodPost, "/api/schedules/missing/run", ""},
		{http.MethodPut, "/api/server/runtime", `{"mode":"invalid"}`},
		{http.MethodPost, "/api/server/world/reset", "{}"},
		{http.MethodPost, "/api/server/safe-restart", `{"waittime":1}`},
		{http.MethodPut, "/api/server/startup", "{"},
		{http.MethodPost, "/api/backups/missing.zip/restore", ""},
		{http.MethodPost, "/api/backups/missing.zip/verify", ""},
		{http.MethodDelete, "/api/backups/missing.zip", ""},
		{http.MethodPut, "/api/config/palworld", "{"},
		{http.MethodPost, "/api/config/palworld/validate", "{"},
		{http.MethodPost, "/api/mods/workshop", `{"item_id":"bad"}`},
		{http.MethodPost, "/api/mods/missing/enable", ""},
		{http.MethodPost, "/api/mods/missing/disable", ""},
		{http.MethodDelete, "/api/mods/missing", ""},
		{http.MethodPut, "/api/ai/translation/config", "{"},
		{http.MethodPost, "/api/ai/translation/test", "{"},
		{http.MethodPut, "/api/security/paldefender/config", "{"},
		{http.MethodPost, "/api/security/paldefender/apply-preset", "{}"},
		{http.MethodPost, "/api/security/paldefender/rest-token", "{}"},
		{http.MethodPost, "/api/players/bans", "{}"},
		{http.MethodDelete, "/api/players/bans/missing", ""},
		{http.MethodPut, "/api/players/whitelist", "{"},
		{http.MethodPost, "/api/players/missing/kick", "{}"},
		{http.MethodPost, "/api/players/missing/ban", "{}"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			authorizeTestRequest(request)
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			allowsDefaults := test.path == "/api/security/paldefender/apply-preset" || test.path == "/api/security/paldefender/rest-token" ||
				test.path == "/api/players/missing/kick" || test.path == "/api/players/missing/ban"
			if (!allowsDefaults && recorder.Code < 400) || recorder.Code >= 600 {
				t.Fatalf("expected validation failure, got %d: %s", recorder.Code, recorder.Body.String())
			}
			if allowsDefaults && recorder.Code == http.StatusOK {
				if !strings.Contains(recorder.Body.String(), `"ok":true`) {
					t.Fatalf("unexpected success envelope: %s", recorder.Body.String())
				}
				return
			}
			if !strings.Contains(recorder.Body.String(), `"ok":false`) {
				t.Fatalf("unexpected error envelope: %s", recorder.Body.String())
			}
		})
	}
}

func TestManagementWriteWorkflows(t *testing.T) {
	router := newSmokeRouter(t)

	recorder := performJSONRequest(t, router, http.MethodPost, "/api/schedules", `{"type":"backup","interval_minutes":30,"timezone":"Asia/Shanghai"}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create schedule = %d: %s", recorder.Code, recorder.Body.String())
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil || created.Data.ID == "" {
		t.Fatalf("decode schedule: %#v, %v", created, err)
	}
	recorder = performJSONRequest(t, router, http.MethodPut, "/api/schedules/"+created.Data.ID, `{"type":"backup","enabled":false,"time_of_day":"04:00"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("update schedule = %d: %s", recorder.Code, recorder.Body.String())
	}
	recorder = performJSONRequest(t, router, http.MethodPost, "/api/schedules/"+created.Data.ID+"/run", "")
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("run schedule = %d: %s", recorder.Code, recorder.Body.String())
	}
	var accepted struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &accepted); err != nil || accepted.Data.ID == "" {
		t.Fatalf("decode accepted job: %#v, %v", accepted, err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		jobRecorder := performJSONRequest(t, router, http.MethodGet, "/api/jobs/"+accepted.Data.ID, "")
		if strings.Contains(jobRecorder.Body.String(), `"status":"completed"`) || strings.Contains(jobRecorder.Body.String(), `"status":"failed"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("scheduled job did not finish: %s", jobRecorder.Body.String())
		}
		time.Sleep(time.Millisecond)
	}
	recorder = performJSONRequest(t, router, http.MethodDelete, "/api/schedules/"+created.Data.ID, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete schedule = %d: %s", recorder.Code, recorder.Body.String())
	}

	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPut, "/api/server/runtime", `{"mode":"wine_docker"}`},
		{http.MethodPut, "/api/server/startup", `{"port":8211,"players":32,"log_format":"text"}`},
		{http.MethodPost, "/api/server/initialize-config", ""},
		{http.MethodPut, "/api/config/palworld", `{"ServerName":"Contract Test"}`},
		{http.MethodPost, "/api/config/palworld/validate", `{"ServerName":"Contract Test"}`},
		{http.MethodPost, "/api/players/bans", `{"steam_id":"76561198000000000","nickname":"Player"}`},
		{http.MethodPut, "/api/players/whitelist", `{"players":[{"steam_id":"76561198000000001"}]}`},
		{http.MethodPost, "/api/server/announce", `{"message":"test"}`},
		{http.MethodPost, "/api/server/save", ""},
		{http.MethodPost, "/api/server/shutdown", `{"waittime":30,"message":"test"}`},
		{http.MethodPost, "/api/players/player/kick", `{"nickname":"Player"}`},
		{http.MethodPost, "/api/players/player/ban", `{"reason":"test"}`},
		{http.MethodPost, "/api/players/player/unban", ""},
	} {
		recorder = performJSONRequest(t, router, request.method, request.path, request.body)
		if recorder.Code < 200 || recorder.Code >= 300 {
			t.Fatalf("%s %s = %d: %s", request.method, request.path, recorder.Code, recorder.Body.String())
		}
	}
	recorder = performJSONRequest(t, router, http.MethodDelete, "/api/players/bans/76561198000000000", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete ban = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func performJSONRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	authorizeTestRequest(request)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func newSmokeRouter(t *testing.T) *gin.Engine {
	t.Helper()
	root := t.TempDir()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/metrics":
			_, _ = w.Write([]byte(`{"currentplayernum":2,"maxplayernum":32,"serverfps":60}`))
		case "/game-data":
			_, _ = w.Write([]byte(`{"players":[]}`))
		default:
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}
	}))
	t.Cleanup(upstream.Close)
	parsedUpstream, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream URL: %v", err)
	}
	upstreamPort, err := strconv.Atoi(parsedUpstream.Port())
	if err != nil {
		t.Fatalf("parse upstream port: %v", err)
	}
	cfg := appconfig.Config{
		DataDir: root, ServerDir: filepath.Join(root, "server"), WinePrefixDir: filepath.Join(root, "wine"),
		ToolsDir: filepath.Join(root, "tools"), SteamCMDDir: filepath.Join(root, "tools", "steamcmd"),
		UploadsDir: filepath.Join(root, "uploads"), BackupsDir: filepath.Join(root, "backups"), LogsDir: filepath.Join(root, "logs"),
		SaveIndexCacheDir: filepath.Join(root, "save-index"), DBPath: filepath.Join(root, "test.db"),
		RequireAuth: true, DockerBinary: "/bin/false", DockerImage: "test", DockerContainer: "test",
		GamePort: 8211, QueryPort: 27015, RESTPort: upstreamPort, PalworldRESTBaseURL: upstream.URL,
		PalworldRESTReadTimeoutMS: 100, PalworldGameDataTimeoutMS: 100, PalworldGameDataMaxBytes: 1024 * 1024,
		MonitorRetentionDays: 7,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.PalWorldSettingsPath()), 0o755); err != nil {
		t.Fatalf("create settings directory: %v", err)
	}
	settings := palconfig.Defaults()
	settings["RESTAPIPort"] = strconv.Itoa(upstreamPort)
	if err := palconfig.Write(cfg.PalWorldSettingsPath(), settings); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	provisionTestPrincipal(t, store, RoleAdmin)
	runner := docker.NewRunner(cfg)
	serverManager := server.NewManager(cfg, store, runner)
	restClient := palrest.New(upstream.URL, "admin", "secret")
	return NewRouter(
		cfg, store, serverManager, mods.NewManager(cfg, store, runner), paldefender.NewManager(cfg, store), restClient,
		monitor.New(cfg, store, serverManager, restClient), scheduler.New(store, serverManager, restClient),
	)
}
