package mods

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/goccy/go-yaml"

	"palpanel/internal/id"
)

const MaxConfigFileBytes int64 = 1 << 20

var editableConfigExtensions = map[string]bool{
	".json": true, ".ini": true, ".cfg": true, ".toml": true,
	".yaml": true, ".yml": true, ".txt": true, ".lua": true,
}

var forbiddenConfigNames = map[string]bool{
	"info.json": true, "installmanifest.json": true, "palmodsettings.ini": true,
}

type ConfigurationError struct {
	Code string
	Err  error
}

func (e ConfigurationError) Error() string {
	if e.Err == nil {
		return e.Code
	}
	return e.Err.Error()
}

func (e ConfigurationError) Unwrap() error { return e.Err }

type ConfigurationAdapter struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Description    string       `json:"description"`
	WorkshopID     string       `json:"workshop_id,omitempty"`
	Available      bool         `json:"available"`
	ReloadBehavior string       `json:"reload_behavior"`
	Files          []ConfigFile `json:"files"`
}

type ConfigFile struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Extension  string    `json:"extension"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
	Revision   string    `json:"revision"`
	Executable bool      `json:"executable"`
	Risk       string    `json:"risk,omitempty"`
}

type ConfigurationField struct {
	Path  string `json:"path"`
	Label string `json:"label"`
	Type  string `json:"type"`
	Value any    `json:"value"`
	Min   *int64 `json:"min,omitempty"`
	Max   *int64 `json:"max,omitempty"`
}

type ConfigDocument struct {
	File    ConfigFile           `json:"file"`
	Content string               `json:"content"`
	Format  string               `json:"format"`
	Fields  []ConfigurationField `json:"fields,omitempty"`
}

type ConfigWriteRequest struct {
	Content           string `json:"content"`
	Revision          string `json:"revision"`
	ConfirmExecutable bool   `json:"confirm_executable,omitempty"`
}

type ConfigBackup struct {
	ID        string    `json:"id"`
	Revision  string    `json:"revision"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

type configTarget struct {
	scope    string
	root     string
	path     string
	relative string
}

type adapterDefinition struct {
	ID, Name, Description, WorkshopID, ReloadBehavior string
	resolve                                           func(Manager, context.Context) ([]configTarget, error)
}

func adapterRegistry() []adapterDefinition {
	return []adapterDefinition{
		{ID: "paldefender", Name: "PalDefender", Description: "安全、防作弊与服务器管理配置", ReloadBehavior: "online_reload", resolve: resolvePalDefenderAdapter},
		{ID: "ue4ss", Name: "UE4SS Experimental", Description: "UE4SS 运行参数和模块启停列表", WorkshopID: "3625223587", ReloadBehavior: "restart_required", resolve: resolveUE4SSAdapter},
		{ID: "palschema", Name: "PalSchema", Description: "PalSchema 运行设置与模块 JSON 配置", WorkshopID: "3625280368", ReloadBehavior: "restart_required", resolve: resolvePalSchemaAdapter},
		{ID: "extended-base-range", Name: "Extended Base Range", Description: "基地范围 Lua 数值参数", WorkshopID: "3625907101", ReloadBehavior: "restart_required", resolve: resolveExtendedBaseRangeAdapter},
	}
}

func (m Manager) ListConfigurations(ctx context.Context) ([]ConfigurationAdapter, error) {
	out := make([]ConfigurationAdapter, 0, len(adapterRegistry()))
	for _, definition := range adapterRegistry() {
		targets, err := definition.resolve(m, ctx)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		files := make([]ConfigFile, 0, len(targets))
		for _, target := range targets {
			file, fileErr := m.describeConfigTarget(target)
			if fileErr == nil {
				files = append(files, file)
			} else if !errors.Is(fileErr, fs.ErrNotExist) {
				return nil, fileErr
			}
		}
		out = append(out, ConfigurationAdapter{
			ID: definition.ID, Name: definition.Name, Description: definition.Description,
			WorkshopID: definition.WorkshopID, Available: len(files) > 0,
			ReloadBehavior: definition.ReloadBehavior, Files: files,
		})
	}
	return out, nil
}

func (m Manager) GetConfiguration(ctx context.Context, adapterID, fileID string) (ConfigDocument, error) {
	target, err := m.resolveAdapterTarget(ctx, adapterID, fileID)
	if err != nil {
		return ConfigDocument{}, err
	}
	return m.readConfigTarget(target)
}

func (m Manager) WriteConfiguration(ctx context.Context, adapterID, fileID string, request ConfigWriteRequest) (ConfigDocument, error) {
	target, err := m.resolveAdapterTarget(ctx, adapterID, fileID)
	if err != nil {
		return ConfigDocument{}, err
	}
	return m.writeConfigTarget(target, request)
}

func (m Manager) ListConfigurationBackups(ctx context.Context, adapterID, fileID string) ([]ConfigBackup, error) {
	target, err := m.resolveAdapterTarget(ctx, adapterID, fileID)
	if err != nil {
		return nil, err
	}
	return m.listBackups(target)
}

func (m Manager) RestoreConfigurationBackup(ctx context.Context, adapterID, fileID, backupID, revision string) (ConfigDocument, error) {
	target, err := m.resolveAdapterTarget(ctx, adapterID, fileID)
	if err != nil {
		return ConfigDocument{}, err
	}
	return m.restoreBackup(target, backupID, revision)
}

func (m Manager) ListModConfigFiles(ctx context.Context, modID string) ([]ConfigFile, error) {
	root, err := m.resolveModConfigRoot(ctx, modID)
	if err != nil {
		return nil, err
	}
	return m.listConfigTargets(configTarget{scope: "mod:" + modID, root: root, path: root})
}

func (m Manager) GetModConfigFile(ctx context.Context, modID, fileID string) (ConfigDocument, error) {
	target, err := m.resolveModFileTarget(ctx, modID, fileID)
	if err != nil {
		return ConfigDocument{}, err
	}
	return m.readConfigTarget(target)
}

func (m Manager) WriteModConfigFile(ctx context.Context, modID, fileID string, request ConfigWriteRequest) (ConfigDocument, error) {
	target, err := m.resolveModFileTarget(ctx, modID, fileID)
	if err != nil {
		return ConfigDocument{}, err
	}
	return m.writeConfigTarget(target, request)
}

func (m Manager) ListModConfigBackups(ctx context.Context, modID, fileID string) ([]ConfigBackup, error) {
	target, err := m.resolveModFileTarget(ctx, modID, fileID)
	if err != nil {
		return nil, err
	}
	return m.listBackups(target)
}

func (m Manager) RestoreModConfigBackup(ctx context.Context, modID, fileID, backupID, revision string) (ConfigDocument, error) {
	target, err := m.resolveModFileTarget(ctx, modID, fileID)
	if err != nil {
		return ConfigDocument{}, err
	}
	return m.restoreBackup(target, backupID, revision)
}

func (m Manager) resolveAdapterTarget(ctx context.Context, adapterID, fileID string) (configTarget, error) {
	for _, definition := range adapterRegistry() {
		if definition.ID != adapterID {
			continue
		}
		targets, err := definition.resolve(m, ctx)
		if err != nil {
			return configTarget{}, configFailure("configuration_unavailable", err)
		}
		if len(targets) == 0 {
			return configTarget{}, configFailure("configuration_unavailable", fs.ErrNotExist)
		}
		if fileID == "" && len(targets) == 1 {
			return targets[0], nil
		}
		for _, target := range targets {
			if configFileID(target.scope, target.relative) == fileID {
				return target, nil
			}
		}
		return configTarget{}, configFailure("configuration_file_not_found", fs.ErrNotExist)
	}
	return configTarget{}, configFailure("configuration_adapter_not_found", fs.ErrNotExist)
}

func (m Manager) resolveModFileTarget(ctx context.Context, modID, fileID string) (configTarget, error) {
	root, err := m.resolveModConfigRoot(ctx, modID)
	if err != nil {
		return configTarget{}, err
	}
	targets, err := m.scanConfigTargets(configTarget{scope: "mod:" + modID, root: root, path: root})
	if err != nil {
		return configTarget{}, err
	}
	for _, target := range targets {
		if configFileID(target.scope, target.relative) == fileID {
			return target, nil
		}
	}
	return configTarget{}, configFailure("configuration_file_not_found", fs.ErrNotExist)
}

func (m Manager) resolveModConfigRoot(ctx context.Context, modID string) (string, error) {
	if strings.TrimSpace(modID) == "" {
		return "", configFailure("mod_not_found", fs.ErrNotExist)
	}
	if record, err := m.store.GetMod(ctx, modID); err == nil {
		return m.validateConfigRoot(record.Path)
	}
	scan, err := m.ScanLocal(ctx)
	if err != nil {
		return "", configFailure("mod_scan_failed", err)
	}
	for _, finding := range scan.Findings {
		if finding.ID != modID {
			continue
		}
		if finding.Source == LocalModSourceLegacyPak || finding.Source == LocalModSourceDatabase {
			return "", configFailure("mod_has_no_config_root", fs.ErrNotExist)
		}
		root := commonConfigRoot(finding.Paths)
		if root == "" {
			return "", configFailure("mod_has_no_config_root", fs.ErrNotExist)
		}
		for _, broadRoot := range []string{m.cfg.LegacyModsDir(), filepath.Join(m.cfg.Win64Dir(), "Mods")} {
			if sameScanPath(root, broadRoot) {
				return "", configFailure("mod_has_no_config_root", fs.ErrNotExist)
			}
		}
		return m.validateConfigRoot(root)
	}
	return "", configFailure("mod_not_found", fs.ErrNotExist)
}

func (m Manager) validateConfigRoot(root string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(strings.TrimSpace(root)))
	if err != nil {
		return "", configFailure("unsafe_configuration_path", err)
	}
	if err := m.cfg.ValidateManagedPath(absolute, false); err != nil {
		return "", configFailure("unsafe_configuration_path", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", configFailure("configuration_root_not_found", err)
	}
	linked, linkErr := localScanPathIsLink(absolute, info)
	if linkErr != nil || linked {
		return "", configFailure("unsafe_configuration_path", fmt.Errorf("configuration root is a link or reparse point"))
	}
	if !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}
	return absolute, nil
}

func commonConfigRoot(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	root := filepath.Clean(paths[0])
	if info, err := os.Lstat(root); err == nil && !info.IsDir() {
		root = filepath.Dir(root)
	}
	for _, path := range paths[1:] {
		candidate := filepath.Clean(path)
		if info, err := os.Lstat(candidate); err == nil && !info.IsDir() {
			candidate = filepath.Dir(candidate)
		}
		for !scanPathWithin(root, candidate) {
			parent := filepath.Dir(root)
			if parent == root {
				return ""
			}
			root = parent
		}
	}
	return root
}

func (m Manager) listConfigTargets(base configTarget) ([]ConfigFile, error) {
	targets, err := m.scanConfigTargets(base)
	if err != nil {
		return nil, err
	}
	out := make([]ConfigFile, 0, len(targets))
	for _, target := range targets {
		file, err := m.describeConfigTarget(target)
		if err != nil {
			var configErr ConfigurationError
			if errors.As(err, &configErr) && (configErr.Code == "configuration_not_utf8" || configErr.Code == "configuration_file_too_large") {
				continue
			}
			return nil, err
		}
		out = append(out, file)
	}
	return out, nil
}

func (m Manager) scanConfigTargets(base configTarget) ([]configTarget, error) {
	root, err := m.validateConfigRoot(base.root)
	if err != nil {
		return nil, err
	}
	var targets []configTarget
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if len(targets) >= 256 {
			return fs.SkipAll
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		linked, err := localScanPathIsLink(path, info)
		if err != nil {
			return err
		}
		if linked {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			relative, _ := filepath.Rel(root, path)
			if relative != "." && len(strings.Split(filepath.ToSlash(relative), "/")) > 12 {
				return fs.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() || !isEditableConfigName(entry.Name()) || info.Size() > MaxConfigFileBytes {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || unsafeRelativePath(relative) {
			return nil
		}
		targets = append(targets, configTarget{scope: base.scope, root: root, path: path, relative: filepath.ToSlash(relative)})
		return nil
	})
	if err != nil {
		return nil, configFailure("configuration_scan_failed", err)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].relative < targets[j].relative })
	return targets, nil
}

func (m Manager) describeConfigTarget(target configTarget) (ConfigFile, error) {
	content, info, err := m.secureReadTarget(target)
	if err != nil {
		return ConfigFile{}, err
	}
	extension := strings.ToLower(filepath.Ext(target.path))
	executable := extension == ".lua"
	risk := ""
	if executable {
		risk = "Lua 是可执行代码；保存前必须确认，并在重启服务器前审查差异。"
	}
	return ConfigFile{
		ID: configFileID(target.scope, target.relative), Name: filepath.Base(target.path), Path: target.relative,
		Extension: extension, Size: info.Size(), ModifiedAt: info.ModTime().UTC(), Revision: configRevision(content),
		Executable: executable, Risk: risk,
	}, nil
}

func (m Manager) readConfigTarget(target configTarget) (ConfigDocument, error) {
	content, _, err := m.secureReadTarget(target)
	if err != nil {
		return ConfigDocument{}, err
	}
	file, err := m.describeConfigTarget(target)
	if err != nil {
		return ConfigDocument{}, err
	}
	return ConfigDocument{File: file, Content: string(content), Format: strings.TrimPrefix(file.Extension, "."), Fields: configFields(file.Extension, content)}, nil
}

func (m Manager) secureReadTarget(target configTarget) ([]byte, fs.FileInfo, error) {
	if err := m.validateTarget(target); err != nil {
		return nil, nil, err
	}
	info, err := os.Lstat(target.path)
	if err != nil {
		return nil, nil, configFailure("configuration_file_not_found", err)
	}
	linked, linkErr := localScanPathIsLink(target.path, info)
	if linkErr != nil || linked || !info.Mode().IsRegular() {
		return nil, nil, configFailure("unsafe_configuration_path", fmt.Errorf("configuration file is not a regular non-linked file"))
	}
	if info.Size() > MaxConfigFileBytes {
		return nil, nil, configFailure("configuration_file_too_large", fmt.Errorf("configuration file exceeds 1 MiB"))
	}
	content, err := os.ReadFile(target.path)
	if err != nil {
		return nil, nil, configFailure("configuration_read_failed", err)
	}
	if err := validateTextSafety(content); err != nil {
		return nil, nil, err
	}
	return content, info, nil
}

func (m Manager) validateTarget(target configTarget) error {
	if unsafeRelativePath(target.relative) || !isEditableConfigName(filepath.Base(target.path)) {
		return configFailure("configuration_file_forbidden", fmt.Errorf("file is not editable"))
	}
	root, err := m.validateConfigRoot(target.root)
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(filepath.Clean(target.path))
	if err != nil || !scanPathWithin(root, absolute) {
		return configFailure("unsafe_configuration_path", fmt.Errorf("configuration path escapes Mod root"))
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil || filepath.ToSlash(relative) != filepath.ToSlash(target.relative) {
		return configFailure("unsafe_configuration_path", fmt.Errorf("configuration file identity changed"))
	}
	for current := absolute; !sameScanPath(current, root); current = filepath.Dir(current) {
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return configFailure("configuration_file_not_found", statErr)
		}
		linked, linkErr := localScanPathIsLink(current, info)
		if linkErr != nil || linked {
			return configFailure("unsafe_configuration_path", fmt.Errorf("configuration path contains a link or reparse point"))
		}
	}
	return nil
}

func (m Manager) writeConfigTarget(target configTarget, request ConfigWriteRequest) (ConfigDocument, error) {
	content := []byte(request.Content)
	if int64(len(content)) > MaxConfigFileBytes {
		return ConfigDocument{}, configFailure("configuration_file_too_large", fmt.Errorf("configuration file exceeds 1 MiB"))
	}
	extension := strings.ToLower(filepath.Ext(target.path))
	if extension == ".lua" && !request.ConfirmExecutable {
		return ConfigDocument{}, configFailure("executable_confirmation_required", fmt.Errorf("saving Lua requires explicit confirmation"))
	}
	if err := validateTextContent(extension, content); err != nil {
		return ConfigDocument{}, err
	}
	current, info, err := m.secureReadTarget(target)
	if err != nil {
		return ConfigDocument{}, err
	}
	if strings.TrimSpace(request.Revision) == "" || request.Revision != configRevision(current) {
		return ConfigDocument{}, configFailure("configuration_revision_conflict", fmt.Errorf("configuration changed since it was opened"))
	}
	if string(current) == request.Content {
		return m.readConfigTarget(target)
	}
	if _, err := m.createBackup(target, current); err != nil {
		return ConfigDocument{}, err
	}
	if err := m.atomicReplace(target, content, info.Mode().Perm()); err != nil {
		return ConfigDocument{}, err
	}
	return m.readConfigTarget(target)
}

func (m Manager) atomicReplace(target configTarget, content []byte, mode fs.FileMode) error {
	if err := m.validateTarget(target); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target.path), ".palpanel-config-*.tmp")
	if err != nil {
		return configFailure("configuration_write_failed", err)
	}
	temporaryPath := temporary.Name()
	complete := false
	defer func() {
		_ = temporary.Close()
		if !complete {
			_ = os.Remove(temporaryPath)
		}
	}()
	if mode == 0 {
		mode = 0o600
	}
	if err := temporary.Chmod(mode); err != nil {
		return configFailure("configuration_write_failed", err)
	}
	if _, err := temporary.Write(content); err != nil {
		return configFailure("configuration_write_failed", err)
	}
	if err := temporary.Sync(); err != nil {
		return configFailure("configuration_write_failed", err)
	}
	if err := temporary.Close(); err != nil {
		return configFailure("configuration_write_failed", err)
	}
	previous := target.path + ".palpanel-old-" + id.New("config")
	if err := os.Rename(target.path, previous); err != nil {
		return configFailure("configuration_write_failed", err)
	}
	if err := os.Rename(temporaryPath, target.path); err != nil {
		_ = os.Rename(previous, target.path)
		return configFailure("configuration_write_failed", err)
	}
	_ = os.Remove(previous)
	complete = true
	return nil
}

func validateTextContent(extension string, content []byte) error {
	if err := validateTextSafety(content); err != nil {
		return err
	}
	switch strings.ToLower(extension) {
	case ".json":
		var value any
		if err := json.Unmarshal(content, &value); err != nil {
			return configFailure("configuration_parse_failed", fmt.Errorf("invalid JSON: %w", err))
		}
	case ".yaml", ".yml":
		var value any
		if err := yaml.Unmarshal(content, &value); err != nil {
			return configFailure("configuration_parse_failed", fmt.Errorf("invalid YAML: %w", err))
		}
	}
	return nil
}

func validateTextSafety(content []byte) error {
	if !utf8.Valid(content) || strings.IndexByte(string(content), 0) >= 0 {
		return configFailure("configuration_not_utf8", fmt.Errorf("configuration must be UTF-8 text without NUL bytes"))
	}
	return nil
}

func (m Manager) createBackup(target configTarget, content []byte) (ConfigBackup, error) {
	directory := m.backupDirectory(target)
	if err := m.cfg.ValidateManagedPath(directory, false); err != nil {
		return ConfigBackup{}, configFailure("configuration_backup_failed", err)
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return ConfigBackup{}, configFailure("configuration_backup_failed", err)
	}
	backupID := time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + id.New("backup")
	path := filepath.Join(directory, backupID+".bak")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return ConfigBackup{}, configFailure("configuration_backup_failed", err)
	}
	return ConfigBackup{ID: backupID, Revision: configRevision(content), Size: int64(len(content)), CreatedAt: time.Now().UTC()}, nil
}

func (m Manager) listBackups(target configTarget) ([]ConfigBackup, error) {
	directory := m.backupDirectory(target)
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return []ConfigBackup{}, nil
	}
	if err != nil {
		return nil, configFailure("configuration_backup_read_failed", err)
	}
	out := make([]ConfigBackup, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".bak") {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		linked, linkErr := localScanPathIsLink(filepath.Join(directory, entry.Name()), info)
		if linkErr != nil || linked || !info.Mode().IsRegular() || info.Size() > MaxConfigFileBytes {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(directory, entry.Name()))
		if readErr != nil {
			continue
		}
		out = append(out, ConfigBackup{ID: strings.TrimSuffix(entry.Name(), ".bak"), Revision: configRevision(body), Size: info.Size(), CreatedAt: info.ModTime().UTC()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (m Manager) restoreBackup(target configTarget, backupID, revision string) (ConfigDocument, error) {
	if backupID == "" || strings.ContainsAny(backupID, `/\\`) || strings.Contains(backupID, "..") {
		return ConfigDocument{}, configFailure("configuration_backup_not_found", fs.ErrNotExist)
	}
	current, info, err := m.secureReadTarget(target)
	if err != nil {
		return ConfigDocument{}, err
	}
	if revision == "" || revision != configRevision(current) {
		return ConfigDocument{}, configFailure("configuration_revision_conflict", fmt.Errorf("configuration changed since it was opened"))
	}
	backupPath := filepath.Join(m.backupDirectory(target), backupID+".bak")
	backupInfo, err := os.Lstat(backupPath)
	if err != nil {
		return ConfigDocument{}, configFailure("configuration_backup_not_found", fs.ErrNotExist)
	}
	linked, linkErr := localScanPathIsLink(backupPath, backupInfo)
	if linkErr != nil || linked || !backupInfo.Mode().IsRegular() || backupInfo.Size() > MaxConfigFileBytes {
		return ConfigDocument{}, configFailure("configuration_backup_not_found", fs.ErrNotExist)
	}
	body, err := os.ReadFile(backupPath)
	if err != nil {
		return ConfigDocument{}, configFailure("configuration_backup_read_failed", err)
	}
	if err := validateTextContent(filepath.Ext(target.path), body); err != nil {
		return ConfigDocument{}, err
	}
	if _, err := m.createBackup(target, current); err != nil {
		return ConfigDocument{}, err
	}
	if err := m.atomicReplace(target, body, info.Mode().Perm()); err != nil {
		return ConfigDocument{}, err
	}
	return m.readConfigTarget(target)
}

func (m Manager) backupDirectory(target configTarget) string {
	hash := sha256.Sum256([]byte(target.scope + "\x00" + target.relative))
	return filepath.Join(m.cfg.BackupsDir, "mod-config", hex.EncodeToString(hash[:16]))
}

func resolvePalDefenderAdapter(m Manager, _ context.Context) ([]configTarget, error) {
	root := m.cfg.PalDefenderDir()
	return existingTargets("adapter:paldefender", root, []string{"Config.json"})
}

func resolveUE4SSAdapter(m Manager, _ context.Context) ([]configTarget, error) {
	root := m.cfg.Win64Dir()
	return existingTargets("adapter:ue4ss", root, []string{"UE4SS-settings.ini", filepath.Join("Mods", "mods.txt")})
}

func resolvePalSchemaAdapter(m Manager, ctx context.Context) ([]configTarget, error) {
	root, err := m.findKnownModRoot(ctx, "3625280368", "palschema")
	if err != nil {
		return nil, err
	}
	targets, err := m.scanConfigTargets(configTarget{scope: "adapter:palschema", root: root, path: root})
	if err != nil {
		return nil, err
	}
	jsonTargets := targets[:0]
	for _, target := range targets {
		if strings.EqualFold(filepath.Ext(target.path), ".json") {
			jsonTargets = append(jsonTargets, target)
		}
	}
	return jsonTargets, nil
}

func resolveExtendedBaseRangeAdapter(m Manager, ctx context.Context) ([]configTarget, error) {
	root, err := m.findKnownModRoot(ctx, "3625907101", "extended base range")
	if err != nil {
		return nil, err
	}
	var targets []configTarget
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || len(targets) > 0 {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(entry.Name(), "main.lua") {
			relative, _ := filepath.Rel(root, path)
			targets = append(targets, configTarget{scope: "adapter:extended-base-range", root: root, path: path, relative: filepath.ToSlash(relative)})
		}
		return nil
	})
	return targets, nil
}

func (m Manager) findKnownModRoot(ctx context.Context, workshopID, name string) (string, error) {
	mods, err := m.store.ListMods(ctx)
	if err != nil {
		return "", err
	}
	for _, mod := range mods {
		if mod.WorkshopID == workshopID || strings.Contains(strings.ToLower(mod.Name+" "+mod.PackageName), strings.ToLower(name)) {
			return m.validateConfigRoot(mod.Path)
		}
	}
	scan, err := m.ScanLocal(ctx)
	if err != nil {
		return "", err
	}
	for _, finding := range scan.Findings {
		if strings.Contains(strings.ToLower(finding.Name+" "+finding.PackageName), strings.ToLower(name)) {
			return m.validateConfigRoot(commonConfigRoot(finding.Paths))
		}
	}
	return "", fs.ErrNotExist
}

func existingTargets(scope, root string, relatives []string) ([]configTarget, error) {
	out := make([]configTarget, 0, len(relatives))
	for _, relative := range relatives {
		path := filepath.Join(root, relative)
		if _, err := os.Lstat(path); err == nil {
			out = append(out, configTarget{scope: scope, root: root, path: path, relative: filepath.ToSlash(relative)})
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return out, nil
}

func configFields(extension string, content []byte) []ConfigurationField {
	switch strings.ToLower(extension) {
	case ".json":
		var value any
		if json.Unmarshal(content, &value) == nil {
			var fields []ConfigurationField
			flattenJSONFields("", value, &fields)
			if len(fields) > 160 {
				fields = fields[:160]
			}
			return fields
		}
	case ".ini", ".cfg", ".toml":
		return keyValueFields(string(content))
	case ".lua":
		return luaNumericFields(string(content))
	}
	return nil
}

func flattenJSONFields(path string, value any, out *[]ConfigurationField) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := key
			if path != "" {
				next = path + "." + key
			}
			flattenJSONFields(next, typed[key], out)
		}
	case bool:
		*out = append(*out, ConfigurationField{Path: path, Label: fieldLabel(path), Type: "boolean", Value: typed})
	case float64:
		fieldType := "number"
		if typed == float64(int64(typed)) {
			fieldType = "integer"
		}
		*out = append(*out, ConfigurationField{Path: path, Label: fieldLabel(path), Type: fieldType, Value: typed})
	case string:
		*out = append(*out, ConfigurationField{Path: path, Label: fieldLabel(path), Type: "string", Value: typed})
	}
}

func keyValueFields(content string) []ConfigurationField {
	section := ""
	var out []ConfigurationField
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, raw, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		path := key
		if section != "" {
			path = section + "." + key
		}
		value, fieldType := parseScalar(strings.TrimSpace(raw))
		out = append(out, ConfigurationField{Path: path, Label: fieldLabel(path), Type: fieldType, Value: value})
		if len(out) >= 160 {
			break
		}
	}
	return out
}

func luaNumericFields(content string) []ConfigurationField {
	var out []ConfigurationField
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		left, right, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(left), "local "))
		if name == "" || strings.ContainsAny(name, " [](){}") {
			continue
		}
		raw := strings.TrimSpace(strings.SplitN(right, "--", 2)[0])
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		min, max := int64(0), int64(1000000)
		out = append(out, ConfigurationField{Path: name, Label: fieldLabel(name), Type: "number", Value: value, Min: &min, Max: &max})
		if len(out) >= 80 {
			break
		}
	}
	return out
}

func parseScalar(raw string) (any, string) {
	if value, err := strconv.ParseBool(strings.ToLower(raw)); err == nil {
		return value, "boolean"
	}
	if value, err := strconv.ParseFloat(raw, 64); err == nil {
		return value, "number"
	}
	return strings.Trim(raw, `"'`), "string"
}

func fieldLabel(path string) string {
	parts := strings.Split(path, ".")
	return parts[len(parts)-1]
}

func isEditableConfigName(name string) bool {
	if forbiddenConfigNames[strings.ToLower(strings.TrimSpace(name))] {
		return false
	}
	return editableConfigExtensions[strings.ToLower(filepath.Ext(name))]
}

func unsafeRelativePath(relative string) bool {
	clean := filepath.Clean(strings.TrimSpace(relative))
	return clean == "" || clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator))
}

func configRevision(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func configFileID(scope, relative string) string {
	hash := sha256.Sum256([]byte(scope + "\x00" + filepath.ToSlash(relative)))
	return hex.EncodeToString(hash[:16])
}

func configFailure(code string, err error) error { return ConfigurationError{Code: code, Err: err} }
