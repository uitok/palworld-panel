package paldefender

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExportPalsNormalizesUserIDAndReturnsOnlyFreshTemplate(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	prepareGMRESTFixture(t, manager)
	setTestRESTToken(t, manager, "rest-secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/pdapi/players" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Meta":    map[string]any{"PlayerCount": 1, "OnlineCount": 1},
			"Players": []map[string]any{{"Name": "Builder", "UserId": "steam_123", "PlayerUID": "uid_1", "Status": "Online"}},
		})
	}))
	defer server.Close()
	manager.restBaseURL = server.URL
	manager.exportPollInterval = 5 * time.Millisecond
	manager.exportTimeout = 500 * time.Millisecond

	oldExportedDir := filepath.Join(manager.cfg.PalDefenderDir(), "Pals", "Exported", "steam_123")
	newExportedDir := filepath.Join(manager.cfg.PalDefenderDir(), "pals", "exported", "steam_123")
	if err := os.MkdirAll(oldExportedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newExportedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldExportedDir, "old.json"), []byte(`{"PalID":"Lamball","Level":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	writeErr := make(chan error, 1)
	manager.exportRCON = func(_ context.Context, identifier string) (RCONResult, error) {
		if identifier != "steam_123" {
			t.Fatalf("RCON identifier = %q", identifier)
		}
		go func() {
			time.Sleep(20 * time.Millisecond)
			writeErr <- os.WriteFile(filepath.Join(newExportedDir, "Anubis new.json"), []byte(`{"PalID":"Anubis","Level":50}`), 0o600)
		}()
		return RCONResult{Command: "/exportpals steam_123", Output: "Export command accepted"}, nil
	}

	result, err := manager.ExportPals(t.Context(), "uid_1")
	if err != nil {
		t.Fatal(err)
	}
	if err := <-writeErr; err != nil {
		t.Fatal(err)
	}
	if result.PlayerID != "steam_123" || result.TemplateInfo.Name != "Anubis new.json" || result.Template.PalID != "Anubis" || len(result.Templates) != 1 {
		t.Fatalf("unexpected export result: %#v", result)
	}
}

func TestExportPalsTimesOutWithoutFreshTemplate(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	prepareGMRESTFixture(t, manager)
	setTestRESTToken(t, manager, "rest-secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Meta":    map[string]any{"PlayerCount": 1, "OnlineCount": 1},
			"Players": []map[string]any{{"Name": "Builder", "UserId": "steam_123", "PlayerUID": "uid_1", "Status": "Online"}},
		})
	}))
	defer server.Close()
	manager.restBaseURL = server.URL
	manager.exportPollInterval = 5 * time.Millisecond
	manager.exportTimeout = 20 * time.Millisecond
	manager.exportRCON = func(context.Context, string) (RCONResult, error) {
		return RCONResult{Output: "Export command accepted"}, nil
	}

	_, err := manager.ExportPals(t.Context(), "uid_1")
	if !errors.Is(err, ErrExportPalsTimeout) {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestClassifyExportPalsOutput(t *testing.T) {
	tests := []struct {
		output string
		want   error
	}{
		{"Player is offline or not loaded", ErrExportPlayerUnavailable},
		{"Player has no pals to export", ErrExportNoPals},
		{"Command rejected: permission denied", ErrExportCommandRejected},
		{"Export command accepted", nil},
	}
	for _, test := range tests {
		t.Run(test.output, func(t *testing.T) {
			if err := classifyExportPalsOutput(test.output); !errors.Is(err, test.want) {
				t.Fatalf("classify error = %v, want %v", err, test.want)
			}
		})
	}
}
