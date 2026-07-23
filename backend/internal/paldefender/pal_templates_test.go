package paldefender

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPalTemplateWriteReadListOverwriteAndDelete(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	level := 50
	partner := 3
	awakening := true
	template := PalTemplate{
		PalID: "Anubis", Nickname: "Arena Anubis", Gender: "none", Level: &level,
		PartnerSkillLevel: &partner, Shiny: boolPointer(true), IsAwakening: &awakening,
		IVs:          map[string]int{"Health": 100, "AttackShot": 90, "Defense": 80},
		PalSouls:     map[string]int{"Health": 10, "Attack": 10},
		ActiveSkills: []string{"SandTornado", "RockLance"}, Passives: []string{"Legend", "CraftSpeed_up3"},
	}
	info, err := manager.WritePalTemplate("reward_anubis", template)
	if err != nil || info.Name != "reward_anubis.json" || info.Size == 0 {
		t.Fatalf("WritePalTemplate = %#v, %v", info, err)
	}
	read, err := manager.ReadPalTemplate("reward_anubis.json")
	if err != nil || read.Gender != "None" || read.Level == nil || *read.Level != 50 || read.IsAwakening == nil || !*read.IsAwakening || len(read.Passives) != 2 {
		t.Fatalf("ReadPalTemplate = %#v, %v", read, err)
	}
	read.Nickname = "Updated"
	if _, err := manager.WritePalTemplate("reward_anubis", read); err != nil {
		t.Fatalf("overwrite template: %v", err)
	}
	list, err := manager.ListPalTemplates()
	if err != nil || len(list) != 1 || list[0].Name != "reward_anubis.json" {
		t.Fatalf("ListPalTemplates = %#v, %v", list, err)
	}
	if err := manager.DeletePalTemplate("reward_anubis"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ReadPalTemplate("reward_anubis"); !os.IsNotExist(err) {
		t.Fatalf("read deleted template error = %v", err)
	}
}

func TestPalTemplateValidationRejectsUnsafeOrInvalidValues(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	level := 0
	tests := []struct {
		name     string
		template PalTemplate
	}{
		{"path", PalTemplate{PalID: "Anubis"}},
		{"pal-id", PalTemplate{PalID: "../Anubis"}},
		{"level", PalTemplate{PalID: "Anubis", Level: &level}},
		{"skills", PalTemplate{PalID: "Anubis", ActiveSkills: []string{"one", "two", "three", "four"}}},
		{"ivs", PalTemplate{PalID: "Anubis", IVs: map[string]int{"AttackShot": 999}}},
	}
	for _, test := range tests {
		name := "valid_name"
		if test.name == "path" {
			name = "../bad"
		}
		if _, err := manager.WritePalTemplate(name, test.template); err == nil {
			t.Errorf("%s template should fail", test.name)
		}
	}
}

func TestPalTemplateValidationUsesLegalGameLimits(t *testing.T) {
	maximumPartnerSkill := 5
	maximumCondensedPals := 116
	valid := PalTemplate{
		PalID:             "Anubis",
		PartnerSkillLevel: &maximumPartnerSkill,
		CondensedPals:     &maximumCondensedPals,
		IVs:               map[string]int{"Health": 100, "AttackMelee": 100, "AttackShot": 100, "Defense": 100},
		PalSouls:          map[string]int{"Health": 20, "Attack": 20, "Defense": 20, "CraftSpeed": 20},
	}
	if err := validatePalTemplate(&valid); err != nil {
		t.Fatalf("legal maximum template should pass: %v", err)
	}

	tests := []struct {
		name     string
		template PalTemplate
	}{
		{"partner-skill", PalTemplate{PalID: "Anubis", PartnerSkillLevel: intPointer(6)}},
		{"condensed-pals", PalTemplate{PalID: "Anubis", CondensedPals: intPointer(117)}},
		{"iv", PalTemplate{PalID: "Anubis", IVs: map[string]int{"Health": 101}}},
		{"soul", PalTemplate{PalID: "Anubis", PalSouls: map[string]int{"Health": 21}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validatePalTemplate(&test.template); err == nil {
				t.Fatal("out-of-range template should fail")
			}
		})
	}
}

func TestListAndReadExportedPalTemplates(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	exportedDir := filepath.Join(manager.cfg.PalDefenderDir(), "Pals", "Exported", "steam_123")
	if err := os.MkdirAll(exportedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := `{"PalID":"Anubis","Nickname":"Existing Pal","Level":50,"ExtraWorkSuitabilities":{"Mining":5,"Anyone":1}}`
	if err := os.WriteFile(filepath.Join(exportedDir, "Anubis existing.json"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(exportedDir, "ignore.txt"), []byte("ignore"), 0o600); err != nil {
		t.Fatal(err)
	}

	list, err := manager.ListExportedPalTemplates("steam_123")
	if err != nil || len(list) != 1 || list[0].Name != "Anubis existing.json" || list[0].PlayerID != "steam_123" {
		t.Fatalf("ListExportedPalTemplates = %#v, %v", list, err)
	}
	template, err := manager.ReadExportedPalTemplate("steam_123", "Anubis existing.json")
	if err != nil || template.Nickname != "Existing Pal" || template.ExtraWorkSuitabilities["Anyone"] != 1 {
		t.Fatalf("ReadExportedPalTemplate = %#v, %v", template, err)
	}
	if _, err := manager.ReadExportedPalTemplate("steam_123", "../outside.json"); err == nil {
		t.Fatal("path traversal should be rejected")
	}
}

func TestReadPalTemplateRejectsTrailingJSON(t *testing.T) {
	manager, cleanup := testManager(t)
	defer cleanup()
	dir := manager.palTemplatesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "invalid.json"), []byte(`{"PalID":"Anubis"}{"PalID":"Lamball"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ReadPalTemplate("invalid"); err == nil {
		t.Fatal("multiple JSON objects should be rejected")
	}
}

func boolPointer(value bool) *bool { return &value }
func intPointer(value int) *int    { return &value }
