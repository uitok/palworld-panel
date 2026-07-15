package server

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/palconfig"
)

func TestInitializeConfigEnablesRESTForPanelOverview(t *testing.T) {
	root := t.TempDir()
	cfg := appconfig.Config{
		DataDir:             root,
		ServerDir:           filepath.Join(root, "server"),
		WinePrefixDir:       filepath.Join(root, "wineprefix"),
		ToolsDir:            filepath.Join(root, "tools"),
		SteamCMDDir:         filepath.Join(root, "tools", "steamcmd"),
		UploadsDir:          filepath.Join(root, "uploads"),
		BackupsDir:          filepath.Join(root, "backups"),
		LogsDir:             filepath.Join(root, "logs"),
		DBPath:              filepath.Join(root, "test.db"),
		DockerBinary:        "docker",
		DockerImage:         "test-image",
		DockerContainer:     "test-container",
		GamePort:            8211,
		QueryPort:           27015,
		RCONPort:            35575,
		RESTPort:            63108,
		PalworldRESTBaseURL: "http://127.0.0.1:63108/v1/api",
		PalworldRESTUser:    "admin",
		PalworldRESTPass:    "secret-admin-password",
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs returned error: %v", err)
	}
	if err := palconfig.Write(cfg.DefaultPalWorldSettingsPath(), palconfig.Settings{
		"RESTAPIEnabled": "False",
		"RESTAPIPort":    "8212",
		"RCONEnabled":    "False",
		"RCONPort":       "25575",
		"AdminPassword":  "",
		"ServerName":     "Default Palworld Server",
	}); err != nil {
		t.Fatalf("writing default settings returned error: %v", err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
	defer store.Close()

	manager := NewManager(cfg, store, docker.NewRunner(cfg))
	if err := manager.InitializeConfig(context.Background()); err != nil {
		t.Fatalf("InitializeConfig returned error: %v", err)
	}
	settings, err := palconfig.Read(cfg.PalWorldSettingsPath())
	if err != nil {
		t.Fatalf("reading initialized config returned error: %v", err)
	}
	if settings["RESTAPIEnabled"] != "True" {
		t.Fatalf("RESTAPIEnabled = %q", settings["RESTAPIEnabled"])
	}
	if settings["RESTAPIPort"] != strconv.Itoa(cfg.RESTPort) {
		t.Fatalf("RESTAPIPort = %q", settings["RESTAPIPort"])
	}
	if settings["AdminPassword"] != "secret-admin-password" {
		t.Fatalf("AdminPassword was not copied from config")
	}
	if settings["RCONEnabled"] != "True" || settings["RCONPort"] != strconv.Itoa(cfg.RCONPort) {
		t.Fatalf("RCON settings = enabled %q, port %q", settings["RCONEnabled"], settings["RCONPort"])
	}
}
