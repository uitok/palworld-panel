package mods

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadInfoRequiresPackageName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Info.json")
	if err := os.WriteFile(path, []byte(`{"Name":"No package"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadInfo(path); err == nil {
		t.Fatal("expected missing PackageName error")
	}
}

func TestReadInfoRejectsPackageNameControlCharacters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Info.json")
	if err := os.WriteFile(path, []byte("{\"Name\":\"Bad\",\"PackageName\":\"Good\\nActiveModList=Injected\"}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadInfo(path); err == nil || !strings.Contains(err.Error(), "invalid PackageName") {
		t.Fatalf("ReadInfo error = %v", err)
	}
}

func TestWriteAndParseModSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PalModSettings.ini")
	err := WriteModSettings(path, ModSettings{GlobalEnabled: true, ActiveMods: []string{"BPackage", "APackage"}})
	if err != nil {
		t.Fatalf("WriteModSettings returned error: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(b), "[PalModSettings]") || !contains(string(b), "ActiveModList=APackage") {
		t.Fatalf("expected official PalModSettings format, got %s", string(b))
	}
	got := ParseModSettings(string(b))
	if !got.GlobalEnabled {
		t.Fatal("expected GlobalEnabled")
	}
	if len(got.ActiveMods) != 2 || got.ActiveMods[0] != "APackage" || got.ActiveMods[1] != "BPackage" {
		t.Fatalf("unexpected active mods: %#v", got.ActiveMods)
	}
}

func TestEnablePackage(t *testing.T) {
	got := EnablePackage(ModSettings{GlobalEnabled: true, ActiveMods: []string{"A"}}, "B", true)
	got = EnablePackage(got, "A", false)
	if len(got.ActiveMods) != 1 || got.ActiveMods[0] != "B" {
		t.Fatalf("unexpected active mods: %#v", got.ActiveMods)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
