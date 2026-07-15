package server

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
)

func TestStopWindowsRefusesReusedPID(t *testing.T) {
	manager, cleanup := newWindowsProcessTestManager(t)
	defer cleanup()
	record := windowsProcessRecord{PID: 4321, Executable: manager.cfg.PalServerExePath(), CreationTime: 100}
	if err := manager.persistWindowsProcess(record); err != nil {
		t.Fatal(err)
	}
	manager.inspectProcess = func(int) (windowsProcessInfo, error) {
		return windowsProcessInfo{Running: true, Executable: filepath.Join(manager.cfg.DataDir, "unrelated.exe"), CreationTime: 200}, nil
	}
	terminated := false
	manager.terminateProcess = func(context.Context, int) error {
		terminated = true
		return nil
	}

	err := manager.stopWindows(t.Context())
	if err == nil || !strings.Contains(err.Error(), "refusing to manage PID") {
		t.Fatalf("stopWindows error = %v", err)
	}
	if terminated {
		t.Fatal("a process with a mismatched identity must not be terminated")
	}
	if _, ok, loadErr := manager.loadWindowsProcess(t.Context()); loadErr != nil || !ok {
		t.Fatalf("the mismatched process record should remain for diagnosis: ok=%v err=%v", ok, loadErr)
	}
}

func TestStopWindowsTerminatesOnlyMatchingProcessAndClearsRecord(t *testing.T) {
	manager, cleanup := newWindowsProcessTestManager(t)
	defer cleanup()
	record := windowsProcessRecord{PID: 1234, Executable: manager.cfg.PalServerExePath(), CreationTime: 99}
	if err := manager.persistWindowsProcess(record); err != nil {
		t.Fatal(err)
	}
	manager.inspectProcess = func(pid int) (windowsProcessInfo, error) {
		if pid != record.PID {
			t.Fatalf("inspected PID %d, want %d", pid, record.PID)
		}
		return windowsProcessInfo{Running: true, Executable: record.Executable, CreationTime: record.CreationTime}, nil
	}
	manager.terminateProcess = func(_ context.Context, pid int) error {
		if pid != record.PID {
			t.Fatalf("terminated PID %d, want %d", pid, record.PID)
		}
		return nil
	}

	if err := manager.stopWindows(t.Context()); err != nil {
		t.Fatalf("stopWindows returned error: %v", err)
	}
	if _, ok, err := manager.loadWindowsProcess(t.Context()); err != nil || ok {
		t.Fatalf("managed process record was not cleared: ok=%v err=%v", ok, err)
	}
}

func newWindowsProcessTestManager(t *testing.T) (Manager, func()) {
	t.Helper()
	root := t.TempDir()
	cfg := appconfig.Config{
		DataDir: root, ServerDir: filepath.Join(root, "server"), WinePrefixDir: filepath.Join(root, "wine"),
		ToolsDir: filepath.Join(root, "tools"), SteamCMDDir: filepath.Join(root, "tools", "steamcmd"),
		UploadsDir: filepath.Join(root, "uploads"), BackupsDir: filepath.Join(root, "backups"),
		LogsDir: filepath.Join(root, "logs"), DBPath: filepath.Join(root, "panel.db"),
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
