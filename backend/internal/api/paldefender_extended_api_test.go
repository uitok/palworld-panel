package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPalDefenderTemplateAccessAndCatalogRoutes(t *testing.T) {
	router, _, cfg, _, cleanup := newPalDefenderGMTestRouter(t, RoleAdmin, "http://127.0.0.1:17993")
	defer cleanup()
	exportedDir := filepath.Join(cfg.PalDefenderDir(), "Pals", "Exported", "steam_1")
	if err := os.MkdirAll(exportedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(exportedDir, "Existing Anubis.json"), []byte(`{"PalID":"Anubis","Level":50,"IVs":{"Health":100}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	request := func(method, path, body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		authorizeTestRequest(req)
		router.ServeHTTP(recorder, req)
		return recorder
	}

	template := `{"PalID":"Anubis","Nickname":"Arena Anubis","Gender":"None","Level":50,"PartnerSkillLevel":3,"IVs":{"Health":100,"AttackShot":90,"Defense":80},"PalSouls":{"Health":10,"Attack":10},"ActiveSkills":["SandTornado","RockLance"],"Passives":["Legend","CraftSpeed_up3"]}`
	if recorder := request(http.MethodPut, "/api/security/paldefender/gm/pal-templates/reward_anubis", template); recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"reward_anubis.json"`) {
		t.Fatalf("put template = %d %s", recorder.Code, recorder.Body.String())
	}
	for _, path := range []string{
		"/api/security/paldefender/gm/pal-templates",
		"/api/security/paldefender/gm/pal-templates/reward_anubis",
		"/api/security/paldefender/gm/commands",
		"/api/security/paldefender/gm/catalog/references",
		"/api/security/paldefender/access",
		"/api/security/paldefender/gm/players/steam_1/exported-pal-templates",
		"/api/security/paldefender/gm/players/steam_1/exported-pal-templates/Existing%20Anubis.json",
	} {
		recorder := request(http.MethodGet, path, "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s = %d %s", path, recorder.Code, recorder.Body.String())
		}
	}
	access := `{"use_whitelist":true,"whitelist_message":"Members only","use_admin_whitelist":true,"admin_auto_login":true,"admin_ips":["127.0.0.1","192.168.*.*"]}`
	if recorder := request(http.MethodPut, "/api/security/paldefender/access", access); recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"reload_required":true`) {
		t.Fatalf("put access = %d %s", recorder.Code, recorder.Body.String())
	}
	if recorder := request(http.MethodDelete, "/api/security/paldefender/gm/pal-templates/reward_anubis", ""); recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"deleted":true`) {
		t.Fatalf("delete template = %d %s", recorder.Code, recorder.Body.String())
	}
	if recorder := request(http.MethodGet, "/api/security/paldefender/gm/pal-templates/reward_anubis", ""); recorder.Code != http.StatusNotFound {
		t.Fatalf("get deleted template = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestPalDefenderExtendedSecurityRoutesRejectOperator(t *testing.T) {
	router, _, _, _, cleanup := newPalDefenderGMTestRouter(t, RoleOperator, "http://127.0.0.1:17993")
	defer cleanup()
	for _, test := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPut, "/api/security/paldefender/access", `{"use_whitelist":false,"whitelist_message":"","use_admin_whitelist":false,"admin_auto_login":false,"admin_ips":[]}`},
		{http.MethodPut, "/api/security/paldefender/gm/pal-templates/test", `{"PalID":"Anubis"}`},
		{http.MethodPost, "/api/security/paldefender/whitelist/steam_1", ""},
		{http.MethodPost, "/api/security/paldefender/admins/steam_1/toggle", ""},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		if test.body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		request.Header.Set("Idempotency-Key", "gm-security-denied-001")
		authorizeTestRequest(request)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s %s = %d %s", test.method, test.path, recorder.Code, recorder.Body.String())
		}
	}
}
