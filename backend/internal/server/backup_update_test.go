package server

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPreUpdateBackupIncludesPalDefenderAndManifestHashes(t *testing.T) {
	m, cleanup := newVersionTestManager(t, "100")
	defer cleanup()
	writeFile(t, filepath.Join(m.cfg.ServerDir, "Pal", "Saved", "SaveGames", "0", "world", "Level.sav"), "world")
	writeFile(t, m.cfg.PalWorldSettingsPath(), "settings")
	writeFile(t, m.cfg.PalModSettingsPath(), "mods")
	writeFile(t, filepath.Join(m.cfg.Win64Dir(), "PalDefender.dll"), "plugin")
	writeFile(t, filepath.Join(m.cfg.Win64Dir(), "d3d9.dll"), "loader")
	writeFile(t, filepath.Join(m.cfg.PalDefenderDir(), "Config.json"), "{}")

	backup, err := m.createBackupArchive("pre-update")
	if err != nil {
		t.Fatalf("createBackupArchive returned error: %v", err)
	}
	verified, err := verifyBackupArchive(backup.Path, backup.Name)
	if err != nil || !verified.Valid || verified.Format != "manifest_v1" {
		t.Fatalf("backup verification = %#v, %v", verified, err)
	}

	reader, err := zip.OpenReader(backup.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, name := range []string{
		"Pal/Binaries/Win64/PalDefender.dll",
		"Pal/Binaries/Win64/d3d9.dll",
		"Pal/Binaries/Win64/PalDefender/Config.json",
		"Pal/Saved/SaveGames/0/world/Level.sav",
		".palpanel-backup.json",
	} {
		if !names[name] {
			t.Errorf("backup is missing %s", name)
		}
	}
}

func TestUpdateProtectedSnapshotDetectsConfigChanges(t *testing.T) {
	m, cleanup := newVersionTestManager(t, "100")
	defer cleanup()
	writeFile(t, m.cfg.PalWorldSettingsPath(), "before")
	before, err := m.snapshotUpdateProtectedFiles()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, m.cfg.PalWorldSettingsPath(), "after")
	if err := m.verifyUpdateProtectedFiles(before); err == nil {
		t.Fatal("expected protected config modification to be detected")
	}
}

func TestBootstrapDoesNotSnapshotUpdateProtectedFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("broken symlink behavior differs on Windows")
	}
	m, cleanup := newVersionTestManager(t, "100")
	defer cleanup()
	savedDir := filepath.Join(m.cfg.ServerDir, "Pal", "Saved")
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(savedDir, "missing"), filepath.Join(savedDir, "broken")); err != nil {
		t.Fatal(err)
	}
	m.installOrUpdateFunc = func(context.Context, string) error { return nil }

	job, err := m.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}
	done := waitForJob(t, m.store, job.ID)
	if done.Status != "completed" {
		t.Fatalf("bootstrap must not read update-only protected files: %#v", done)
	}
}
