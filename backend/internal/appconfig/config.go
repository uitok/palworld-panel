package appconfig

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const DefaultDockerRunnerBaseImage = "scottyhardy/docker-wine:latest@sha256:477aae36af41923cfb5eefb23923b035f8010caa49eaded952316f937dd8a49b"
const DefaultPalDefenderRESTPort = 17993
const DefaultSteamAPIBaseURL = "https://api.steampowered.com"
const DefaultSteamAPITimeoutSeconds = 15
const DefaultAITranslationTimeoutSeconds = 90
const DefaultMonitorRetentionDays = 7

var DefaultDockerRunnerBaseImageMirrorPrefixes = []string{
	"docker.m.daocloud.io",
	"docker.1ms.run",
	"registry.cyou",
	"dockerproxy.net",
	"dockerproxy.link",
	"docker.jiaxin.site",
	"docker.xuanyuan.me",
	"free.hubfast.cn",
}

type Config struct {
	ListenAddr                   string
	DataDir                      string
	ServerDir                    string
	WinePrefixDir                string
	ToolsDir                     string
	SteamCMDDir                  string
	UploadsDir                   string
	BackupsDir                   string
	LogsDir                      string
	DBPath                       string
	RequireAuth                  bool
	CORSOrigins                  []string
	FrontendDist                 string
	MaxUploadBytes               int64
	DockerBinary                 string
	DockerImage                  string
	DockerContainer              string
	DockerRunnerBaseImage        string
	DockerRunnerBaseImageMirrors []string
	SteamWebAPIKey               string
	SteamWebAPIKeySource         string
	SteamAPIBaseURL              string
	SteamAPITimeoutSeconds       int
	WorkshopAppID                string
	GamePort                     int
	QueryPort                    int
	RESTPort                     int
	PalworldRESTBaseURL          string
	PalworldRESTUser             string
	PalworldRESTPass             string
	PalworldRESTReadTimeoutMS    int
	PalworldGameDataTimeoutMS    int
	PalworldGameDataMaxBytes     int64
	PalDefenderRESTBaseURL       string
	PalDefenderRESTPort          int
	SaveIndexerEnabled           bool
	SaveIndexerURL               string
	SaveIndexCacheDir            string
	SaveIndexTimeoutSeconds      int
	PerfSlowRequestMS            int
	MonitorRetentionDays         int
	AITranslationTimeoutSeconds  int
	LogLevel                     string
	RunnerDir                    string
}

func Load() (Config, error) {
	if err := validateScalarEnvironment(); err != nil {
		return Config{}, err
	}
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

	steamWebAPIKey, steamWebAPIKeySource := resolveSteamWebAPIKey()
	palDefenderRESTPort := envInt("PALPANEL_PALDEFENDER_REST_PORT", DefaultPalDefenderRESTPort)
	cfg := Config{
		ListenAddr:                   env("PALPANEL_LISTEN_ADDR", "127.0.0.1:8080"),
		DataDir:                      dataDir,
		ServerDir:                    env("PALPANEL_SERVER_DIR", filepath.Join(dataDir, "server")),
		WinePrefixDir:                env("PALPANEL_WINE_PREFIX_DIR", filepath.Join(dataDir, "wineprefix")),
		ToolsDir:                     env("PALPANEL_TOOLS_DIR", filepath.Join(dataDir, "tools")),
		SteamCMDDir:                  env("PALPANEL_STEAMCMD_DIR", filepath.Join(dataDir, "tools", "steamcmd")),
		UploadsDir:                   env("PALPANEL_UPLOADS_DIR", filepath.Join(dataDir, "uploads")),
		BackupsDir:                   env("PALPANEL_BACKUPS_DIR", filepath.Join(dataDir, "backups")),
		LogsDir:                      env("PALPANEL_LOGS_DIR", filepath.Join(dataDir, "logs")),
		DBPath:                       env("PALPANEL_DB_PATH", filepath.Join(dataDir, "palpanel.db")),
		RequireAuth:                  envBool("PALPANEL_REQUIRE_AUTH", true),
		CORSOrigins:                  envList("PALPANEL_CORS_ORIGINS", []string{"http://127.0.0.1:3000", "http://localhost:3000"}),
		FrontendDist:                 env("PALPANEL_FRONTEND_DIST", filepath.Join(root, "frontend", "dist")),
		MaxUploadBytes:               int64(envInt("PALPANEL_MAX_UPLOAD_MB", 256)) * 1024 * 1024,
		DockerBinary:                 env("PALPANEL_DOCKER_BIN", "docker"),
		DockerImage:                  env("PALPANEL_DOCKER_IMAGE", "palworld-wine-runner:local"),
		DockerContainer:              env("PALPANEL_DOCKER_CONTAINER", "palworld-wine-server"),
		DockerRunnerBaseImage:        env("PALPANEL_DOCKER_RUNNER_BASE_IMAGE", DefaultDockerRunnerBaseImage),
		DockerRunnerBaseImageMirrors: envList("PALPANEL_DOCKER_RUNNER_BASE_IMAGE_MIRRORS", DefaultDockerRunnerBaseImageMirrorPrefixes),
		SteamWebAPIKey:               steamWebAPIKey,
		SteamWebAPIKeySource:         steamWebAPIKeySource,
		SteamAPIBaseURL:              strings.TrimRight(env("PALPANEL_STEAM_API_BASE_URL", DefaultSteamAPIBaseURL), "/"),
		SteamAPITimeoutSeconds:       envInt("PALPANEL_STEAM_API_TIMEOUT_SECONDS", DefaultSteamAPITimeoutSeconds),
		WorkshopAppID:                env("PALPANEL_WORKSHOP_APP_ID", "1623730"),
		GamePort:                     envInt("PALPANEL_GAME_PORT", 8211),
		QueryPort:                    envInt("PALPANEL_QUERY_PORT", 27015),
		RESTPort:                     envInt("PALPANEL_REST_PORT", 8212),
		PalworldRESTBaseURL:          env("PALWORLD_REST_BASE_URL", "http://127.0.0.1:8212/v1/api"),
		PalworldRESTUser:             env("PALWORLD_REST_USER", "admin"),
		PalworldRESTPass:             env("PALWORLD_ADMIN_PASSWORD", ""),
		PalworldRESTReadTimeoutMS:    envInt("PALPANEL_PALWORLD_REST_READ_TIMEOUT_MS", 1200),
		PalworldGameDataTimeoutMS:    envInt("PALPANEL_GAME_DATA_TIMEOUT_MS", 3000),
		PalworldGameDataMaxBytes:     int64(envInt("PALPANEL_GAME_DATA_MAX_MB", 16)) * 1024 * 1024,
		PalDefenderRESTBaseURL:       env("PALPANEL_PALDEFENDER_REST_BASE_URL", fmt.Sprintf("http://127.0.0.1:%d", palDefenderRESTPort)),
		PalDefenderRESTPort:          palDefenderRESTPort,
		SaveIndexerEnabled:           envBool("PALPANEL_SAVE_INDEXER_ENABLED", false),
		SaveIndexerURL:               env("PALPANEL_SAVE_INDEXER_URL", "http://127.0.0.1:8090"),
		SaveIndexCacheDir:            env("PALPANEL_SAVE_INDEX_CACHE_DIR", filepath.Join(dataDir, "save-index")),
		SaveIndexTimeoutSeconds:      envInt("PALPANEL_SAVE_INDEX_TIMEOUT_SECONDS", 120),
		PerfSlowRequestMS:            envInt("PALPANEL_PERF_SLOW_REQUEST_MS", 500),
		MonitorRetentionDays:         envInt("PALPANEL_MONITOR_RETENTION_DAYS", DefaultMonitorRetentionDays),
		AITranslationTimeoutSeconds:  envInt("PALPANEL_AI_TRANSLATION_TIMEOUT_SECONDS", DefaultAITranslationTimeoutSeconds),
		LogLevel:                     strings.ToLower(env("PALPANEL_LOG_LEVEL", "info")),
		RunnerDir:                    env("PALPANEL_RUNNER_DIR", filepath.Join(backendDir, "deployments", "wine-runner")),
	}
	if err := validateListenAddress(cfg.ListenAddr); err != nil {
		return Config{}, err
	}
	if err := validateHTTPBaseURL("PALPANEL_STEAM_API_BASE_URL", cfg.SteamAPIBaseURL); err != nil {
		return Config{}, err
	}
	if cfg.SteamAPITimeoutSeconds < 1 || cfg.SteamAPITimeoutSeconds > 300 {
		return Config{}, fmt.Errorf("PALPANEL_STEAM_API_TIMEOUT_SECONDS must be between 1 and 300")
	}
	if cfg.AITranslationTimeoutSeconds < 1 || cfg.AITranslationTimeoutSeconds > 600 {
		return Config{}, fmt.Errorf("PALPANEL_AI_TRANSLATION_TIMEOUT_SECONDS must be between 1 and 600")
	}
	if cfg.MonitorRetentionDays < 0 || cfg.MonitorRetentionDays > 3650 {
		return Config{}, fmt.Errorf("PALPANEL_MONITOR_RETENTION_DAYS must be between 0 and 3650")
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("PALPANEL_LOG_LEVEL must be one of debug, info, warn, or error")
	}

	return cfg, nil
}

func (c Config) EnsureDirs() error {
	dirs := []string{c.DataDir, c.ServerDir, c.WinePrefixDir, c.ToolsDir, c.SteamCMDDir, c.UploadsDir, c.BackupsDir, c.LogsDir, c.SaveIndexCacheDir}
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
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

func (c Config) EffectivePalDefenderRESTPort() int {
	if c.PalDefenderRESTPort > 0 {
		return c.PalDefenderRESTPort
	}
	return DefaultPalDefenderRESTPort
}

func (c Config) EffectivePalDefenderRESTBaseURL() string {
	if baseURL := strings.TrimSpace(c.PalDefenderRESTBaseURL); baseURL != "" {
		return strings.TrimRight(baseURL, "/")
	}
	return fmt.Sprintf("http://127.0.0.1:%d", c.EffectivePalDefenderRESTPort())
}

func (c Config) SteamCMDBinaryPath() string {
	return filepath.Join(c.SteamCMDDir, "steamcmd.exe")
}

func (c Config) ServerLogPath() string {
	return filepath.Join(c.LogsDir, "palserver.log")
}

func (c Config) AITranslationKeyPath() string {
	return filepath.Join(c.DataDir, "secrets", "ai-translation.key")
}

func (c Config) EffectiveSteamWebAPIKey() string {
	if key := strings.TrimSpace(c.SteamWebAPIKey); key != "" {
		return key
	}
	return bundledSteamWebAPIKey()
}

func (c Config) SteamWebAPIKeyConfigured() bool {
	return strings.TrimSpace(c.EffectiveSteamWebAPIKey()) != ""
}

func (c Config) SteamWebAPIKeySourceName() string {
	if strings.TrimSpace(c.EffectiveSteamWebAPIKey()) == "" {
		return ""
	}
	if source := strings.TrimSpace(c.SteamWebAPIKeySource); source != "" {
		return source
	}
	if strings.TrimSpace(c.SteamWebAPIKey) == "" {
		return "bundled"
	}
	return "environment"
}

func resolveSteamWebAPIKey() (string, string) {
	if key := strings.TrimSpace(os.Getenv("STEAM_WEB_API_KEY")); key != "" {
		return key, "environment"
	}
	return bundledSteamWebAPIKey(), "bundled"
}

func validateHTTPBaseURL(name, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%s must be an absolute HTTP(S) URL without credentials, query, or fragment", name)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme != "http" {
		return fmt.Errorf("%s must use HTTPS, except for loopback HTTP endpoints", name)
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("%s must use HTTPS, except for loopback HTTP endpoints", name)
	}
	return nil
}

func validateScalarEnvironment() error {
	for _, name := range []string{"PALPANEL_REQUIRE_AUTH", "PALPANEL_SAVE_INDEXER_ENABLED"} {
		raw := strings.TrimSpace(os.Getenv(name))
		if raw == "" {
			continue
		}
		if _, err := strconv.ParseBool(raw); err != nil {
			return fmt.Errorf("%s must be true or false", name)
		}
	}
	for _, name := range []string{
		"PALPANEL_PALDEFENDER_REST_PORT",
		"PALPANEL_MAX_UPLOAD_MB",
		"PALPANEL_STEAM_API_TIMEOUT_SECONDS",
		"PALPANEL_GAME_PORT",
		"PALPANEL_QUERY_PORT",
		"PALPANEL_REST_PORT",
		"PALPANEL_PALWORLD_REST_READ_TIMEOUT_MS",
		"PALPANEL_GAME_DATA_TIMEOUT_MS",
		"PALPANEL_GAME_DATA_MAX_MB",
		"PALPANEL_SAVE_INDEX_TIMEOUT_SECONDS",
		"PALPANEL_PERF_SLOW_REQUEST_MS",
		"PALPANEL_MONITOR_RETENTION_DAYS",
		"PALPANEL_AI_TRANSLATION_TIMEOUT_SECONDS",
	} {
		raw := strings.TrimSpace(os.Getenv(name))
		if raw == "" {
			continue
		}
		if _, err := strconv.Atoi(raw); err != nil {
			return fmt.Errorf("%s must be an integer", name)
		}
	}
	return nil
}

func validateListenAddress(address string) error {
	_, port, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return fmt.Errorf("PALPANEL_LISTEN_ADDR must be a host:port address: %w", err)
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return fmt.Errorf("PALPANEL_LISTEN_ADDR port must be between 1 and 65535")
	}
	return nil
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

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envList(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	if raw == "*" {
		return []string{"*"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
