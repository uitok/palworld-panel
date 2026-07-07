package paldefender

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
)

func TestInstallReleaseFromZip(t *testing.T) {
	zipBytes := makePalDefenderZip(t)
	sum := sha256.Sum256(zipBytes)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBytes)
	}))
	defer server.Close()

	manager, cleanup := testManager(t)
	defer cleanup()
	release := Release{
		TagName: "v-test",
		Assets: []Asset{{
			Name:               "PalDefender.zip",
			Digest:             "sha256:" + hex.EncodeToString(sum[:]),
			BrowserDownloadURL: server.URL + "/PalDefender.zip",
		}},
	}
	if err := manager.installRelease(context.Background(), release); err != nil {
		t.Fatalf("installRelease returned error: %v", err)
	}
	for _, name := range []string{"PalDefender.dll", "d3d9.dll"} {
		if !fileExists(filepath.Join(manager.cfg.Win64Dir(), name)) {
			t.Fatalf("%s was not installed", name)
		}
	}
}

func TestBalancedPresetAndRESTToken(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	cfg, err := manager.ApplyPreset("balanced")
	if err != nil {
		t.Fatalf("ApplyPreset returned error: %v", err)
	}
	if cfg["shouldKickCheaters"] != true || cfg["shouldBanCheaters"] != false {
		t.Fatalf("unexpected balanced preset: %#v", cfg)
	}
	token, err := manager.CreateRESTToken(context.Background(), "Panel", nil)
	if err != nil {
		t.Fatalf("CreateRESTToken returned error: %v", err)
	}
	if token.Token == "" || !fileExists(token.Path) || !manager.restEnabled() {
		t.Fatalf("unexpected token result: %#v", token)
	}
}

func testManager(t *testing.T) (Manager, func()) {
	t.Helper()
	root := t.TempDir()
	cfg := appconfig.Config{
		DataDir:       root,
		ServerDir:     filepath.Join(root, "server"),
		ToolsDir:      filepath.Join(root, "tools"),
		SteamCMDDir:   filepath.Join(root, "tools", "steamcmd"),
		BackupsDir:    filepath.Join(root, "backups"),
		UploadsDir:    filepath.Join(root, "uploads"),
		WinePrefixDir: filepath.Join(root, "wineprefix"),
		LogsDir:       filepath.Join(root, "logs"),
		DBPath:        filepath.Join(root, "test.db"),
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs returned error: %v", err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
	return NewManager(cfg, store), func() { _ = store.Close() }
}

func makePalDefenderZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"PalDefender.dll": "paldefender",
		"d3d9.dll":        "loader",
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestVerifyDigestRejectsMismatch(t *testing.T) {
	err := verifyDigest(Asset{Name: "x", Digest: "sha256:deadbeef"}, []byte("content"))
	if err == nil {
		t.Fatal("expected digest mismatch")
	}
}

func TestRollbackRestoresBackup(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	if err := os.MkdirAll(filepath.Join(manager.cfg.BackupsDir, "paldefender-20260101T000000Z"), 0o755); err != nil {
		t.Fatal(err)
	}
	backupFile := filepath.Join(manager.cfg.BackupsDir, "paldefender-20260101T000000Z", "PalDefender.dll")
	if err := os.WriteFile(backupFile, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Rollback(context.Background()); err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(manager.cfg.Win64Dir(), "PalDefender.dll"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "old" {
		t.Fatalf("unexpected restored file: %q", string(b))
	}
}

func TestReloadConfigUsesOfficialEndpoint(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	if err := manager.store.SetKV(context.Background(), kvRESTToken, "secret"); err != nil {
		t.Fatal(err)
	}

	var paths []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path != "/ReloadConfig" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("missing bearer token")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer api.Close()
	manager.restBaseURL = api.URL

	if err := manager.ReloadConfig(context.Background()); err != nil {
		t.Fatalf("ReloadConfig returned error: %v", err)
	}
	if len(paths) != 1 || paths[0] != "/ReloadConfig" {
		t.Fatalf("unexpected paths: %#v", paths)
	}
}

func TestReloadConfigFallsBackToLegacyEndpoint(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	if err := manager.store.SetKV(context.Background(), kvRESTToken, "secret"); err != nil {
		t.Fatal(err)
	}

	var paths []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/ReloadConfig" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/v1/pdapi/ReloadConfig" {
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			return
		}
		http.NotFound(w, r)
	}))
	defer api.Close()
	manager.restBaseURL = api.URL

	if err := manager.ReloadConfig(context.Background()); err != nil {
		t.Fatalf("ReloadConfig returned error: %v", err)
	}
	if len(paths) != 2 || paths[0] != "/ReloadConfig" || paths[1] != "/v1/pdapi/ReloadConfig" {
		t.Fatalf("unexpected paths: %#v", paths)
	}
}
