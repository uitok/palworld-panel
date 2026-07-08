package server

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/id"
	"palpanel/internal/palconfig"
)

const (
	kvRuntimeMode = "runtime_mode"
	kvStartup     = "startup_config"
	kvInstalled   = "installed"
	kvPID         = "windows_pid"
)

type Manager struct {
	cfg                 appconfig.Config
	store               *db.Store
	runner              docker.Runner
	remoteBuildIDFunc   func(context.Context) (string, string, error)
	installOrUpdateFunc func(context.Context, string) error
}

type Status struct {
	Installed      bool                   `json:"installed"`
	PendingRestart bool                   `json:"pending_restart"`
	RuntimeMode    string                 `json:"runtime_mode"`
	SetupStep      string                 `json:"setup_step"`
	ConfigExists   bool                   `json:"config_exists"`
	Container      docker.ContainerStatus `json:"container"`
	StartupArgs    []string               `json:"startup_args"`
	Ports          map[string]int         `json:"ports"`
	Warnings       []string               `json:"warnings"`
	Paths          map[string]string      `json:"paths"`
}

type Prerequisite struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	OK       bool   `json:"ok"`
	Required bool   `json:"required"`
	Message  string `json:"message,omitempty"`
}

type BackupInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
	Reason    string `json:"reason,omitempty"`
	Status    string `json:"status"`
}

type BackupManifest struct {
	Version   int                  `json:"version"`
	Reason    string               `json:"reason"`
	CreatedAt string               `json:"created_at"`
	Files     []BackupManifestFile `json:"files"`
}

type BackupManifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type BackupVerifyResult struct {
	Name         string   `json:"name"`
	Valid        bool     `json:"valid"`
	Format       string   `json:"format"`
	CheckedFiles int      `json:"checked_files"`
	Errors       []string `json:"errors"`
}

type LogQuery struct {
	Tail   int
	Search string
	Level  string
	Since  string
}

type RestartNotifier func(ctx context.Context, wait int, message string) error

func NewManager(cfg appconfig.Config, store *db.Store, runner docker.Runner) Manager {
	return Manager{cfg: cfg, store: store, runner: runner}
}

func (m Manager) RuntimeMode(ctx context.Context) (string, error) {
	mode, ok, err := m.store.GetKV(ctx, kvRuntimeMode)
	if err != nil {
		return "", err
	}
	if !ok || strings.TrimSpace(mode) == "" {
		return RecommendedRuntimeForOS(runtime.GOOS), nil
	}
	if mode != RuntimeWineDocker && mode != RuntimeWindowsSteamCMD {
		return RecommendedRuntimeForOS(runtime.GOOS), nil
	}
	return mode, nil
}

func (m Manager) SetRuntimeMode(ctx context.Context, mode string) error {
	mode = strings.TrimSpace(mode)
	if mode != RuntimeWineDocker && mode != RuntimeWindowsSteamCMD {
		return fmt.Errorf("unsupported runtime mode: %s", mode)
	}
	return m.store.SetKV(ctx, kvRuntimeMode, mode)
}

func (m Manager) StartupConfig(ctx context.Context) (StartupConfig, error) {
	raw, _, err := m.store.GetKV(ctx, kvStartup)
	if err != nil {
		return StartupConfig{}, err
	}
	return DecodeStartupConfig(raw, m.cfg), nil
}

func (m Manager) SetStartupConfig(ctx context.Context, cfg StartupConfig) (StartupConfig, error) {
	cfg = cfg.Normalize(m.cfg)
	if issues := cfg.Validate(); hasErrors(issues) {
		return cfg, fmt.Errorf("startup config has validation errors")
	}
	raw, err := EncodeStartupConfig(cfg, m.cfg)
	if err != nil {
		return cfg, err
	}
	return cfg, m.store.SetKV(ctx, kvStartup, raw)
}

func (m Manager) Prerequisites(ctx context.Context) ([]Prerequisite, error) {
	mode, err := m.RuntimeMode(ctx)
	if err != nil {
		return nil, err
	}
	checks := []Prerequisite{
		{ID: "data_dir", Label: "Data directory", OK: dirExists(m.cfg.DataDir), Required: true, Message: m.cfg.DataDir},
		{ID: "server_dir", Label: "Server directory", OK: dirExists(m.cfg.ServerDir), Required: true, Message: m.cfg.ServerDir},
	}
	if mode == RuntimeWineDocker {
		dockerCapability := detectDocker(ctx, m.cfg.DockerBinary)
		cliOK := dockerCapability.CLIInstalled
		if !cliOK {
			if _, statErr := os.Stat(`C:\Program Files\Docker\Docker\resources\bin\docker.exe`); statErr == nil {
				cliOK = true
			}
		}
		cliMessage := dockerCapability.CLIPath
		if cliMessage == "" {
			cliMessage = m.cfg.DockerBinary
		}
		daemonMessage := dockerCapability.Version
		if daemonMessage == "" {
			daemonMessage = dockerCapability.Error
		}
		checks = append(checks,
			Prerequisite{ID: "docker", Label: "Docker CLI", OK: cliOK, Required: true, Message: cliMessage},
			Prerequisite{ID: "docker_daemon", Label: "Docker daemon", OK: dockerCapability.DaemonReachable, Required: true, Message: daemonMessage},
		)
	} else {
		checks = append(checks,
			Prerequisite{ID: "windows", Label: "Windows host", OK: runtime.GOOS == "windows", Required: true, Message: runtime.GOOS},
			Prerequisite{ID: "steamcmd", Label: "SteamCMD", OK: fileExists(m.cfg.SteamCMDBinaryPath()), Required: false, Message: m.cfg.SteamCMDBinaryPath()},
		)
	}
	return checks, nil
}

func (m Manager) Install(ctx context.Context) (db.Job, error) {
	return m.startJob(ctx, "install", "queued install", func(jobID string) {
		if m.runInstallOrUpdateJob(jobID, false, false, nil) {
			m.update(jobID, "completed", 100, "install completed", "")
		}
	})
}

func (m Manager) Update(ctx context.Context) (db.Job, error) {
	return m.UpdateWithPreUpdate(ctx, nil)
}

func (m Manager) UpdateWithPreUpdate(ctx context.Context, preUpdate func(context.Context) error) (db.Job, error) {
	return m.startJob(ctx, "update", "queued update", func(jobID string) {
		if m.runInstallOrUpdateJob(jobID, true, true, preUpdate) {
			m.update(jobID, "completed", 100, "update completed", "")
		}
	})
}

func (m Manager) Bootstrap(ctx context.Context) (db.Job, error) {
	return m.startJob(ctx, "bootstrap", "queued bootstrap", func(jobID string) {
		if !m.runInstallOrUpdateJob(jobID, false, false, nil) {
			return
		}
		m.update(jobID, "running", 80, "initializing configuration", "")
		if err := m.InitializeConfig(context.Background()); err != nil {
			m.update(jobID, "failed", 80, "config initialization failed", err.Error())
			return
		}
		m.update(jobID, "completed", 100, "bootstrap completed", "")
	})
}

func (m Manager) runInstallOrUpdateJob(jobID string, backupFirst bool, update bool, preUpdate func(context.Context) error) bool {
	mode, err := m.RuntimeMode(context.Background())
	if err != nil {
		m.update(jobID, "failed", 10, "runtime mode read failed", err.Error())
		return false
	}
	action := "install"
	if update {
		action = "update"
	}
	wasRunning := false
	if update {
		status, err := m.Status(context.Background())
		if err != nil {
			m.update(jobID, "failed", 5, "server status read failed", err.Error())
			return false
		}
		wasRunning = status.Container.Status == "running"
		if wasRunning {
			if preUpdate != nil {
				m.update(jobID, "running", 5, "announcing update and saving world", "")
				if err := preUpdate(context.Background()); err != nil {
					m.update(jobID, "failed", 5, "pre-update notification/save failed", err.Error())
					return false
				}
			}
			m.update(jobID, "running", 10, "stopping server before update", "")
			if err := m.Stop(context.Background()); err != nil {
				m.update(jobID, "failed", 10, "stop before update failed", err.Error())
				return false
			}
		}
	}
	if backupFirst {
		m.update(jobID, "running", 15, "creating backup before update", "")
		if _, err := m.createBackupArchive("pre-update"); err != nil {
			m.update(jobID, "failed", 15, "backup failed", err.Error())
			return false
		}
	}
	if mode == RuntimeWindowsSteamCMD {
		m.update(jobID, "running", 25, "preparing SteamCMD", "")
		if m.installOrUpdateFunc == nil {
			if err := m.ensureSteamCMD(context.Background()); err != nil {
				m.update(jobID, "failed", 25, "steamcmd setup failed", err.Error())
				return false
			}
		}
		m.update(jobID, "running", 60, action+"ing Palworld Windows dedicated server", "")
		if err := m.installOrUpdateRuntime(context.Background(), mode); err != nil {
			m.update(jobID, "failed", 60, action+" failed", err.Error())
			return false
		}
	} else {
		m.update(jobID, "running", 20, "building wine runner image", "")
		if m.installOrUpdateFunc == nil {
			if err := m.runner.BuildImage(context.Background()); err != nil {
				m.update(jobID, "failed", 20, "build failed", err.Error())
				return false
			}
		}
		m.update(jobID, "running", 60, action+"ing Palworld Windows dedicated server", "")
		if err := m.installOrUpdateRuntime(context.Background(), mode); err != nil {
			m.update(jobID, "failed", 60, action+" failed", err.Error())
			return false
		}
	}
	_ = m.store.SetKV(context.Background(), kvInstalled, "true")
	if update && wasRunning {
		m.update(jobID, "running", 85, "starting server after update", "")
		if err := m.Start(context.Background()); err != nil {
			m.update(jobID, "failed", 85, "restart after update failed", err.Error())
			return false
		}
	}
	m.update(jobID, "running", 95, action+" completed", "")
	return true
}

func (m Manager) InitializeConfig(ctx context.Context) error {
	if fileExists(m.cfg.PalWorldSettingsPath()) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.cfg.PalWorldSettingsPath()), 0o755); err != nil {
		return err
	}
	settings := palconfig.Defaults()
	if fileExists(m.cfg.DefaultPalWorldSettingsPath()) {
		next, err := palconfig.Read(m.cfg.DefaultPalWorldSettingsPath())
		if err != nil {
			return err
		}
		for key, value := range next {
			settings[key] = value
		}
	}
	applyPanelDefaults(settings, m.cfg)
	return palconfig.Write(m.cfg.PalWorldSettingsPath(), settings)
}

func applyPanelDefaults(settings palconfig.Settings, cfg appconfig.Config) {
	settings["RESTAPIEnabled"] = "True"
	settings["RESTAPIPort"] = "8212"
	if cfg.PalworldRESTPass != "" {
		settings["AdminPassword"] = cfg.PalworldRESTPass
	}
}

func (m Manager) ValidateStartup(ctx context.Context) []ValidationIssue {
	var issues []ValidationIssue
	startup, err := m.StartupConfig(ctx)
	if err != nil {
		return []ValidationIssue{{Severity: "error", Message: err.Error()}}
	}
	issues = append(issues, startup.Validate()...)
	if !fileExists(m.cfg.PalServerExePath()) {
		issues = append(issues, ValidationIssue{Field: "server", Severity: "error", Message: "PalServer.exe not found; install server first"})
	}
	if !fileExists(m.cfg.PalWorldSettingsPath()) {
		issues = append(issues, ValidationIssue{Field: "config", Severity: "warning", Message: "PalWorldSettings.ini not found; run initialize-config"})
	}
	settings, err := palconfig.Read(m.cfg.PalWorldSettingsPath())
	if err == nil {
		for _, issue := range palconfig.Validate(settings) {
			issues = append(issues, ValidationIssue{Field: issue.Field, Severity: issue.Severity, Message: issue.Message})
		}
	}
	return issues
}

func (m Manager) Start(ctx context.Context) error {
	if issues := m.ValidateStartup(ctx); hasErrors(issues) {
		return fmt.Errorf("startup validation failed: %s", validationIssueSummary(issues))
	}
	startup, err := m.StartupConfig(ctx)
	if err != nil {
		return err
	}
	mode, err := m.RuntimeMode(ctx)
	if err != nil {
		return err
	}
	if mode == RuntimeWindowsSteamCMD {
		err = m.startWindows(ctx, startup.Args(m.cfg))
	} else {
		err = m.runner.StartWithArgs(ctx, startup.Args(m.cfg))
	}
	if err == nil {
		_ = m.store.SetKV(ctx, "pending_restart", "false")
	}
	return err
}

func validationIssueSummary(issues []ValidationIssue) string {
	var parts []string
	for _, issue := range issues {
		if issue.Severity != "error" {
			continue
		}
		if issue.Field != "" {
			parts = append(parts, issue.Field+": "+issue.Message)
		} else {
			parts = append(parts, issue.Message)
		}
	}
	if len(parts) == 0 {
		return "unknown validation error"
	}
	return strings.Join(parts, "; ")
}

func (m Manager) Stop(ctx context.Context) error {
	mode, err := m.RuntimeMode(ctx)
	if err != nil {
		return err
	}
	if mode == RuntimeWindowsSteamCMD {
		return m.stopWindows(ctx)
	}
	return m.runner.Stop(ctx)
}

func (m Manager) Restart(ctx context.Context) error {
	mode, err := m.RuntimeMode(ctx)
	if err != nil {
		return err
	}
	startup, err := m.StartupConfig(ctx)
	if err != nil {
		return err
	}
	if mode == RuntimeWindowsSteamCMD {
		_ = m.stopWindows(ctx)
		err = m.startWindows(ctx, startup.Args(m.cfg))
	} else {
		err = m.runner.RestartWithArgs(ctx, startup.Args(m.cfg))
	}
	if err == nil {
		_ = m.store.SetKV(ctx, "pending_restart", "false")
	}
	return err
}

func (m Manager) SafeRestart(ctx context.Context, waitSeconds int, message string, notify RestartNotifier) (db.Job, error) {
	if waitSeconds < 5 || waitSeconds > 300 {
		return db.Job{}, fmt.Errorf("waittime must be between 5 and 300 seconds")
	}
	if strings.TrimSpace(message) == "" {
		message = "Server maintenance restart"
	}
	return m.startJob(ctx, "safe_restart", "queued safe restart", func(jobID string) {
		m.update(jobID, "running", 10, "saving world and notifying players", "")
		if notify != nil {
			if err := notify(context.Background(), waitSeconds, message); err != nil {
				m.update(jobID, "running", 20, "notification failed; continuing with managed restart", err.Error())
			}
		}
		m.update(jobID, "running", 35, "waiting for player countdown", "")
		time.Sleep(time.Duration(waitSeconds) * time.Second)
		m.update(jobID, "running", 55, "stopping server", "")
		if err := m.Stop(context.Background()); err != nil {
			m.update(jobID, "failed", 55, "stop failed", err.Error())
			return
		}
		m.update(jobID, "running", 75, "starting server", "")
		if err := m.Start(context.Background()); err != nil {
			m.update(jobID, "failed", 75, "start failed", err.Error())
			return
		}
		m.update(jobID, "completed", 100, "safe restart completed", "")
	})
}

func (m Manager) Logs(ctx context.Context, query LogQuery) (string, error) {
	mode, err := m.RuntimeMode(ctx)
	if err != nil {
		return "", err
	}
	tail := query.Tail
	if mode == RuntimeWindowsSteamCMD {
		logs, err := tailFile(m.cfg.ServerLogPath(), tail)
		if err != nil {
			return "", err
		}
		return filterLogs(logs, query), nil
	}
	logs, err := m.runner.Logs(ctx, tail)
	if err != nil {
		return "", err
	}
	return filterLogs(logs, query), nil
}

func (m Manager) Status(ctx context.Context) (Status, error) {
	mode, err := m.RuntimeMode(ctx)
	if err != nil {
		return Status{}, err
	}
	container := docker.ContainerStatus{Exists: false, Status: "missing"}
	var statusErr error
	if mode == RuntimeWineDocker {
		container, err = m.runner.Status(ctx)
		if err != nil {
			statusErr = err
			container = docker.ContainerStatus{Exists: false, Status: "error"}
		}
	} else {
		container = m.windowsStatus(ctx)
	}
	installedValue, ok, err := m.store.GetKV(ctx, kvInstalled)
	if err != nil {
		return Status{}, err
	}
	pendingValue, _, err := m.store.GetKV(ctx, "pending_restart")
	if err != nil {
		return Status{}, err
	}
	installed := ok && installedValue == "true"
	if !installed {
		installed = fileExists(m.cfg.PalServerExePath())
	}
	configExists := fileExists(m.cfg.PalWorldSettingsPath())
	startup, _ := m.StartupConfig(ctx)
	warnings := m.statusWarnings(mode, installed, configExists)
	if statusErr != nil {
		warnings = append(warnings, statusErr.Error())
	}
	return Status{
		Installed:      installed,
		PendingRestart: parseBool(pendingValue),
		RuntimeMode:    mode,
		SetupStep:      setupStep(installed, configExists, container.Status),
		ConfigExists:   configExists,
		Container:      container,
		StartupArgs:    startup.Args(m.cfg),
		Ports: map[string]int{
			"game":  startup.Normalize(m.cfg).Port,
			"query": m.cfg.QueryPort,
			"rest":  m.cfg.RESTPort,
		},
		Warnings: warnings,
		Paths: map[string]string{
			"data":               m.cfg.DataDir,
			"server":             m.cfg.ServerDir,
			"palworld_settings":  m.cfg.PalWorldSettingsPath(),
			"default_settings":   m.cfg.DefaultPalWorldSettingsPath(),
			"pal_mod_settings":   m.cfg.PalModSettingsPath(),
			"workshop_mods":      m.cfg.WorkshopModsDir(),
			"legacy_mods":        m.cfg.LegacyModsDir(),
			"steamcmd":           m.cfg.SteamCMDBinaryPath(),
			"paldefender":        m.cfg.PalDefenderDir(),
			"windows_server_log": m.cfg.ServerLogPath(),
			"paldefender_win64":  m.cfg.Win64Dir(),
		},
	}, nil
}

func (m Manager) MarkPendingRestart(ctx context.Context) error {
	return m.store.SetKV(ctx, "pending_restart", "true")
}

func (m Manager) Backup(ctx context.Context) (db.Job, error) {
	return m.startJob(ctx, "backup", "queued backup", func(jobID string) {
		m.update(jobID, "running", 20, "creating backup archive", "")
		backup, err := m.createBackupArchive("manual")
		if err != nil {
			m.update(jobID, "failed", 20, "backup failed", err.Error())
			return
		}
		m.update(jobID, "completed", 100, "backup created: "+backup.Name, "")
	})
}

func (m Manager) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(m.cfg.BackupsDir)
	if err != nil {
		return nil, err
	}
	out := []BackupInfo{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		out = append(out, BackupInfo{
			Name:      entry.Name(),
			Path:      filepath.Join(m.cfg.BackupsDir, entry.Name()),
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339Nano),
			Reason:    backupReason(entry.Name()),
			Status:    "available",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

func (m Manager) RestoreBackup(ctx context.Context, name string) (db.Job, error) {
	pathAbs, name, err := m.backupPath(name)
	if err != nil {
		return db.Job{}, err
	}
	if !fileExists(pathAbs) {
		return db.Job{}, fmt.Errorf("backup not found")
	}
	return m.startJob(ctx, "restore", "queued backup restore", func(jobID string) {
		m.update(jobID, "running", 5, "verifying backup archive", "")
		result, err := verifyBackupArchive(pathAbs, name)
		if err != nil {
			m.update(jobID, "failed", 5, "backup verify failed", err.Error())
			return
		}
		if !result.Valid {
			m.update(jobID, "failed", 5, "backup verify failed", strings.Join(result.Errors, "; "))
			return
		}
		m.update(jobID, "running", 10, "stopping server before restore", "")
		if err := m.Stop(context.Background()); err != nil {
			m.update(jobID, "failed", 10, "stop failed", err.Error())
			return
		}
		m.update(jobID, "running", 30, "creating pre-restore backup", "")
		if _, err := m.createBackupArchive("pre-restore"); err != nil {
			m.update(jobID, "failed", 30, "pre-restore backup failed", err.Error())
			return
		}
		m.update(jobID, "running", 65, "restoring backup archive", "")
		if err := extractZipSafe(pathAbs, m.cfg.ServerDir); err != nil {
			m.update(jobID, "failed", 65, "restore failed", err.Error())
			return
		}
		_ = m.store.SetKV(context.Background(), "pending_restart", "true")
		m.update(jobID, "completed", 100, "backup restored; start the server after verifying files", "")
	})
}

func (m Manager) BackupDownloadPath(name string) (string, error) {
	path, _, err := m.backupPath(name)
	if err != nil {
		return "", err
	}
	if !fileExists(path) {
		return "", fmt.Errorf("backup not found")
	}
	return path, nil
}

func (m Manager) DeleteBackup(name string) error {
	path, _, err := m.backupPath(name)
	if err != nil {
		return err
	}
	if !fileExists(path) {
		return fmt.Errorf("backup not found")
	}
	return os.Remove(path)
}

func (m Manager) VerifyBackup(name string) (BackupVerifyResult, error) {
	path, cleanName, err := m.backupPath(name)
	if err != nil {
		return BackupVerifyResult{}, err
	}
	if !fileExists(path) {
		return BackupVerifyResult{}, fmt.Errorf("backup not found")
	}
	return verifyBackupArchive(path, cleanName)
}

func (m Manager) startJob(ctx context.Context, typ, message string, fn func(jobID string)) (db.Job, error) {
	j, err := m.store.CreateJob(ctx, id.New("job"), typ, message)
	if err != nil {
		return db.Job{}, err
	}
	go fn(j.ID)
	return j, nil
}

func (m Manager) update(jobID, status string, progress int, message, errText string) {
	_ = m.store.UpdateJob(context.Background(), jobID, status, progress, message, errText)
}

func (m Manager) ensureSteamCMD(ctx context.Context) error {
	if fileExists(m.cfg.SteamCMDBinaryPath()) {
		return nil
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("windows_steamcmd runtime requires Windows host")
	}
	if err := os.MkdirAll(m.cfg.SteamCMDDir, 0o755); err != nil {
		return err
	}
	zipPath := filepath.Join(m.cfg.ToolsDir, "steamcmd.zip")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("download steamcmd returned status %d", resp.StatusCode)
	}
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return extractZipSafe(zipPath, m.cfg.SteamCMDDir)
}

func (m Manager) installOrUpdateWindows(ctx context.Context) error {
	args := []string{
		"+force_install_dir", m.cfg.ServerDir,
		"+login", "anonymous",
		"+app_update", "2394010", "validate",
		"+quit",
	}
	cmd := exec.CommandContext(ctx, m.cfg.SteamCMDBinaryPath(), args...)
	cmd.Dir = m.cfg.SteamCMDDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("steamcmd failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (m Manager) installOrUpdateRuntime(ctx context.Context, mode string) error {
	if m.installOrUpdateFunc != nil {
		return m.installOrUpdateFunc(ctx, mode)
	}
	if mode == RuntimeWindowsSteamCMD {
		return m.installOrUpdateWindows(ctx)
	}
	return m.runner.InstallOrUpdate(ctx)
}

func (m Manager) startWindows(ctx context.Context, args []string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("windows_steamcmd runtime requires Windows host")
	}
	if !fileExists(m.cfg.PalServerExePath()) {
		return fmt.Errorf("PalServer.exe not found")
	}
	if err := os.MkdirAll(filepath.Dir(m.cfg.ServerLogPath()), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(m.cfg.ServerLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, m.cfg.PalServerExePath(), args...)
	cmd.Dir = m.cfg.ServerDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	_ = m.store.SetKV(ctx, kvPID, strconv.Itoa(cmd.Process.Pid))
	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()
	return nil
}

func (m Manager) stopWindows(ctx context.Context) error {
	pid, _, err := m.store.GetKV(ctx, kvPID)
	if err != nil {
		return err
	}
	pid = strings.TrimSpace(pid)
	if pid == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "taskkill", "/PID", pid, "/T", "/F")
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(strings.ToLower(string(out)), "not found") {
		return fmt.Errorf("taskkill failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return m.store.SetKV(ctx, kvPID, "")
}

func (m Manager) windowsStatus(ctx context.Context) docker.ContainerStatus {
	pid, _, err := m.store.GetKV(ctx, kvPID)
	if err != nil || strings.TrimSpace(pid) == "" {
		return docker.ContainerStatus{Exists: false, Status: "missing"}
	}
	cmd := exec.CommandContext(ctx, "tasklist", "/FI", "PID eq "+pid)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), pid) {
		return docker.ContainerStatus{Exists: false, Status: "missing"}
	}
	return docker.ContainerStatus{Exists: true, Status: "running"}
}

func (m Manager) statusWarnings(mode string, installed, configExists bool) []string {
	var warnings []string
	if mode == RuntimeWineDocker && runtime.GOOS != "linux" {
		warnings = append(warnings, "Docker Desktop on Windows/macOS is not recommended by official docs for production save-data IO; create backups before updates.")
	}
	if !installed {
		warnings = append(warnings, "server is not installed")
	}
	if !configExists {
		warnings = append(warnings, "PalWorldSettings.ini is not initialized")
	}
	if dirExists(m.cfg.LegacyModsDir()) {
		warnings = append(warnings, "legacy mod path detected; official server mods now use server-root Mods/Workshop and Mods/PalModSettings.ini")
	}
	return warnings
}

func (m Manager) createBackupArchive(reason string) (BackupInfo, error) {
	if err := os.MkdirAll(m.cfg.BackupsDir, 0o755); err != nil {
		return BackupInfo{}, err
	}
	now := time.Now().UTC()
	createdAt := now.Format(time.RFC3339Nano)
	name := fmt.Sprintf("%s-%s.zip", now.Format("20060102T150405.000000000Z"), reason)
	path := filepath.Join(m.cfg.BackupsDir, name)
	out, err := os.Create(path)
	if err != nil {
		return BackupInfo{}, err
	}
	zw := zip.NewWriter(out)
	var files []BackupManifestFile
	for _, root := range []string{
		filepath.Join(m.cfg.ServerDir, "Pal", "Saved"),
		m.cfg.ModsDir(),
		m.cfg.PalWorldSettingsPath(),
		m.cfg.PalModSettingsPath(),
	} {
		added, err := addPathToZip(zw, m.cfg.ServerDir, root)
		if err != nil {
			_ = zw.Close()
			_ = out.Close()
			return BackupInfo{}, err
		}
		files = append(files, added...)
	}
	manifest := BackupManifest{Version: 1, Reason: reason, CreatedAt: createdAt, Files: files}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = zw.Close()
		_ = out.Close()
		return BackupInfo{}, err
	}
	w, err := zw.Create(".palpanel-backup.json")
	if err != nil {
		_ = zw.Close()
		_ = out.Close()
		return BackupInfo{}, err
	}
	if _, err := w.Write(append(manifestBytes, '\n')); err != nil {
		_ = zw.Close()
		_ = out.Close()
		return BackupInfo{}, err
	}
	if err := zw.Close(); err != nil {
		_ = out.Close()
		return BackupInfo{}, err
	}
	if err := out.Close(); err != nil {
		return BackupInfo{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{Name: name, Path: path, SizeBytes: info.Size(), CreatedAt: info.ModTime().UTC().Format(time.RFC3339Nano), Reason: reason, Status: "available"}, nil
}

func addPathToZip(zw *zip.Writer, base, path string) ([]BackupManifestFile, error) {
	if !fileExists(path) && !dirExists(path) {
		return nil, nil
	}
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	var files []BackupManifestFile
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(baseAbs, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		hasher := sha256.New()
		_, copyErr := io.Copy(io.MultiWriter(w, hasher), in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		files = append(files, BackupManifestFile{Path: rel, Size: info.Size(), SHA256: hex.EncodeToString(hasher.Sum(nil))})
		return nil
	})
	return files, err
}

func extractZipSafe(zipPath, dst string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	dst, err = filepath.Abs(dst)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		if file.Name == ".palpanel-backup.json" {
			continue
		}
		target := filepath.Join(dst, file.Name)
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if targetAbs != dst && !strings.HasPrefix(targetAbs, dst+string(os.PathSeparator)) {
			return fmt.Errorf("zip contains unsafe path: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetAbs, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		_ = in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func setupStep(installed, configExists bool, processState string) string {
	if !installed {
		return "prerequisites"
	}
	if !configExists {
		return "installed"
	}
	if processState == "running" {
		return "ready"
	}
	return "configured"
}

func tailFile(path string, tail int) (string, error) {
	if tail <= 0 || tail > 5000 {
		tail = 200
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}
	return strings.Join(lines, "\n"), nil
}

func filterLogs(logs string, query LogQuery) string {
	search := strings.ToLower(strings.TrimSpace(query.Search))
	level := strings.ToLower(strings.TrimSpace(query.Level))
	since := strings.TrimSpace(query.Since)
	if search == "" && level == "" && since == "" {
		return logs
	}
	lines := strings.Split(logs, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		lower := strings.ToLower(line)
		if search != "" && !strings.Contains(lower, search) {
			continue
		}
		if level != "" && level != "all" && !strings.Contains(lower, level) {
			continue
		}
		if since != "" && !strings.Contains(line, since) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func backupReason(name string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if i := strings.Index(name, "-"); i >= 0 && i+1 < len(name) {
		return name[i+1:]
	}
	return "manual"
}

func (m Manager) backupPath(name string) (string, string, error) {
	trimmed := strings.TrimSpace(name)
	cleanName := filepath.Base(trimmed)
	if cleanName == "." || cleanName == "" || cleanName != trimmed || !strings.HasSuffix(strings.ToLower(cleanName), ".zip") {
		return "", "", fmt.Errorf("invalid backup name")
	}
	baseAbs, err := filepath.Abs(m.cfg.BackupsDir)
	if err != nil {
		return "", "", err
	}
	pathAbs, err := filepath.Abs(filepath.Join(m.cfg.BackupsDir, cleanName))
	if err != nil {
		return "", "", err
	}
	if pathAbs != baseAbs && !strings.HasPrefix(pathAbs, baseAbs+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("invalid backup path")
	}
	return pathAbs, cleanName, nil
}

func verifyBackupArchive(path, name string) (BackupVerifyResult, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return BackupVerifyResult{}, err
	}
	defer reader.Close()

	result := BackupVerifyResult{Name: name, Valid: true, Format: "legacy"}
	byPath := map[string]*zip.File{}
	var manifestFile *zip.File
	for _, file := range reader.File {
		if file.Name == ".palpanel-backup.json" {
			manifestFile = file
			continue
		}
		if unsafeZipName(file.Name) {
			result.Errors = append(result.Errors, "unsafe path: "+file.Name)
		}
		if !file.FileInfo().IsDir() {
			byPath[file.Name] = file
		}
	}
	if manifestFile == nil {
		for name, file := range byPath {
			if err := readZipFile(file); err != nil {
				result.Errors = append(result.Errors, "read failed: "+name+": "+err.Error())
				continue
			}
			result.CheckedFiles++
		}
		result.Valid = len(result.Errors) == 0
		return result, nil
	}

	result.Format = "manifest_v1"
	manifestReader, err := manifestFile.Open()
	if err != nil {
		return BackupVerifyResult{}, err
	}
	var manifest BackupManifest
	if err := json.NewDecoder(manifestReader).Decode(&manifest); err != nil {
		_ = manifestReader.Close()
		result.Valid = false
		result.Errors = append(result.Errors, "manifest parse failed: "+err.Error())
		return result, nil
	}
	_ = manifestReader.Close()
	if manifest.Version != 1 {
		result.Errors = append(result.Errors, fmt.Sprintf("unsupported manifest version: %d", manifest.Version))
	}

	expectedPaths := map[string]bool{}
	for _, expected := range manifest.Files {
		expectedPaths[expected.Path] = true
		file := byPath[expected.Path]
		if file == nil {
			result.Errors = append(result.Errors, "missing file: "+expected.Path)
			continue
		}
		if file.UncompressedSize64 != uint64(expected.Size) {
			result.Errors = append(result.Errors, "size mismatch: "+expected.Path)
		}
		r, err := file.Open()
		if err != nil {
			result.Errors = append(result.Errors, "read failed: "+expected.Path)
			continue
		}
		hasher := sha256.New()
		_, copyErr := io.Copy(hasher, r)
		closeErr := r.Close()
		if copyErr != nil {
			result.Errors = append(result.Errors, "hash failed: "+expected.Path)
			continue
		}
		if closeErr != nil {
			result.Errors = append(result.Errors, "close failed: "+expected.Path)
			continue
		}
		if !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), expected.SHA256) {
			result.Errors = append(result.Errors, "sha256 mismatch: "+expected.Path)
		}
		result.CheckedFiles++
	}
	for path := range byPath {
		if !expectedPaths[path] {
			result.Errors = append(result.Errors, "unexpected file: "+path)
		}
	}
	result.Valid = len(result.Errors) == 0
	return result, nil
}

func readZipFile(file *zip.File) error {
	r, err := file.Open()
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(io.Discard, r)
	closeErr := r.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func unsafeZipName(name string) bool {
	if strings.TrimSpace(name) == "" || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return true
	}
	clean := filepath.Clean(name)
	return clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || filepath.IsAbs(clean)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func parseBool(v string) bool {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}

var ErrNotReady = errors.New("server is not ready")
