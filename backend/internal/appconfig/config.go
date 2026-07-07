package appconfig

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	ListenAddr          string
	DataDir             string
	ServerDir           string
	WinePrefixDir       string
	ToolsDir            string
	SteamCMDDir         string
	UploadsDir          string
	BackupsDir          string
	LogsDir             string
	DBPath              string
	PanelToken          string
	DockerBinary        string
	DockerImage         string
	DockerContainer     string
	GamePort            int
	QueryPort           int
	RESTPort            int
	PalworldRESTBaseURL string
	PalworldRESTUser    string
	PalworldRESTPass    string
	RunnerDir           string
}

func Load() (Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}

	root := cwd
	if filepath.Base(cwd) == "backend" {
		root = filepath.Dir(cwd)
	}

	dataDir := env("PALPANEL_DATA_DIR", filepath.Join(root, "data"))
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return Config{}, err
	}

	backendDir := env("PALPANEL_BACKEND_DIR", filepath.Join(root, "backend"))
	backendDir, err = filepath.Abs(backendDir)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ListenAddr:          env("PALPANEL_LISTEN_ADDR", ":8080"),
		DataDir:             dataDir,
		ServerDir:           env("PALPANEL_SERVER_DIR", filepath.Join(dataDir, "server")),
		WinePrefixDir:       env("PALPANEL_WINE_PREFIX_DIR", filepath.Join(dataDir, "wineprefix")),
		ToolsDir:            env("PALPANEL_TOOLS_DIR", filepath.Join(dataDir, "tools")),
		SteamCMDDir:         env("PALPANEL_STEAMCMD_DIR", filepath.Join(dataDir, "tools", "steamcmd")),
		UploadsDir:          env("PALPANEL_UPLOADS_DIR", filepath.Join(dataDir, "uploads")),
		BackupsDir:          env("PALPANEL_BACKUPS_DIR", filepath.Join(dataDir, "backups")),
		LogsDir:             env("PALPANEL_LOGS_DIR", filepath.Join(dataDir, "logs")),
		DBPath:              env("PALPANEL_DB_PATH", filepath.Join(dataDir, "palpanel.db")),
		PanelToken:          env("PANEL_TOKEN", "change-me"),
		DockerBinary:        env("PALPANEL_DOCKER_BIN", "docker"),
		DockerImage:         env("PALPANEL_DOCKER_IMAGE", "palworld-wine-runner:local"),
		DockerContainer:     env("PALPANEL_DOCKER_CONTAINER", "palworld-wine-server"),
		GamePort:            envInt("PALPANEL_GAME_PORT", 8211),
		QueryPort:           envInt("PALPANEL_QUERY_PORT", 27015),
		RESTPort:            envInt("PALPANEL_REST_PORT", 8212),
		PalworldRESTBaseURL: env("PALWORLD_REST_BASE_URL", "http://127.0.0.1:8212/v1/api"),
		PalworldRESTUser:    env("PALWORLD_REST_USER", "admin"),
		PalworldRESTPass:    env("PALWORLD_ADMIN_PASSWORD", ""),
		RunnerDir:           env("PALPANEL_RUNNER_DIR", filepath.Join(backendDir, "deployments", "wine-runner")),
	}

	return cfg, nil
}

func (c Config) EnsureDirs() error {
	dirs := []string{c.DataDir, c.ServerDir, c.WinePrefixDir, c.ToolsDir, c.SteamCMDDir, c.UploadsDir, c.BackupsDir, c.LogsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (c Config) PalServerExePath() string {
	return filepath.Join(c.ServerDir, "PalServer.exe")
}

func (c Config) DefaultPalWorldSettingsPath() string {
	return filepath.Join(c.ServerDir, "DefaultPalWorldSettings.ini")
}

func (c Config) PalWorldSettingsPath() string {
	return filepath.Join(c.ServerDir, "Pal", "Saved", "Config", "WindowsServer", "PalWorldSettings.ini")
}

func (c Config) ModsDir() string {
	return filepath.Join(c.ServerDir, "Mods")
}

func (c Config) WorkshopModsDir() string {
	return filepath.Join(c.ModsDir(), "Workshop")
}

func (c Config) PalModSettingsPath() string {
	return filepath.Join(c.ModsDir(), "PalModSettings.ini")
}

func (c Config) LegacyModsDir() string {
	return filepath.Join(c.ServerDir, "Pal", "Content", "Paks", "LogicMods", "Mods")
}

func (c Config) Win64Dir() string {
	return filepath.Join(c.ServerDir, "Pal", "Binaries", "Win64")
}

func (c Config) PalDefenderDir() string {
	return filepath.Join(c.Win64Dir(), "PalDefender")
}

func (c Config) SteamCMDBinaryPath() string {
	return filepath.Join(c.SteamCMDDir, "steamcmd.exe")
}

func (c Config) ServerLogPath() string {
	return filepath.Join(c.LogsDir, "palserver.log")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}
