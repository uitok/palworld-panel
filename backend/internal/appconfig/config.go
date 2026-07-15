package appconfig

import (
	"encoding/hex"
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
const DefaultSteamCMDDownloadURL = "https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip"
const DefaultSteamCMDDownloadMaxMB = 64
const DefaultUE4SSVersion = "v3.0.1"
const DefaultUE4SSDownloadURL = "https://github.com/UE4SS-RE/RE-UE4SS/releases/download/v3.0.1/UE4SS_v3.0.1.zip"
const DefaultUE4SSArchiveSHA256 = "4b47d4bceddd2f561a4e395bfa00924ccfc945af576a2d0c613e6537846c57ec"
const DefaultUE4SSDownloadMaxMB = 64
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
	RuntimeRoot                  string
	RepositoryRoot               string
	DevelopmentMode              bool
	ListenAddr                   string
	DataDir                      string
	ServerDir                    string
	WinePrefixDir                string
	ToolsDir                     string
	SteamCMDDir                  string
	UE4SSDir                     string
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
	SteamCMDDownloadURL          string
	SteamCMDDownloadMaxBytes     int64
	UE4SSVersion                 string
	UE4SSDownloadURL             string
	UE4SSArchiveSHA256           string
	UE4SSDownloadMaxBytes        int64
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
	layout, err := ResolveRuntimeLayout(os.Getenv("PALPANEL_RUNTIME_ROOT"))
	if err != nil {
		return Config{}, err
	}
	root := layout.ApplicationRoot
	if layout.RepositoryRoot != "" {
		root = layout.RepositoryRoot
	}
	mutableBase := root
	dataDefault := filepath.Join(root, "data")
	serverDefault := ""
	toolsDefault := ""
	steamCMDDefault := ""
	ue4ssDefault := ""
	uploadsDefault := ""
	backupsDefault := ""
	logsDefault := ""
	dbDefault := ""
	saveIndexDefault := ""
	winePrefixDefault := ""
	if layout.Structured {
		mutableBase = layout.RuntimeRoot
		dataDefault = filepath.Join(layout.RuntimeRoot, "data")
		serverDefault = filepath.Join(layout.RuntimeRoot, "palworld")
		toolsDefault = filepath.Join(layout.RuntimeRoot, "temp")
		steamCMDDefault = filepath.Join(layout.RuntimeRoot, "steamcmd")
		ue4ssDefault = filepath.Join(layout.RuntimeRoot, "ue4ss")
		uploadsDefault = filepath.Join(layout.RuntimeRoot, "mods", "staging")
		backupsDefault = filepath.Join(dataDefault, "backups")
		logsDefault = filepath.Join(dataDefault, "logs")
		dbDefault = filepath.Join(dataDefault, "database", "palpanel.db")
		saveIndexDefault = filepath.Join(dataDefault, "save-index")
		winePrefixDefault = filepath.Join(layout.RuntimeRoot, "wineprefix")
	}
	dataDir, err := configuredPath("PALPANEL_DATA_DIR", dataDefault, mutableBase)
	if err != nil {
		return Config{}, err
	}
	if serverDefault == "" {
		serverDefault = filepath.Join(dataDir, "server")
		toolsDefault = filepath.Join(dataDir, "tools")
		steamCMDDefault = filepath.Join(dataDir, "tools", "steamcmd")
		ue4ssDefault = filepath.Join(dataDir, "tools", "ue4ss")
		uploadsDefault = filepath.Join(dataDir, "uploads")
		backupsDefault = filepath.Join(dataDir, "backups")
		logsDefault = filepath.Join(dataDir, "logs")
		dbDefault = filepath.Join(dataDir, "palpanel.db")
		saveIndexDefault = filepath.Join(dataDir, "save-index")
		winePrefixDefault = filepath.Join(dataDir, "wineprefix")
	}
	serverDir, err := configuredPath("PALPANEL_SERVER_DIR", serverDefault, mutableBase)
	if err != nil {
		return Config{}, err
	}
	winePrefixDir, err := configuredPath("PALPANEL_WINE_PREFIX_DIR", winePrefixDefault, mutableBase)
	if err != nil {
		return Config{}, err
	}
	toolsDir, err := configuredPath("PALPANEL_TOOLS_DIR", toolsDefault, mutableBase)
	if err != nil {
		return Config{}, err
	}
	steamCMDDir, err := configuredPath("PALPANEL_STEAMCMD_DIR", steamCMDDefault, mutableBase)
	if err != nil {
		return Config{}, err
	}
	ue4ssDir, err := configuredPath("PALPANEL_UE4SS_DIR", ue4ssDefault, mutableBase)
	if err != nil {
		return Config{}, err
	}
	uploadsDir, err := configuredPath("PALPANEL_UPLOADS_DIR", uploadsDefault, mutableBase)
	if err != nil {
		return Config{}, err
	}
	backupsDir, err := configuredPath("PALPANEL_BACKUPS_DIR", backupsDefault, mutableBase)
	if err != nil {
		return Config{}, err
	}
	logsDir, err := configuredPath("PALPANEL_LOGS_DIR", logsDefault, mutableBase)
	if err != nil {
		return Config{}, err
	}
	dbPath, err := configuredPath("PALPANEL_DB_PATH", dbDefault, mutableBase)
	if err != nil {
		return Config{}, err
	}
	saveIndexCacheDir, err := configuredPath("PALPANEL_SAVE_INDEX_CACHE_DIR", saveIndexDefault, mutableBase)
	if err != nil {
		return Config{}, err
	}
	backendDir, err := configuredPath("PALPANEL_BACKEND_DIR", filepath.Join(root, "backend"), root)
	if err != nil {
		return Config{}, err
	}
	frontendDist, err := configuredPath("PALPANEL_FRONTEND_DIST", filepath.Join(root, "frontend", "dist"), root)
	if err != nil {
		return Config{}, err
	}
	runnerDir, err := configuredPath("PALPANEL_RUNNER_DIR", filepath.Join(backendDir, "deployments", "wine-runner"), root)
	if err != nil {
		return Config{}, err
	}

	steamWebAPIKey, steamWebAPIKeySource := resolveSteamWebAPIKey()
	palDefenderRESTPort := envInt("PALPANEL_PALDEFENDER_REST_PORT", DefaultPalDefenderRESTPort)
	cfg := Config{
		RuntimeRoot:                  layout.RuntimeRoot,
		RepositoryRoot:               layout.RepositoryRoot,
		DevelopmentMode:              layout.Development,
		ListenAddr:                   env("PALPANEL_LISTEN_ADDR", "127.0.0.1:8080"),
		DataDir:                      dataDir,
		ServerDir:                    serverDir,
		WinePrefixDir:                winePrefixDir,
		ToolsDir:                     toolsDir,
		SteamCMDDir:                  steamCMDDir,
		UE4SSDir:                     ue4ssDir,
		UploadsDir:                   uploadsDir,
		BackupsDir:                   backupsDir,
		LogsDir:                      logsDir,
		DBPath:                       dbPath,
		RequireAuth:                  envBool("PALPANEL_REQUIRE_AUTH", true),
		CORSOrigins:                  envList("PALPANEL_CORS_ORIGINS", []string{"http://127.0.0.1:3000", "http://localhost:3000"}),
		FrontendDist:                 frontendDist,
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
		SteamCMDDownloadURL:          strings.TrimSpace(env("PALPANEL_STEAMCMD_DOWNLOAD_URL", DefaultSteamCMDDownloadURL)),
		SteamCMDDownloadMaxBytes:     int64(envInt("PALPANEL_STEAMCMD_DOWNLOAD_MAX_MB", DefaultSteamCMDDownloadMaxMB)) * 1024 * 1024,
		UE4SSVersion:                 strings.TrimSpace(env("PALPANEL_UE4SS_VERSION", DefaultUE4SSVersion)),
		UE4SSDownloadURL:             strings.TrimSpace(env("PALPANEL_UE4SS_DOWNLOAD_URL", DefaultUE4SSDownloadURL)),
		UE4SSArchiveSHA256:           strings.ToLower(strings.TrimSpace(env("PALPANEL_UE4SS_ARCHIVE_SHA256", DefaultUE4SSArchiveSHA256))),
		UE4SSDownloadMaxBytes:        int64(envInt("PALPANEL_UE4SS_DOWNLOAD_MAX_MB", DefaultUE4SSDownloadMaxMB)) * 1024 * 1024,
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
		SaveIndexCacheDir:            saveIndexCacheDir,
		SaveIndexTimeoutSeconds:      envInt("PALPANEL_SAVE_INDEX_TIMEOUT_SECONDS", 120),
		PerfSlowRequestMS:            envInt("PALPANEL_PERF_SLOW_REQUEST_MS", 500),
		MonitorRetentionDays:         envInt("PALPANEL_MONITOR_RETENTION_DAYS", DefaultMonitorRetentionDays),
		AITranslationTimeoutSeconds:  envInt("PALPANEL_AI_TRANSLATION_TIMEOUT_SECONDS", DefaultAITranslationTimeoutSeconds),
		LogLevel:                     strings.ToLower(env("PALPANEL_LOG_LEVEL", "info")),
		RunnerDir:                    runnerDir,
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
	if err := validateHTTPBaseURL("PALPANEL_STEAMCMD_DOWNLOAD_URL", cfg.SteamCMDDownloadURL); err != nil {
		return Config{}, err
	}
	if cfg.SteamCMDDownloadMaxBytes < 1*1024*1024 || cfg.SteamCMDDownloadMaxBytes > 1024*1024*1024 {
		return Config{}, fmt.Errorf("PALPANEL_STEAMCMD_DOWNLOAD_MAX_MB must be between 1 and 1024")
	}
	if err := validateHTTPBaseURL("PALPANEL_UE4SS_DOWNLOAD_URL", cfg.UE4SSDownloadURL); err != nil {
		return Config{}, err
	}
	if cfg.UE4SSVersion == "" {
		return Config{}, fmt.Errorf("PALPANEL_UE4SS_VERSION must not be empty")
	}
	if len(cfg.UE4SSArchiveSHA256) != 64 {
		return Config{}, fmt.Errorf("PALPANEL_UE4SS_ARCHIVE_SHA256 must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(cfg.UE4SSArchiveSHA256); err != nil {
		return Config{}, fmt.Errorf("PALPANEL_UE4SS_ARCHIVE_SHA256 must be hexadecimal")
	}
	if cfg.UE4SSDownloadMaxBytes < 1*1024*1024 || cfg.UE4SSDownloadMaxBytes > 1024*1024*1024 {
		return Config{}, fmt.Errorf("PALPANEL_UE4SS_DOWNLOAD_MAX_MB must be between 1 and 1024")
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
	if c.RuntimeRoot != "" {
		if err := c.ValidateManagedPath(c.RuntimeRoot, true); err != nil {
			return err
		}
		if err := os.MkdirAll(c.RuntimeRoot, 0o755); err != nil {
			return err
		}
	}
	dirs := []string{c.DataDir, c.ServerDir, c.WinePrefixDir, c.ToolsDir, c.SteamCMDDir, c.UE4SSDir, c.UploadsDir, c.BackupsDir, c.LogsDir, c.SaveIndexCacheDir}
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		if c.RuntimeRoot != "" {
			if err := c.ValidateManagedPath(dir, true); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if c.RuntimeRoot != "" {
		if err := c.ValidateManagedPath(c.DBPath, false); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(c.DBPath), 0o755); err != nil {
		return err
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
		"PALPANEL_STEAMCMD_DOWNLOAD_MAX_MB",
		"PALPANEL_UE4SS_DOWNLOAD_MAX_MB",
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

func configuredPath(key, fallback, base string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		value = fallback
	}
	path, err := absoluteFrom(base, value)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", key, err)
	}
	return path, nil
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
