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

func TestValidateRejectsNonFiniteNumbersAndIntOverflow(t *testing.T) {
	for _, test := range []struct {
		field string
		value string
	}{
		{field: "DayTimeSpeedRate", value: "NaN"},
		{field: "DayTimeSpeedRate", value: "+Inf"},
		{field: "ServerPlayerMaxNum", value: "NaN"},
		{field: "ServerPlayerMaxNum", value: "9223372036854775808"},
		{field: "ServerPlayerMaxNum", value: "1e100"},
	} {
		issues := Validate(Settings{test.field: test.value})
		if !hasIssue(issues, test.field, "error") {
			t.Errorf("%s=%q was accepted: %#v", test.field, test.value, issues)
		}
	}
}

func TestValidateReturnsLocalizedMessages(t *testing.T) {
	issues := Validate(Settings{
		"RCONEnabled":        "yes",
		"ServerPlayerMaxNum": "many",
		"DeathPenalty":       "Everything",
	})

	assertIssueMessage(t, issues, "RCONEnabled", "必须是 True 或 False")
	assertIssueMessage(t, issues, "ServerPlayerMaxNum", "必须是整数")
	assertIssueMessage(t, issues, "DeathPenalty", "必须是以下值之一：None, Item, ItemAndEquipment, All")
}

func TestCrossplayPlatformsUsesStructuredListSchema(t *testing.T) {
	for _, field := range Schema() {
		if field.Key == "CrossplayPlatforms" {
			if field.Type != TypeList {
				t.Fatalf("CrossplayPlatforms type = %q, want list", field.Type)
			}
			return
		}
	}
	t.Fatal("CrossplayPlatforms schema missing")
}

func TestValidateRejectsMalformedStructuredLists(t *testing.T) {
	for _, value := range []string{
		"(Steam,)",
		"(Steam",
		"Steam,Xbox",
		"(Steam),ServerPassword=bad",
		`("unterminated)`,
		"((Steam))",
	} {
		issues := Validate(Settings{"CrossplayPlatforms": value})
		if !hasIssue(issues, "CrossplayPlatforms", "error") {
			t.Errorf("CrossplayPlatforms=%q was accepted: %#v", value, issues)
		}
	}
}

func hasIssue(issues []ValidationIssue, field, severity string) bool {
	for _, issue := range issues {
		if issue.Field == field && issue.Severity == severity {
			return true
		}
	}
	return false
}

func TestSchemaCoversOfficialOneDotZeroFields(t *testing.T) {
	fields := schemaByKey()
	for _, key := range []string{
		"BaseCampMaxNum",
		"BuildObjectDamageRate",
		"BuildObjectDeteriorationDamageRate",
		"CollectionObjectHpRate",
		"CollectionObjectRespawnSpeedRate",
		"EquipmentDurabilityDamageRate",
		"GuildPlayerMaxNum",
		"GuildRejoinCooldownMinutes",
		"ItemCorruptionMultiplier",
		"ItemWeightRate",
		"MonsterFarmActionSpeedRate",
		"PalAutoHPRegeneRate",
		"PalAutoHpRegeneRateInSleep",
		"PhysicsActiveDropItemMaxNum",
		"PlayerAutoHPRegeneRate",
		"PlayerAutoHpRegeneRateInSleep",
		"RespawnPenaltyDurationThreshold",
		"RespawnPenaltyTimeScale",
		"VoiceChatMaxVolumeDistance",
		"VoiceChatZeroVolumeDistance",
		"bCharacterRecreateInHardcore",
		"bEnableAimAssistPad",
		"bEnableBuildingPlayerUIdDisplay",
		"bEnableInvaderEnemy",
		"bEnableVoiceChat",
		"bInvisibleOtherGuildBaseCampAreaFX",
		"bIsRandomizerPalLevelRandom",
		"bPalLost",
	} {
		if _, ok := fields[key]; !ok {
			t.Errorf("schema is missing official 1.0.0 field %s", key)
		}
	}
	if _, ok := fields["AllowConnectPlatform"]; ok {
		t.Fatal("schema must exclude unavailable AllowConnectPlatform")
	}
}

func TestSchemaUsesUpdatedDedicatedServerDefaults(t *testing.T) {
	fields := schemaByKey()
	want := map[string]string{
		"PhysicsActiveDropItemMaxNum":     "-1",
		"MonsterFarmActionSpeedRate":      "1.0",
		"bEnableVoiceChat":                "False",
		"VoiceChatMaxVolumeDistance":      "3000.0",
		"VoiceChatZeroVolumeDistance":     "15000.0",
		"bEnableBuildingPlayerUIdDisplay": "False",
		"DeathPenalty":                    "Item",
		"PalEggDefaultHatchingTime":       "1.0",
	}
	for key, expected := range want {
		if fields[key].Default == nil || *fields[key].Default != expected {
			t.Errorf("%s default = %#v, want %q", key, fields[key].Default, expected)
		}
	}
}

func TestSchemaOnlyPublishesSupportedNumericRanges(t *testing.T) {
	fields := schemaByKey()
	if fields["ServerPlayerMaxNum"].Min != nil || fields["ServerPlayerMaxNum"].Max != nil {
		t.Fatal("ServerPlayerMaxNum must not publish an undocumented range")
	}
	if fields["BaseCampMaxNumInGuild"].Max == nil || *fields["BaseCampMaxNumInGuild"].Max != 10 {
		t.Fatal("BaseCampMaxNumInGuild must publish the official maximum of 10")
	}
	if fields["ServerReplicatePawnCullDistance"].Min == nil || *fields["ServerReplicatePawnCullDistance"].Min != 5000 {
		t.Fatal("ServerReplicatePawnCullDistance must publish the official minimum")
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
