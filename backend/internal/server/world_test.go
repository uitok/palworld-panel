package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/palconfig"
)

func TestValidateWorldIDRejectsPaths(t *testing.T) {
	for _, value := range []string{"", ".", "..", "../world", "world/child", `world\child`, "/absolute", "world name"} {
		if err := validateWorldID(value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
	for _, value := range []string{"ABCDEF0123456789", "world-1", "world_name"} {
		if err := validateWorldID(value); err != nil {
			t.Errorf("expected %q to be accepted: %v", value, err)
		}
	}
}

func TestStoppedWorldResetCreatesVerifiedBackupAndPreservesServerData(t *testing.T) {
	m, cleanup := newVersionTestManager(t, "100")
	defer cleanup()
	m.cfg.SaveIndexCacheDir = filepath.Join(m.cfg.DataDir, "save-index")
	writeWorldFixture(t, m, "world-one")
	writeFile(t, filepath.Join(m.cfg.ModsDir(), "Workshop", "123", "Info.json"), `{}`)
	writeFile(t, filepath.Join(m.cfg.PalDefenderDir(), "Config.json"), `{}`)
	writeFile(t, filepath.Join(m.cfg.SaveIndexCacheDir, "index-cache.json"), `{}`)

	preview, err := m.WorldInfo(t.Context())
	if err != nil || !preview.ResetAvailable || preview.ServerRunning || preview.ActiveWorldID != "world-one" {
		t.Fatalf("unexpected preview: %#v, %v", preview, err)
	}
	job, err := m.ResetWorld(t.Context(), "world-one", worldResetConfirmation, WorldResetHooks{})
	if err != nil {
		t.Fatalf("ResetWorld returned error: %v", err)
	}
	done := waitForJob(t, m.store, job.ID)
	if done.Status != "completed" {
		t.Fatalf("reset failed: %#v", done)
	}
	if _, err := os.Stat(m.worldPath("world-one")); !os.IsNotExist(err) {
		t.Fatalf("old world still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.cfg.ServerDir, "Pal", "Saved", ".palpanel-world-reset", job.ID)); !os.IsNotExist(err) {
		t.Fatalf("staged world was not removed: %v", err)
	}
	for _, path := range []string{
		m.cfg.PalWorldSettingsPath(),
		filepath.Join(m.cfg.ModsDir(), "Workshop", "123", "Info.json"),
		filepath.Join(m.cfg.PalDefenderDir(), "Config.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("preserved file %s is missing: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(m.cfg.SaveIndexCacheDir, "index-cache.json")); !os.IsNotExist(err) {
		t.Fatalf("save index cache was not invalidated: %v", err)
	}
	backups, err := m.ListBackups()
	if err != nil || len(backups) != 1 || backups[0].Reason != "pre-world-reset" {
		t.Fatalf("unexpected backups: %#v, %v", backups, err)
	}
	verified, err := m.VerifyBackup(backups[0].Name)
	if err != nil || !verified.Valid || verified.Format != "manifest_v1" {
		t.Fatalf("backup verification = %#v, %v", verified, err)
	}
}

func TestWorldResetRechecksDedicatedServerNameAfterQueue(t *testing.T) {
	m, cleanup := newVersionTestManager(t, "100")
	defer cleanup()
	writeWorldFixture(t, m, "world-one")
	m.operationMu.Lock()
	job, err := m.ResetWorld(t.Context(), "world-one", worldResetConfirmation, WorldResetHooks{})
	if err != nil {
		m.operationMu.Unlock()
		t.Fatal(err)
	}
	if err := palconfig.Write(m.cfg.PalWorldSettingsPath(), palconfig.Settings{"DedicatedServerName": "world-two"}); err != nil {
		m.operationMu.Unlock()
		t.Fatal(err)
	}
	m.operationMu.Unlock()
	done := waitForJob(t, m.store, job.ID)
	if done.Status != "failed" || !strings.Contains(done.Error, "world-two") {
		t.Fatalf("stale world request was not rejected: %#v", done)
	}
	if !levelSaveReady(filepath.Join(m.worldPath("world-one"), "Level.sav")) {
		t.Fatal("stale reset moved the original world")
	}
}

func TestRunningWorldResetGeneratesNewWorldAndInvalidatesCaches(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Wine Docker runtime")
	}
	m, cleanup := newRunningWorldTestManager(t, true)
	defer cleanup()
	var prepared atomic.Int32
	var invalidated atomic.Int32
	job, err := m.ResetWorld(t.Context(), "running-world", worldResetConfirmation, WorldResetHooks{
		Prepare: func(context.Context) error {
			prepared.Add(1)
			return nil
		},
		Invalidate: func() { invalidated.Add(1) },
	})
	if err != nil {
		t.Fatal(err)
	}
	done := waitForJob(t, m.store, job.ID)
	if done.Status != "completed" {
		t.Fatalf("reset failed: %#v", done)
	}
	if prepared.Load() != 1 || invalidated.Load() < 1 {
		t.Fatalf("hooks prepared=%d invalidated=%d", prepared.Load(), invalidated.Load())
	}
	body, err := os.ReadFile(filepath.Join(m.worldPath("running-world"), "Level.sav"))
	if err != nil || string(body) != "new-world" {
		t.Fatalf("new world was not generated: %q, %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(m.worldPath("running-world"), "Players", "old.sav")); !os.IsNotExist(err) {
		t.Fatalf("old player progress survived reset: %v", err)
	}
}

func TestWorldResetStartFailureRetainsBackupAndStagedWorld(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Wine Docker runtime")
	}
	m, cleanup := newRunningWorldTestManager(t, false)
	defer cleanup()
	job, err := m.ResetWorld(t.Context(), "running-world", worldResetConfirmation, WorldResetHooks{Prepare: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	done := waitForJob(t, m.store, job.ID)
	if done.Status != "failed" || !strings.Contains(done.Error, "verified backup retained at") || !strings.Contains(done.Error, "old world retained at") {
		t.Fatalf("failure did not report recovery paths: %#v", done)
	}
	stagedLevel := filepath.Join(m.cfg.ServerDir, "Pal", "Saved", ".palpanel-world-reset", job.ID, "Level.sav")
	if !levelSaveReady(stagedLevel) {
		t.Fatalf("staged old world is missing at %s", stagedLevel)
	}
	backups, err := m.ListBackups()
	if err != nil || len(backups) != 1 {
		t.Fatalf("verified backup not retained: %#v, %v", backups, err)
	}
	verified, err := m.VerifyBackup(backups[0].Name)
	if err != nil || !verified.Valid {
		t.Fatalf("retained backup is invalid: %#v, %v", verified, err)
	}
}

func writeWorldFixture(t *testing.T, m Manager, worldID string) {
	t.Helper()
	if err := palconfig.Write(m.cfg.PalWorldSettingsPath(), palconfig.Settings{"DedicatedServerName": worldID}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(m.worldPath(worldID), "Level.sav"), "old-world")
	writeFile(t, filepath.Join(m.worldPath(worldID), "Players", "old.sav"), "old-player")
}

func newRunningWorldTestManager(t *testing.T, startSucceeds bool) (Manager, func()) {
	t.Helper()
	root := t.TempDir()
	statePath := filepath.Join(root, "stopped")
	serverDir := filepath.Join(root, "server")
	newLevel := filepath.Join(serverDir, "Pal", "Saved", "SaveGames", "0", "running-world", "Level.sav")
	fakeDocker := filepath.Join(root, "docker")
	runAction := "echo start failed >&2; exit 1"
	if startSucceeds {
		runAction = fmt.Sprintf("mkdir -p %s; printf new-world > %s; rm -f %s; echo container-id; exit 0", shellTestQuote(filepath.Dir(newLevel)), shellTestQuote(newLevel), shellTestQuote(statePath))
	}
	script := fmt.Sprintf(`#!/bin/sh
case "$1" in
  inspect) if [ -f %s ]; then printf '%%s\n' '[{"RestartCount":0,"State":{"Status":"exited","OOMKilled":false,"ExitCode":0,"StartedAt":"","FinishedAt":""}}]'; else printf '%%s\n' '[{"RestartCount":0,"State":{"Status":"running","OOMKilled":false,"ExitCode":0,"StartedAt":"","FinishedAt":""}}]'; fi ;;
  stop) touch %s ;;
  rm) exit 0 ;;
  run) %s ;;
  logs) exit 0 ;;
  *) exit 0 ;;
esac
`, shellTestQuote(statePath), shellTestQuote(statePath), runAction)
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := appconfig.Config{
		DataDir: root, ServerDir: serverDir, WinePrefixDir: filepath.Join(root, "wineprefix"), ToolsDir: filepath.Join(root, "tools"),
		SteamCMDDir: filepath.Join(root, "steamcmd"), UploadsDir: filepath.Join(root, "uploads"), BackupsDir: filepath.Join(root, "backups"),
		LogsDir: filepath.Join(root, "logs"), DBPath: filepath.Join(root, "test.db"), DockerBinary: fakeDocker, DockerImage: "test-image",
		DockerContainer: "test-container", GamePort: 8211, QueryPort: 27015, RESTPort: 8212, SaveIndexCacheDir: filepath.Join(root, "save-index"),
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(cfg, store, docker.NewRunner(cfg))
	writeFile(t, cfg.PalServerExePath(), "exe")
	writeWorldFixture(t, m, "running-world")
	return m, func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = m.jobs.Shutdown(shutdownCtx)
		_ = store.Close()
	}
}

func shellTestQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
