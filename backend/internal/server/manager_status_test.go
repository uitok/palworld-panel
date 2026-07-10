package server

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
)

func TestStatusReportsDockerErrorAsWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("missing executable status differs from the Linux Docker runtime")
	}
	root := t.TempDir()
	cfg := appconfig.Config{
		DataDir:         root,
		ServerDir:       filepath.Join(root, "server"),
		WinePrefixDir:   filepath.Join(root, "wineprefix"),
		ToolsDir:        filepath.Join(root, "tools"),
		SteamCMDDir:     filepath.Join(root, "tools", "steamcmd"),
		UploadsDir:      filepath.Join(root, "uploads"),
		BackupsDir:      filepath.Join(root, "backups"),
		LogsDir:         filepath.Join(root, "logs"),
		DBPath:          filepath.Join(root, "test.db"),
		DockerBinary:    filepath.Join(root, "missing-docker"),
		DockerImage:     "test-image",
		DockerContainer: "test-container",
		GamePort:        8211,
		QueryPort:       27015,
		RESTPort:        8212,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs returned error: %v", err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
	defer store.Close()

	manager := NewManager(cfg, store, docker.NewRunner(cfg))
	status, err := manager.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.Container.Status != "error" {
		t.Fatalf("expected container error status, got %#v", status.Container)
	}
	if len(status.Warnings) == 0 {
		t.Fatal("expected docker status warning")
	}
	joined := strings.Join(status.Warnings, "\n")
	if !strings.Contains(joined, "missing-docker") {
		t.Fatalf("expected warning to mention docker error, got %q", joined)
	}
}
