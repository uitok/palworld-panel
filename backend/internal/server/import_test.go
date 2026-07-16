package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
)

func TestImportServerDirectoryBindsSteamLibraryInPlace(t *testing.T) {
	base := t.TempDir()
	runtimeRoot := filepath.Join(base, "runtime")
	cfg := (appconfig.Config{
		RuntimeRoot: runtimeRoot,
		DataDir:     filepath.Join(runtimeRoot, "data"),
		ServerDir:   filepath.Join(runtimeRoot, "palworld"),
		ToolsDir:    filepath.Join(runtimeRoot, "temp"),
		SteamCMDDir: filepath.Join(runtimeRoot, "steamcmd"),
		UploadsDir:  filepath.Join(runtimeRoot, "mods", "staging"),
		BackupsDir:  filepath.Join(runtimeRoot, "data", "backups"),
		LogsDir:     filepath.Join(runtimeRoot, "data", "logs"),
		DBPath:      filepath.Join(runtimeRoot, "data", "palpanel.db"),
		GamePort:    8211,
		QueryPort:   27015,
		RESTPort:    8212,
	}).WithServerDirectoryState()
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	library := filepath.Join(base, "SteamLibrary")
	serverRoot := filepath.Join(library, "steamapps", "common", "PalServer")
	writeFile(t, filepath.Join(serverRoot, "PalServer.exe"), "MZ-palserver")
	writeFile(t, filepath.Join(serverRoot, "Pal", "Binaries", "Win64", "PalServer-Win64-Shipping-Cmd.exe"), "MZ-command")
	writeFile(t, filepath.Join(library, "steamapps", "appmanifest_2394010.acf"), `"AppState"\n{\n  "buildid" "24681012"\n}`)
	writeFile(t, filepath.Join(serverRoot, "Pal", "Saved", "Config", "WindowsServer", "PalWorldSettings.ini"), "settings")
	writeFile(t, filepath.Join(serverRoot, "keep-me.txt"), "original")
	expectedServerRoot, err := filepath.EvalSymlinks(serverRoot)
	if err != nil {
		t.Fatal(err)
	}

	manager := NewManager(cfg, store, docker.NewRunner(cfg))
	manager.goos = "windows"
	result, err := manager.ImportServerDirectory(context.Background(), library)
	if err != nil {
		t.Fatalf("ImportServerDirectory returned error: %v", err)
	}
	if !sameServerDirectory(result.Path, expectedServerRoot, "windows") || result.BuildID != "24681012" || !result.ConfigExists {
		t.Fatalf("unexpected import result: %#v", result)
	}
	if got := manager.cfg.ServerDirectory(); !sameServerDirectory(got, expectedServerRoot, "windows") || !manager.cfg.ServerDirectoryImported() {
		t.Fatalf("server directory state = %q imported=%v", got, manager.cfg.ServerDirectoryImported())
	}
	if body, err := os.ReadFile(filepath.Join(serverRoot, "keep-me.txt")); err != nil || string(body) != "original" {
		t.Fatalf("source directory was changed: %q, %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(runtimeRoot, "palworld", "PalServer.exe")); !os.IsNotExist(err) {
		t.Fatalf("server files were copied into the managed directory: %v", err)
	}
	if err := manager.cfg.ValidateManagedPath(filepath.Join(manager.cfg.ServerDirectory(), "Mods", "Workshop", "123"), false); err != nil {
		t.Fatalf("imported server content is not writable: %v", err)
	}

	status, err := manager.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || !status.ServerImported || !sameServerDirectory(status.Paths["server"], expectedServerRoot, "windows") {
		t.Fatalf("unexpected status after import: %#v", status)
	}

	restored := (appconfig.Config{
		RuntimeRoot: runtimeRoot,
		DataDir:     filepath.Join(runtimeRoot, "data"),
		ServerDir:   filepath.Join(runtimeRoot, "palworld"),
	}).WithServerDirectoryState()
	if err := RestoreImportedServerDirectory(context.Background(), restored, store); err != nil {
		t.Fatalf("RestoreImportedServerDirectory returned error: %v", err)
	}
	if !sameServerDirectory(restored.ServerDirectory(), expectedServerRoot, "windows") || !restored.ServerDirectoryImported() {
		t.Fatalf("restored state = %q imported=%v", restored.ServerDirectory(), restored.ServerDirectoryImported())
	}
}

func TestImportServerDirectoryRejectsIncompleteInstall(t *testing.T) {
	root := t.TempDir()
	cfg := (appconfig.Config{
		RuntimeRoot: filepath.Join(root, "runtime"),
		DataDir:     filepath.Join(root, "runtime", "data"),
		ServerDir:   filepath.Join(root, "runtime", "palworld"),
		DBPath:      filepath.Join(root, "runtime", "data", "palpanel.db"),
	}).WithServerDirectoryState()
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := NewManager(cfg, store, docker.NewRunner(cfg))
	manager.goos = "windows"
	if _, err := manager.ImportServerDirectory(context.Background(), filepath.Join(root, "missing")); err == nil {
		t.Fatal("incomplete server directory was accepted")
	}
}
