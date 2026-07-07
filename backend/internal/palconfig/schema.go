package palconfig

import (
	"fmt"
	"strconv"
	"strings"
)

type FieldType string

const (
	TypeString FieldType = "string"
	TypeBool   FieldType = "bool"
	TypeInt    FieldType = "int"
	TypeFloat  FieldType = "float"
	TypeEnum   FieldType = "enum"
	TypeList   FieldType = "list"
)

type FieldSchema struct {
	Key             string    `json:"key"`
	Group           string    `json:"group"`
	Type            FieldType `json:"type"`
	Default         string    `json:"default"`
	Enum            []string  `json:"enum,omitempty"`
	Min             *float64  `json:"min,omitempty"`
	Max             *float64  `json:"max,omitempty"`
	RequiresRestart bool      `json:"requires_restart"`
	Risk            string    `json:"risk,omitempty"`
	Description     string    `json:"description"`
}

type ValidationIssue struct {
	Field    string `json:"field,omitempty"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func Schema() []FieldSchema {
	return []FieldSchema{
		strField("ServerName", "server_management", "Palworld Server", "Server name"),
		strField("ServerDescription", "server_management", "", "Server description"),
		strField("AdminPassword", "server_management", "", "Password used for admin commands and REST API auth"),
		strField("ServerPassword", "server_management", "", "Password required to join the server"),
		intField("PublicPort", "server_management", "8211", 1, 65535, "External public port for community listing; does not change listen port"),
		strField("PublicIP", "server_management", "", "External public IP for community listing"),
		intField("ServerPlayerMaxNum", "server_management", "32", 1, 128, "Maximum number of players"),
		boolField("RCONEnabled", "server_management", "False", "Enable RCON"),
		intField("RCONPort", "server_management", "25575", 1, 65535, "RCON port"),
		boolField("RESTAPIEnabled", "server_management", "False", "Enable official REST API"),
		intField("RESTAPIPort", "server_management", "8212", 1, 65535, "Official REST API port"),
		enumField("LogFormatType", "server_management", "Text", []string{"Text", "Json"}, "Server log format"),
		boolField("bIsShowJoinLeftMessage", "server_management", "True", "Show join/leave messages"),
		boolField("bAllowClientMod", "server_management", "False", "Allow clients with mods to join"),
		intField("ChatPostLimitPerMinute", "server_management", "10", 1, 120, "Maximum chat messages per minute"),
		strField("CrossplayPlatforms", "server_management", "(Steam,Xbox,PS5,Mac)", "Allowed platforms"),

		intField("BaseCampMaxNumInGuild", "performance", "4", 1, 10, "Maximum bases per guild"),
		intField("BaseCampWorkerMaxNum", "performance", "15", 1, 50, "Maximum Pals per base"),
		intField("MaxBuildingLimitNum", "performance", "0", 0, 1000000, "Per-player building cap; 0 means unlimited"),
		intField("ServerReplicatePawnCullDistance", "performance", "15000", 5000, 15000, "Pal sync distance from players in cm"),
		floatField("ItemContainerForceMarkDirtyInterval", "performance", "1.0", 0, 3600, "Container UI resync interval in seconds"),

		floatField("DayTimeSpeedRate", "game_balance", "1.0", 0.1, 10, "Daytime progression speed"),
		floatField("NightTimeSpeedRate", "game_balance", "1.0", 0.1, 10, "Nighttime progression speed"),
		floatField("ExpRate", "game_balance", "1.0", 0.1, 20, "EXP gain multiplier"),
		floatField("PalCaptureRate", "game_balance", "1.0", 0.1, 20, "Capture rate multiplier"),
		floatField("PalSpawnNumRate", "game_balance", "1.0", 0.1, 10, "Pal spawn multiplier"),
		floatField("PalDamageRateAttack", "game_balance", "1.0", 0.1, 20, "Damage dealt by Pals"),
		floatField("PalDamageRateDefense", "game_balance", "1.0", 0.1, 20, "Damage taken by Pals"),
		floatField("PlayerDamageRateAttack", "game_balance", "1.0", 0.1, 20, "Damage dealt by players"),
		floatField("PlayerDamageRateDefense", "game_balance", "1.0", 0.1, 20, "Damage taken by players"),
		floatField("CollectionDropRate", "game_balance", "1.0", 0.1, 20, "Gatherable item drop multiplier"),
		floatField("EnemyDropItemRate", "game_balance", "1.0", 0.1, 20, "Enemy item drop multiplier"),
		floatField("PalEggDefaultHatchingTime", "game_balance", "72.0", 0, 240, "Time to hatch a Huge Egg in hours"),
		enumField("DeathPenalty", "game_balance", "All", []string{"None", "Item", "ItemAndEquipment", "All"}, "Death penalty"),
		floatField("PalStaminaDecreaceRate", "game_balance", "1.0", 0.1, 20, "Pal stamina depletion multiplier"),
		floatField("PalStomachDecreaceRate", "game_balance", "1.0", 0.1, 20, "Pal hunger depletion multiplier"),
		floatField("PlayerStaminaDecreaceRate", "game_balance", "1.0", 0.1, 20, "Player stamina depletion multiplier"),
		floatField("PlayerStomachDecreaceRate", "game_balance", "1.0", 0.1, 20, "Player hunger depletion multiplier"),
		intField("SupplyDropSpan", "game_balance", "180", 1, 10080, "Meteorite / supply drop interval in minutes"),
		floatField("BlockRespawnTime", "game_balance", "0.0", 0, 3600, "Respawn cooldown in seconds"),

		boolField("bEnableFastTravel", "features", "True", "Enable fast travel"),
		boolField("bEnableFastTravelOnlyBaseCamp", "features", "False", "Restrict fast travel to bases"),
		boolField("bExistPlayerAfterLogout", "features", "False", "Keep players sleeping in-world after logout"),
		boolField("bIsStartLocationSelectByMap", "features", "True", "Allow start location selection"),
		boolField("bHardcore", "features", "False", "Enable hardcore mode"),
		boolField("bShowPlayerList", "features", "True", "Show player list in ESC menu"),
		boolField("bIsUseBackupSaveData", "features", "True", "Enable game save-data backups"),
		boolField("bAllowGlobalPalboxExport", "features", "False", "Allow export to Global Palbox"),
		boolField("bAllowGlobalPalboxImport", "features", "False", "Allow import from Global Palbox"),
		boolField("bAutoResetGuildNoOnlinePlayers", "features", "False", "Auto-delete inactive guild structures and base Pals"),
		floatField("AutoResetGuildTimeNoOnlinePlayers", "features", "72.0", 0, 10000, "Inactive guild timeout"),
		strField("RandomizerType", "features", "None", "Pal spawn randomizer mode"),
		strField("RandomizerSeed", "features", "", "Pal spawn randomizer seed"),

		boolField("bIsPvP", "pvp", "False", "Enable PvP"),
		boolField("bEnablePlayerToPlayerDamage", "pvp", "False", "Allow player-to-player damage"),
		boolField("bEnableDefenseOtherGuildPlayer", "pvp", "False", "Base Pals attack hostile players"),
		boolField("bAllowEnhanceStat_Health", "pvp", "True", "Allow HP stat allocation"),
		boolField("bAllowEnhanceStat_Attack", "pvp", "True", "Allow Attack stat allocation"),
		boolField("bAllowEnhanceStat_Stamina", "pvp", "True", "Allow Stamina stat allocation"),
		boolField("bAllowEnhanceStat_Weight", "pvp", "True", "Allow Weight stat allocation"),
		boolField("bAllowEnhanceStat_WorkSpeed", "pvp", "True", "Allow Work Speed stat allocation"),
		boolField("bBuildAreaLimit", "pvp", "False", "Restrict building in important locations"),
		boolField("bCanPickupOtherGuildDeathPenaltyDrop", "pvp", "False", "Allow picking up other guild death drops"),
		boolField("bAdditionalDropItemWhenPlayerKillingInPvPMode", "pvp", "False", "Drop special item on PvP kill"),
		strField("AdditionalDropItemWhenPlayerKillingInPvPMode", "pvp", "", "PvP kill reward item"),
		floatField("AdditionalDropItemNumWhenPlayerKillingInPvPMode", "pvp", "1.0", 0, 1000, "PvP kill reward amount"),
		boolField("bDisplayPvPItemNumOnWorldMap_BaseCamp", "pvp", "False", "Show PvP item counts in bases"),
		boolField("bDisplayPvPItemNumOnWorldMap_Player", "pvp", "False", "Show player PvP item counts"),

		listField("DenyTechnologyList", "technology", "", "Technology IDs to disable"),
	}
}

func Defaults() Settings {
	out := Settings{}
	for _, f := range Schema() {
		if f.Default != "" {
			out[f.Key] = f.Default
		}
	}
	return out
}

func Validate(settings Settings) []ValidationIssue {
	var issues []ValidationIssue
	lookup := map[string]FieldSchema{}
	for _, f := range Schema() {
		lookup[f.Key] = f
	}
	for key, value := range settings {
		schema, ok := lookup[key]
		if !ok {
			continue
		}
		if issue := validateField(schema, value); issue != nil {
			issues = append(issues, *issue)
		}
	}
	if strings.EqualFold(settings["RESTAPIEnabled"], "True") && strings.TrimSpace(settings["AdminPassword"]) == "" {
		issues = append(issues, ValidationIssue{Field: "AdminPassword", Severity: "warning", Message: "REST API requires AdminPassword for authenticated access"})
	}
	if parseFloat(settings["PalSpawnNumRate"]) > 3 {
		issues = append(issues, ValidationIssue{Field: "PalSpawnNumRate", Severity: "warning", Message: "high spawn rates can increase server load"})
	}
	if parseFloat(settings["BaseCampWorkerMaxNum"]) > 30 {
		issues = append(issues, ValidationIssue{Field: "BaseCampWorkerMaxNum", Severity: "warning", Message: "large base worker counts can increase server load"})
	}
	if strings.EqualFold(settings["bIsPvP"], "True") {
		for _, key := range []string{"bEnablePlayerToPlayerDamage", "bEnableDefenseOtherGuildPlayer"} {
			if !strings.EqualFold(settings[key], "True") {
				issues = append(issues, ValidationIssue{Field: key, Severity: "warning", Message: "official PvP setup recommends enabling this together with bIsPvP"})
			}
		}
	}
	return issues
}

func validateField(schema FieldSchema, value string) *ValidationIssue {
	switch schema.Type {
	case TypeBool:
		if !isBool(value) {
			return &ValidationIssue{Field: schema.Key, Severity: "error", Message: "must be True or False"}
		}
	case TypeInt:
		i, err := strconv.Atoi(value)
		if err != nil {
			return &ValidationIssue{Field: schema.Key, Severity: "error", Message: "must be an integer"}
		}
		return validateNumber(schema, float64(i))
	case TypeFloat:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return &ValidationIssue{Field: schema.Key, Severity: "error", Message: "must be a number"}
		}
		return validateNumber(schema, f)
	case TypeEnum:
		for _, item := range schema.Enum {
			if strings.EqualFold(item, value) {
				return nil
			}
		}
		return &ValidationIssue{Field: schema.Key, Severity: "error", Message: fmt.Sprintf("must be one of: %s", strings.Join(schema.Enum, ", "))}
	}
	return nil
}

func validateNumber(schema FieldSchema, value float64) *ValidationIssue {
	if schema.Min != nil && value < *schema.Min {
		return &ValidationIssue{Field: schema.Key, Severity: "error", Message: fmt.Sprintf("must be >= %v", *schema.Min)}
	}
	if schema.Max != nil && value > *schema.Max {
		return &ValidationIssue{Field: schema.Key, Severity: "error", Message: fmt.Sprintf("must be <= %v", *schema.Max)}
	}
	return nil
}

func isBool(value string) bool {
	return strings.EqualFold(value, "True") || strings.EqualFold(value, "False")
}

func parseFloat(value string) float64 {
	f, _ := strconv.ParseFloat(value, 64)
	return f
}

func strField(key, group, def, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeString, Default: def, RequiresRestart: true, Description: desc}
}

func boolField(key, group, def, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeBool, Default: def, RequiresRestart: true, Description: desc}
}

func intField(key, group, def string, min, max float64, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeInt, Default: def, Min: &min, Max: &max, RequiresRestart: true, Description: desc}
}

func floatField(key, group, def string, min, max float64, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeFloat, Default: def, Min: &min, Max: &max, RequiresRestart: true, Description: desc}
}

func enumField(key, group, def string, enum []string, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeEnum, Default: def, Enum: enum, RequiresRestart: true, Description: desc}
}

func listField(key, group, def, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeList, Default: def, RequiresRestart: true, Description: desc}
}
