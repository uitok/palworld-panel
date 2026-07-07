package palconfig

import "testing"

func TestSchemaContainsOfficialEggField(t *testing.T) {
	found := false
	for _, field := range Schema() {
		if field.Key == "PalEggDefaultHatchingTime" {
			found = true
		}
		if field.Key == "EggHatchTime" {
			t.Fatal("schema must not expose obsolete EggHatchTime")
		}
	}
	if !found {
		t.Fatal("missing PalEggDefaultHatchingTime")
	}
}

func TestValidateWarnsRESTWithoutAdminPassword(t *testing.T) {
	issues := Validate(Settings{"RESTAPIEnabled": "True"})
	found := false
	for _, issue := range issues {
		if issue.Field == "AdminPassword" && issue.Severity == "warning" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected AdminPassword warning: %#v", issues)
	}
}

func TestSerializeKeepsEnumAndListBare(t *testing.T) {
	got := Serialize(Settings{"DeathPenalty": "All", "DenyTechnologyList": `("GrapplingGun")`})
	if !contains(got, "DeathPenalty=All") {
		t.Fatalf("DeathPenalty should not be quoted: %s", got)
	}
	if !contains(got, `DenyTechnologyList=("GrapplingGun")`) {
		t.Fatalf("list should not be quoted: %s", got)
	}
}
