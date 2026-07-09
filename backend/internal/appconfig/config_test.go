package appconfig

import (
	"os"
	"regexp"
	"testing"
)

var steamWebAPIKeyPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

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
	t.Setenv("STEAM_WEB_API_KEY", "steam-key")
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
	if cfg.SteamWebAPIKey != "steam-key" {
		t.Fatalf("SteamWebAPIKey = %q", cfg.SteamWebAPIKey)
	}
	if cfg.SteamWebAPIKeySource != "env" {
		t.Fatalf("SteamWebAPIKeySource = %q", cfg.SteamWebAPIKeySource)
	}
	if cfg.WorkshopAppID != "1623730" {
		t.Fatalf("WorkshopAppID = %q", cfg.WorkshopAppID)
	}
	if cfg.PalDefenderRESTBaseURL != "http://127.0.0.1:17993" {
		t.Fatalf("PalDefenderRESTBaseURL = %q", cfg.PalDefenderRESTBaseURL)
	}
	if cfg.PalDefenderRESTPort != 17993 {
		t.Fatalf("PalDefenderRESTPort = %d", cfg.PalDefenderRESTPort)
	}
}

func TestLoadUsesPalDefenderRESTOverrides(t *testing.T) {
	t.Setenv("PANEL_TOKEN", "")
	t.Setenv("PALPANEL_REQUIRE_AUTH", "false")
	t.Setenv("PALPANEL_PALDEFENDER_REST_BASE_URL", "http://10.0.0.4:28080/")
	t.Setenv("PALPANEL_PALDEFENDER_REST_PORT", "28080")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PALPANEL_DATA_DIR", cwd)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.PalDefenderRESTBaseURL != "http://10.0.0.4:28080/" {
		t.Fatalf("PalDefenderRESTBaseURL = %q", cfg.PalDefenderRESTBaseURL)
	}
	if cfg.EffectivePalDefenderRESTBaseURL() != "http://10.0.0.4:28080" {
		t.Fatalf("EffectivePalDefenderRESTBaseURL = %q", cfg.EffectivePalDefenderRESTBaseURL())
	}
	if cfg.PalDefenderRESTPort != 28080 {
		t.Fatalf("PalDefenderRESTPort = %d", cfg.PalDefenderRESTPort)
	}
}

func TestLoadUsesEmbeddedSteamWebAPIKeyWhenEnvUnset(t *testing.T) {
	t.Setenv("PANEL_TOKEN", "")
	t.Setenv("PALPANEL_REQUIRE_AUTH", "false")
	t.Setenv("STEAM_WEB_API_KEY", "")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PALPANEL_DATA_DIR", cwd)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.SteamWebAPIKeySource != "embedded" {
		t.Fatalf("SteamWebAPIKeySource = %q", cfg.SteamWebAPIKeySource)
	}
	if !steamWebAPIKeyPattern.MatchString(cfg.SteamWebAPIKey) {
		t.Fatal("embedded Steam Web API key has invalid format")
	}
}

func TestDefaultSteamWebAPIKeyFormat(t *testing.T) {
	if !steamWebAPIKeyPattern.MatchString(DefaultSteamWebAPIKey()) {
		t.Fatal("default Steam Web API key has invalid format")
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
