package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

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

func TestPalDefenderGMRoutesProxyOfficialContract(t *testing.T) {
	var expectedToken string
	var giveBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+expectedToken || expectedToken == "" {
			t.Fatalf("unexpected upstream Authorization header")
		}
		switch r.URL.Path {
		case "/v1/pdapi/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"Version": map[string]any{"Major": 1, "Minor": 8, "Patch": 1, "Build": 0, "Version": "1.8.1", "VersionLong": "1.8.1", "Beta": false}})
		case "/v1/pdapi/players":
			_ = json.NewEncoder(w).Encode(map[string]any{"Meta": map[string]any{"PlayerCount": 1, "OnlineCount": 1}, "Players": []map[string]any{{"Name": "Builder", "UserId": "steam_1", "PlayerUID": "uid_1", "Status": "Online"}}})
		case "/v1/pdapi/items/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Meta": map[string]any{"PlayerUID": "uid_1", "Player": "steam_1"}, "Inventory": map[string]any{"Items": map[string]any{"Available": true, "Slots": map[string]any{"0": map[string]any{"ItemID": "Money", "Count": 5}}}}})
		case "/v1/pdapi/give/items/steam_1":
			if err := json.NewDecoder(r.Body).Decode(&giveBody); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"Granted": map[string]any{"Items": 10}})
		case "/v1/pdapi/SendPlayerMessage", "/v1/pdapi/Broadcast", "/v1/pdapi/Alert", "/v1/pdapi/kick/steam_1", "/v1/pdapi/ban/steam_1", "/v1/pdapi/unban/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Success": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	router, manager, cleanup := newPalDefenderGMTestRouter(t, RoleOperator, upstream.URL)
	defer cleanup()
	token, err := manager.CreateRESTToken(t.Context(), "GMTest", []string{"REST.*"})
	if err != nil {
		t.Fatal(err)
	}
	expectedToken = token.Token

	tests := []struct {
		method string
		path   string
		body   string
		want   string
	}{
		{http.MethodGet, "/api/security/paldefender/gm/status", "", `"available":true`},
		{http.MethodGet, "/api/security/paldefender/gm/players", "", `"UserId":"steam_1"`},
		{http.MethodGet, "/api/security/paldefender/gm/items?q=Money", "", `"id":"Money"`},
		{http.MethodGet, "/api/security/paldefender/gm/players/steam_1/inventory", "", `"ItemID":"Money"`},
		{http.MethodPost, "/api/security/paldefender/gm/players/steam_1/items", `{"Items":[{"ItemID":"Money","Count":10}]}`, `"Items":10`},
		{http.MethodPost, "/api/security/paldefender/gm/players/steam_1/message", `{"SendType":"PlayerChat","Message":"hello"}`, `"Success":true`},
		{http.MethodPost, "/api/security/paldefender/gm/broadcast", `{"message":"maintenance","alert":false}`, `"Success":true`},
		{http.MethodPost, "/api/security/paldefender/gm/players/steam_1/kick", `{"Reason":"AFK"}`, `"Success":true`},
		{http.MethodPost, "/api/security/paldefender/gm/players/steam_1/ban", `{"Reason":"abuse","IP":true}`, `"Success":true`},
		{http.MethodPost, "/api/security/paldefender/gm/players/steam_1/unban", `{"Reason":"appeal"}`, `"Success":true`},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		if test.body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		authorizeTestRequest(request)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), test.want) {
			t.Errorf("%s %s = %d %s", test.method, test.path, recorder.Code, recorder.Body.String())
		}
	}
	items, ok := giveBody["Items"].([]any)
	if !ok || len(items) != 1 || items[0].(map[string]any)["ItemID"] != "Money" {
		t.Fatalf("upstream give body = %#v", giveBody)
	}
}

func TestPalDefenderGMRoutesPermissionsAndErrors(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/pdapi/players" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"Error":{"Code":"../../MISSING PERMISSION","Message":"permission denied"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Version": map[string]any{"Version": "1.8.1"}})
	}))
	defer upstream.Close()

	router, manager, cleanup := newPalDefenderGMTestRouter(t, RoleViewer, upstream.URL)
	defer cleanup()
	if _, err := manager.CreateRESTToken(t.Context(), "GMTest", []string{"REST.*"}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/security/paldefender/gm/players", nil)
	authorizeTestRequest(request)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), `"code":"paldefender_missing_permission"`) || !strings.Contains(recorder.Body.String(), "permission denied") {
		t.Fatalf("upstream error = %d %s", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/security/paldefender/gm/players/steam_1/items", strings.NewReader(`{"Items":[{"ItemID":"Money","Count":1}]}`))
	request.Header.Set("Content-Type", "application/json")
	authorizeTestRequest(request)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("viewer write = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestPalDefenderGMRoutesRequireConfigurationAndValidateInput(t *testing.T) {
	router, _, cleanup := newPalDefenderGMTestRouter(t, RoleOperator, "http://127.0.0.1:1")
	defer cleanup()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/security/paldefender/gm/players", nil)
	authorizeTestRequest(request)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "paldefender_rest_not_configured") {
		t.Fatalf("unconfigured response = %d %s", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/security/paldefender/gm/players/bad.id/items", strings.NewReader(`{"Items":[]}`))
	request.Header.Set("Content-Type", "application/json")
	authorizeTestRequest(request)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid request = %d %s", recorder.Code, recorder.Body.String())
	}
}

func newPalDefenderGMTestRouter(t *testing.T, role Role, baseURL string) (*gin.Engine, paldefender.Manager, func()) {
	t.Helper()
	root := t.TempDir()
	cfg := appconfig.Config{
		DataDir:                   root,
		ServerDir:                 filepath.Join(root, "server"),
		WinePrefixDir:             filepath.Join(root, "wineprefix"),
		ToolsDir:                  filepath.Join(root, "tools"),
		SteamCMDDir:               filepath.Join(root, "tools", "steamcmd"),
		UploadsDir:                filepath.Join(root, "uploads"),
		BackupsDir:                filepath.Join(root, "backups"),
		LogsDir:                   filepath.Join(root, "logs"),
		DBPath:                    filepath.Join(root, "test.db"),
		RequireAuth:               true,
		DockerBinary:              "docker",
		DockerImage:               "test-image",
		DockerContainer:           "test-container",
		PalDefenderRESTBaseURL:    baseURL,
		PalDefenderRESTPort:       appconfig.DefaultPalDefenderRESTPort,
		PalworldGameDataTimeoutMS: 100,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	provisionTestPrincipal(t, store, role)
	runner := docker.NewRunner(cfg)
	serverManager := server.NewManager(cfg, store, runner)
	restClient := palrest.New("", "", "")
	manager := paldefender.NewManager(cfg, store)
	router := NewRouter(
		cfg,
		store,
		serverManager,
		mods.NewManager(cfg, store, runner),
		manager,
		restClient,
		monitor.New(cfg, store, serverManager, restClient),
		scheduler.New(store, serverManager, restClient),
	)
	return router, manager, func() { _ = store.Close() }
}
