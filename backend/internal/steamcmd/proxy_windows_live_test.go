//go:build windows

package steamcmd

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"

	"palpanel/internal/appconfig"
	"palpanel/internal/networkproxy"
)

// TestLiveInstallOrUpdateThroughManagedProxy updates an existing real Palworld
// installation through the managed proxy and verifies that the current-user
// Internet proxy is byte-for-byte equivalent afterward. It is opt-in because
// SteamCMD may download several gigabytes when the local game is outdated.
func TestLiveInstallOrUpdateThroughManagedProxy(t *testing.T) {
	if os.Getenv("PALPANEL_LIVE_INSTALL_PROXY") != "1" {
		t.Skip("set PALPANEL_LIVE_INSTALL_PROXY=1 and PALPANEL_LIVE_INSTALL_PROXY_URL to run the live proxy install check")
	}
	rawProxy := strings.TrimSpace(os.Getenv("PALPANEL_LIVE_INSTALL_PROXY_URL"))
	if _, err := networkproxy.ValidateURL(rawProxy); err != nil {
		t.Fatalf("PALPANEL_LIVE_INSTALL_PROXY_URL is invalid: %v", err)
	}
	cfg, err := appconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePEExecutable(cfg.SteamCMDBinaryPath()); err != nil {
		t.Fatalf("existing SteamCMD is required for the bounded live check: %v", err)
	}
	if err := ValidatePEExecutable(filepath.Join(cfg.ServerDirectory(), "PalServer.exe")); err != nil {
		t.Fatalf("existing Palworld server is required for the bounded live check: %v", err)
	}

	configPath := cfg.NetworkProxyConfigPath()
	previousConfig, readErr := os.ReadFile(configPath)
	previousExisted := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	defer func() {
		if previousExisted {
			_ = os.MkdirAll(filepath.Dir(configPath), 0o700)
			_ = os.WriteFile(configPath, previousConfig, 0o600)
		} else {
			_ = os.Remove(configPath)
		}
	}()

	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	before := captureProxyRegistry(key)
	_ = key.Close()

	enabled := true
	if _, err := networkproxy.New(cfg).Update(networkproxy.ConfigUpdate{InstallEnabled: &enabled, InstallProxyURL: &rawProxy}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Minute)
	defer cancel()
	if err := New(cfg).InstallOrUpdate(ctx, "2394010", cfg.ServerDirectory()); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePEExecutable(filepath.Join(cfg.ServerDirectory(), "PalServer.exe")); err != nil {
		t.Fatalf("Palworld server validation after proxied update failed: %v", err)
	}
	manifest := filepath.Join(cfg.ServerDirectory(), "steamapps", "appmanifest_2394010.acf")
	if info, err := os.Stat(manifest); err != nil || info.Size() == 0 {
		t.Fatalf("Palworld app manifest is missing after proxied update: %v", err)
	}
	if _, err := os.Stat(cfg.SteamCMDProxyRestorePath()); !os.IsNotExist(err) {
		t.Fatalf("SteamCMD proxy restoration marker remains after success: %v", err)
	}

	key, err = registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	after := captureProxyRegistry(key)
	_ = key.Close()
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("current-user Internet proxy settings were not restored: before=%#v after=%#v", before, after)
	}
}
