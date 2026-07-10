package server

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
)

func TestLogsPreferPersistentFileAndRemainAvailableWhenStopped(t *testing.T) {
	m, cleanup := newLogTestManager(t, `
case "$1" in
  logs) echo "docker output" ;;
  inspect) echo exited ;;
esac`)
	defer cleanup()
	writeFile(t, m.cfg.ServerLogPath(), "[INFO] keep\n[ERROR] selected\n")

	result, err := m.Logs(context.Background(), LogQuery{Tail: 80, Level: "error"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != "file" || !result.Available || !strings.Contains(result.Logs, "selected") || strings.Contains(result.Logs, "keep") {
		t.Fatalf("unexpected log result: %#v", result)
	}
	if result.UpdatedAt == "" {
		t.Fatal("file log result is missing updated_at")
	}
}

func TestLogsFallBackToDocker(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises Docker log fallback")
	}
	m, cleanup := newLogTestManager(t, `
case "$1" in
  logs) echo "docker fallback output" ;;
  inspect) echo running ;;
esac`)
	defer cleanup()

	result, err := m.Logs(context.Background(), LogQuery{Tail: 80, Search: "fallback"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != "docker" || !result.Available || !strings.Contains(result.Logs, "fallback") {
		t.Fatalf("unexpected Docker fallback: %#v", result)
	}
}

func TestLogsReturnMetadataInsteadOfErrorWithoutCollectionSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises Docker log fallback")
	}
	m, cleanup := newLogTestManager(t, `
echo "No such object" >&2
exit 1`)
	defer cleanup()

	result, err := m.Logs(context.Background(), LogQuery{Tail: 80})
	if err != nil {
		t.Fatalf("empty logs must not be an API failure: %v", err)
	}
	if result.Source != "none" || result.Available || result.Reason != "not_started" || result.Logs != "" {
		t.Fatalf("unexpected unavailable result: %#v", result)
	}
}

func TestRollingLogWriterKeepsBoundedBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "palserver.log")
	writer, err := newRollingLogWriter(path, 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("abcdefghijklmnopqrstuvwxyz")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		path:        "uvwxyz",
		path + ".1": "klmnopqrst",
		path + ".2": "abcdefghij",
	}
	for file, expected := range want {
		body, err := os.ReadFile(file)
		if err != nil || string(body) != expected {
			t.Errorf("%s = %q, %v; want %q", file, body, err, expected)
		}
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Fatalf("unexpected extra backup: %v", err)
	}
}

func newLogTestManager(t *testing.T, scriptBody string) (Manager, func()) {
	t.Helper()
	root := t.TempDir()
	fakeDocker := filepath.Join(root, "docker")
	if err := os.WriteFile(fakeDocker, []byte("#!/bin/sh\n"+scriptBody+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := appconfig.Config{
		DataDir: root, ServerDir: filepath.Join(root, "server"), WinePrefixDir: filepath.Join(root, "wineprefix"),
		ToolsDir: filepath.Join(root, "tools"), SteamCMDDir: filepath.Join(root, "steamcmd"), UploadsDir: filepath.Join(root, "uploads"),
		BackupsDir: filepath.Join(root, "backups"), LogsDir: filepath.Join(root, "logs"), DBPath: filepath.Join(root, "test.db"),
		DockerBinary: fakeDocker, DockerImage: "image", DockerContainer: "container", GamePort: 8211, QueryPort: 27015, RESTPort: 8212,
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
