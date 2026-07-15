package paldefender

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/id"
	"palpanel/internal/server"
)

const (
	UE4SSNotChecked       = "not_checked"
	UE4SSChecking         = "checking"
	UE4SSMissing          = "missing"
	UE4SSInstalling       = "installing"
	UE4SSInstalled        = "installed"
	UE4SSIncompatible     = "incompatible"
	UE4SSFailed           = "failed"
	UE4SSRollbackRequired = "rollback_required"
)

var ue4ssRequiredFiles = []string{
	"UE4SS.dll",
	"dwmapi.dll",
	"UE4SS-settings.ini",
	filepath.Join("Mods", "mods.txt"),
}

type ServerState interface {
	Status(context.Context) (server.Status, error)
}

type UE4SSDependencyStatus struct {
	State         string          `json:"state"`
	Installed     bool            `json:"installed"`
	Version       string          `json:"version,omitempty"`
	Compatible    bool            `json:"compatible"`
	Files         map[string]bool `json:"files"`
	Path          string          `json:"path"`
	Message       string          `json:"message"`
	Error         string          `json:"error,omitempty"`
	ArchiveSHA256 string          `json:"archive_sha256,omitempty"`
	LoadVerified  bool            `json:"load_verified"`
	LoadEvidence  string          `json:"load_evidence,omitempty"`
}

type dependencyTracker struct {
	mu      sync.RWMutex
	state   string
	message string
	errText string
}

type ue4ssManifest struct {
	Version       string            `json:"version"`
	ArchiveSHA256 string            `json:"archive_sha256"`
	SourceURL     string            `json:"source_url"`
	InstalledAt   string            `json:"installed_at"`
	Files         map[string]string `json:"files"`
}

type ue4ssMutation struct {
	destination string
	oldPath     string
	existed     bool
}

func newDependencyTracker() *dependencyTracker {
	return &dependencyTracker{state: UE4SSNotChecked}
}

func (m Manager) WithServerState(state ServerState) Manager {
	m.serverState = state
	return m
}

func (m Manager) UE4SSStatus() UE4SSDependencyStatus {
	detected := m.detectUE4SS()
	tracker := m.ue4ss
	if tracker == nil {
		return detected
	}
	tracker.mu.RLock()
	state, message, errText := tracker.state, tracker.message, tracker.errText
	tracker.mu.RUnlock()
	switch state {
	case UE4SSChecking, UE4SSInstalling, UE4SSFailed, UE4SSRollbackRequired:
		detected.State = state
		if message != "" {
			detected.Message = message
		}
		detected.Error = errText
	default:
		m.setUE4SSState(detected.State, detected.Message, "")
	}
	return detected
}

func (m Manager) detectUE4SS() UE4SSDependencyStatus {
	status := UE4SSDependencyStatus{
		State: UE4SSMissing, Files: map[string]bool{}, Path: m.cfg.Win64Dir(),
		Message: "UE4SS is missing; installing PalDefender will install the pinned UE4SS dependency first.",
	}
	present := 0
	for _, relative := range ue4ssRequiredFiles {
		exists := fileExists(filepath.Join(m.cfg.Win64Dir(), relative))
		status.Files[filepath.ToSlash(relative)] = exists
		if exists {
			present++
		}
	}
	if present == 0 {
		return status
	}
	if present != len(ue4ssRequiredFiles) {
		status.State = UE4SSFailed
		status.Message = "UE4SS is incomplete; stop the server and run PalDefender repair/install."
		return status
	}
	status.Installed = true
	manifest, err := m.readUE4SSManifest()
	if err != nil {
		status.State = UE4SSIncompatible
		status.Message = "UE4SS files exist but their version is unknown; run repair/install before PalDefender."
		status.Error = err.Error()
		return status
	}
	status.Version = manifest.Version
	status.ArchiveSHA256 = manifest.ArchiveSHA256
	if !strings.EqualFold(strings.TrimSpace(manifest.Version), m.effectiveUE4SSVersion()) {
		status.State = UE4SSIncompatible
		status.Message = fmt.Sprintf("UE4SS %s is not the pinned compatible version %s; update or repair it before PalDefender.", manifest.Version, m.effectiveUE4SSVersion())
		return status
	}
	for relative, expected := range manifest.Files {
		if relative != "UE4SS.dll" && relative != "dwmapi.dll" {
			continue
		}
		actual, err := fileSHA256(filepath.Join(m.cfg.Win64Dir(), filepath.FromSlash(relative)))
		if err != nil || !strings.EqualFold(actual, expected) {
			status.State = UE4SSFailed
			status.Message = "UE4SS core files changed or are damaged; stop the server and run repair/install."
			if err != nil {
				status.Error = err.Error()
			}
			return status
		}
	}
	status.State = UE4SSInstalled
	status.Compatible = true
	status.LoadVerified, status.LoadEvidence = m.ue4ssLoadEvidence()
	if status.LoadVerified {
		status.Message = "UE4SS is installed, compatible, and confirmed in a server startup log."
	} else {
		status.Message = "UE4SS files are installed and compatible; start the server once to verify that UE4SS loads."
	}
	return status
}

func (m Manager) ensureUE4SS(ctx context.Context) (UE4SSDependencyStatus, error) {
	m.setUE4SSState(UE4SSChecking, "Checking UE4SS files and compatibility.", "")
	if err := m.ensureGameStopped(ctx); err != nil {
		m.setUE4SSState(UE4SSFailed, "UE4SS prerequisite check failed.", err.Error())
		return m.UE4SSStatus(), err
	}
	current := m.detectUE4SS()
	if current.State == UE4SSInstalled && current.Compatible {
		m.setUE4SSState(UE4SSInstalled, current.Message, "")
		return current, nil
	}
	m.setUE4SSState(UE4SSInstalling, "Downloading and installing the pinned UE4SS release.", "")
	if err := m.installUE4SS(ctx); err != nil {
		state := UE4SSFailed
		message := "UE4SS installation failed; existing files were restored."
		var rollbackErr *ue4ssRollbackError
		if errors.As(err, &rollbackErr) {
			state = UE4SSRollbackRequired
			message = "UE4SS installation failed and automatic rollback was incomplete; manual recovery is required."
		}
		m.setUE4SSState(state, message, err.Error())
		return m.UE4SSStatus(), err
	}
	installed := m.detectUE4SS()
	if installed.State != UE4SSInstalled || !installed.Compatible {
		err := fmt.Errorf("UE4SS post-install verification failed: %s", installed.Message)
		m.setUE4SSState(UE4SSFailed, installed.Message, err.Error())
		return installed, err
	}
	m.setUE4SSState(UE4SSInstalled, installed.Message, "")
	return installed, nil
}

type ue4ssRollbackError struct {
	installErr  error
	rollbackErr error
}

func (e *ue4ssRollbackError) Error() string {
	return fmt.Sprintf("install: %v; rollback: %v", e.installErr, e.rollbackErr)
}

func (m Manager) installUE4SS(ctx context.Context) error {
	cacheRoot := m.effectiveUE4SSDir()
	for _, path := range []string{cacheRoot, m.cfg.Win64Dir()} {
		if err := m.cfg.ValidateManagedPath(path, false); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(cacheRoot, "ue4ss-stage-")
	if err != nil {
		return err
	}
	defer func() { _ = m.removeManagedDirectory(stage) }()
	archivePath := filepath.Join(stage, "UE4SS.zip")
	if err := m.downloadPinnedUE4SS(ctx, archivePath); err != nil {
		return err
	}
	extracted := filepath.Join(stage, "extracted")
	validate := func(path string) error { return m.cfg.ValidateManagedPath(path, false) }
	if err := extractZipSafeValidated(archivePath, extracted, m.effectiveUE4SSDownloadMaxBytes()*4, validate); err != nil {
		return fmt.Errorf("extract UE4SS: %w", err)
	}
	for _, relative := range ue4ssRequiredFiles {
		if !fileExists(filepath.Join(extracted, relative)) {
			return fmt.Errorf("UE4SS archive is missing %s", filepath.ToSlash(relative))
		}
	}

	backupDir := filepath.Join(m.cfg.BackupsDir, "ue4ss-"+time.Now().UTC().Format("20060102T150405.000000000Z")+"-"+id.New("backup"))
	if err := m.cfg.ValidateManagedPath(backupDir, false); err != nil {
		return err
	}
	mutations := make([]ue4ssMutation, 0, 5)
	rollback := func(installErr error) error {
		if rollbackErr := rollbackUE4SSMutations(m, mutations); rollbackErr != nil {
			return &ue4ssRollbackError{installErr: installErr, rollbackErr: rollbackErr}
		}
		return installErr
	}

	for _, relative := range []string{"UE4SS.dll", "dwmapi.dll"} {
		mutation, err := m.replaceUE4SSFile(filepath.Join(extracted, relative), filepath.Join(m.cfg.Win64Dir(), relative), filepath.Join(backupDir, relative))
		if err != nil {
			return rollback(err)
		}
		mutations = append(mutations, mutation)
	}
	for _, relative := range []string{"UE4SS-settings.ini", filepath.Join("Mods", "mods.txt")} {
		destination := filepath.Join(m.cfg.Win64Dir(), relative)
		if fileExists(destination) {
			continue
		}
		mutation, err := m.replaceUE4SSFile(filepath.Join(extracted, relative), destination, filepath.Join(backupDir, relative))
		if err != nil {
			return rollback(err)
		}
		mutations = append(mutations, mutation)
	}

	manifest := ue4ssManifest{
		Version: m.effectiveUE4SSVersion(), ArchiveSHA256: m.effectiveUE4SSSHA256(),
		SourceURL: m.effectiveUE4SSDownloadURL(), InstalledAt: time.Now().UTC().Format(time.RFC3339Nano),
		Files: map[string]string{},
	}
	for _, relative := range []string{"UE4SS.dll", "dwmapi.dll"} {
		hash, err := fileSHA256(filepath.Join(m.cfg.Win64Dir(), relative))
		if err != nil {
			return rollback(err)
		}
		manifest.Files[relative] = hash
	}
	manifestBody, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return rollback(err)
	}
	manifestSource := filepath.Join(stage, "manifest.json")
	if err := os.WriteFile(manifestSource, append(manifestBody, '\n'), 0o600); err != nil {
		return rollback(err)
	}
	manifestDestination := m.ue4ssManifestPath()
	mutation, err := m.replaceUE4SSFile(manifestSource, manifestDestination, filepath.Join(backupDir, filepath.Base(manifestDestination)))
	if err != nil {
		return rollback(err)
	}
	mutations = append(mutations, mutation)
	for _, mutation := range mutations {
		if mutation.oldPath != "" {
			_ = os.Remove(mutation.oldPath)
		}
	}
	return nil
}

func (m Manager) replaceUE4SSFile(source, destination, persistentBackup string) (ue4ssMutation, error) {
	for _, path := range []string{destination, persistentBackup} {
		if err := m.cfg.ValidateManagedPath(path, false); err != nil {
			return ue4ssMutation{}, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return ue4ssMutation{}, err
	}
	newPath := destination + ".palpanel-new-" + id.New("file")
	if err := m.cfg.ValidateManagedPath(newPath, false); err != nil {
		return ue4ssMutation{}, err
	}
	if err := copyFile(source, newPath); err != nil {
		return ue4ssMutation{}, err
	}
	mutation := ue4ssMutation{destination: destination}
	if fileExists(destination) {
		mutation.existed = true
		if err := copyFile(destination, persistentBackup); err != nil {
			_ = os.Remove(newPath)
			return ue4ssMutation{}, err
		}
		mutation.oldPath = destination + ".palpanel-old-" + id.New("file")
		if err := os.Rename(destination, mutation.oldPath); err != nil {
			_ = os.Remove(newPath)
			return ue4ssMutation{}, err
		}
	}
	if err := os.Rename(newPath, destination); err != nil {
		if mutation.oldPath != "" {
			_ = os.Rename(mutation.oldPath, destination)
		}
		_ = os.Remove(newPath)
		return ue4ssMutation{}, err
	}
	return mutation, nil
}

func rollbackUE4SSMutations(m Manager, mutations []ue4ssMutation) error {
	var failures []string
	for index := len(mutations) - 1; index >= 0; index-- {
		mutation := mutations[index]
		if err := m.cfg.ValidateManagedPath(mutation.destination, false); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		if err := os.Remove(mutation.destination); err != nil && !os.IsNotExist(err) {
			failures = append(failures, err.Error())
			continue
		}
		if mutation.existed && mutation.oldPath != "" {
			if err := os.Rename(mutation.oldPath, mutation.destination); err != nil {
				failures = append(failures, err.Error())
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}

func (m Manager) downloadPinnedUE4SS(ctx context.Context, destination string) error {
	var failures []string
	for attempt := 1; attempt <= 3; attempt++ {
		err := m.downloadPinnedUE4SSOnce(ctx, destination)
		if err == nil {
			return nil
		}
		failures = append(failures, fmt.Sprintf("attempt %d: %v", attempt, err))
		if attempt < 3 {
			timer := time.NewTimer(time.Duration(attempt) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return fmt.Errorf("download UE4SS failed after 3 attempts: %s", strings.Join(failures, "; "))
}

func (m Manager) downloadPinnedUE4SSOnce(ctx context.Context, destination string) error {
	if err := m.cfg.ValidateManagedPath(destination, false); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, m.effectiveUE4SSDownloadURL(), nil)
	if err != nil {
		return err
	}
	response, err := m.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("UE4SS download returned HTTP %d", response.StatusCode)
	}
	limit := m.effectiveUE4SSDownloadMaxBytes()
	if response.ContentLength > limit {
		return fmt.Errorf("UE4SS Content-Length exceeds %d bytes", limit)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".ue4ss-download-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	complete := false
	defer func() {
		_ = temporary.Close()
		if !complete {
			_ = os.Remove(temporaryPath)
		}
	}()
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hasher), io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if written > limit {
		return fmt.Errorf("UE4SS download exceeds %d bytes", limit)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actual, m.effectiveUE4SSSHA256()) {
		return fmt.Errorf("UE4SS archive SHA-256 mismatch: got %s", actual)
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	_ = os.Remove(destination)
	if err := os.Rename(temporaryPath, destination); err != nil {
		return err
	}
	complete = true
	return nil
}

func (m Manager) ensureGameStopped(ctx context.Context) error {
	if m.serverState != nil {
		status, err := m.serverState.Status(ctx)
		if err != nil {
			return fmt.Errorf("read Palworld server status: %w", err)
		}
		if !status.Installed {
			return fmt.Errorf("install and verify Palworld Dedicated Server before installing UE4SS")
		}
		if status.Container.Exists && status.Container.Status != "missing" && status.Container.Status != "exited" {
			return fmt.Errorf("stop Palworld Dedicated Server before installing UE4SS or PalDefender")
		}
		return nil
	}
	if !fileExists(m.cfg.PalServerExePath()) {
		return fmt.Errorf("install Palworld Dedicated Server before installing UE4SS")
	}
	return nil
}

func (m Manager) readUE4SSManifest() (ue4ssManifest, error) {
	body, err := os.ReadFile(m.ue4ssManifestPath())
	if err != nil {
		return ue4ssManifest{}, err
	}
	var manifest ue4ssManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return ue4ssManifest{}, err
	}
	if strings.TrimSpace(manifest.Version) == "" || len(manifest.Files) == 0 {
		return ue4ssManifest{}, fmt.Errorf("UE4SS manifest is incomplete")
	}
	return manifest, nil
}

func (m Manager) ue4ssManifestPath() string {
	return filepath.Join(m.cfg.Win64Dir(), ".palpanel-ue4ss.json")
}

func (m Manager) effectiveUE4SSDir() string {
	if value := strings.TrimSpace(m.cfg.UE4SSDir); value != "" {
		return value
	}
	return filepath.Join(m.cfg.ToolsDir, "ue4ss")
}

func (m Manager) effectiveUE4SSVersion() string {
	if value := strings.TrimSpace(m.cfg.UE4SSVersion); value != "" {
		return value
	}
	return appconfig.DefaultUE4SSVersion
}

func (m Manager) effectiveUE4SSDownloadURL() string {
	if value := strings.TrimSpace(m.cfg.UE4SSDownloadURL); value != "" {
		return value
	}
	return appconfig.DefaultUE4SSDownloadURL
}

func (m Manager) effectiveUE4SSSHA256() string {
	if value := strings.TrimSpace(m.cfg.UE4SSArchiveSHA256); value != "" {
		return strings.ToLower(value)
	}
	return appconfig.DefaultUE4SSArchiveSHA256
}

func (m Manager) effectiveUE4SSDownloadMaxBytes() int64 {
	if m.cfg.UE4SSDownloadMaxBytes > 0 {
		return m.cfg.UE4SSDownloadMaxBytes
	}
	return int64(appconfig.DefaultUE4SSDownloadMaxMB) << 20
}

func (m Manager) setUE4SSState(state, message, errText string) {
	if m.ue4ss == nil {
		return
	}
	m.ue4ss.mu.Lock()
	m.ue4ss.state = state
	m.ue4ss.message = message
	m.ue4ss.errText = errText
	m.ue4ss.mu.Unlock()
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (m Manager) ue4ssLoadEvidence() (bool, string) {
	if logContainsMarkers(filepath.Join(m.cfg.Win64Dir(), "UE4SS.log"), "ue4ss") {
		return true, "ue4ss_log"
	}
	if logContainsMarkers(m.cfg.ServerLogPath(), "ue4ss", "load") || logContainsMarkers(m.cfg.ServerLogPath(), "ue4ss", "initial") {
		return true, "palserver_log"
	}
	return false, ""
}

func logContainsMarkers(path string, markers ...string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() || info.Size() == 0 {
		return false
	}
	const maxLogProbeBytes int64 = 2 << 20
	start := info.Size() - maxLogProbeBytes
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(file, maxLogProbeBytes))
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(body))
	for _, marker := range markers {
		if !strings.Contains(lower, strings.ToLower(marker)) {
			return false
		}
	}
	return true
}

func (m Manager) removeManagedDirectory(path string) error {
	if err := m.cfg.ValidateManagedPath(path, false); err != nil {
		return err
	}
	if err := m.cfg.ValidateManagedPath(path, false); err != nil {
		return err
	}
	return os.RemoveAll(path)
}
