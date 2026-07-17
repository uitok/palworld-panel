package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
		case "/v1/pdapi/progression/steam_1":
			if r.Method == http.MethodPost {
				_ = json.NewEncoder(w).Encode(map[string]any{"Granted": map[string]any{"TechnologyPoints": 10}, "Totals": map[string]any{"TechnologyPoints": 15}})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"Meta": map[string]any{"Player": "steam_1", "PlayerUID": "uid_1"}, "Progression": map[string]any{"Player": map[string]any{"level": 20, "exp": 1000, "unusedStatusPoints": 2}, "Currencies": map[string]any{"technologyPoints": 5, "ancientTechnologyPoints": 1, "relics": map[string]any{}}, "Bosses": map[string]any{}, "Captures": map[string]any{}, "Activities": map[string]any{}}})
			}
		case "/v1/pdapi/give/progression/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Granted": map[string]any{"TechnologyPoints": 10}, "Totals": map[string]any{"TechnologyPoints": 15}})
		case "/v1/pdapi/techs/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Meta": map[string]any{"Player": "steam_1", "PlayerUID": "uid_1", "UnlockedCount": 1, "LockedCount": 1, "TotalCount": 2}, "Techs": map[string]any{"Unlocked": []string{"Technology_ElecBaton"}}})
		case "/v1/pdapi/learntech/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"UnlockedCount": 1, "Unlocked": []string{"Technology_GrapplingGun"}, "Skipped": []string{}})
		case "/v1/pdapi/forgettech/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"ForgottenCount": 1, "Forgotten": []string{"Technology_ElecBaton"}, "Skipped": []string{}})
		case "/v1/pdapi/pals/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Meta": map[string]any{"Player": "steam_1", "PlayerUID": "uid_1", "TeamCount": 1, "PalboxCount": 0, "BaseCampCount": 0}, "Pals": map[string]any{"Team": map[string]any{"pal-1": map[string]any{"PalID": "Anubis"}}, "Palbox": map[string]any{}, "BaseCamps": []any{}}})
		case "/v1/pdapi/give/pals/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Granted": map[string]any{"Pals": 1}})
		case "/v1/pdapi/give/paltemplate/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Granted": map[string]any{"PalTemplates": 1}})
		case "/v1/pdapi/SendPlayerMessage", "/v1/pdapi/Broadcast", "/v1/pdapi/Alert", "/v1/pdapi/kick/steam_1", "/v1/pdapi/ban/steam_1", "/v1/pdapi/unban/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Success": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	router, manager, _, store, cleanup := newPalDefenderGMTestRouter(t, RoleOperator, upstream.URL)
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
		{http.MethodGet, "/api/security/paldefender/gm/players/steam_1", "", `"UserId":"steam_1"`},
		{http.MethodGet, "/api/security/paldefender/gm/items?q=Money", "", `"id":"Money"`},
		{http.MethodGet, "/api/security/paldefender/gm/players/steam_1/inventory", "", `"ItemID":"Money"`},
		{http.MethodPost, "/api/security/paldefender/gm/players/steam_1/items", `{"Items":[{"ItemID":"Money","Count":10}]}`, `"Items":10`},
		{http.MethodGet, "/api/security/paldefender/gm/players/steam_1/progression", "", `"technologyPoints":5`},
		{http.MethodPost, "/api/security/paldefender/gm/players/steam_1/progression", `{"TechnologyPoints":10}`, `"TechnologyPoints":15`},
		{http.MethodGet, "/api/security/paldefender/gm/players/steam_1/techs", "", `"Technology_ElecBaton"`},
		{http.MethodPost, "/api/security/paldefender/gm/players/steam_1/techs/learn", `{"Technology":"Technology_GrapplingGun"}`, `"UnlockedCount":1`},
		{http.MethodPost, "/api/security/paldefender/gm/players/steam_1/techs/forget", `{"Technology":"Technology_ElecBaton"}`, `"ForgottenCount":1`},
		{http.MethodGet, "/api/security/paldefender/gm/players/steam_1/pals", "", `"PalID":"Anubis"`},
		{http.MethodPost, "/api/security/paldefender/gm/players/steam_1/pals", `{"Pals":[{"PalID":"Anubis","Level":50}]}`, `"Pals":1`},
		{http.MethodPost, "/api/security/paldefender/gm/players/steam_1/pal-templates", `{"PalTemplates":["reward_anubis"]}`, `"PalTemplates":1`},
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
			request.Header.Set("Idempotency-Key", "gm-test-"+strconv.Itoa(len(test.path))+strings.ReplaceAll(test.path, "/", "-"))
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
	audits, err := store.ListAuditLogs(t.Context(), 20)
	if err != nil {
		t.Fatal(err)
	}
	var gmWrites int
	for _, audit := range audits {
		if strings.HasPrefix(audit.Action, "POST /api/security/paldefender/gm/") && audit.Status == "success" {
			gmWrites++
		}
	}
	if gmWrites != 11 {
		t.Fatalf("successful GM write audits = %d: %#v", gmWrites, audits)
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

	router, manager, _, _, cleanup := newPalDefenderGMTestRouter(t, RoleViewer, upstream.URL)
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
	router, _, _, _, cleanup := newPalDefenderGMTestRouter(t, RoleOperator, "http://127.0.0.1:1")
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
	request.Header.Set("Idempotency-Key", "gm-invalid-player-001")
	authorizeTestRequest(request)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid request = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestPalDefenderGMAllWritesRequireBackendPermissionAndAuthentication(t *testing.T) {
	router, manager, _, _, cleanup := newPalDefenderGMTestRouter(t, RoleViewer, "http://127.0.0.1:1")
	defer cleanup()
	if _, err := manager.CreateRESTToken(t.Context(), "GMTest", []string{"REST.*"}); err != nil {
		t.Fatal(err)
	}

	writes := []struct {
		path string
		body string
	}{
		{"/api/security/paldefender/gm/players/steam_1/items", `{"Items":[{"ItemID":"Money","Count":1}]}`},
		{"/api/security/paldefender/gm/players/steam_1/message", `{"SendType":"PlayerChat","Message":"hello"}`},
		{"/api/security/paldefender/gm/broadcast", `{"message":"hello","alert":false}`},
		{"/api/security/paldefender/gm/players/steam_1/kick", `{"Reason":"AFK"}`},
		{"/api/security/paldefender/gm/players/steam_1/ban", `{"Reason":"abuse"}`},
		{"/api/security/paldefender/gm/players/steam_1/unban", `{"Reason":"appeal"}`},
	}
	for index, write := range writes {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, write.path, strings.NewReader(write.body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "gm-viewer-denied-"+strconv.Itoa(index))
		authorizeTestRequest(request)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), `"code":"permission_denied"`) {
			t.Errorf("viewer POST %s = %d %s", write.path, recorder.Code, recorder.Body.String())
		}
	}

	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/security/paldefender/gm/players", nil),
		httptest.NewRequest(http.MethodPost, writes[0].path, strings.NewReader(writes[0].body)),
	} {
		recorder := httptest.NewRecorder()
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "gm-unauthenticated-001")
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated %s %s = %d %s", request.Method, request.URL.Path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestPalDefenderGMIdempotencyAndWriteAudit(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/pdapi/give/items/steam_1" {
			http.NotFound(w, r)
			return
		}
		upstreamCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"Granted": map[string]any{"Items": 1}})
	}))
	defer upstream.Close()

	router, manager, _, store, cleanup := newPalDefenderGMTestRouter(t, RoleOperator, upstream.URL)
	defer cleanup()
	if _, err := manager.CreateRESTToken(t.Context(), "GMTest", []string{"REST.*"}); err != nil {
		t.Fatal(err)
	}

	requestWrite := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/security/paldefender/gm/players/steam_1/items", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "gm-retry-same-intent-001")
		authorizeTestRequest(request)
		router.ServeHTTP(recorder, request)
		return recorder
	}
	first := requestWrite(`{"Items":[{"ItemID":"Money","Count":1}]}`)
	second := requestWrite(`{"Items":[{"ItemID":"Money","Count":1}]}`)
	conflict := requestWrite(`{"Items":[{"ItemID":"Money","Count":2}]}`)
	if first.Code != http.StatusOK || second.Code != http.StatusOK || second.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("idempotent responses = first %d %s; second %d %s headers=%v", first.Code, first.Body.String(), second.Code, second.Body.String(), second.Header())
	}
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), `"code":"idempotency_key_reused"`) {
		t.Fatalf("idempotency conflict = %d %s", conflict.Code, conflict.Body.String())
	}
	if upstreamCalls.Load() != 1 {
		t.Fatalf("duplicate requests reached upstream %d times", upstreamCalls.Load())
	}

	audits, err := store.ListAuditLogs(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	var successes, failures int
	for _, audit := range audits {
		if audit.Action != "POST /api/security/paldefender/gm/players/:id/items" || audit.Target != "steam_1" || audit.Actor != "test-admin" {
			continue
		}
		if audit.Status == "success" {
			successes++
		} else if audit.Status == "failed" {
			failures++
		}
	}
	if successes != 2 || failures != 1 {
		t.Fatalf("GM write audits = %#v", audits)
	}
	for name, key := range map[string]string{"missing": "", "too_short": "short"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/security/paldefender/gm/players/steam_1/items", strings.NewReader(`{"Items":[{"ItemID":"Money","Count":1}]}`))
		request.Header.Set("Content-Type", "application/json")
		if key != "" {
			request.Header.Set("Idempotency-Key", key)
		}
		authorizeTestRequest(request)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "idempotency_key") {
			t.Errorf("%s idempotency key = %d %s", name, recorder.Code, recorder.Body.String())
		}
	}
}

func TestPalDefenderGMProtocolFailureMapping(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantStatus int
		wantCode   string
	}{
		{"player_offline_during_write", http.StatusConflict, `{"Error":{"Code":"PLAYER_OFFLINE","Message":"player disconnected"}}`, http.StatusConflict, "paldefender_player_offline"},
		{"player_disappeared", http.StatusNotFound, `{"Error":{"Code":"PLAYER_NOT_FOUND","Message":"player was not found"}}`, http.StatusNotFound, "paldefender_player_not_found"},
		{"upstream_failure", http.StatusServiceUnavailable, `{"Error":{"Code":"SERVER_STOPPED","Message":"server stopped"}}`, http.StatusBadGateway, "paldefender_server_stopped"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer upstream.Close()
			router, manager, _, _, cleanup := newPalDefenderGMTestRouter(t, RoleOperator, upstream.URL)
			defer cleanup()
			if _, err := manager.CreateRESTToken(t.Context(), "GMTest", []string{"REST.*"}); err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/security/paldefender/gm/players/steam_1/items", strings.NewReader(`{"Items":[{"ItemID":"Money","Count":1}]}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "gm-protocol-failure-001")
			authorizeTestRequest(request)
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}

	t.Run("invalid_json_response", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{not-json}`))
		}))
		defer upstream.Close()
		router, manager, _, _, cleanup := newPalDefenderGMTestRouter(t, RoleViewer, upstream.URL)
		defer cleanup()
		if _, err := manager.CreateRESTToken(t.Context(), "GMTest", []string{"REST.*"}); err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/security/paldefender/gm/players", nil)
		authorizeTestRequest(request)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), `"code":"paldefender_invalid_response"`) {
			t.Fatalf("invalid response = %d %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("timeout", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer upstream.Close()
		router, manager, _, _, cleanup := newPalDefenderGMTestRouter(t, RoleViewer, upstream.URL)
		defer cleanup()
		if _, err := manager.CreateRESTToken(t.Context(), "GMTest", []string{"REST.*"}); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		defer cancel()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/security/paldefender/gm/players", nil).WithContext(ctx)
		authorizeTestRequest(request)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusGatewayTimeout || !strings.Contains(recorder.Body.String(), `"code":"paldefender_rest_timeout"`) {
			t.Fatalf("timeout response = %d %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("server_not_running", func(t *testing.T) {
		router, manager, _, _, cleanup := newPalDefenderGMTestRouter(t, RoleViewer, "http://127.0.0.1:1")
		defer cleanup()
		if _, err := manager.CreateRESTToken(t.Context(), "GMTest", []string{"REST.*"}); err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/security/paldefender/gm/players", nil)
		authorizeTestRequest(request)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"code":"paldefender_rest_unavailable"`) {
			t.Fatalf("stopped response = %d %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestPalDefenderGMPrerequisitesAndStrictInput(t *testing.T) {
	var upstreamCalls atomic.Int32
	var messageUserID string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		if r.URL.Path == "/v1/pdapi/SendPlayerMessage" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			messageUserID, _ = body["UserID"].(string)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Success": true})
	}))
	defer upstream.Close()
	router, manager, cfg, _, cleanup := newPalDefenderGMTestRouter(t, RoleOperator, upstream.URL)
	defer cleanup()
	if _, err := manager.CreateRESTToken(t.Context(), "GMTest", []string{"REST.*"}); err != nil {
		t.Fatal(err)
	}

	request := func(path, body, key string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		authorizeTestRequest(req)
		router.ServeHTTP(recorder, req)
		return recorder
	}
	invalid := []struct {
		path string
		body string
	}{
		{"/api/security/paldefender/gm/players/steam_1/items", `{"Items":[{"ItemID":"Money","Count":1}],"Command":"quit"}`},
		{"/api/security/paldefender/gm/players/steam_1/items", `{"Items":[{"ItemID":"Money;quit","Count":1}]}`},
		{"/api/security/paldefender/gm/players/steam_1/items", `{"Items":[{"ItemID":"Money","Count":1}]} {}`},
		{"/api/security/paldefender/gm/players/bad.id/items", `{"Items":[{"ItemID":"Money","Count":1}]}`},
		{"/api/security/paldefender/gm/players/steam_1/message", `{"SendType":"PlayerChat","Message":"hello","UserID":"attacker"}`},
	}
	for index, test := range invalid {
		recorder := request(test.path, test.body, "gm-invalid-input-"+strconv.Itoa(index))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("invalid input %d = %d %s", index, recorder.Code, recorder.Body.String())
		}
	}
	if upstreamCalls.Load() != 0 {
		t.Fatalf("invalid requests reached upstream %d times", upstreamCalls.Load())
	}

	valid := request(
		"/api/security/paldefender/gm/players/steam_1/message",
		`{"SendType":"PlayerChat","Message":"; shutdown"}`,
		"gm-fixed-target-message-001",
	)
	if valid.Code != http.StatusOK || messageUserID != "steam_1" {
		t.Fatalf("fixed-target message = %d %s UserID=%q", valid.Code, valid.Body.String(), messageUserID)
	}

	if err := os.Remove(filepath.Join(cfg.Win64Dir(), "PalDefender.dll")); err != nil {
		t.Fatal(err)
	}
	missing := request("/api/security/paldefender/gm/players/steam_1/message", `{"Message":"hello"}`, "gm-not-installed-001")
	if missing.Code != http.StatusConflict || !strings.Contains(missing.Body.String(), `"code":"paldefender_not_installed"`) {
		t.Fatalf("not installed = %d %s", missing.Code, missing.Body.String())
	}
	if err := os.WriteFile(filepath.Join(cfg.Win64Dir(), "PalDefender.dll"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cfg.ServerLogPath()); err != nil {
		t.Fatal(err)
	}
	notLoaded := request("/api/security/paldefender/gm/players/steam_1/message", `{"Message":"hello"}`, "gm-not-loaded-001")
	if notLoaded.Code != http.StatusConflict || !strings.Contains(notLoaded.Body.String(), `"code":"paldefender_not_loaded"`) {
		t.Fatalf("not loaded = %d %s", notLoaded.Code, notLoaded.Body.String())
	}
	if err := os.WriteFile(cfg.ServerLogPath(), []byte("PalDefender load complete\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.PalDefenderDir(), "RESTAPI", "RESTConfig.json"), []byte(`{"Enabled":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	restDisabled := request("/api/security/paldefender/gm/players/steam_1/message", `{"Message":"hello"}`, "gm-rest-disabled-001")
	if restDisabled.Code != http.StatusConflict || !strings.Contains(restDisabled.Body.String(), `"code":"paldefender_rest_disabled"`) {
		t.Fatalf("REST disabled = %d %s", restDisabled.Code, restDisabled.Body.String())
	}
}

func newPalDefenderGMTestRouter(t *testing.T, role Role, baseURL string) (*gin.Engine, paldefender.Manager, appconfig.Config, *db.Store, func()) {
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
	if err := os.MkdirAll(cfg.Win64Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"PalDefender.dll", "d3d9.dll"} {
		if err := os.WriteFile(filepath.Join(cfg.Win64Dir(), name), []byte("test fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(cfg.PalDefenderDir(), "RESTAPI"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.PalDefenderDir(), "RESTAPI", "RESTConfig.json"), []byte(`{"Enabled":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.ServerLogPath(), []byte("PalDefender load complete\n"), 0o600); err != nil {
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
	return router, manager, cfg, store, func() { _ = store.Close() }
}
