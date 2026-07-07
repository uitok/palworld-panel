package appconfig

import (
	"os"
	"testing"
)

func TestLoadRejectsMissingPanelTokenByDefault(t *testing.T) {
	t.Setenv("PANEL_TOKEN", "")
	t.Setenv("PALPANEL_REQUIRE_AUTH", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected missing PANEL_TOKEN to fail")
	}
}

func TestLoadAllowsExplicitDevNoAuth(t *testing.T) {
	t.Setenv("PANEL_TOKEN", "")
	t.Setenv("PALPANEL_REQUIRE_AUTH", "false")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PALPANEL_DATA_DIR", cwd)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.RequireAuth {
		t.Fatal("expected auth to be disabled")
	}
}

func TestLoadRejectsWeakRoleTokens(t *testing.T) {
	t.Setenv("PANEL_TOKEN", "replace-with-a-random-32-byte-token")
	t.Setenv("PANEL_OPERATOR_TOKEN", "change-me")
	t.Setenv("PALPANEL_REQUIRE_AUTH", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected weak operator token to fail")
	}
}
