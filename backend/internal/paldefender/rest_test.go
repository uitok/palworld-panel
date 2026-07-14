package paldefender

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRESTReadsVersionPlayersAndInventory(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	setTestRESTToken(t, manager, "rest-secret")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer rest-secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Origin") != panelRESTOrigin {
			t.Fatalf("Origin = %q", r.Header.Get("Origin"))
		}
		switch r.URL.Path {
		case "/v1/pdapi/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"Version": map[string]any{"Major": 1, "Minor": 8, "Patch": 1, "Version": "1.8.1"}})
		case "/v1/pdapi/players":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Meta":    map[string]any{"PlayerCount": 1, "OnlineCount": 1},
				"Players": []map[string]any{{"Name": "Builder", "UserId": "steam_1", "PlayerUID": "uid_1", "Status": "Online", "MapLocation": map[string]any{"x": 1.5, "y": 2.5, "z": 3.5}}},
			})
		case "/v1/pdapi/items/steam_1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Meta":      map[string]any{"PlayerUID": "uid_1", "Player": "steam_1"},
				"Inventory": map[string]any{"Items": map[string]any{"Available": true, "UsedSlots": 1, "MaxSlots": 42, "FreeSlots": 41, "Slots": map[string]any{"0": map[string]any{"ItemID": "Money", "Count": 25}}}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	manager.restBaseURL = server.URL

	version, err := manager.RESTVersion(context.Background())
	if err != nil || version.Version != "1.8.1" || version.Minor != 8 {
		t.Fatalf("RESTVersion = %#v, %v", version, err)
	}
	players, err := manager.RESTPlayers(context.Background())
	if err != nil || players.Meta.OnlineCount != 1 || len(players.Players) != 1 || players.Players[0].UserID != "steam_1" {
		t.Fatalf("RESTPlayers = %#v, %v", players, err)
	}
	inventory, err := manager.RESTInventory(context.Background(), "steam_1")
	if err != nil || inventory.Inventory.Items.Slots["0"].ItemID != "Money" || inventory.Inventory.Items.FreeSlots != 41 {
		t.Fatalf("RESTInventory = %#v, %v", inventory, err)
	}

	status := manager.GMStatus(context.Background())
	if !status.Configured || !status.Available || status.Version == nil || status.Version.Version != "1.8.1" || status.Error != "" {
		t.Fatalf("GMStatus = %#v", status)
	}
}

func TestRESTGMWriteRequests(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	setTestRESTToken(t, manager, "rest-secret")

	type capturedRequest struct {
		Method string
		Path   string
		Body   map[string]any
	}
	var captured []capturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		captured = append(captured, capturedRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		switch {
		case strings.Contains(r.URL.Path, "/give/items/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"Granted": map[string]any{"Items": 12}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"Success": true})
		}
	}))
	defer server.Close()
	manager.restBaseURL = server.URL

	granted, err := manager.RESTGiveItems(context.Background(), "steam_1", GiveItemsRequest{Items: []ItemGrant{{ItemID: " Money ", Count: 12}}})
	if err != nil || granted.Granted.Items != 12 {
		t.Fatalf("RESTGiveItems = %#v, %v", granted, err)
	}
	if _, err := manager.RESTSendMessage(context.Background(), "uid_1", SendMessageRequest{Message: " Notice "}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RESTBroadcast(context.Background(), "Maintenance", true); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RESTKick(context.Background(), "steam_1", PunishmentRequest{Reason: "AFK"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RESTBan(context.Background(), "steam_1", PunishmentRequest{Reason: "abuse", IP: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RESTUnban(context.Background(), "steam_1", PunishmentRequest{Reason: "appeal", IP: true}); err != nil {
		t.Fatal(err)
	}

	if len(captured) != 6 {
		t.Fatalf("captured %d requests: %#v", len(captured), captured)
	}
	if captured[0].Method != http.MethodPost || captured[0].Path != "/v1/pdapi/give/items/steam_1" {
		t.Fatalf("give request = %#v", captured[0])
	}
	items, ok := captured[0].Body["Items"].([]any)
	if !ok || len(items) != 1 || items[0].(map[string]any)["ItemID"] != "Money" {
		t.Fatalf("give body = %#v", captured[0].Body)
	}
	if captured[1].Path != "/v1/pdapi/SendPlayerMessage" || captured[1].Body["SendType"] != "PlayerLogImportant" || captured[1].Body["UserID"] != "uid_1" {
		t.Fatalf("message request = %#v", captured[1])
	}
	if captured[2].Path != "/v1/pdapi/Alert" || captured[2].Body["Message"] != "Maintenance" {
		t.Fatalf("alert request = %#v", captured[2])
	}
	if captured[4].Body["IP"] != true || captured[5].Body["IP"] != nil {
		t.Fatalf("punishment bodies = %#v %#v", captured[4].Body, captured[5].Body)
	}
}

func TestRESTValidationDoesNotSendRequests(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	setTestRESTToken(t, manager, "rest-secret")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	manager.restBaseURL = server.URL

	tests := []error{}
	_, err := manager.RESTInventory(context.Background(), "../../bad")
	tests = append(tests, err)
	_, err = manager.RESTGiveItems(context.Background(), "steam_1", GiveItemsRequest{})
	tests = append(tests, err)
	_, err = manager.RESTGiveItems(context.Background(), "steam_1", GiveItemsRequest{Items: []ItemGrant{{ItemID: "Money", Count: 0}}})
	tests = append(tests, err)
	_, err = manager.RESTSendMessage(context.Background(), "steam_1", SendMessageRequest{Message: " "})
	tests = append(tests, err)
	_, err = manager.RESTBroadcast(context.Background(), "", false)
	tests = append(tests, err)
	_, err = manager.RESTBan(context.Background(), "steam_1", PunishmentRequest{Reason: strings.Repeat("x", 1025)})
	tests = append(tests, err)
	for _, validationErr := range tests {
		if !errors.Is(validationErr, ErrInvalidRESTRequest) {
			t.Errorf("validation error = %v", validationErr)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("sent %d requests for invalid input", requests.Load())
	}
}

func TestRESTErrorsLimitsMissingTokenAndRedirects(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	if _, err := manager.RESTPlayers(context.Background()); !errors.Is(err, ErrRESTTokenMissing) {
		t.Fatalf("missing token error = %v", err)
	}
	status := manager.GMStatus(context.Background())
	if status.Configured || status.Available || status.Error != "" {
		t.Fatalf("unconfigured status = %#v", status)
	}
	setTestRESTToken(t, manager, "rest-secret")

	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		redirected.Store(true)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/pdapi/players":
			w.Header().Set("Location", target.URL)
			w.WriteHeader(http.StatusFound)
		case "/v1/pdapi/version":
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"Error":{"Code":"MISSING_PERMISSION","Message":"permission denied"}}`)
		case "/v1/pdapi/items/steam_1":
			_, _ = io.WriteString(w, strings.Repeat("x", restResponseLimit+1))
		}
	}))
	defer server.Close()
	manager.restBaseURL = server.URL

	if _, err := manager.RESTPlayers(context.Background()); err == nil {
		t.Fatal("expected redirect response to fail")
	}
	if redirected.Load() {
		t.Fatal("REST client followed redirect with a bearer credential")
	}
	_, err := manager.RESTVersion(context.Background())
	var restErr *RESTError
	if !errors.As(err, &restErr) || restErr.Status != http.StatusForbidden || restErr.Code != "MISSING_PERMISSION" || restErr.Message != "permission denied" {
		t.Fatalf("REST error = %#v, %v", restErr, err)
	}
	if _, err := manager.RESTInventory(context.Background(), "steam_1"); !errors.Is(err, ErrRESTResponseTooLarge) {
		t.Fatalf("large response error = %v", err)
	}

	transport, ok := newRESTHTTPClient().Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatalf("REST transport must disable proxies: %#v", transport)
	}
}

func setTestRESTToken(t *testing.T, manager Manager, token string) {
	t.Helper()
	if err := manager.store.SetKV(context.Background(), kvRESTToken, token); err != nil {
		t.Fatal(err)
	}
}
