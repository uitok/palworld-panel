//go:build windows

package server

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
)

func TestEnsureSteamCMDRetriesAtomicallyAndIsIdempotent(t *testing.T) {
	archive := steamCMDTestArchive(t, map[string]string{"steamcmd.exe": "MZ-steamcmd", "steamclient.dll": "fixture"})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	manager, cleanup := newSteamCMDTestManager(t, server.URL)
	defer cleanup()
	if err := os.MkdirAll(manager.cfg.SteamCMDDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldMarker := filepath.Join(manager.cfg.SteamCMDDir, "partial-download.txt")
	if err := os.WriteFile(oldMarker, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := manager.ensureSteamCMD(t.Context()); err != nil {
		t.Fatalf("ensureSteamCMD returned error: %v", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("download requests = %d, want 2", requests.Load())
	}
	if err := validatePEExecutable(manager.cfg.SteamCMDBinaryPath()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldMarker); !os.IsNotExist(err) {
		t.Fatalf("partial previous install was retained: %v", err)
	}
	if err := manager.ensureSteamCMD(t.Context()); err != nil {
		t.Fatalf("idempotent ensureSteamCMD returned error: %v", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("valid existing SteamCMD triggered another download: %d", requests.Load())
	}
}

func TestEnsureSteamCMDCancellationKeepsPreviousDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	manager, cleanup := newSteamCMDTestManager(t, server.URL)
	defer cleanup()
	if err := os.MkdirAll(manager.cfg.SteamCMDDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(manager.cfg.SteamCMDDir, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	err := manager.ensureSteamCMD(ctx)
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("ensureSteamCMD error = %v", err)
	}
	if body, readErr := os.ReadFile(marker); readErr != nil || string(body) != "keep" {
		t.Fatalf("previous directory changed after cancellation: %q, %v", body, readErr)
	}
}

func newSteamCMDTestManager(t *testing.T, downloadURL string) (Manager, func()) {
	t.Helper()
	runtimeRoot := filepath.Join(t.TempDir(), "runtime with space 中文")
	cfg := appconfig.Config{
		RuntimeRoot: runtimeRoot, DataDir: filepath.Join(runtimeRoot, "data"),
		ServerDir: filepath.Join(runtimeRoot, "palworld"), ToolsDir: filepath.Join(runtimeRoot, "temp"),
		SteamCMDDir: filepath.Join(runtimeRoot, "steamcmd"), UploadsDir: filepath.Join(runtimeRoot, "mods", "staging"),
		BackupsDir: filepath.Join(runtimeRoot, "data", "backups"), LogsDir: filepath.Join(runtimeRoot, "data", "logs"),
		DBPath: filepath.Join(runtimeRoot, "data", "database", "panel.db"), SaveIndexCacheDir: filepath.Join(runtimeRoot, "data", "save-index"),
		SteamCMDDownloadURL: downloadURL, SteamCMDDownloadMaxBytes: 4 << 20,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	return NewManager(cfg, store, docker.NewRunner(cfg)), func() { _ = store.Close() }
}

func steamCMDTestArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
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
