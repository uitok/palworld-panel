package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
)

type ServerImportResult struct {
	Path          string `json:"path"`
	ManifestPath  string `json:"manifest_path"`
	BuildID       string `json:"build_id"`
	ConfigExists  bool   `json:"config_exists"`
	AlreadyBound  bool   `json:"already_bound"`
	OriginalInput string `json:"original_input"`
}

// RestoreImportedServerDirectory reloads the path selected in the setup
// wizard before managers begin serving requests. The database stores only a
// validated absolute path; no game files are copied into PalPanel's data tree.
func RestoreImportedServerDirectory(ctx context.Context, cfg appconfig.Config, store *db.Store) error {
	path, ok, err := store.GetKV(ctx, kvServerDir)
	if err != nil {
		return err
	}
	if !ok || strings.TrimSpace(path) == "" {
		return nil
	}
	path, err = normalizeStoredServerDirectory(path)
	if err != nil {
		return fmt.Errorf("restore imported server directory: %w", err)
	}
	return cfg.BindImportedServerDirectory(path)
}

func (m Manager) ImportServerDirectory(ctx context.Context, input string) (ServerImportResult, error) {
	if m.goos != "windows" {
		return ServerImportResult{}, fmt.Errorf("existing server directory import is available only on Windows")
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return ServerImportResult{}, fmt.Errorf("server directory is required")
	}

	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	if record, ok, err := m.loadWindowsProcess(ctx); err != nil {
		return ServerImportResult{}, err
	} else if ok {
		running, verifyErr := m.verifyWindowsProcess(record)
		if verifyErr != nil {
			return ServerImportResult{}, verifyErr
		}
		if running {
			return ServerImportResult{}, fmt.Errorf("stop the currently managed Palworld server before changing its directory")
		}
	}

	path, err := m.resolveImportCandidate(input)
	if err != nil {
		return ServerImportResult{}, err
	}
	if executable, running, err := unmanagedWindowsServerProcess(path); err != nil {
		return ServerImportResult{}, err
	} else if running {
		return ServerImportResult{}, fmt.Errorf("stop the Palworld server running from %s before importing this directory", executable)
	}
	current := m.cfg.ServerDirectory()
	samePath := sameServerDirectory(current, path, m.goos)
	alreadyBound := samePath && m.cfg.ServerDirectoryImported()
	if !samePath && m.validateWindowsServerInstall() == nil {
		return ServerImportResult{}, fmt.Errorf("PalPanel is already managing a valid server at %s", current)
	}
	if err := m.validateImportBoundary(path); err != nil {
		return ServerImportResult{}, err
	}
	if err := validateWindowsServerDirectory(path); err != nil {
		return ServerImportResult{}, err
	}

	manifest := appManifestPathForRoot(path)
	buildID, err := readAppManifestBuildID(manifest)
	if err != nil {
		return ServerImportResult{}, err
	}
	if err := m.SetRuntimeMode(ctx, RuntimeWindowsSteamCMD); err != nil {
		return ServerImportResult{}, err
	}
	if err := m.store.SetKV(ctx, kvServerDir, path); err != nil {
		return ServerImportResult{}, err
	}
	if err := m.cfg.BindImportedServerDirectory(path); err != nil {
		return ServerImportResult{}, err
	}

	return ServerImportResult{
		Path:          path,
		ManifestPath:  manifest,
		BuildID:       buildID,
		ConfigExists:  fileExists(m.cfg.PalWorldSettingsPath()),
		AlreadyBound:  alreadyBound,
		OriginalInput: input,
	}, nil
}

func (m Manager) resolveImportCandidate(input string) (string, error) {
	input = strings.Trim(strings.TrimSpace(input), `"`)
	if strings.EqualFold(filepath.Base(input), "PalServer.exe") {
		input = filepath.Dir(input)
	}
	abs, err := filepath.Abs(filepath.Clean(input))
	if err != nil {
		return "", fmt.Errorf("resolve server directory: %w", err)
	}
	if m.goos == "windows" && strings.HasPrefix(abs, `\\`) {
		return "", fmt.Errorf("network paths are not supported; choose a local Windows drive")
	}

	candidates := []string{
		abs,
		filepath.Join(abs, "PalServer"),
		filepath.Join(abs, "common", "PalServer"),
		filepath.Join(abs, "steamapps", "common", "PalServer"),
	}
	seen := map[string]bool{}
	var lastValidationErr error
	for _, candidate := range candidates {
		key := candidate
		if m.goos == "windows" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		info, statErr := os.Stat(candidate)
		if statErr != nil || !info.IsDir() {
			continue
		}
		resolved, evalErr := filepath.EvalSymlinks(candidate)
		if evalErr != nil {
			return "", fmt.Errorf("resolve server directory links: %w", evalErr)
		}
		resolved, err = filepath.Abs(filepath.Clean(resolved))
		if err != nil {
			return "", err
		}
		if validationErr := validateWindowsServerDirectory(resolved); validationErr == nil {
			return resolved, nil
		} else {
			lastValidationErr = validationErr
		}
	}
	if lastValidationErr != nil {
		return "", fmt.Errorf("no complete Palworld Dedicated Server installation was found: %w", lastValidationErr)
	}
	return "", fmt.Errorf("no complete Palworld Dedicated Server installation was found; select the directory that contains PalServer.exe")
}

func (m Manager) validateImportBoundary(path string) error {
	if filepath.Dir(path) == path {
		return fmt.Errorf("a drive root cannot be used as the Palworld server directory")
	}
	if sameServerDirectory(path, m.cfg.ServerDirectory(), m.goos) {
		return nil
	}
	for _, protected := range []string{
		m.cfg.RuntimeRoot,
		m.cfg.DataDir,
		m.cfg.ToolsDir,
		m.cfg.SteamCMDDir,
		m.cfg.UploadsDir,
		m.cfg.BackupsDir,
		m.cfg.LogsDir,
		m.cfg.RepositoryRoot,
	} {
		protected = strings.TrimSpace(protected)
		if protected == "" {
			continue
		}
		protectedAbs, err := filepath.Abs(filepath.Clean(protected))
		if err != nil {
			return err
		}
		if pathsOverlap(path, protectedAbs) {
			return fmt.Errorf("the imported server directory overlaps PalPanel-managed data: %s", protectedAbs)
		}
	}
	return nil
}

func normalizeStoredServerDirectory(path string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || !filepath.IsAbs(path) {
		return "", fmt.Errorf("stored server directory must be an absolute path")
	}
	return path, nil
}

func sameServerDirectory(left, right, goos string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if goos == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func pathsOverlap(left, right string) bool {
	return pathContains(left, right) || pathContains(right, left)
}

func pathContains(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)))
}
