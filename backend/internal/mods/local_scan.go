package mods

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"palpanel/internal/db"
)

type LocalModOwnership string

const (
	LocalModManaged LocalModOwnership = "managed"
	LocalModManual  LocalModOwnership = "manual"
)

type LocalModState string

const (
	LocalModPresent      LocalModState = "present"
	LocalModMissingFiles LocalModState = "missing_files"
	LocalModUnknown      LocalModState = "unknown"
	LocalModDisabled     LocalModState = "disabled"
	LocalModDuplicate    LocalModState = "duplicate"
	LocalModIncomplete   LocalModState = "incomplete"
)

type LocalModSource string

const (
	LocalModSourceWorkshop  LocalModSource = "workshop"
	LocalModSourceLegacyPak LocalModSource = "legacy_pak"
	LocalModSourceUE4SS     LocalModSource = "ue4ss"
	LocalModSourceDatabase  LocalModSource = "database"
)

type LocalModConfidence string

const (
	LocalModConfidenceHigh   LocalModConfidence = "high"
	LocalModConfidenceMedium LocalModConfidence = "medium"
	LocalModConfidenceLow    LocalModConfidence = "low"
)

type LocalModClassification string

const (
	LocalModClassificationManaged      LocalModClassification = "managed"
	LocalModClassificationManual       LocalModClassification = "manual"
	LocalModClassificationPresent      LocalModClassification = "present"
	LocalModClassificationMissingFiles LocalModClassification = "missing_files"
	LocalModClassificationUnknown      LocalModClassification = "unknown"
	LocalModClassificationDisabled     LocalModClassification = "disabled"
	LocalModClassificationDuplicate    LocalModClassification = "duplicate"
	LocalModClassificationIncomplete   LocalModClassification = "incomplete"
)

// LocalModFinding is a read-only reconciliation view of one on-disk Mod and
// any database records that refer to it. A finding can be both disabled and a
// duplicate, so Classifications preserves the complete set of labels while
// State carries the primary actionable state.
type LocalModFinding struct {
	ID              string                     `json:"id"`
	Revision        string                     `json:"revision"`
	Ownership       LocalModOwnership          `json:"ownership"`
	State           LocalModState              `json:"state"`
	Source          LocalModSource             `json:"source"`
	Confidence      LocalModConfidence         `json:"confidence"`
	Name            string                     `json:"name"`
	PackageName     string                     `json:"package_name,omitempty"`
	Version         string                     `json:"version,omitempty"`
	Enabled         bool                       `json:"enabled"`
	Duplicate       bool                       `json:"duplicate"`
	Paths           []string                   `json:"paths"`
	DatabaseMods    []db.Mod                   `json:"database_mods,omitempty"`
	Classifications []LocalModClassification   `json:"classifications"`
	Issues          []string                   `json:"issues,omitempty"`
	Ignored         bool                       `json:"ignored"`
	Actions         []LocalModActionCapability `json:"actions"`
}

type LocalScanResult struct {
	ServerDir    string            `json:"server_dir"`
	ScannedAt    string            `json:"scanned_at"`
	Findings     []LocalModFinding `json:"findings"`
	SkippedPaths []string          `json:"skipped_paths"`
	Warnings     []string          `json:"warnings"`
}

type localScanFinding struct {
	LocalModFinding
	matchPaths   []string
	identity     string
	enabledKnown bool
}

type localScanEntry struct {
	path string
	info fs.FileInfo
}

type localScanner struct {
	ctx        context.Context
	serverDir  string
	database   []db.Mod
	findings   []localScanFinding
	skipped    []string
	warnings   []string
	rootExists bool
}

var errLocalScanSkippedLink = errors.New("local Mod scan skipped link or reparse point")

// ScanLocal scans the configured server directory and reconciles real files
// with db.Mod records. It never mutates the database or the scanned files.
func (m Manager) ScanLocal(ctx context.Context) (LocalScanResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if m.store == nil {
		return LocalScanResult{}, fmt.Errorf("local Mod scan requires a database store")
	}
	serverDir := strings.TrimSpace(m.cfg.ServerDir)
	if serverDir == "" {
		return LocalScanResult{}, fmt.Errorf("local Mod scan requires PALPANEL_SERVER_DIR")
	}
	absolute, err := filepath.Abs(filepath.Clean(serverDir))
	if err != nil {
		return LocalScanResult{}, fmt.Errorf("resolve server directory: %w", err)
	}
	database, err := m.store.ListMods(ctx)
	if err != nil {
		return LocalScanResult{}, fmt.Errorf("list database Mods: %w", err)
	}
	scanner := localScanner{ctx: ctx, serverDir: absolute, database: database, findings: make([]localScanFinding, 0)}
	if err := scanner.scan(); err != nil {
		return LocalScanResult{}, err
	}
	scanner.reconcileDatabase()
	scanner.markDuplicates()
	result := scanner.result()
	if err := m.decorateLocalScan(ctx, &result); err != nil {
		return LocalScanResult{}, err
	}
	return result, nil
}

func (s *localScanner) scan() error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(s.serverDir)
	if os.IsNotExist(err) {
		s.warnings = append(s.warnings, "server directory does not exist: "+s.serverDir)
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect server directory: %w", err)
	}
	linked, err := localScanPathIsLink(s.serverDir, info)
	if err != nil {
		return fmt.Errorf("inspect server directory attributes: %w", err)
	}
	if linked {
		return fmt.Errorf("server directory is a link or reparse point and will not be scanned: %s", s.serverDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("server directory is not a directory: %s", s.serverDir)
	}
	s.rootExists = true

	workshopRoots, err := s.resolveKnownDirectories("Mods", "Workshop")
	if err != nil {
		return err
	}
	for _, root := range workshopRoots {
		settings, known := s.readWorkshopSettings(filepath.Dir(root))
		if err := s.scanInfoRoot(root, LocalModSourceWorkshop, settings, known); err != nil {
			return err
		}
	}

	legacyInfoRoots, err := s.resolveKnownDirectories("Pal", "Content", "Paks", "LogicMods", "Mods")
	if err != nil {
		return err
	}
	for _, root := range legacyInfoRoots {
		if err := s.scanInfoRoot(root, LocalModSourceLegacyPak, ModSettings{}, false); err != nil {
			return err
		}
	}

	pakRoots := make([]string, 0)
	for _, components := range [][]string{
		{"Pal", "Content", "Paks", "~mods"},
		{"Pal", "Content", "Paks", "LogicMods"},
	} {
		roots, resolveErr := s.resolveKnownDirectories(components...)
		if resolveErr != nil {
			return resolveErr
		}
		pakRoots = appendUniquePaths(pakRoots, roots...)
	}
	for _, root := range pakRoots {
		skipLegacyInfoTree := strings.EqualFold(filepath.Base(root), "LogicMods")
		if err := s.scanPakRoot(root, skipLegacyInfoTree); err != nil {
			return err
		}
	}

	ue4ssRoots := make([]string, 0)
	for _, components := range [][]string{
		{"Pal", "Binaries", "Win64", "Mods"},
		{"Pal", "Binaries", "Win64", "UE4SS", "Mods"},
		{"Pal", "Binaries", "Win64", "RE-UE4SS", "Mods"},
	} {
		roots, resolveErr := s.resolveKnownDirectories(components...)
		if resolveErr != nil {
			return resolveErr
		}
		ue4ssRoots = appendUniquePaths(ue4ssRoots, roots...)
	}
	for _, root := range ue4ssRoots {
		if err := s.scanUE4SSRoot(root); err != nil {
			return err
		}
	}
	return nil
}

func (s *localScanner) resolveKnownDirectories(components ...string) ([]string, error) {
	current := []string{s.serverDir}
	for _, component := range components {
		next := make([]string, 0)
		for _, parent := range current {
			entries, err := s.directoryEntries(parent)
			if err != nil {
				if errors.Is(err, errLocalScanSkippedLink) {
					continue
				}
				return nil, err
			}
			for _, entry := range entries {
				if entry.info.IsDir() && strings.EqualFold(filepath.Base(entry.path), component) {
					next = appendUniquePaths(next, entry.path)
				}
			}
		}
		current = next
		if len(current) == 0 {
			return nil, nil
		}
	}
	return current, nil
}

func (s *localScanner) readWorkshopSettings(modsRoot string) (ModSettings, bool) {
	settings := ModSettings{GlobalEnabled: true}
	entries, err := s.directoryEntries(modsRoot)
	if err != nil {
		if !errors.Is(err, errLocalScanSkippedLink) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			s.warn(fmt.Sprintf("read Mod settings directory %s: %v", modsRoot, err))
		}
		return settings, false
	}
	var matches []string
	for _, entry := range entries {
		if entry.info.Mode().IsRegular() && strings.EqualFold(filepath.Base(entry.path), "PalModSettings.ini") {
			matches = append(matches, entry.path)
		}
	}
	if len(matches) == 0 {
		// An absent settings file means no official package is active.
		return settings, true
	}
	sortScanPaths(matches)
	if len(matches) > 1 {
		s.warn("multiple case-insensitive PalModSettings.ini files found under " + modsRoot)
	}
	parsed, err := ReadModSettings(matches[0])
	if err != nil {
		s.warn(fmt.Sprintf("read %s: %v", matches[0], err))
		return settings, false
	}
	return parsed, true
}

func (s *localScanner) scanInfoRoot(root string, source LocalModSource, settings ModSettings, settingsKnown bool) error {
	entries, err := s.directoryEntries(root)
	if err != nil {
		if errors.Is(err, errLocalScanSkippedLink) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		name := filepath.Base(entry.path)
		if entry.info.IsDir() {
			if strings.EqualFold(name, ".palpanel-imports") {
				continue
			}
			if err := s.scanInfoCandidate(entry.path, source, settings, settingsKnown); err != nil {
				return err
			}
			continue
		}
		state := LocalModUnknown
		issue := "unrecognized file in Mod metadata root"
		if strings.EqualFold(name, "Info.json") {
			state = LocalModIncomplete
			issue = "Info.json must be inside a Mod directory"
		}
		s.findings = append(s.findings, localScanFinding{LocalModFinding: LocalModFinding{
			Ownership: LocalModManual, State: state, Source: source, Confidence: LocalModConfidenceLow,
			Name: name, Paths: []string{entry.path}, Issues: []string{issue},
		}, matchPaths: []string{entry.path}, identity: "path:" + scanPathKey(entry.path)})
	}
	return nil
}

func (s *localScanner) scanInfoCandidate(candidateRoot string, source LocalModSource, settings ModSettings, settingsKnown bool) error {
	entries, err := s.walk(candidateRoot, 32, nil)
	if err != nil {
		return err
	}
	var infoPaths []string
	var allFiles []string
	for _, entry := range entries {
		if !entry.info.Mode().IsRegular() {
			continue
		}
		allFiles = append(allFiles, entry.path)
		if strings.EqualFold(filepath.Base(entry.path), "Info.json") {
			infoPaths = append(infoPaths, entry.path)
		}
	}
	sortScanPaths(infoPaths)
	sortScanPaths(allFiles)
	if len(infoPaths) == 0 {
		paths := allFiles
		if len(paths) == 0 {
			paths = []string{candidateRoot}
		}
		s.findings = append(s.findings, localScanFinding{LocalModFinding: LocalModFinding{
			Ownership: LocalModManual, State: LocalModIncomplete, Source: source, Confidence: LocalModConfidenceMedium,
			Name: filepath.Base(candidateRoot), Paths: paths, Issues: []string{"Mod directory is missing Info.json"},
		}, matchPaths: append([]string{candidateRoot}, paths...), identity: "path:" + scanPathKey(candidateRoot)})
		return nil
	}
	for _, infoPath := range infoPaths {
		metadata, readErr := ReadInfo(infoPath)
		modRoot := filepath.Dir(infoPath)
		paths := filesWithin(allFiles, modRoot)
		if len(paths) == 0 {
			paths = []string{infoPath}
		}
		finding := localScanFinding{LocalModFinding: LocalModFinding{
			Ownership: LocalModManual, State: LocalModPresent, Source: source, Confidence: LocalModConfidenceHigh,
			Name: filepath.Base(modRoot), Paths: paths,
		}, matchPaths: append([]string{candidateRoot, modRoot}, paths...)}
		if readErr != nil {
			finding.State = LocalModIncomplete
			finding.Confidence = LocalModConfidenceHigh
			finding.Issues = append(finding.Issues, "invalid Info.json: "+readErr.Error())
			finding.identity = "path:" + scanPathKey(modRoot)
		} else {
			finding.Name = metadata.Name
			finding.PackageName = metadata.PackageName
			finding.Version = metadata.Version
			finding.identity = "package:" + strings.ToLower(metadata.PackageName)
			if !sameScanPath(modRoot, candidateRoot) {
				finding.State = LocalModIncomplete
				finding.Confidence = LocalModConfidenceMedium
				finding.Issues = append(finding.Issues, "Info.json is nested below the standard Mod directory level")
			}
			if source == LocalModSourceWorkshop {
				finding.enabledKnown = settingsKnown
				if settingsKnown {
					finding.Enabled = settings.GlobalEnabled && containsFold(settings.ActiveMods, metadata.PackageName)
					if finding.State == LocalModPresent && !finding.Enabled {
						finding.State = LocalModDisabled
					}
				}
			} else {
				finding.Enabled = true
			}
		}
		if len(infoPaths) > 1 {
			finding.Issues = append(finding.Issues, fmt.Sprintf("directory tree contains %d Info.json files", len(infoPaths)))
			if finding.State == LocalModPresent {
				finding.State = LocalModIncomplete
			}
		}
		s.findings = append(s.findings, finding)
	}
	return nil
}

type localPakGroup struct {
	name          string
	paths         []string
	matchPaths    []string
	extensions    map[string]int
	disabledCount int
	enabledCount  int
}

func (s *localScanner) scanPakRoot(root string, skipLegacyInfoTree bool) error {
	filter := func(path string, info fs.FileInfo) bool {
		if !skipLegacyInfoTree || !info.IsDir() {
			return true
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return false
		}
		first := strings.Split(relative, string(os.PathSeparator))[0]
		return !strings.EqualFold(first, "Mods")
	}
	entries, err := s.walk(root, 32, filter)
	if err != nil {
		return err
	}
	groups := map[string]*localPakGroup{}
	var unknown []localScanEntry
	for _, entry := range entries {
		if !entry.info.Mode().IsRegular() {
			continue
		}
		name, extension, disabled, ok := parsePakArtifactName(filepath.Base(entry.path))
		if !ok {
			unknown = append(unknown, entry)
			continue
		}
		key := scanPathKey(filepath.Dir(entry.path)) + "\x00" + strings.ToLower(name)
		group := groups[key]
		if group == nil {
			group = &localPakGroup{name: name, extensions: map[string]int{}}
			groups[key] = group
		}
		group.paths = append(group.paths, entry.path)
		group.matchPaths = appendUniquePaths(group.matchPaths, filepath.Dir(entry.path), entry.path)
		group.extensions[extension]++
		if disabled {
			group.disabledCount++
		} else {
			group.enabledCount++
		}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := groups[key]
		sortScanPaths(group.paths)
		state := LocalModPresent
		enabled := true
		issues := make([]string, 0)
		if group.extensions[".pak"] == 0 {
			state = LocalModIncomplete
			enabled = false
			issues = append(issues, "Pak sidecar files exist without a .pak file")
		} else if group.disabledCount > 0 && group.enabledCount == 0 {
			state = LocalModDisabled
			enabled = false
		} else if group.disabledCount > 0 {
			state = LocalModDuplicate
			issues = append(issues, "enabled and disabled copies of the same Pak artifact coexist")
		}
		for extension, count := range group.extensions {
			if count > 1 {
				state = LocalModDuplicate
				issues = append(issues, fmt.Sprintf("multiple %s artifacts use the same case-insensitive name", extension))
			}
		}
		s.findings = append(s.findings, localScanFinding{LocalModFinding: LocalModFinding{
			Ownership: LocalModManual, State: state, Source: LocalModSourceLegacyPak, Confidence: LocalModConfidenceHigh,
			Name: group.name, Enabled: enabled, Duplicate: state == LocalModDuplicate, Paths: group.paths, Issues: issues,
		}, matchPaths: group.matchPaths, identity: "pak:" + strings.ToLower(group.name), enabledKnown: true})
	}
	for _, entry := range unknown {
		s.findings = append(s.findings, localScanFinding{LocalModFinding: LocalModFinding{
			Ownership: LocalModManual, State: LocalModUnknown, Source: LocalModSourceLegacyPak, Confidence: LocalModConfidenceLow,
			Name: filepath.Base(entry.path), Paths: []string{entry.path}, Issues: []string{"unrecognized file in legacy Pak Mod directory"},
		}, matchPaths: []string{entry.path}, identity: "path:" + scanPathKey(entry.path)})
	}
	return nil
}

func (s *localScanner) scanUE4SSRoot(root string) error {
	entries, err := s.directoryEntries(root)
	if err != nil {
		if errors.Is(err, errLocalScanSkippedLink) {
			return nil
		}
		return err
	}
	enabledByName := map[string]bool{}
	for _, entry := range entries {
		if !entry.info.Mode().IsRegular() || !strings.EqualFold(filepath.Base(entry.path), "mods.txt") {
			continue
		}
		body, readErr := os.ReadFile(entry.path)
		if readErr != nil {
			s.warn(fmt.Sprintf("read UE4SS Mod list %s: %v", entry.path, readErr))
			continue
		}
		for name, enabled := range parseUE4SSModList(string(body)) {
			enabledByName[name] = enabled
		}
	}
	for _, entry := range entries {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		name := filepath.Base(entry.path)
		if !entry.info.IsDir() {
			if strings.EqualFold(name, "mods.txt") {
				continue
			}
			s.findings = append(s.findings, localScanFinding{LocalModFinding: LocalModFinding{
				Ownership: LocalModManual, State: LocalModUnknown, Source: LocalModSourceUE4SS, Confidence: LocalModConfidenceLow,
				Name: name, Paths: []string{entry.path}, Issues: []string{"unrecognized loose file in UE4SS Mods directory"},
			}, matchPaths: []string{entry.path}, identity: "path:" + scanPathKey(entry.path)})
			continue
		}
		tree, walkErr := s.walk(entry.path, 32, nil)
		if walkErr != nil {
			return walkErr
		}
		var paths []string
		payloadCount := 0
		mainPayload := false
		explicitEnabled := false
		explicitDisabled := false
		for _, child := range tree {
			if !child.info.Mode().IsRegular() {
				continue
			}
			paths = append(paths, child.path)
			base := filepath.Base(child.path)
			extension := strings.ToLower(filepath.Ext(base))
			if extension == ".lua" || extension == ".dll" {
				payloadCount++
				if strings.EqualFold(base, "main.lua") || strings.EqualFold(base, "main.dll") {
					mainPayload = true
				}
			}
			if sameScanPath(filepath.Dir(child.path), entry.path) {
				switch {
				case strings.EqualFold(base, "enabled.txt"):
					explicitEnabled = true
				case strings.EqualFold(base, "disabled.txt"):
					explicitDisabled = true
				}
			}
		}
		sortScanPaths(paths)
		configuredEnabled, configured := enabledByName[strings.ToLower(name)]
		state := LocalModPresent
		enabled := true
		enabledKnown := explicitEnabled || explicitDisabled || configured
		issues := make([]string, 0)
		if payloadCount == 0 {
			state = LocalModIncomplete
			enabled = false
			issues = append(issues, "UE4SS Mod directory has no Lua or DLL payload")
		} else if explicitDisabled || (configured && !configuredEnabled) {
			state = LocalModDisabled
			enabled = false
		} else if configured {
			enabled = configuredEnabled
		} else if explicitEnabled {
			enabled = true
		}
		if explicitEnabled && explicitDisabled {
			state = LocalModIncomplete
			enabled = false
			issues = append(issues, "UE4SS Mod has both enabled.txt and disabled.txt")
		}
		confidence := LocalModConfidenceMedium
		if mainPayload {
			confidence = LocalModConfidenceHigh
		}
		matchPaths := append([]string{entry.path}, paths...)
		s.findings = append(s.findings, localScanFinding{LocalModFinding: LocalModFinding{
			Ownership: LocalModManual, State: state, Source: LocalModSourceUE4SS, Confidence: confidence,
			Name: name, Enabled: enabled, Paths: nonEmptyPaths(paths, entry.path), Issues: issues,
		}, matchPaths: matchPaths, identity: "ue4ss:" + strings.ToLower(name), enabledKnown: enabledKnown})
	}
	return nil
}

func (s *localScanner) reconcileDatabase() {
	matched := make([]bool, len(s.database))
	for findingIndex := range s.findings {
		finding := &s.findings[findingIndex]
		for databaseIndex := range s.database {
			record := s.database[databaseIndex]
			if strings.TrimSpace(record.Path) == "" || !anySameScanPath(record.Path, finding.matchPaths) {
				continue
			}
			finding.DatabaseMods = append(finding.DatabaseMods, record)
			matched[databaseIndex] = true
		}
		if len(finding.DatabaseMods) == 0 && finding.PackageName != "" {
			for databaseIndex := range s.database {
				record := s.database[databaseIndex]
				if strings.TrimSpace(record.Path) == "" && strings.EqualFold(strings.TrimSpace(record.PackageName), finding.PackageName) {
					finding.DatabaseMods = append(finding.DatabaseMods, record)
					matched[databaseIndex] = true
				}
			}
		}
		if len(finding.DatabaseMods) == 0 {
			continue
		}
		finding.Ownership = LocalModManaged
		sort.Slice(finding.DatabaseMods, func(left, right int) bool { return finding.DatabaseMods[left].ID < finding.DatabaseMods[right].ID })
		if len(finding.DatabaseMods) > 1 {
			finding.Duplicate = true
			if finding.State == LocalModPresent {
				finding.State = LocalModDuplicate
			}
			finding.Issues = append(finding.Issues, fmt.Sprintf("%d database Mod records refer to the same on-disk artifact", len(finding.DatabaseMods)))
		}
		if !finding.enabledKnown && finding.State != LocalModIncomplete && finding.State != LocalModUnknown {
			finding.Enabled = finding.DatabaseMods[0].Enabled
			if !finding.Enabled && finding.State == LocalModPresent {
				finding.State = LocalModDisabled
			}
		} else if finding.enabledKnown {
			for _, record := range finding.DatabaseMods {
				if record.Enabled != finding.Enabled {
					finding.Issues = append(finding.Issues, fmt.Sprintf("database enabled state for %s differs from on-disk activation state", record.ID))
				}
			}
		}
	}

	for index, record := range s.database {
		if matched[index] {
			continue
		}
		finding := localScanFinding{LocalModFinding: LocalModFinding{
			Ownership: LocalModManaged, State: LocalModMissingFiles, Source: LocalModSourceDatabase, Confidence: LocalModConfidenceHigh,
			Name: firstNonEmpty(record.Name, record.PackageName, record.ID), PackageName: record.PackageName,
			Version: record.Version, Enabled: record.Enabled, DatabaseMods: []db.Mod{record},
		}, identity: databaseIdentity(record)}
		if strings.TrimSpace(record.Path) == "" {
			finding.Issues = []string{"database Mod record has no installation path"}
		} else {
			finding.Paths = []string{filepath.Clean(record.Path)}
			finding.matchPaths = []string{record.Path}
			state, issue := s.inspectUnmatchedDatabasePath(record.Path)
			finding.State = state
			finding.Issues = []string{issue}
			if state == LocalModUnknown {
				finding.Confidence = LocalModConfidenceLow
			}
		}
		s.findings = append(s.findings, finding)
	}
}

func (s *localScanner) inspectUnmatchedDatabasePath(path string) (LocalModState, string) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return LocalModMissingFiles, "database Mod path is invalid: " + err.Error()
	}
	if !scanPathWithin(s.serverDir, absolute) {
		return LocalModUnknown, "database Mod path is outside ServerDir and was not scanned"
	}
	if !s.rootExists {
		return LocalModMissingFiles, "database Mod path is missing because ServerDir does not exist"
	}
	relative, err := filepath.Rel(s.serverDir, absolute)
	if err != nil {
		return LocalModMissingFiles, "database Mod path cannot be resolved beneath ServerDir"
	}
	current := s.serverDir
	for _, component := range strings.Split(relative, string(os.PathSeparator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			return LocalModMissingFiles, "database Mod record points to missing files"
		}
		if statErr != nil {
			s.warn(fmt.Sprintf("inspect database Mod path %s: %v", current, statErr))
			return LocalModUnknown, "database Mod path could not be inspected safely"
		}
		linked, linkErr := localScanPathIsLink(current, info)
		if linkErr != nil {
			s.warn(fmt.Sprintf("inspect database Mod path attributes %s: %v", current, linkErr))
			return LocalModUnknown, "database Mod path attributes could not be inspected safely"
		}
		if linked {
			s.skip(current)
			return LocalModMissingFiles, "database Mod path crosses a link or reparse point and was not scanned"
		}
	}
	return LocalModUnknown, "database Mod path exists but is not in a recognized Mod layout"
}

func (s *localScanner) markDuplicates() {
	groups := map[string][]int{}
	for index, finding := range s.findings {
		if finding.identity == "" {
			continue
		}
		groups[finding.identity] = append(groups[finding.identity], index)
	}
	for _, indexes := range groups {
		if len(indexes) < 2 {
			continue
		}
		for _, index := range indexes {
			finding := &s.findings[index]
			finding.Duplicate = true
			if finding.State == LocalModPresent {
				finding.State = LocalModDuplicate
			}
			finding.Issues = append(finding.Issues, fmt.Sprintf("%d scan findings share the same case-insensitive Mod identity", len(indexes)))
		}
	}
}

func (s *localScanner) result() LocalScanResult {
	sort.Slice(s.findings, func(left, right int) bool {
		leftFinding, rightFinding := s.findings[left], s.findings[right]
		if leftFinding.Source != rightFinding.Source {
			return leftFinding.Source < rightFinding.Source
		}
		if !strings.EqualFold(leftFinding.Name, rightFinding.Name) {
			return strings.ToLower(leftFinding.Name) < strings.ToLower(rightFinding.Name)
		}
		return firstPath(leftFinding.Paths) < firstPath(rightFinding.Paths)
	})
	findings := make([]LocalModFinding, 0, len(s.findings))
	for _, internal := range s.findings {
		finding := internal.LocalModFinding
		finding.Paths = uniqueSortedPaths(finding.Paths)
		finding.Issues = uniqueSortedStrings(finding.Issues)
		finding.Classifications = classificationsFor(finding)
		finding.ID = localFindingID(s.serverDir, internal)
		findings = append(findings, finding)
	}
	return LocalScanResult{
		ServerDir: s.serverDir, ScannedAt: time.Now().UTC().Format(time.RFC3339Nano), Findings: findings,
		SkippedPaths: uniqueSortedPaths(s.skipped), Warnings: uniqueSortedStrings(s.warnings),
	}
}

func (s *localScanner) directoryEntries(path string) ([]localScanEntry, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		s.warn(fmt.Sprintf("inspect directory %s: %v", path, err))
		return nil, nil
	}
	linked, err := localScanPathIsLink(path, info)
	if err != nil {
		s.warn(fmt.Sprintf("inspect directory attributes %s: %v", path, err))
		return nil, nil
	}
	if linked {
		s.skip(path)
		return nil, errLocalScanSkippedLink
	}
	if !info.IsDir() {
		return nil, nil
	}
	// Recheck immediately before ReadDir to narrow replacement races.
	info, err = os.Lstat(path)
	if err != nil {
		s.warn(fmt.Sprintf("recheck directory %s: %v", path, err))
		return nil, nil
	}
	linked, err = localScanPathIsLink(path, info)
	if err != nil || linked {
		if err != nil {
			s.warn(fmt.Sprintf("recheck directory attributes %s: %v", path, err))
		}
		s.skip(path)
		return nil, errLocalScanSkippedLink
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		s.warn(fmt.Sprintf("read directory %s: %v", path, err))
		return nil, nil
	}
	out := make([]localScanEntry, 0, len(entries))
	for _, entry := range entries {
		if err := s.ctx.Err(); err != nil {
			return nil, err
		}
		entryPath := filepath.Join(path, entry.Name())
		entryInfo, statErr := os.Lstat(entryPath)
		if statErr != nil {
			s.warn(fmt.Sprintf("inspect path %s: %v", entryPath, statErr))
			continue
		}
		entryLinked, linkErr := localScanPathIsLink(entryPath, entryInfo)
		if linkErr != nil {
			s.warn(fmt.Sprintf("inspect path attributes %s: %v", entryPath, linkErr))
			continue
		}
		if entryLinked {
			s.skip(entryPath)
			continue
		}
		out = append(out, localScanEntry{path: entryPath, info: entryInfo})
	}
	return out, nil
}

func (s *localScanner) walk(root string, maxDepth int, include func(string, fs.FileInfo) bool) ([]localScanEntry, error) {
	var out []localScanEntry
	var visit func(string, int) error
	visit = func(directory string, depth int) error {
		entries, err := s.directoryEntries(directory)
		if err != nil {
			if errors.Is(err, errLocalScanSkippedLink) {
				return nil
			}
			return err
		}
		for _, entry := range entries {
			if include != nil && !include(entry.path, entry.info) {
				continue
			}
			out = append(out, entry)
			if !entry.info.IsDir() {
				continue
			}
			if depth >= maxDepth {
				s.warn(fmt.Sprintf("scan depth limit reached at %s", entry.path))
				continue
			}
			if err := visit(entry.path, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root, 0); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *localScanner) warn(message string) {
	if strings.TrimSpace(message) != "" {
		s.warnings = append(s.warnings, message)
	}
}

func (s *localScanner) skip(path string) {
	s.skipped = append(s.skipped, filepath.Clean(path))
}

func parsePakArtifactName(name string) (string, string, bool, bool) {
	base := name
	disabled := false
	if strings.HasSuffix(strings.ToLower(base), ".disabled") {
		disabled = true
		base = base[:len(base)-len(".disabled")]
	}
	extension := strings.ToLower(filepath.Ext(base))
	switch extension {
	case ".pak", ".utoc", ".ucas", ".sig":
	default:
		return "", "", false, false
	}
	stem := strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
	if stem == "" {
		return "", "", false, false
	}
	return stem, extension, disabled, true
}

func parseUE4SSModList(content string) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		fields := strings.Fields(value)
		if len(fields) == 0 {
			continue
		}
		value = strings.ToLower(strings.TrimSpace(fields[0]))
		if name == "" {
			continue
		}
		switch value {
		case "1", "true", "enabled", "on":
			out[strings.ToLower(name)] = true
		case "0", "false", "disabled", "off":
			out[strings.ToLower(name)] = false
		}
	}
	return out
}

func classificationsFor(finding LocalModFinding) []LocalModClassification {
	out := make([]LocalModClassification, 0, 3)
	if finding.Ownership == LocalModManaged {
		out = append(out, LocalModClassificationManaged)
	} else {
		out = append(out, LocalModClassificationManual)
	}
	switch finding.State {
	case LocalModMissingFiles:
		out = append(out, LocalModClassificationMissingFiles)
	case LocalModUnknown:
		out = append(out, LocalModClassificationUnknown)
	case LocalModDisabled:
		out = append(out, LocalModClassificationDisabled)
	case LocalModDuplicate:
		out = append(out, LocalModClassificationDuplicate)
	case LocalModIncomplete:
		out = append(out, LocalModClassificationIncomplete)
	default:
		out = append(out, LocalModClassificationPresent)
	}
	if finding.Duplicate && finding.State != LocalModDuplicate {
		out = append(out, LocalModClassificationDuplicate)
	}
	return out
}

func databaseIdentity(record db.Mod) string {
	if packageName := strings.TrimSpace(record.PackageName); packageName != "" {
		return "package:" + strings.ToLower(packageName)
	}
	return "database:" + strings.ToLower(strings.TrimSpace(record.ID))
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func filesWithin(paths []string, root string) []string {
	out := make([]string, 0)
	for _, path := range paths {
		if scanPathWithin(root, path) {
			out = append(out, path)
		}
	}
	return out
}

func nonEmptyPaths(paths []string, fallback string) []string {
	if len(paths) > 0 {
		return paths
	}
	return []string{fallback}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return "Unknown Mod"
}

func firstPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return strings.ToLower(paths[0])
}

func anySameScanPath(path string, candidates []string) bool {
	for _, candidate := range candidates {
		if sameScanPath(path, candidate) {
			return true
		}
	}
	return false
}

func sameScanPath(left, right string) bool {
	leftAbsolute, leftErr := filepath.Abs(filepath.Clean(left))
	rightAbsolute, rightErr := filepath.Abs(filepath.Clean(right))
	if leftErr != nil || rightErr != nil {
		return false
	}
	return strings.EqualFold(leftAbsolute, rightAbsolute)
}

func scanPathWithin(root, target string) bool {
	rootAbsolute, rootErr := filepath.Abs(filepath.Clean(root))
	targetAbsolute, targetErr := filepath.Abs(filepath.Clean(target))
	if rootErr != nil || targetErr != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbsolute, targetAbsolute)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return true
	}
	if runtime.GOOS != "windows" {
		return false
	}
	rootWithSeparator := strings.TrimRight(rootAbsolute, `\/`) + string(os.PathSeparator)
	return strings.EqualFold(rootAbsolute, targetAbsolute) || strings.HasPrefix(strings.ToLower(targetAbsolute), strings.ToLower(rootWithSeparator))
}

func scanPathKey(path string) string {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return strings.ToLower(filepath.Clean(path))
	}
	return strings.ToLower(absolute)
}

func appendUniquePaths(paths []string, candidates ...string) []string {
	for _, candidate := range candidates {
		found := false
		for _, path := range paths {
			if sameScanPath(path, candidate) {
				found = true
				break
			}
		}
		if !found {
			paths = append(paths, filepath.Clean(candidate))
		}
	}
	return paths
}

func uniqueSortedPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	out = appendUniquePaths(out, paths...)
	sortScanPaths(out)
	return out
}

func sortScanPaths(paths []string) {
	sort.Slice(paths, func(left, right int) bool {
		leftLower, rightLower := strings.ToLower(paths[left]), strings.ToLower(paths[right])
		if leftLower == rightLower {
			return paths[left] < paths[right]
		}
		return leftLower < rightLower
	})
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
