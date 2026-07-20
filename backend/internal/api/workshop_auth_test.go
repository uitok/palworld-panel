package api

import (
	"errors"
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
	"palpanel/internal/steamcmd"
)

func TestWorkshopAuthRejectsUnknownOrTrailingFieldsWithoutEchoingValues(t *testing.T) {
	server, cleanup := newWorkshopAuthTestServer(t)
	defer cleanup()
	for _, body := range []string{
		`{"account_name":"fixture_user","guard_code":"123456"}`,
		`{"account_name":"fixture_user"}{"account_name":"second_user"}`,
	} {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/api/mods/workshop/auth/start", server.startWorkshopAuth)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/mods/workshop/auth/start", strings.NewReader(body))
		request.RemoteAddr = "127.0.0.1:41234"
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"invalid_steam_auth_request"`) {
			t.Fatalf("response = %d: %s", recorder.Code, recorder.Body.String())
		}
		if strings.Contains(recorder.Body.String(), "never-store-this") || strings.Contains(recorder.Body.String(), "123456") {
			t.Fatalf("response echoed credential value: %s", recorder.Body.String())
		}
	}
}

func TestWorkshopAuthStatusWithoutAccountDoesNotProbeOrInstall(t *testing.T) {
	server, cleanup := newWorkshopAuthTestServer(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/mods/workshop/auth/status", server.workshopAuthStatus)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/mods/workshop/auth/status", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"verification_required":true`) {
		t.Fatalf("response = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestWorkshopSearchDoesNotRequireSteamCredentials(t *testing.T) {
	server, cleanup := newWorkshopAuthTestServer(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/mods/workshop/search", server.searchWorkshopMods)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/mods/workshop/search", nil))
	if recorder.Code == http.StatusUnauthorized || strings.Contains(recorder.Body.String(), `"code":"steam_login_required"`) {
		t.Fatalf("response = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestWorkshopAuthInvalidAccountHasStableErrorCode(t *testing.T) {
	server, cleanup := newWorkshopAuthTestServer(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/mods/workshop/auth/start", server.startWorkshopAuth)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/mods/workshop/auth/start", strings.NewReader(`{"account_name":"+quit"}`))
	request.RemoteAddr = "[::1]:41234"
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"invalid_steam_account"`) {
		t.Fatalf("response = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestSteamAuthPostsRejectRemoteClientsEvenWithSpoofedForwardedIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, handler := range []gin.HandlerFunc{Server{}.startWorkshopAuth, Server{}.verifyWorkshopAuth} {
		router := gin.New()
		router.POST("/auth", handler)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader(`{"account_name":"fixture_user"}`))
		request.RemoteAddr = "203.0.113.42:51234"
		request.Header.Set("X-Forwarded-For", "127.0.0.1")
		request.Header.Set("X-Real-IP", "::1")
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), `"code":"steam_login_local_only"`) {
			t.Fatalf("response = %d: %s", recorder.Code, recorder.Body.String())
		}
	}
}

func TestSteamAuthPostsRequireAdminOnlyPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []Role{RoleViewer, RoleOperator} {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(principalKey, Principal{Name: string(role), Role: role})
		})
		router.POST("/auth", Require(PermSecurityWrite), Server{}.startWorkshopAuth)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader(`{"account_name":"fixture_user"}`))
		request.RemoteAddr = "127.0.0.1:41234"
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), `"code":"permission_denied"`) {
			t.Fatalf("role %s response = %d: %s", role, recorder.Code, recorder.Body.String())
		}
	}
}

func TestSteamAuthOperationalErrorIsStableAndRedacted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	failSteamAuth(context, "verify", errors.New("private SteamCMD output and local path"))
	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), `"code":"steam_login_verify_failed"`) {
		t.Fatalf("response = %d: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "private") || strings.Contains(recorder.Body.String(), "local path") {
		t.Fatalf("response exposed operational detail: %s", recorder.Body.String())
	}
}

func TestSteamAuthCredentialErrorsHaveStableCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, fixture := range []struct {
		err    error
		status int
		code   string
	}{
		{err: steamcmd.ErrSteamGuardRequired, status: http.StatusConflict, code: "steam_guard_required"},
		{err: steamcmd.ErrInvalidCredentials, status: http.StatusUnauthorized, code: "invalid_steam_credentials"},
	} {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		failSteamAuth(context, "verify", fixture.err)
		if recorder.Code != fixture.status || !strings.Contains(recorder.Body.String(), `"code":"`+fixture.code+`"`) {
			t.Fatalf("response = %d: %s", recorder.Code, recorder.Body.String())
		}
	}
}

func newWorkshopAuthTestServer(t *testing.T) (Server, func()) {
	t.Helper()
	root := t.TempDir()
	cfg := appconfig.Config{
		RuntimeRoot: root,
		DataDir:     root, ServerDir: filepath.Join(root, "server"), WinePrefixDir: filepath.Join(root, "wine"),
		ToolsDir: filepath.Join(root, "tools"), SteamCMDDir: filepath.Join(root, "tools", "steamcmd"),
		UploadsDir: filepath.Join(root, "uploads"), BackupsDir: filepath.Join(root, "backups"), LogsDir: filepath.Join(root, "logs"),
		DBPath: filepath.Join(root, "test.db"), WorkshopAppID: "1623730", DockerBinary: "false", DockerImage: "test",
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	server := Server{cfg: cfg, store: store, mods: mods.NewManager(cfg, store, docker.NewRunner(cfg))}
	if err := store.SetKV(t.Context(), "runtime_mode", "windows_steamcmd"); err != nil {
		t.Fatal(err)
	}
	return server, func() { _ = store.Close() }
}
