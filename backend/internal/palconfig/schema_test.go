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

func TestSchemaIncludesLocalizedPresentationMetadata(t *testing.T) {
	fields := schemaByKey()

	serverName := fields["ServerName"]
	if serverName.Label != "服务器名称" {
		t.Fatalf("ServerName label = %q", serverName.Label)
	}
	if serverName.Description == "" || serverName.Description == "Server name" {
		t.Fatalf("ServerName description was not localized: %q", serverName.Description)
	}

	deathPenalty := fields["DeathPenalty"]
	if deathPenalty.EnumLabels["All"] != "全部掉落（物品、装备和队伍帕鲁）" {
		t.Fatalf("DeathPenalty enum label for All = %q", deathPenalty.EnumLabels["All"])
	}

	for _, field := range Schema() {
		if field.Label == "" {
			t.Fatalf("field %s is missing localized label", field.Key)
		}
	}
}

func TestValidateWarnsRESTWithoutAdminPassword(t *testing.T) {
	issues := Validate(Settings{"RESTAPIEnabled": "True"})
	found := false
	for _, issue := range issues {
		if issue.Field == "AdminPassword" && issue.Severity == "warning" && contains(issue.Message, "管理员密码") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected AdminPassword warning: %#v", issues)
	}
}

func TestValidateAcceptsWholeNumberFloatForIntFields(t *testing.T) {
	issues := Validate(Settings{"ServerReplicatePawnCullDistance": "15000.000000"})
	for _, issue := range issues {
		if issue.Field == "ServerReplicatePawnCullDistance" && issue.Severity == "error" {
			t.Fatalf("expected whole-number float to be accepted for int field: %#v", issues)
		}
	}
}

func TestValidateRejectsFractionalFloatForIntFields(t *testing.T) {
	issues := Validate(Settings{"ServerReplicatePawnCullDistance": "15000.5"})
	for _, issue := range issues {
		if issue.Field == "ServerReplicatePawnCullDistance" && issue.Severity == "error" {
			return
		}
	}
	t.Fatalf("expected fractional float to be rejected for int field: %#v", issues)
}

func TestValidateReturnsLocalizedMessages(t *testing.T) {
	issues := Validate(Settings{
		"RCONEnabled":        "yes",
		"ServerPlayerMaxNum": "0",
		"DeathPenalty":       "Everything",
	})

	assertIssueMessage(t, issues, "RCONEnabled", "必须是 True 或 False")
	assertIssueMessage(t, issues, "ServerPlayerMaxNum", "数值必须大于等于 1")
	assertIssueMessage(t, issues, "DeathPenalty", "必须是以下值之一：None, Item, ItemAndEquipment, All")
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

func schemaByKey() map[string]FieldSchema {
	fields := map[string]FieldSchema{}
	for _, field := range Schema() {
		fields[field.Key] = field
	}
	return fields
}

func assertIssueMessage(t *testing.T, issues []ValidationIssue, field, message string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Field == field && issue.Message == message {
			return
		}
	}
	t.Fatalf("expected %s issue %q, got %#v", field, message, issues)
}
