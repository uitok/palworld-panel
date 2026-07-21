package server

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"palpanel/internal/appconfig"
)

const (
	RuntimeWineDocker      = "wine_docker"
	RuntimeWindowsSteamCMD = "windows_steamcmd"
	RuntimeLinuxSteamCMD   = "linux_steamcmd"
)

type StartupConfig struct {
	Port                        int    `json:"port"`
	Players                     int    `json:"players"`
	PublicLobby                 bool   `json:"public_lobby"`
	PublicIP                    string `json:"public_ip,omitempty"`
	PublicPort                  int    `json:"public_port,omitempty"`
	LogFormat                   string `json:"log_format"`
	UsePerfThreads              bool   `json:"use_perf_threads"`
	NoAsyncLoadingThread        bool   `json:"no_async_loading_thread"`
	UseMultithreadForDS         bool   `json:"use_multithread_for_ds"`
	NumberOfWorkerThreadsServer int    `json:"number_of_worker_threads_server,omitempty"`
	WorkshopDir                 string `json:"workshop_dir,omitempty"`
	NoMods                      bool   `json:"no_mods"`
}

func DefaultStartupConfig(cfg appconfig.Config) StartupConfig {
	return StartupConfig{
		Port:                 cfg.GamePort,
		Players:              32,
		PublicPort:           cfg.GamePort,
		LogFormat:            "text",
		UsePerfThreads:       true,
		NoAsyncLoadingThread: true,
		UseMultithreadForDS:  true,
		WorkshopDir:          cfg.WorkshopModsDir(),
	}
}

func (s StartupConfig) Normalize(cfg appconfig.Config) StartupConfig {
	defaults := DefaultStartupConfig(cfg)
	if s.Port == 0 {
		s.Port = defaults.Port
	}
	if s.Players == 0 {
		s.Players = defaults.Players
	}
	if s.PublicPort == 0 {
		s.PublicPort = defaults.PublicPort
	}
	if strings.TrimSpace(s.LogFormat) == "" {
		s.LogFormat = defaults.LogFormat
	}
	if strings.TrimSpace(s.WorkshopDir) == "" {
		s.WorkshopDir = defaults.WorkshopDir
	}
	return s
}

func (s StartupConfig) Args(cfg appconfig.Config) []string {
	s = s.Normalize(cfg)
	args := []string{
		"-port=" + strconv.Itoa(s.Port),
		"-players=" + strconv.Itoa(s.Players),
		"-enable-gamedata-api",
	}
	if s.PublicLobby {
		args = append(args, "-publiclobby")
	}
	if s.PublicIP != "" {
		args = append(args, "-publicip="+s.PublicIP)
	}
	if s.PublicPort > 0 {
		args = append(args, "-publicport="+strconv.Itoa(s.PublicPort))
	}
	if s.LogFormat != "" {
		args = append(args, "-logformat="+s.LogFormat)
	}
	if s.UsePerfThreads {
		args = append(args, "-useperfthreads")
	}
	if s.NoAsyncLoadingThread {
		args = append(args, "-NoAsyncLoadingThread")
	}
	if s.UseMultithreadForDS {
		args = append(args, "-UseMultithreadForDS")
	}
	if s.NumberOfWorkerThreadsServer > 0 {
		args = append(args, "-NumberOfWorkerThreadsServer="+strconv.Itoa(s.NumberOfWorkerThreadsServer))
	}
	if s.WorkshopDir != "" {
		args = append(args, "-workshopdir="+s.WorkshopDir)
	}
	if s.NoMods {
		args = append(args, "-NoMods")
	}
	return appendUniqueArgs(args, "-log", "-stdout", "-FullStdOutLogOutput")
}

func appendUniqueArgs(args []string, required ...string) []string {
	seen := make(map[string]bool, len(args)+len(required))
	out := make([]string, 0, len(args)+len(required))
	for _, arg := range append(append([]string{}, args...), required...) {
		key := strings.ToLower(strings.TrimSpace(arg))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, arg)
	}
	return out
}

func (s StartupConfig) Validate() []ValidationIssue {
	s = s.Normalize(appconfig.Config{GamePort: 8211})
	var issues []ValidationIssue
	if s.Port < 1 || s.Port > 65535 {
		issues = append(issues, ValidationIssue{Field: "port", Severity: "error", Message: "port must be between 1 and 65535"})
	}
	if s.PublicPort < 1 || s.PublicPort > 65535 {
		issues = append(issues, ValidationIssue{Field: "public_port", Severity: "error", Message: "public_port must be between 1 and 65535"})
	}
	if s.Players < 1 || s.Players > 128 {
		issues = append(issues, ValidationIssue{Field: "players", Severity: "error", Message: "players should be between 1 and 128"})
	}
	if s.LogFormat != "text" && s.LogFormat != "json" && s.LogFormat != "Json" && s.LogFormat != "Text" {
		issues = append(issues, ValidationIssue{Field: "log_format", Severity: "error", Message: "log_format must be text or json"})
	}
	if s.NumberOfWorkerThreadsServer < 0 {
		issues = append(issues, ValidationIssue{Field: "number_of_worker_threads_server", Severity: "error", Message: "worker threads cannot be negative"})
	}
	if s.PublicLobby && s.PublicIP == "" {
		issues = append(issues, ValidationIssue{Field: "public_ip", Severity: "warning", Message: "public lobby can auto-detect IP, but manual public_ip is safer behind NAT"})
	}
	return issues
}

func DecodeStartupConfig(raw string, cfg appconfig.Config) StartupConfig {
	if strings.TrimSpace(raw) == "" {
		return DefaultStartupConfig(cfg)
	}
	var s StartupConfig
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return DefaultStartupConfig(cfg)
	}
	return s.Normalize(cfg)
}

func EncodeStartupConfig(s StartupConfig, cfg appconfig.Config) (string, error) {
	s = s.Normalize(cfg)
	if issues := s.Validate(); hasErrors(issues) {
		return "", fmt.Errorf("invalid startup config")
	}
	b, err := json.MarshalIndent(s, "", "  ")
	return string(b), err
}

func hasErrors(issues []ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

type ValidationIssue struct {
	Field    string `json:"field,omitempty"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}
