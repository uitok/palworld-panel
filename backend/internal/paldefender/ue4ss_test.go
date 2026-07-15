package paldefender

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUE4SSInstallPreservesUserConfigurationAndIsIdempotent(t *testing.T) {
	archive := makeUE4SSZip(t, nil)
	manager, cleanup := ue4ssFixtureManager(t, archive)
	defer cleanup()
	settingsPath := filepath.Join(manager.cfg.Win64Dir(), "UE4SS-settings.ini")
	modsPath := filepath.Join(manager.cfg.Win64Dir(), "Mods", "mods.txt")
	writeTestFile(t, settingsPath, "custom-settings")
	writeTestFile(t, modsPath, "custom-mods")

	if err := manager.installUE4SS(t.Context()); err != nil {
		t.Fatalf("installUE4SS returned error: %v", err)
	}
	status := manager.UE4SSStatus()
	if status.State != UE4SSInstalled || !status.Installed || !status.Compatible || status.Version != "v-test" {
		t.Fatalf("UE4SS status = %#v", status)
	}
	if status.LoadVerified {
		t.Fatal("file installation was incorrectly reported as startup-log verification")
	}
	writeTestFile(t, manager.cfg.ServerLogPath(), "[UE4SS] loader initialized")
	status = manager.UE4SSStatus()
	if !status.LoadVerified || status.LoadEvidence != "palserver_log" {
		t.Fatalf("UE4SS startup-log evidence was not detected: %#v", status)
	}
	for path, want := range map[string]string{settingsPath: "custom-settings", modsPath: "custom-mods"} {
		body, err := os.ReadFile(path)
		if err != nil || string(body) != want {
			t.Fatalf("preserved file %s = %q, %v", path, body, err)
		}
	}
	if err := manager.installUE4SS(t.Context()); err != nil {
		t.Fatalf("repeated installUE4SS returned error: %v", err)
	}
	if body, _ := os.ReadFile(settingsPath); string(body) != "custom-settings" {
		t.Fatalf("repeat install overwrote custom settings: %q", body)
	}
}

func TestUE4SSRejectsHashMismatchBeforeWritingGameDirectory(t *testing.T) {
	archive := makeUE4SSZip(t, nil)
	manager, cleanup := ue4ssFixtureManager(t, archive)
	defer cleanup()
	manager.cfg.UE4SSArchiveSHA256 = strings.Repeat("0", 64)

	err := manager.installUE4SS(t.Context())
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("installUE4SS error = %v", err)
	}
	if fileExists(filepath.Join(manager.cfg.Win64Dir(), "UE4SS.dll")) {
		t.Fatal("hash mismatch wrote UE4SS files into the game directory")
	}
}

func TestUE4SSRollbackRestoresEarlierCoreFile(t *testing.T) {
	archive := makeUE4SSZip(t, nil)
	manager, cleanup := ue4ssFixtureManager(t, archive)
	defer cleanup()
	ue4ssDLL := filepath.Join(manager.cfg.Win64Dir(), "UE4SS.dll")
	writeTestFile(t, ue4ssDLL, "old-core")
	blockedDestination := filepath.Join(manager.cfg.Win64Dir(), "dwmapi.dll")
	if err := os.MkdirAll(blockedDestination, 0o755); err != nil {
		t.Fatal(err)
	}

	err := manager.installUE4SS(t.Context())
	if err == nil {
		t.Fatal("expected the blocked destination to fail installation")
	}
	body, readErr := os.ReadFile(ue4ssDLL)
	if readErr != nil || string(body) != "old-core" {
		t.Fatalf("old core was not restored: %q, %v", body, readErr)
	}
}

func TestUE4SSStatusDetectsUnknownAndDamagedInstallations(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	for _, relative := range ue4ssRequiredFiles {
		writeTestFile(t, filepath.Join(manager.cfg.Win64Dir(), relative), "fixture")
	}
	status := manager.UE4SSStatus()
	if status.State != UE4SSIncompatible || !status.Installed {
		t.Fatalf("unknown installation status = %#v", status)
	}

	archive := makeUE4SSZip(t, nil)
	configured, configuredCleanup := ue4ssFixtureManager(t, archive)
	defer configuredCleanup()
	if err := configured.installUE4SS(t.Context()); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(configured.cfg.Win64Dir(), "UE4SS.dll"), "changed")
	status = configured.UE4SSStatus()
	if status.State != UE4SSFailed || !strings.Contains(status.Message, "damaged") {
		t.Fatalf("damaged installation status = %#v", status)
	}
}

func ue4ssFixtureManager(t *testing.T, archive []byte) (Manager, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	}))
	manager, cleanup := testManager(t)
	sum := sha256.Sum256(archive)
	manager.cfg.UE4SSVersion = "v-test"
	manager.cfg.UE4SSDownloadURL = server.URL
	manager.cfg.UE4SSArchiveSHA256 = hex.EncodeToString(sum[:])
	manager.cfg.UE4SSDownloadMaxBytes = 4 << 20
	return manager, func() {
		server.Close()
		cleanup()
	}
}

func makeUE4SSZip(t *testing.T, overrides map[string]string) []byte {
	t.Helper()
	files := map[string]string{
		"UE4SS.dll":          "new-core",
		"dwmapi.dll":         "new-loader",
		"UE4SS-settings.ini": "default-settings",
		"Mods/mods.txt":      "default-mods",
	}
	for name, body := range overrides {
		if body == "" {
			delete(files, name)
		} else {
			files[name] = body
		}
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, body := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
