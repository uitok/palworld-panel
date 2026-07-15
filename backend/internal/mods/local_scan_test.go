package mods

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
)

func TestScanLocalEmptyServerDirectory(t *testing.T) {
	manager, _ := newLocalScanTestManager(t, filepath.Join(t.TempDir(), "empty server"))

	result, err := manager.ScanLocal(context.Background())
	if err != nil {
		t.Fatalf("ScanLocal returned error: %v", err)
	}
	if len(result.Findings) != 0 || len(result.SkippedPaths) != 0 {
		t.Fatalf("unexpected empty scan result: %#v", result)
	}
	if result.ServerDir != manager.cfg.ServerDir || result.ScannedAt == "" {
		t.Fatalf("scan metadata = %#v", result)
	}
}

func TestScanLocalReconcilesWorkshopFilesCaseInsensitively(t *testing.T) {
	serverDir := filepath.Join(t.TempDir(), "服务器 路径", "Pal Server")
	manager, store := newLocalScanTestManager(t, serverDir)
	workshopRoot := filepath.Join(serverDir, "mOdS", "wOrKsHoP")
	managedRoot := filepath.Join(workshopRoot, "管理 Mod")
	manualRoot := filepath.Join(workshopRoot, "手工 Mod")
	writeLocalScanFile(t, filepath.Join(managedRoot, "iNfO.JsOn"), `{"Name":"Managed 中文","PackageName":"CasePackage","Version":"1.2"}`)
	writeLocalScanFile(t, filepath.Join(managedRoot, "payload.pak"), "managed")
	writeLocalScanFile(t, filepath.Join(manualRoot, "Info.JSON"), `{"Name":"Manual 中文","PackageName":"ManualPackage","Version":"2.0"}`)
	writeLocalScanFile(t, filepath.Join(serverDir, "mOdS", "PALMODSETTINGS.INI"), "[PalModSettings]\nActiveModList=casepackage\n")

	managedRecord := db.Mod{
		ID: "mod_managed", Name: "Managed record", Source: "local_zip", PackageName: "CASEPACKAGE",
		Path: swapPathCase(managedRoot), Version: "1.1", Enabled: true,
	}
	missingRecord := db.Mod{
		ID: "mod_missing", Name: "Missing Mod", Source: "workshop", PackageName: "MissingPackage",
		Path: filepath.Join(workshopRoot, "missing files"), Version: "9", Enabled: false,
	}
	for _, record := range []db.Mod{managedRecord, missingRecord} {
		if err := store.UpsertMod(context.Background(), record); err != nil {
			t.Fatal(err)
		}
	}

	result, err := manager.ScanLocal(context.Background())
	if err != nil {
		t.Fatalf("ScanLocal returned error: %v", err)
	}
	managed := findLocalScanFinding(t, result, "CasePackage", "")
	if managed.Ownership != LocalModManaged || managed.State != LocalModPresent || !managed.Enabled || managed.Confidence != LocalModConfidenceHigh {
		t.Fatalf("managed finding = %#v", managed)
	}
	if len(managed.DatabaseMods) != 1 || managed.DatabaseMods[0].ID != managedRecord.ID || !containsPathFold(managed.Paths, filepath.Join(managedRoot, "iNfO.JsOn")) {
		t.Fatalf("managed reconciliation = %#v", managed)
	}

	manual := findLocalScanFinding(t, result, "ManualPackage", "")
	if manual.Ownership != LocalModManual || manual.State != LocalModDisabled || manual.Enabled {
		t.Fatalf("manual finding = %#v", manual)
	}
	missing := findLocalScanFinding(t, result, "MissingPackage", "")
	if missing.Ownership != LocalModManaged || missing.State != LocalModMissingFiles || missing.Source != LocalModSourceDatabase {
		t.Fatalf("missing finding = %#v", missing)
	}
	if len(result.Findings) != 3 {
		t.Fatalf("findings = %#v", result.Findings)
	}
	stored, err := store.ListMods(context.Background())
	if err != nil || len(stored) != 2 {
		t.Fatalf("scan mutated database: %#v, %v", stored, err)
	}
}

func TestScanLocalMarksDuplicatePackagesAcrossVersions(t *testing.T) {
	manager, _ := newLocalScanTestManager(t, filepath.Join(t.TempDir(), "server"))
	root := filepath.Join(manager.cfg.ServerDir, "Mods", "Workshop")
	writeLocalScanFile(t, filepath.Join(root, "Version One", "Info.json"), `{"Name":"One","PackageName":"SharedPackage","Version":"1"}`)
	writeLocalScanFile(t, filepath.Join(root, "Version Two", "INFO.JSON"), `{"Name":"Two","PackageName":"sharedpackage","Version":"2"}`)
	writeLocalScanFile(t, filepath.Join(manager.cfg.ServerDir, "Mods", "PalModSettings.ini"), "ActiveModList=SHAREDPACKAGE\n")

	result, err := manager.ScanLocal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	duplicates := 0
	for _, finding := range result.Findings {
		if !strings.EqualFold(finding.PackageName, "SharedPackage") {
			continue
		}
		duplicates++
		if !finding.Duplicate || finding.State != LocalModDuplicate || !hasClassification(finding, LocalModClassificationDuplicate) {
			t.Fatalf("duplicate finding = %#v", finding)
		}
	}
	if duplicates != 2 {
		t.Fatalf("duplicate count = %d; findings = %#v", duplicates, result.Findings)
	}
}

func TestScanLocalRecognizesLegacyPakAndUE4SSLayouts(t *testing.T) {
	manager, _ := newLocalScanTestManager(t, filepath.Join(t.TempDir(), "Server With Space"))
	pakRoot := filepath.Join(manager.cfg.ServerDir, "PAL", "content", "PAKS", "~MODS", "嵌套")
	for _, name := range []string{"CoolMod.PAK", "CoolMod.UTOC", "CoolMod.UCAS"} {
		writeLocalScanFile(t, filepath.Join(pakRoot, name), name)
	}
	writeLocalScanFile(t, filepath.Join(pakRoot, "Broken.UTOC"), "sidecar")
	writeLocalScanFile(t, filepath.Join(pakRoot, "Mixed.PAK"), "enabled")
	writeLocalScanFile(t, filepath.Join(pakRoot, "Mixed.PAK.disabled"), "disabled")
	writeLocalScanFile(t, filepath.Join(pakRoot, "notes.md"), "unknown")

	ue4ssRoot := filepath.Join(manager.cfg.ServerDir, "Pal", "Binaries", "WIN64", "mOdS")
	writeLocalScanFile(t, filepath.Join(ue4ssRoot, "mods.TXT"), "Lua 中文 Mod : 0\n")
	writeLocalScanFile(t, filepath.Join(ue4ssRoot, "Lua 中文 Mod", "Scripts", "MAIN.LUA"), "return {}")
	writeLocalScanFile(t, filepath.Join(ue4ssRoot, "Incomplete Mod", "config.ini"), "value=true")

	result, err := manager.ScanLocal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cool := findLocalScanFinding(t, result, "", "CoolMod")
	if cool.Source != LocalModSourceLegacyPak || cool.State != LocalModPresent || !cool.Enabled || cool.Confidence != LocalModConfidenceHigh || len(cool.Paths) != 3 {
		t.Fatalf("Pak finding = %#v", cool)
	}
	broken := findLocalScanFinding(t, result, "", "Broken")
	if broken.State != LocalModIncomplete {
		t.Fatalf("incomplete Pak finding = %#v", broken)
	}
	mixed := findLocalScanFinding(t, result, "", "Mixed")
	if mixed.State != LocalModDuplicate || !mixed.Duplicate || !hasClassification(mixed, LocalModClassificationDuplicate) {
		t.Fatalf("duplicate Pak finding = %#v", mixed)
	}
	unknown := findLocalScanFinding(t, result, "", "notes.md")
	if unknown.State != LocalModUnknown || unknown.Confidence != LocalModConfidenceLow {
		t.Fatalf("unknown finding = %#v", unknown)
	}
	lua := findLocalScanFinding(t, result, "", "Lua 中文 Mod")
	if lua.Source != LocalModSourceUE4SS || lua.State != LocalModDisabled || lua.Enabled || lua.Confidence != LocalModConfidenceHigh {
		t.Fatalf("UE4SS finding = %#v", lua)
	}
	incomplete := findLocalScanFinding(t, result, "", "Incomplete Mod")
	if incomplete.Source != LocalModSourceUE4SS || incomplete.State != LocalModIncomplete {
		t.Fatalf("incomplete UE4SS finding = %#v", incomplete)
	}
}

func TestScanLocalSkipsDirectoryLinksAndReparsePoints(t *testing.T) {
	manager, _ := newLocalScanTestManager(t, filepath.Join(t.TempDir(), "server"))
	workshopRoot := filepath.Join(manager.cfg.ServerDir, "Mods", "Workshop")
	if err := os.MkdirAll(workshopRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	writeLocalScanFile(t, filepath.Join(outside, "Info.json"), `{"Name":"Outside","PackageName":"OutsidePackage"}`)
	marker := filepath.Join(outside, "must-remain.txt")
	writeLocalScanFile(t, marker, "keep")
	link := filepath.Join(workshopRoot, "linked Mod")
	createLocalScanDirectoryLink(t, outside, link)

	result, err := manager.ScanLocal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("linked content was scanned: %#v", result.Findings)
	}
	if !containsPathFold(result.SkippedPaths, link) {
		t.Fatalf("skipped paths = %#v", result.SkippedPaths)
	}
	if body, readErr := os.ReadFile(marker); readErr != nil || string(body) != "keep" {
		t.Fatalf("scan modified linked content: %q, %v", body, readErr)
	}
	if _, statErr := os.Lstat(link); statErr != nil {
		t.Fatalf("scan removed link: %v", statErr)
	}
}

func newLocalScanTestManager(t *testing.T, serverDir string) (Manager, *db.Store) {
	t.Helper()
	serverDir, err := filepath.Abs(serverDir)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Dir(serverDir)
	cfg := appconfig.Config{
		DataDir: dataDir, ServerDir: serverDir, UploadsDir: filepath.Join(dataDir, "uploads"),
		BackupsDir: filepath.Join(dataDir, "backups"), LogsDir: filepath.Join(dataDir, "logs"),
		DBPath: filepath.Join(dataDir, "scan.db"), MaxUploadBytes: 1 << 20,
		WorkshopAppID: "1623730", DockerBinary: "false", DockerImage: "test",
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewManager(cfg, store, docker.NewRunner(cfg)), store
}

func writeLocalScanFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findLocalScanFinding(t *testing.T, result LocalScanResult, packageName, name string) LocalModFinding {
	t.Helper()
	for _, finding := range result.Findings {
		if packageName != "" && strings.EqualFold(finding.PackageName, packageName) {
			return finding
		}
		if name != "" && strings.EqualFold(finding.Name, name) {
			return finding
		}
	}
	t.Fatalf("finding package=%q name=%q not found in %#v", packageName, name, result.Findings)
	return LocalModFinding{}
}

func hasClassification(finding LocalModFinding, classification LocalModClassification) bool {
	for _, item := range finding.Classifications {
		if item == classification {
			return true
		}
	}
	return false
}

func containsPathFold(paths []string, target string) bool {
	for _, path := range paths {
		if strings.EqualFold(filepath.Clean(path), filepath.Clean(target)) {
			return true
		}
	}
	return false
}

func swapPathCase(path string) string {
	var builder strings.Builder
	for _, character := range path {
		switch {
		case character >= 'a' && character <= 'z':
			builder.WriteRune(character - ('a' - 'A'))
		case character >= 'A' && character <= 'Z':
			builder.WriteRune(character + ('a' - 'A'))
		default:
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func createLocalScanDirectoryLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err == nil {
		return
	}
	if runtime.GOOS != "windows" {
		t.Skip("directory symlink creation is unavailable")
	}
	output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Skipf("cannot create directory link or junction: %v: %s", err, output)
	}
	t.Cleanup(func() {
		if removeErr := os.Remove(link); removeErr != nil && !os.IsNotExist(removeErr) {
			t.Logf("remove test junction: %v", removeErr)
		}
	})
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("junction was not created: %v (%s)", err, fmt.Sprint(output))
	}
}
