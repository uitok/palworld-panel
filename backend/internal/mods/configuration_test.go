package mods

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
)

func TestModConfigurationFilesWriteBackupRestoreAndRevision(t *testing.T) {
	manager, store, root := newConfigurationTestManager(t)
	modRoot := filepath.Join(root, "server", "Mods", "Workshop", "mod_one")
	writeConfigurationTestFile(t, filepath.Join(modRoot, "Config.json"), `{"Enabled":true,"Limit":10}`)
	writeConfigurationTestFile(t, filepath.Join(modRoot, "Info.json"), `{}`)
	writeConfigurationTestFile(t, filepath.Join(modRoot, "payload.dll"), "binary")
	if err := store.UpsertMod(t.Context(), db.Mod{ID: "mod_one", Name: "One", PackageName: "One", Path: modRoot}); err != nil {
		t.Fatal(err)
	}

	files, err := manager.ListModConfigFiles(t.Context(), "mod_one")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "Config.json" || len(files[0].ID) != 32 {
		t.Fatalf("unexpected editable files: %#v", files)
	}
	document, err := manager.GetModConfigFile(t.Context(), "mod_one", files[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Fields) != 2 {
		t.Fatalf("expected JSON fields, got %#v", document.Fields)
	}
	written, err := manager.WriteModConfigFile(t.Context(), "mod_one", files[0].ID, ConfigWriteRequest{Content: `{"Enabled":false,"Limit":20}`, Revision: document.File.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if written.File.Revision == document.File.Revision {
		t.Fatal("revision did not change")
	}
	if _, err := manager.WriteModConfigFile(t.Context(), "mod_one", files[0].ID, ConfigWriteRequest{Content: `{}`, Revision: document.File.Revision}); configurationErrorCode(err) != "configuration_revision_conflict" {
		t.Fatalf("stale write error = %v", err)
	}
	backups, err := manager.ListModConfigBackups(t.Context(), "mod_one", files[0].ID)
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %#v, %v", backups, err)
	}
	restored, err := manager.RestoreModConfigBackup(t.Context(), "mod_one", files[0].ID, backups[0].ID, written.File.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Content != document.Content {
		t.Fatalf("restored content = %q, want %q", restored.Content, document.Content)
	}
}

func TestModConfigurationRejectsLuaWithoutConfirmationAndInvalidText(t *testing.T) {
	manager, store, root := newConfigurationTestManager(t)
	modRoot := filepath.Join(root, "server", "Mods", "Workshop", "mod_lua")
	writeConfigurationTestFile(t, filepath.Join(modRoot, "Scripts", "main.lua"), "Range = 100\n")
	writeConfigurationTestFile(t, filepath.Join(modRoot, "bad.txt"), string([]byte{'a', 0, 'b'}))
	if err := store.UpsertMod(t.Context(), db.Mod{ID: "mod_lua", Name: "Lua", PackageName: "Lua", Path: modRoot}); err != nil {
		t.Fatal(err)
	}
	files, err := manager.ListModConfigFiles(t.Context(), "mod_lua")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || !files[0].Executable {
		t.Fatalf("files = %#v", files)
	}
	document, err := manager.GetModConfigFile(t.Context(), "mod_lua", files[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.WriteModConfigFile(t.Context(), "mod_lua", files[0].ID, ConfigWriteRequest{Content: "Range = 200\n", Revision: document.File.Revision})
	if configurationErrorCode(err) != "executable_confirmation_required" {
		t.Fatalf("Lua write error = %v", err)
	}
	_, err = manager.WriteModConfigFile(t.Context(), "mod_lua", files[0].ID, ConfigWriteRequest{Content: "Range = 200\n", Revision: document.File.Revision, ConfirmExecutable: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestModConfigurationRejectsOversizedInvalidJSONAndLinks(t *testing.T) {
	manager, store, root := newConfigurationTestManager(t)
	modRoot := filepath.Join(root, "server", "Mods", "Workshop", "mod_safe")
	writeConfigurationTestFile(t, filepath.Join(modRoot, "Config.json"), `{}`)
	writeConfigurationTestFile(t, filepath.Join(modRoot, "large.ini"), strings.Repeat("x", int(MaxConfigFileBytes)+1))
	outside := filepath.Join(root, "outside.json")
	writeConfigurationTestFile(t, outside, `{"outside":true}`)
	if runtime.GOOS != "windows" {
		if err := os.Symlink(outside, filepath.Join(modRoot, "linked.json")); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.UpsertMod(t.Context(), db.Mod{ID: "mod_safe", Name: "Safe", PackageName: "Safe", Path: modRoot}); err != nil {
		t.Fatal(err)
	}
	files, err := manager.ListModConfigFiles(t.Context(), "mod_safe")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "Config.json" {
		t.Fatalf("unsafe files were exposed: %#v", files)
	}
	document, err := manager.GetModConfigFile(t.Context(), "mod_safe", files[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.WriteModConfigFile(t.Context(), "mod_safe", files[0].ID, ConfigWriteRequest{Content: `{`, Revision: document.File.Revision})
	if configurationErrorCode(err) != "configuration_parse_failed" {
		t.Fatalf("invalid JSON error = %v", err)
	}
}

func TestDedicatedConfigurationAdapters(t *testing.T) {
	manager, store, root := newConfigurationTestManager(t)
	writeConfigurationTestFile(t, filepath.Join(manager.cfg.PalDefenderDir(), "Config.json"), `{"BanListEnabled":true}`)
	writeConfigurationTestFile(t, filepath.Join(manager.cfg.Win64Dir(), "UE4SS-settings.ini"), "[General]\nbUseUObjectArrayCache=true\n")
	writeConfigurationTestFile(t, filepath.Join(manager.cfg.Win64Dir(), "Mods", "mods.txt"), "PalSchema : 1\n")
	palschemaRoot := filepath.Join(root, "server", "Mods", "Workshop", "palschema")
	extendedRoot := filepath.Join(root, "server", "Mods", "Workshop", "extended")
	writeConfigurationTestFile(t, filepath.Join(palschemaRoot, "settings.json"), `{"Enabled":true}`)
	writeConfigurationTestFile(t, filepath.Join(extendedRoot, "Scripts", "main.lua"), "BaseRange = 1200\n")
	for _, record := range []db.Mod{
		{ID: "palschema", Name: "PalSchema", PackageName: "PalSchema", WorkshopID: "3625280368", Path: palschemaRoot},
		{ID: "extended", Name: "Extended Base Range", PackageName: "ExtendedBaseRange", WorkshopID: "3625907101", Path: extendedRoot},
	} {
		if err := store.UpsertMod(context.Background(), record); err != nil {
			t.Fatal(err)
		}
	}
	adapters, err := manager.ListConfigurations(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(adapters) != 4 {
		t.Fatalf("adapters = %#v", adapters)
	}
	for _, adapter := range adapters {
		if !adapter.Available || len(adapter.Files) == 0 {
			t.Fatalf("adapter unavailable: %#v", adapter)
		}
	}
	extended := adapters[3]
	document, err := manager.GetConfiguration(t.Context(), extended.ID, extended.Files[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Fields) != 1 || document.Fields[0].Path != "BaseRange" {
		t.Fatalf("extended fields = %#v", document.Fields)
	}
}

func newConfigurationTestManager(t *testing.T) (Manager, *db.Store, string) {
	t.Helper()
	root := t.TempDir()
	cfg := appconfig.Config{
		RuntimeRoot: root, DataDir: filepath.Join(root, "data"), ServerDir: filepath.Join(root, "server"),
		UploadsDir: filepath.Join(root, "uploads"), BackupsDir: filepath.Join(root, "backups"),
		UE4SSDir: filepath.Join(root, "ue4ss"), DBPath: filepath.Join(root, "test.db"),
	}
	for _, path := range []string{cfg.DataDir, cfg.ServerDir, cfg.UploadsDir, cfg.BackupsDir, cfg.UE4SSDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewManager(cfg, store, docker.NewRunner(cfg)), store, root
}

func writeConfigurationTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func configurationErrorCode(err error) string {
	var target ConfigurationError
	if errors.As(err, &target) {
		return target.Code
	}
	return ""
}
