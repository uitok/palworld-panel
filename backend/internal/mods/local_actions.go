package mods

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"palpanel/internal/db"
	"palpanel/internal/id"
)

const localIgnoredFindingsKey = "mods.local.ignored.v1"

type LocalModAction string

const (
	LocalModActionImport   LocalModAction = "import"
	LocalModActionRepair   LocalModAction = "repair"
	LocalModActionIgnore   LocalModAction = "ignore"
	LocalModActionUnignore LocalModAction = "unignore"
	LocalModActionDelete   LocalModAction = "delete"
)

type LocalModActionCapability struct {
	Action               LocalModAction `json:"action"`
	Available            bool           `json:"available"`
	ConfirmationRequired bool           `json:"confirmation_required"`
	Reason               string         `json:"reason,omitempty"`
}

type LocalModActionRequest struct {
	Action   LocalModAction `json:"action"`
	Revision string         `json:"revision"`
	Confirm  bool           `json:"confirm,omitempty"`
}

type LocalModActionResult struct {
	Action    LocalModAction  `json:"action"`
	FindingID string          `json:"finding_id"`
	Message   string          `json:"message"`
	Mod       *db.Mod         `json:"mod,omitempty"`
	Scan      LocalScanResult `json:"scan"`
}

type LocalModActionError struct {
	Code string
	Err  error
}

func (e LocalModActionError) Error() string {
	if e.Err == nil {
		return e.Code
	}
	return e.Err.Error()
}

func (e LocalModActionError) Unwrap() error { return e.Err }

type localActionState struct {
	operations sync.Mutex
	ignored    sync.Mutex
}

type localDiskStamp struct {
	Path     string `json:"path"`
	Exists   bool   `json:"exists"`
	Mode     uint32 `json:"mode,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Modified int64  `json:"modified,omitempty"`
	Linked   bool   `json:"linked,omitempty"`
	Error    string `json:"error,omitempty"`
}

func localFindingID(serverDir string, finding localScanFinding) string {
	anchor := localFindingAnchor(finding)
	material := strings.Join([]string{
		string(finding.Source),
		strings.ToLower(strings.TrimSpace(finding.identity)),
		localRelativePath(serverDir, anchor),
	}, "\x00")
	sum := sha256.Sum256([]byte(material))
	return "localmod_" + hex.EncodeToString(sum[:16])
}

func localFindingAnchor(finding localScanFinding) string {
	for _, path := range finding.Paths {
		if strings.EqualFold(filepath.Base(path), "Info.json") {
			return filepath.Dir(path)
		}
	}
	if finding.Source == LocalModSourceLegacyPak && len(finding.Paths) > 0 {
		return filepath.Dir(finding.Paths[0])
	}
	if len(finding.matchPaths) > 0 {
		return finding.matchPaths[0]
	}
	if len(finding.Paths) > 0 {
		return finding.Paths[0]
	}
	if len(finding.DatabaseMods) > 0 {
		return "database:" + finding.DatabaseMods[0].ID
	}
	return finding.Name
}

func localRelativePath(serverDir, path string) string {
	if strings.HasPrefix(path, "database:") {
		return strings.ToLower(path)
	}
	relative, err := filepath.Rel(serverDir, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return scanPathKey(path)
	}
	return strings.ToLower(filepath.ToSlash(filepath.Clean(relative)))
}

func localFindingRevision(serverDir string, finding LocalModFinding) string {
	paths := append([]string(nil), finding.Paths...)
	sort.Slice(paths, func(i, j int) bool { return scanPathKey(paths[i]) < scanPathKey(paths[j]) })
	stamps := make([]localDiskStamp, 0, len(paths))
	for _, path := range paths {
		stamp := localDiskStamp{Path: localRelativePath(serverDir, path)}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			stamps = append(stamps, stamp)
			continue
		}
		if err != nil {
			stamp.Error = err.Error()
			stamps = append(stamps, stamp)
			continue
		}
		stamp.Exists = true
		stamp.Mode = uint32(info.Mode())
		stamp.Size = info.Size()
		stamp.Modified = info.ModTime().UnixNano()
		linked, linkErr := localScanPathIsLink(path, info)
		if linkErr != nil {
			stamp.Error = linkErr.Error()
		} else {
			stamp.Linked = linked
		}
		stamps = append(stamps, stamp)
	}
	payload := struct {
		Finding LocalModFinding  `json:"finding"`
		Stamps  []localDiskStamp `json:"stamps"`
	}{Finding: finding, Stamps: stamps}
	payload.Finding.Revision = ""
	payload.Finding.Actions = nil
	body, _ := json.Marshal(payload)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (m Manager) decorateLocalScan(ctx context.Context, result *LocalScanResult) error {
	ignored, err := m.ignoredFindingIDs(ctx)
	if err != nil {
		return fmt.Errorf("load ignored local Mods: %w", err)
	}
	for index := range result.Findings {
		finding := &result.Findings[index]
		finding.Ignored = ignored[finding.ID]
		finding.Revision = localFindingRevision(result.ServerDir, *finding)
		finding.Actions = m.localActionCapabilities(ctx, *finding)
	}
	return nil
}

func (m Manager) localActionCapabilities(ctx context.Context, finding LocalModFinding) []LocalModActionCapability {
	capabilities := []LocalModActionCapability{
		{Action: LocalModActionImport},
		{Action: LocalModActionRepair},
		{Action: LocalModActionIgnore},
		{Action: LocalModActionUnignore},
		{Action: LocalModActionDelete, ConfirmationRequired: true},
	}
	for index := range capabilities {
		capability := &capabilities[index]
		switch capability.Action {
		case LocalModActionIgnore:
			capability.Available = !finding.Ignored
			if !capability.Available {
				capability.Reason = "This finding is already ignored."
			}
		case LocalModActionUnignore:
			capability.Available = finding.Ignored
			if !capability.Available {
				capability.Reason = "This finding is not ignored."
			}
		default:
			if finding.Ignored {
				capability.Reason = "Stop ignoring this finding before changing it."
				continue
			}
			switch capability.Action {
			case LocalModActionImport:
				_, _, err := m.localMetadataRoot(ctx, finding)
				capability.Available = err == nil && finding.Ownership == LocalModManual && len(finding.DatabaseMods) == 0 && !finding.Duplicate
				capability.Reason = localImportReason(finding, err, capability.Available)
			case LocalModActionRepair:
				_, _, err := m.localRepairRoot(ctx, finding)
				capability.Available = err == nil && finding.Ownership == LocalModManual && len(finding.DatabaseMods) == 0 && !finding.Duplicate
				if !capability.Available {
					capability.Reason = localRepairReason(finding, err)
				}
			case LocalModActionDelete:
				_, _, err := m.localDeleteTarget(ctx, finding)
				capability.Available = err == nil
				if err != nil {
					capability.Reason = err.Error()
				}
			}
		}
	}
	return capabilities
}

func localImportReason(finding LocalModFinding, err error, available bool) string {
	if available {
		return ""
	}
	if finding.Source == LocalModSourceUE4SS {
		return "UE4SS Mods cannot be imported safely by the current Info.json Workshop installer."
	}
	if finding.Source == LocalModSourceLegacyPak && !findingHasInfo(finding) {
		return "Pak Mods cannot be imported safely by the current Info.json Workshop installer."
	}
	if finding.State == LocalModUnknown || finding.State == LocalModIncomplete {
		return "Unknown or incomplete files are never imported automatically."
	}
	if finding.Ownership != LocalModManual || len(finding.DatabaseMods) > 0 {
		return "This finding already has a PalPanel database record."
	}
	if finding.Duplicate {
		return "Resolve duplicate Mod identities before importing."
	}
	if err != nil {
		return err.Error()
	}
	return "This Mod type is not supported by the current importer."
}

func localRepairReason(finding LocalModFinding, err error) string {
	if finding.Source == LocalModSourceUE4SS || (finding.Source == LocalModSourceLegacyPak && !findingHasInfo(finding)) {
		return "Only a valid Info.json Mod already located directly in the Workshop directory can be repaired."
	}
	if finding.Ownership != LocalModManual || len(finding.DatabaseMods) > 0 {
		return "This finding already has a PalPanel database record."
	}
	if finding.Duplicate {
		return "Resolve duplicate Mod identities before repairing the database record."
	}
	if err != nil {
		return err.Error()
	}
	return "This finding is not eligible for database repair."
}

func findingHasInfo(finding LocalModFinding) bool {
	for _, path := range finding.Paths {
		if strings.EqualFold(filepath.Base(path), "Info.json") {
			return true
		}
	}
	return false
}

func (m Manager) ActOnLocalFinding(ctx context.Context, findingID string, request LocalModActionRequest) (LocalModActionResult, error) {
	if m.local == nil {
		return LocalModActionResult{}, LocalModActionError{Code: "local_action_unavailable", Err: fmt.Errorf("local Mod actions are not initialized")}
	}
	findingID = strings.TrimSpace(findingID)
	if findingID == "" || strings.TrimSpace(request.Revision) == "" {
		return LocalModActionResult{}, LocalModActionError{Code: "invalid_local_action", Err: fmt.Errorf("finding id and revision are required")}
	}
	if !validLocalAction(request.Action) {
		return LocalModActionResult{}, LocalModActionError{Code: "invalid_local_action", Err: fmt.Errorf("unsupported local Mod action %q", request.Action)}
	}

	m.local.operations.Lock()
	defer m.local.operations.Unlock()

	scan, finding, err := m.currentLocalFinding(ctx, findingID)
	if err != nil {
		return LocalModActionResult{}, err
	}
	if finding.Revision != request.Revision {
		return LocalModActionResult{}, LocalModActionError{Code: "local_finding_stale", Err: fmt.Errorf("local Mod finding changed; rescan before retrying")}
	}
	capability := localCapability(finding, request.Action)
	if !capability.Available {
		return LocalModActionResult{}, LocalModActionError{Code: "local_action_unsupported", Err: fmt.Errorf("%s", firstNonEmpty(capability.Reason, "action is not available for this finding"))}
	}
	if capability.ConfirmationRequired && !request.Confirm {
		return LocalModActionResult{}, LocalModActionError{Code: "local_action_confirmation_required", Err: fmt.Errorf("explicit confirmation is required")}
	}

	result := LocalModActionResult{Action: request.Action, FindingID: findingID}
	switch request.Action {
	case LocalModActionIgnore, LocalModActionUnignore:
		ignored := request.Action == LocalModActionIgnore
		if err := m.setFindingIgnored(ctx, findingID, ignored); err != nil {
			return LocalModActionResult{}, LocalModActionError{Code: "local_ignore_failed", Err: err}
		}
		if ignored {
			result.Message = "Local Mod finding is now ignored."
		} else {
			result.Message = "Local Mod finding is no longer ignored."
		}
	case LocalModActionImport:
		mod, actionErr := m.importLocalFinding(ctx, finding)
		if actionErr != nil {
			return LocalModActionResult{}, actionErr
		}
		result.Mod = &mod
		result.Message = "Local Mod was copied into the PalPanel-managed Workshop directory and imported disabled."
	case LocalModActionRepair:
		mod, actionErr := m.repairLocalFinding(ctx, finding)
		if actionErr != nil {
			return LocalModActionResult{}, actionErr
		}
		result.Mod = &mod
		result.Message = "Missing PalPanel database record was repaired without moving the existing Mod files."
	case LocalModActionDelete:
		_, record, actionErr := m.localDeleteTarget(ctx, finding)
		if actionErr != nil {
			return LocalModActionResult{}, LocalModActionError{Code: "local_delete_unsafe", Err: actionErr}
		}
		if actionErr := m.Delete(ctx, record.ID); actionErr != nil {
			return LocalModActionResult{}, LocalModActionError{Code: "local_delete_failed", Err: actionErr}
		}
		result.Message = "PalPanel-managed Mod files and database record were deleted."
	}
	result.Scan, err = m.ScanLocal(ctx)
	if err != nil {
		return LocalModActionResult{}, LocalModActionError{Code: "local_rescan_failed", Err: err}
	}
	_ = scan
	return result, nil
}

func validLocalAction(action LocalModAction) bool {
	switch action {
	case LocalModActionImport, LocalModActionRepair, LocalModActionIgnore, LocalModActionUnignore, LocalModActionDelete:
		return true
	default:
		return false
	}
}

func localCapability(finding LocalModFinding, action LocalModAction) LocalModActionCapability {
	for _, capability := range finding.Actions {
		if capability.Action == action {
			return capability
		}
	}
	return LocalModActionCapability{Action: action, Reason: "Action capability was not reported by the current scan."}
}

func (m Manager) currentLocalFinding(ctx context.Context, findingID string) (LocalScanResult, LocalModFinding, error) {
	scan, err := m.ScanLocal(ctx)
	if err != nil {
		return LocalScanResult{}, LocalModFinding{}, LocalModActionError{Code: "local_scan_failed", Err: err}
	}
	var matches []LocalModFinding
	for _, finding := range scan.Findings {
		if finding.ID == findingID {
			matches = append(matches, finding)
		}
	}
	if len(matches) != 1 {
		return scan, LocalModFinding{}, LocalModActionError{Code: "local_finding_stale", Err: fmt.Errorf("local Mod finding no longer exists or is ambiguous; rescan before retrying")}
	}
	return scan, matches[0], nil
}

func (m Manager) importLocalFinding(ctx context.Context, finding LocalModFinding) (db.Mod, error) {
	root, _, err := m.localMetadataRoot(ctx, finding)
	if err != nil {
		return db.Mod{}, LocalModActionError{Code: "local_action_unsupported", Err: err}
	}
	record, err := m.imports.newRecord(finding.Name)
	if err != nil {
		return db.Mod{}, LocalModActionError{Code: "local_import_staging_failed", Err: err}
	}
	defer func() { _ = m.removeManagedDirectory(record.directory) }()
	staged := filepath.Join(record.directory, "local-copy")
	if err := m.copyLocalTree(ctx, root, staged); err != nil {
		return db.Mod{}, LocalModActionError{Code: "local_import_copy_failed", Err: err}
	}
	_, current, err := m.currentLocalFinding(ctx, finding.ID)
	if err != nil || current.Revision != finding.Revision {
		return db.Mod{}, LocalModActionError{Code: "local_finding_stale", Err: fmt.Errorf("local Mod changed while it was being copied; no files were installed")}
	}
	mod, err := m.installPrepared(ctx, staged, "local_scan_import", "", WorkshopItem{}, false)
	if err != nil {
		return db.Mod{}, LocalModActionError{Code: "local_import_failed", Err: err}
	}
	return mod, nil
}

func (m Manager) repairLocalFinding(ctx context.Context, finding LocalModFinding) (db.Mod, error) {
	root, metadata, err := m.localRepairRoot(ctx, finding)
	if err != nil {
		return db.Mod{}, LocalModActionError{Code: "local_repair_unsafe", Err: err}
	}
	if existing, findErr := m.findModByPackage(ctx, metadata.PackageName); findErr != nil {
		return db.Mod{}, LocalModActionError{Code: "local_repair_conflict", Err: findErr}
	} else if existing != nil {
		return db.Mod{}, LocalModActionError{Code: "local_finding_stale", Err: fmt.Errorf("a database record for PackageName %q now exists", metadata.PackageName)}
	}
	if err := m.validateLocalActionTree(ctx, root); err != nil {
		return db.Mod{}, LocalModActionError{Code: "local_repair_unsafe", Err: err}
	}
	mod := db.Mod{
		ID: id.New("mod"), Name: metadata.Name, Source: "local_scan_repair", PackageName: metadata.PackageName,
		Path: root, Version: metadata.Version, Enabled: finding.Enabled,
	}
	if err := m.store.UpsertMod(ctx, mod); err != nil {
		return db.Mod{}, LocalModActionError{Code: "local_repair_failed", Err: err}
	}
	return mod, nil
}

func (m Manager) localMetadataRoot(ctx context.Context, finding LocalModFinding) (string, Info, error) {
	if finding.Ownership != LocalModManual || len(finding.DatabaseMods) > 0 {
		return "", Info{}, fmt.Errorf("only an unmanaged local Mod can be imported")
	}
	if finding.Duplicate || finding.State == LocalModUnknown || finding.State == LocalModIncomplete || finding.State == LocalModMissingFiles {
		return "", Info{}, fmt.Errorf("duplicate, unknown, incomplete, or missing Mod findings cannot be imported")
	}
	var infoPaths []string
	for _, path := range finding.Paths {
		if strings.EqualFold(filepath.Base(path), "Info.json") {
			infoPaths = append(infoPaths, path)
		}
	}
	if len(infoPaths) != 1 {
		return "", Info{}, fmt.Errorf("the current importer requires exactly one valid Info.json")
	}
	root := filepath.Dir(infoPaths[0])
	if err := m.validateLocalActionTree(ctx, root); err != nil {
		return "", Info{}, err
	}
	validated, metadata, err := inspectModDirectory(root)
	if err != nil {
		return "", Info{}, err
	}
	if !sameScanPath(validated, root) {
		return "", Info{}, fmt.Errorf("Info.json must be at the root of the recognized Mod directory")
	}
	return root, metadata, nil
}

func (m Manager) localRepairRoot(ctx context.Context, finding LocalModFinding) (string, Info, error) {
	root, metadata, err := m.localMetadataRoot(ctx, finding)
	if err != nil {
		return "", Info{}, err
	}
	if finding.Source != LocalModSourceWorkshop {
		return "", Info{}, fmt.Errorf("database repair is limited to Mods already located in the configured Workshop directory; use import to copy this Mod")
	}
	if !isDirectChild(m.cfg.WorkshopModsDir(), root) {
		return "", Info{}, fmt.Errorf("database repair requires a direct child of the configured Workshop directory")
	}
	return root, metadata, nil
}

func (m Manager) localDeleteTarget(ctx context.Context, finding LocalModFinding) (string, db.Mod, error) {
	if finding.Ownership != LocalModManaged || len(finding.DatabaseMods) != 1 {
		return "", db.Mod{}, fmt.Errorf("delete is limited to one unambiguous PalPanel-managed database record")
	}
	record := finding.DatabaseMods[0]
	target, err := palPanelOwnedModTarget(m.cfg.WorkshopModsDir(), record.ID, record.Path)
	if err != nil {
		return "", db.Mod{}, err
	}
	if _, statErr := os.Lstat(target); statErr == nil {
		if err := m.validateLocalActionTree(ctx, target); err != nil {
			return "", db.Mod{}, err
		}
	} else if !os.IsNotExist(statErr) {
		return "", db.Mod{}, statErr
	} else if err := m.validateLocalActionPath(target, true); err != nil {
		return "", db.Mod{}, err
	}
	return target, record, nil
}

func palPanelOwnedModTarget(root, modID, storedPath string) (string, error) {
	if strings.TrimSpace(storedPath) == "" {
		return "", fmt.Errorf("database record has no managed installation path")
	}
	target, err := safeModTarget(root, modID, storedPath)
	if err != nil {
		return "", err
	}
	if !isDirectChild(root, target) || !strings.EqualFold(filepath.Base(target), strings.TrimSpace(modID)) {
		return "", fmt.Errorf("managed Mod path must be a direct Workshop child named after Mod ID %q", modID)
	}
	return target, nil
}

func isDirectChild(root, target string) bool {
	rootAbsolute, rootErr := filepath.Abs(filepath.Clean(root))
	targetAbsolute, targetErr := filepath.Abs(filepath.Clean(target))
	if rootErr != nil || targetErr != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbsolute, targetAbsolute)
	return err == nil && relative != "." && relative != ".." && filepath.Dir(relative) == "."
}

func (m Manager) validateLocalActionTree(ctx context.Context, root string) error {
	if err := m.validateLocalActionPath(root, false); err != nil {
		return err
	}
	entries := 0
	var totalBytes int64
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > maxArchiveFiles+1 {
			return fmt.Errorf("local Mod contains more than %d entries", maxArchiveFiles)
		}
		if err := m.validateLocalActionPath(path, false); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		linked, err := localScanPathIsLink(path, info)
		if err != nil {
			return err
		}
		if linked {
			return fmt.Errorf("local Mod path is a link or reparse point: %s", path)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("local Mod contains unsupported file type: %s", path)
		}
		if info.Mode().IsRegular() {
			totalBytes += info.Size()
			if totalBytes > maxExtractedBytes {
				return fmt.Errorf("local Mod exceeds the %d byte size limit", maxExtractedBytes)
			}
		}
		return nil
	})
}

func (m Manager) validateLocalActionPath(path string, allowMissing bool) error {
	serverDir, err := filepath.Abs(filepath.Clean(m.cfg.ServerDirectory()))
	if err != nil {
		return err
	}
	target, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}
	if sameScanPath(serverDir, target) || !scanPathWithin(serverDir, target) {
		return fmt.Errorf("local Mod action target must be below ServerDir")
	}
	if err := m.cfg.ValidateManagedPath(target, false); err != nil {
		return err
	}
	if m.localActionPathProtected(target) {
		return fmt.Errorf("local Mod action target overlaps a protected repository or runtime root")
	}
	relative, err := filepath.Rel(serverDir, target)
	if err != nil {
		return err
	}
	current := serverDir
	for _, component := range strings.Split(relative, string(os.PathSeparator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) && allowMissing {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		linked, linkErr := localScanPathIsLink(current, info)
		if linkErr != nil {
			return linkErr
		}
		if linked {
			return fmt.Errorf("local Mod action path crosses a link or reparse point: %s", current)
		}
	}
	return nil
}

func (m Manager) localActionPathProtected(target string) bool {
	for _, root := range []string{m.cfg.RuntimeRoot, m.cfg.DataDir, m.cfg.ServerDirectory(), m.cfg.RepositoryRoot} {
		if strings.TrimSpace(root) != "" && sameScanPath(root, target) {
			return true
		}
	}
	if strings.TrimSpace(m.cfg.RepositoryRoot) == "" {
		return false
	}
	for _, name := range []string{".git", "backend", "frontend", "sav-cli", "scripts"} {
		if scanPathWithin(filepath.Join(m.cfg.RepositoryRoot, name), target) {
			return true
		}
	}
	return false
}

func (m Manager) copyLocalTree(ctx context.Context, source, destination string) error {
	if err := m.validateLocalActionTree(ctx, source); err != nil {
		return err
	}
	if err := m.cfg.ValidateManagedPath(destination, false); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		if err == nil {
			return fmt.Errorf("local Mod staging destination already exists")
		}
		return err
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := m.validateLocalActionPath(path, false); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		linked, err := localScanPathIsLink(path, info)
		if err != nil || linked {
			if err != nil {
				return err
			}
			return fmt.Errorf("local Mod changed into a link or reparse point while copying: %s", path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("local Mod copy path escaped source root")
		}
		target := filepath.Join(destination, relative)
		if !scanPathWithin(destination, target) {
			return fmt.Errorf("local Mod copy path escaped staging root")
		}
		if err := m.cfg.ValidateManagedPath(target, false); err != nil {
			return err
		}
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("local Mod contains unsupported file type: %s", path)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyLocalRegularFile(ctx, path, target, info)
	})
}

func copyLocalRegularFile(ctx context.Context, source, destination string, before fs.FileInfo) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	opened, err := in.Stat()
	if err != nil {
		return err
	}
	if !opened.Mode().IsRegular() || opened.Size() != before.Size() || !opened.ModTime().Equal(before.ModTime()) {
		return fmt.Errorf("local Mod file changed before copy: %s", source)
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			_ = out.Close()
			_ = os.Remove(destination)
			return err
		}
		read, readErr := in.Read(buffer)
		if read > 0 {
			if _, writeErr := out.Write(buffer[:read]); writeErr != nil {
				_ = out.Close()
				_ = os.Remove(destination)
				return writeErr
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = out.Close()
			_ = os.Remove(destination)
			return readErr
		}
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(destination)
		return err
	}
	after, err := os.Lstat(source)
	if err != nil || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || after.Mode() != before.Mode() {
		_ = os.Remove(destination)
		return fmt.Errorf("local Mod file changed while copying: %s", source)
	}
	return nil
}

func (m Manager) ignoredFindingIDs(ctx context.Context) (map[string]bool, error) {
	if m.local == nil {
		return map[string]bool{}, nil
	}
	m.local.ignored.Lock()
	defer m.local.ignored.Unlock()
	return m.readIgnoredFindingIDs(ctx)
}

func (m Manager) readIgnoredFindingIDs(ctx context.Context) (map[string]bool, error) {
	value, found, err := m.store.GetKV(ctx, localIgnoredFindingsKey)
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	if !found || strings.TrimSpace(value) == "" {
		return result, nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(value), &ids); err != nil {
		return nil, fmt.Errorf("decode ignored local Mod findings: %w", err)
	}
	for _, findingID := range ids {
		if strings.HasPrefix(findingID, "localmod_") {
			result[findingID] = true
		}
	}
	return result, nil
}

func (m Manager) setFindingIgnored(ctx context.Context, findingID string, ignored bool) error {
	m.local.ignored.Lock()
	defer m.local.ignored.Unlock()
	ids, err := m.readIgnoredFindingIDs(ctx)
	if err != nil {
		return err
	}
	if ignored {
		ids[findingID] = true
	} else {
		delete(ids, findingID)
	}
	values := make([]string, 0, len(ids))
	for id := range ids {
		values = append(values, id)
	}
	sort.Strings(values)
	body, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return m.store.SetKV(ctx, localIgnoredFindingsKey, string(body))
}
