package paldefender

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestExtendedRESTReadsAndWritesOfficialContracts(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	prepareGMRESTFixture(t, manager)
	setTestRESTToken(t, manager, "rest-secret")

	var captured = map[string]map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		captured[r.URL.Path] = body
		switch r.URL.Path {
		case "/v1/pdapi/progression/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Meta": map[string]any{"Player": "steam_1", "PlayerUID": "uid_1"}, "Progression": map[string]any{"Player": map[string]any{"level": 42, "exp": 1000, "unusedStatusPoints": 3}, "Currencies": map[string]any{"technologyPoints": 8, "ancientTechnologyPoints": 2, "relics": map[string]any{"CapturePower": 4}}, "Bosses": map[string]any{}, "Captures": map[string]any{}, "Activities": map[string]any{}}})
		case "/v1/pdapi/give/progression/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Granted": map[string]any{"TechnologyPoints": 10}, "Totals": map[string]any{"TechnologyPoints": 18}})
		case "/v1/pdapi/techs/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Meta": map[string]any{"Player": "steam_1", "PlayerUID": "uid_1", "UnlockedCount": 1, "LockedCount": 2, "TotalCount": 3}, "Techs": map[string]any{"Unlocked": []string{"Technology_ElecBaton"}}})
		case "/v1/pdapi/learntech/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"UnlockedCount": 1, "Unlocked": []string{"Technology_GrapplingGun"}, "Skipped": []string{}})
		case "/v1/pdapi/forgettech/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"ForgottenCount": 1, "Forgotten": []string{"Technology_ElecBaton"}, "Skipped": []string{}})
		case "/v1/pdapi/pals/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Meta": map[string]any{"Player": "steam_1", "PlayerUID": "uid_1", "TeamCount": 1, "PalboxCount": 0, "BaseCampCount": 0}, "Pals": map[string]any{"Team": map[string]any{"pal-1": map[string]any{"PalID": "Anubis", "Level": 50}}, "Palbox": map[string]any{}, "BaseCamps": []any{}}})
		case "/v1/pdapi/give/pals/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Granted": map[string]any{"Pals": 1}})
		case "/v1/pdapi/give/paltemplate/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"Granted": map[string]any{"PalTemplates": 1}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	manager.restBaseURL = server.URL

	progression, err := manager.RESTProgression(context.Background(), "steam_1")
	if err != nil || progression.Progression.Player.Level != 42 || progression.Progression.Currencies.TechnologyPoints != 8 {
		t.Fatalf("RESTProgression = %#v, %v", progression, err)
	}
	points := int64(10)
	if _, err := manager.RESTGiveProgression(context.Background(), "steam_1", GiveProgressionRequest{TechnologyPoints: &points}); err != nil {
		t.Fatal(err)
	}
	techs, err := manager.RESTTechs(context.Background(), "steam_1")
	if err != nil || techs.Meta.TotalCount != 3 || len(techs.Techs.Unlocked) != 1 {
		t.Fatalf("RESTTechs = %#v, %v", techs, err)
	}
	if _, err := manager.RESTLearnTechnology(context.Background(), "steam_1", TechnologyRequest{Technology: []string{"Technology_GrapplingGun"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RESTForgetTechnology(context.Background(), "steam_1", TechnologyRequest{Technology: "Technology_ElecBaton"}); err != nil {
		t.Fatal(err)
	}
	pals, err := manager.RESTPals(context.Background(), "steam_1")
	if err != nil || pals.Meta.TeamCount != 1 || pals.Pals["Team"] == nil {
		t.Fatalf("RESTPals = %#v, %v", pals, err)
	}
	if _, err := manager.RESTGivePals(context.Background(), "steam_1", GivePalsRequest{Pals: []PalGrant{{PalID: "Anubis", Level: 50}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RESTGivePalTemplates(context.Background(), "steam_1", GivePalTemplatesRequest{PalTemplates: []string{"reward_anubis"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RESTGiveCustomPal(context.Background(), "steam_1", PalTemplate{PalID: "Anubis", Passives: []string{"Legend", "CraftSpeed_up3"}}); err != nil {
		t.Fatal(err)
	}
	if captured["/v1/pdapi/give/progression/steam_1"]["TechnologyPoints"] != float64(10) {
		t.Fatalf("progression request = %#v", captured["/v1/pdapi/give/progression/steam_1"])
	}
	customTemplate := captured["/v1/pdapi/give/paltemplate/steam_1"]["PalTemplates"].([]any)[0].(string)
	if !strings.HasPrefix(customTemplate, "palpanel_grant_") || !strings.HasSuffix(customTemplate, ".json") {
		t.Fatalf("template request = %#v", captured["/v1/pdapi/give/paltemplate/steam_1"])
	}
	if _, err := os.Stat(filepath.Join(manager.palTemplatesDir(), customTemplate)); !os.IsNotExist(err) {
		t.Fatalf("temporary custom Pal template was not removed: %v", err)
	}
}

func TestExtendedRESTValidationDoesNotSendRequests(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	prepareGMRESTFixture(t, manager)
	setTestRESTToken(t, manager, "rest-secret")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	manager.restBaseURL = server.URL

	zero := int64(0)
	_, progressionErr := manager.RESTGiveProgression(t.Context(), "steam_1", GiveProgressionRequest{TechnologyPoints: &zero})
	_, techErr := manager.RESTLearnTechnology(t.Context(), "steam_1", TechnologyRequest{Technology: []string{"All"}})
	_, palErr := manager.RESTGivePals(t.Context(), "steam_1", GivePalsRequest{Pals: []PalGrant{{PalID: "../Anubis", Level: 50}}})
	_, templateErr := manager.RESTGivePalTemplates(t.Context(), "steam_1", GivePalTemplatesRequest{PalTemplates: []string{"../bad"}})
	for _, err := range []error{progressionErr, techErr, palErr, templateErr} {
		if !errors.Is(err, ErrInvalidRESTRequest) {
			t.Errorf("validation error = %v", err)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("sent %d requests for invalid input", requests.Load())
	}
	if _, err := normalizeTechnologySelection(strings.Repeat("x", 129)); !errors.Is(err, ErrInvalidRESTRequest) {
		t.Fatalf("long technology identifier error = %v", err)
	}
}
