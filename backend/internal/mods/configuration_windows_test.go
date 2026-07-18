//go:build windows

package mods

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"palpanel/internal/db"
)

func TestModConfigurationRejectsWindowsJunctions(t *testing.T) {
	manager, store, root := newConfigurationTestManager(t)
	modRoot := filepath.Join(root, "server", "Mods", "Workshop", "mod_junction")
	writeConfigurationTestFile(t, filepath.Join(modRoot, "Config.json"), `{}`)
	if err := store.UpsertMod(t.Context(), db.Mod{ID: "mod_junction", Name: "Junction", PackageName: "Junction", Path: modRoot}); err != nil {
		t.Fatal(err)
	}
	files, err := manager.ListModConfigFiles(t.Context(), "mod_junction")
	if err != nil || len(files) != 1 {
		t.Fatalf("files = %#v, %v", files, err)
	}
	target, err := manager.resolveModFileTarget(t.Context(), "mod_junction", files[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	backupDir := manager.backupDirectory(target)
	outside := filepath.Join(root, "outside-backups")
	if err := os.MkdirAll(filepath.Dir(backupDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	createTestJunction(t, backupDir, outside)
	if _, err := manager.ListModConfigBackups(t.Context(), "mod_junction", files[0].ID); configurationErrorCode(err) != "configuration_backup_read_failed" {
		t.Fatalf("backup junction error = %v", err)
	}

	linkedDir := filepath.Join(modRoot, "linked")
	createTestJunction(t, linkedDir, outside)
	writeConfigurationTestFile(t, filepath.Join(outside, "outside.json"), `{"outside":true}`)
	listed, err := manager.ListModConfigFiles(t.Context(), "mod_junction")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Name != "Config.json" {
		t.Fatalf("junction file was exposed: %#v", listed)
	}
}

func createTestJunction(t *testing.T, junction, target string) {
	t.Helper()
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("cannot create test junction: %v: %s", err, output)
	}
	t.Cleanup(func() { _ = os.Remove(junction) })
}
