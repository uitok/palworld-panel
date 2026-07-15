package mods

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"palpanel/internal/db"
)

func TestLocalFindingIgnorePersistsAndRejectsStaleRevision(t *testing.T) {
	manager, store := newLocalScanTestManager(t, filepath.Join(t.TempDir(), "server"))
	root := filepath.Join(manager.cfg.WorkshopModsDir(), "Manual")
	writeLocalScanFile(t, filepath.Join(root, "Info.json"), `{"Name":"Manual","PackageName":"ManualPackage","Version":"1"}`)
	writeLocalScanFile(t, filepath.Join(root, "payload.pak"), "one")

	scan, err := manager.ScanLocal(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	finding := findLocalScanFinding(t, scan, "ManualPackage", "")
	if finding.ID == "" || finding.Revision == "" || !localCapability(finding, LocalModActionIgnore).Available {
		t.Fatalf("finding identity/actions = %#v", finding)
	}
	ignored, err := manager.ActOnLocalFinding(t.Context(), finding.ID, LocalModActionRequest{Action: LocalModActionIgnore, Revision: finding.Revision})
	if err != nil {
		t.Fatal(err)
	}
	ignoredFinding := findLocalScanFinding(t, ignored.Scan, "ManualPackage", "")
	if !ignoredFinding.Ignored || !localCapability(ignoredFinding, LocalModActionUnignore).Available {
		t.Fatalf("ignored finding = %#v", ignoredFinding)
	}
	reloaded := NewManager(manager.cfg, store, manager.runner)
	reloadedScan, err := reloaded.ScanLocal(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	reloadedFinding := findLocalScanFinding(t, reloadedScan, "ManualPackage", "")
	if !reloadedFinding.Ignored {
		t.Fatalf("persistent ignored scan = %#v", reloadedScan)
	}

	if err := os.WriteFile(filepath.Join(root, "payload.pak"), []byte("changed-size"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = manager.ActOnLocalFinding(t.Context(), finding.ID, LocalModActionRequest{Action: LocalModActionUnignore, Revision: reloadedFinding.Revision})
	var actionErr LocalModActionError
	if !errors.As(err, &actionErr) || actionErr.Code != "local_finding_stale" {
		t.Fatalf("stale action error = %#v", err)
	}
}

func TestLocalFindingImportCopiesIntoManagedWorkshopDirectory(t *testing.T) {
	manager, _ := newLocalScanTestManager(t, filepath.Join(t.TempDir(), "server"))
	manualRoot := filepath.Join(manager.cfg.WorkshopModsDir(), "HandInstalled")
	writeLocalScanFile(t, filepath.Join(manualRoot, "Info.json"), `{"Name":"Hand Installed","PackageName":"HandPackage","Version":"2"}`)
	writeLocalScanFile(t, filepath.Join(manualRoot, "payload.pak"), "payload")

	scan, err := manager.ScanLocal(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	finding := findLocalScanFinding(t, scan, "HandPackage", "")
	if !localCapability(finding, LocalModActionImport).Available || !localCapability(finding, LocalModActionRepair).Available {
		t.Fatalf("manual capabilities = %#v", finding.Actions)
	}
	result, err := manager.ActOnLocalFinding(t.Context(), finding.ID, LocalModActionRequest{Action: LocalModActionImport, Revision: finding.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if result.Mod == nil || result.Mod.Source != "local_scan_import" || result.Mod.Enabled {
		t.Fatalf("import result = %#v", result)
	}
	if !isDirectChild(manager.cfg.WorkshopModsDir(), result.Mod.Path) || !strings.EqualFold(filepath.Base(result.Mod.Path), result.Mod.ID) {
		t.Fatalf("managed path = %q, id = %q", result.Mod.Path, result.Mod.ID)
	}
	if _, err := os.Stat(filepath.Join(manualRoot, "payload.pak")); err != nil {
		t.Fatalf("manual source should remain untouched: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(result.Mod.Path, "payload.pak")); err != nil || string(body) != "payload" {
		t.Fatalf("managed copy = %q, %v", body, err)
	}
}

func TestLocalFindingRepairDoesNotMakeManualDirectoryDeletable(t *testing.T) {
	manager, _ := newLocalScanTestManager(t, filepath.Join(t.TempDir(), "server"))
	manualRoot := filepath.Join(manager.cfg.WorkshopModsDir(), "RepairMe")
	writeLocalScanFile(t, filepath.Join(manualRoot, "Info.json"), `{"Name":"Repair Me","PackageName":"RepairPackage"}`)
	writeLocalScanFile(t, filepath.Join(manualRoot, "payload.pak"), "payload")

	scan, err := manager.ScanLocal(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	finding := findLocalScanFinding(t, scan, "RepairPackage", "")
	result, err := manager.ActOnLocalFinding(t.Context(), finding.ID, LocalModActionRequest{Action: LocalModActionRepair, Revision: finding.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if result.Mod == nil || result.Mod.Source != "local_scan_repair" || result.Mod.Path != manualRoot {
		t.Fatalf("repair result = %#v", result)
	}
	if err := manager.Delete(t.Context(), result.Mod.ID); err == nil {
		t.Fatal("Delete accepted a repaired manual directory")
	}
	if _, err := os.Stat(filepath.Join(manualRoot, "payload.pak")); err != nil {
		t.Fatalf("repaired manual files were removed: %v", err)
	}
}

func TestLocalFindingDeleteRequiresConfirmationForOwnedDirectory(t *testing.T) {
	manager, store := newLocalScanTestManager(t, filepath.Join(t.TempDir(), "server"))
	record := db.Mod{ID: "mod_owned", Name: "Owned", Source: "upload", PackageName: "OwnedPackage", Path: filepath.Join(manager.cfg.WorkshopModsDir(), "mod_owned")}
	writeLocalScanFile(t, filepath.Join(record.Path, "Info.json"), `{"Name":"Owned","PackageName":"OwnedPackage"}`)
	writeLocalScanFile(t, filepath.Join(record.Path, "payload.pak"), "payload")
	if err := store.UpsertMod(t.Context(), record); err != nil {
		t.Fatal(err)
	}
	scan, err := manager.ScanLocal(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	finding := findLocalScanFinding(t, scan, "OwnedPackage", "")
	if !localCapability(finding, LocalModActionDelete).Available {
		t.Fatalf("delete capability = %#v", finding.Actions)
	}
	_, err = manager.ActOnLocalFinding(t.Context(), finding.ID, LocalModActionRequest{Action: LocalModActionDelete, Revision: finding.Revision})
	var actionErr LocalModActionError
	if !errors.As(err, &actionErr) || actionErr.Code != "local_action_confirmation_required" {
		t.Fatalf("confirmation error = %#v", err)
	}
	if _, err := manager.ActOnLocalFinding(t.Context(), finding.ID, LocalModActionRequest{Action: LocalModActionDelete, Revision: finding.Revision, Confirm: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(record.Path); !os.IsNotExist(err) {
		t.Fatalf("owned directory still exists: %v", err)
	}
}
