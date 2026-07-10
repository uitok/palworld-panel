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
	if cfg.SteamWebAPIKeySource != "environment" {
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

func TestLoadLeavesSteamWebAPIKeyUnconfiguredWhenEnvUnset(t *testing.T) {
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
	if cfg.SteamWebAPIKeySource != "" {
		t.Fatalf("SteamWebAPIKeySource = %q", cfg.SteamWebAPIKeySource)
	}
	if cfg.SteamWebAPIKey != "" || cfg.SteamWebAPIKeyConfigured() {
		t.Fatal("Steam Web API key should be unconfigured")
	}
}

func TestLoadUsesProductionNetworkAndProviderDefaults(t *testing.T) {
	t.Setenv("PANEL_TOKEN", "")
	t.Setenv("PALPANEL_REQUIRE_AUTH", "false")
	t.Setenv("PALPANEL_LISTEN_ADDR", "")
	t.Setenv("PALPANEL_STEAM_API_BASE_URL", "")
	t.Setenv("PALPANEL_STEAM_API_TIMEOUT_SECONDS", "")
	t.Setenv("PALPANEL_AI_TRANSLATION_TIMEOUT_SECONDS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "127.0.0.1:8080" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.SteamAPIBaseURL != DefaultSteamAPIBaseURL || cfg.SteamAPITimeoutSeconds != 15 {
		t.Fatalf("Steam defaults = %q, %d", cfg.SteamAPIBaseURL, cfg.SteamAPITimeoutSeconds)
	}
	if cfg.AITranslationTimeoutSeconds != 90 {
		t.Fatalf("AITranslationTimeoutSeconds = %d", cfg.AITranslationTimeoutSeconds)
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

func TestLoadRejectsInvalidScalarConfiguration(t *testing.T) {
	tests := map[string]string{
		"PALPANEL_REQUIRE_AUTH":                   "sometimes",
		"PALPANEL_STEAM_API_TIMEOUT_SECONDS":      "soon",
		"PALPANEL_AI_TRANSLATION_TIMEOUT_SECONDS": "0",
		"PALPANEL_LISTEN_ADDR":                    "127.0.0.1:not-a-port",
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("PANEL_TOKEN", "")
			t.Setenv("PALPANEL_REQUIRE_AUTH", "false")
			t.Setenv(name, value)
			if _, err := Load(); err == nil {
				t.Fatalf("expected %s=%q to fail", name, value)
			}
		})
	}
}
