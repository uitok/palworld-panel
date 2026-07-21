package paldefender

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"palpanel/internal/db"
	"palpanel/internal/jobs"
)

const linuxUE4SSLatestURL = "https://api.github.com/repos/XarminaEu/ue4ss-linux/releases/latest"

func (m Manager) isLinuxRuntime() bool {
	mode, found, err := m.store.GetKV(context.Background(), kvRuntimeMode)
	if err != nil {
		return false
	}
	if !found || strings.TrimSpace(mode) == "" {
		return runtime.GOOS == "linux"
	}
	return strings.TrimSpace(mode) == runtimeLinuxSteamCMD
}

func (m Manager) linuxUE4SSManifestPath() string {
	return filepath.Join(m.cfg.LinuxBinariesDir(), ".palpanel-ue4ss-linux.json")
}

func (m Manager) detectLinuxUE4SS() UE4SSDependencyStatus {
	files := map[string]bool{
		"libUE4SS.so":        fileExists(m.cfg.LinuxUE4SSPath()),
		"Mods/mods.txt":      fileExists(filepath.Join(m.cfg.LinuxBinariesDir(), "Mods", "mods.txt")),
		"UE4SS-settings.ini": fileExists(filepath.Join(m.cfg.LinuxBinariesDir(), "UE4SS-settings.ini")),
	}
	status := UE4SSDependencyStatus{State: UE4SSMissing, Files: files, Path: m.cfg.LinuxBinariesDir(), Message: "Native Linux UE4SS is not installed."}
	if !files["libUE4SS.so"] {
		return status
	}
	status.Installed = true
	var manifest ue4ssManifest
	body, err := os.ReadFile(m.linuxUE4SSManifestPath())
	if err != nil || json.Unmarshal(body, &manifest) != nil {
		status.State = UE4SSIncompatible
		status.Message = "Native Linux UE4SS exists but its release metadata is missing; run install/update to repair it."
		return status
	}
	status.Version = manifest.Version
	status.ArchiveSHA256 = manifest.ArchiveSHA256
	expected := manifest.Files["libUE4SS.so"]
	actual, err := fileSHA256(m.cfg.LinuxUE4SSPath())
	if err != nil || expected == "" || !strings.EqualFold(actual, expected) {
		status.State = UE4SSFailed
		status.Message = "Native Linux UE4SS library failed integrity verification."
		if err != nil {
			status.Error = err.Error()
		}
		return status
	}
	status.State, status.Compatible = UE4SSInstalled, true
	status.LoadVerified, status.LoadEvidence = m.ue4ssLoadEvidence()
	if status.LoadVerified {
		status.Message = "Native Linux UE4SS is installed and confirmed in the server log."
	} else {
		status.Message = "Native Linux UE4SS is installed; start the server once to verify LD_PRELOAD loading."
	}
	return status
}

func (m Manager) ensureLinuxUE4SS(ctx context.Context) (UE4SSDependencyStatus, error) {
	if err := m.ensureGameStopped(ctx); err != nil {
		return m.detectLinuxUE4SS(), err
	}
	m.setUE4SSState(UE4SSInstalling, "Downloading native Linux UE4SS release.", "")
	var release Release
	if err := m.getJSON(ctx, linuxUE4SSLatestURL, &release); err != nil {
		return m.detectLinuxUE4SS(), err
	}
	var asset Asset
	for _, candidate := range release.Assets {
		if candidate.Name == "libUE4SS-linux-x64.so" {
			asset = candidate
			break
		}
	}
	if asset.BrowserDownloadURL == "" || !strings.HasPrefix(strings.ToLower(asset.Digest), "sha256:") {
		return m.detectLinuxUE4SS(), fmt.Errorf("latest ue4ss-linux release has no digest-verified x64 library")
	}
	dir := m.cfg.LinuxBinariesDir()
	if err := m.cfg.ValidateManagedPath(dir, false); err != nil {
		return m.detectLinuxUE4SS(), err
	}
	if err := os.MkdirAll(filepath.Join(dir, "Mods"), 0o755); err != nil {
		return m.detectLinuxUE4SS(), err
	}
	if err := os.MkdirAll(m.cfg.ToolsDir, 0o755); err != nil {
		return m.detectLinuxUE4SS(), err
	}
	stage, err := os.MkdirTemp(m.cfg.ToolsDir, "ue4ss-linux-")
	if err != nil {
		return m.detectLinuxUE4SS(), err
	}
	defer func() { _ = m.removeManagedDirectory(stage) }()
	downloaded := filepath.Join(stage, asset.Name)
	if err := m.downloadAsset(ctx, asset, downloaded); err != nil {
		return m.detectLinuxUE4SS(), err
	}
	backup := filepath.Join(m.cfg.BackupsDir, "ue4ss-linux-"+time.Now().UTC().Format("20060102T150405.000000000Z"), "libUE4SS.so")
	mutation, err := m.replaceUE4SSFile(downloaded, m.cfg.LinuxUE4SSPath(), backup)
	if err != nil {
		return m.detectLinuxUE4SS(), err
	}
	rollback := func(cause error) (UE4SSDependencyStatus, error) {
		_ = rollbackUE4SSMutations(m, []ue4ssMutation{mutation})
		return m.detectLinuxUE4SS(), cause
	}
	settings := filepath.Join(dir, "UE4SS-settings.ini")
	if !fileExists(settings) {
		body := "[General]\nEnableHotReloadSystem=true\nEnableAutoReloadingLuaMods=true\nUseCache=true\nInvalidateCacheIfDLLDiffers=true\nEnableDebugKeyBindings=false\n"
		if err := os.WriteFile(settings, []byte(body), 0o600); err != nil {
			return rollback(err)
		}
	}
	mods := filepath.Join(dir, "Mods", "mods.txt")
	if !fileExists(mods) {
		if err := os.WriteFile(mods, []byte("UE4SSStatus : 1\n"), 0o600); err != nil {
			return rollback(err)
		}
	}
	hash, err := fileSHA256(m.cfg.LinuxUE4SSPath())
	if err != nil {
		return rollback(err)
	}
	manifest := ue4ssManifest{Version: release.TagName, ArchiveSHA256: strings.TrimPrefix(asset.Digest, "sha256:"), SourceURL: asset.BrowserDownloadURL, InstalledAt: time.Now().UTC().Format(time.RFC3339Nano), Files: map[string]string{"libUE4SS.so": hash}}
	manifestBody, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return rollback(err)
	}
	if err := os.WriteFile(m.linuxUE4SSManifestPath(), append(manifestBody, '\n'), 0o600); err != nil {
		return rollback(err)
	}
	if mutation.oldPath != "" {
		_ = os.Remove(mutation.oldPath)
	}
	installed := m.detectLinuxUE4SS()
	m.setUE4SSState(installed.State, installed.Message, "")
	return installed, nil
}

func (m Manager) InstallUE4SS(ctx context.Context) (db.Job, error) {
	return m.jobs.Submit(ctx, jobs.ClassLifecycle, "ue4ss_install", "queued UE4SS install", func(jobCtx context.Context, jobID string) {
		m.update(jobID, "running", 10, "checking server state and UE4SS release", "")
		status, err := m.ensureUE4SS(jobCtx)
		if err != nil {
			m.update(jobID, "failed", 50, "UE4SS install failed", err.Error())
			return
		}
		m.update(jobID, "completed", 100, "UE4SS "+status.Version+" installed", "")
	})
}
