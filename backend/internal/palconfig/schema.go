package palconfig

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const SchemaVersion = "1.0.0"

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
	Key             string            `json:"key"`
	Label           string            `json:"label"`
	Group           string            `json:"group"`
	Type            FieldType         `json:"type"`
	Default         *string           `json:"default"`
	Enum            []string          `json:"enum,omitempty"`
	EnumLabels      map[string]string `json:"enum_labels,omitempty"`
	Min             *float64          `json:"min,omitempty"`
	Max             *float64          `json:"max,omitempty"`
	RequiresRestart bool              `json:"requires_restart"`
	Risk            string            `json:"risk,omitempty"`
	Description     string            `json:"description"`
}

type ValidationIssue struct {
	Field    string `json:"field,omitempty"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func Schema() []FieldSchema {
	return localizeSchema([]FieldSchema{
		strField("ServerName", "server_management", "Default Palworld Server", "服务器名称。会显示在服务端列表和玩家连接信息中。"),
		strField("ServerDescription", "server_management", "", "服务器简介。用于说明服务器规则、玩法或公告。"),
		strField("AdminPassword", "server_management", "", "管理员密码。用于游戏内管理员指令和官方 REST API 认证。"),
		strField("ServerPassword", "server_management", "", "服务器加入密码。留空表示玩家连接时不需要密码。"),
		intField("PublicPort", "server_management", "8211", 1, 65535, "社区服务器对外公布的端口；不会改变服务端实际监听端口。"),
		strField("PublicIP", "server_management", "", "社区服务器对外公布的公网 IP。留空时由服务端自动检测。"),
		intField("ServerPlayerMaxNum", "server_management", "32", 1, 128, "允许加入服务器的最大玩家数。"),
		boolField("RCONEnabled", "server_management", "False", "是否启用 RCON 远程控制。"),
		intField("RCONPort", "server_management", "25575", 1, 65535, "RCON 使用的端口。"),
		boolField("RESTAPIEnabled", "server_management", "False", "是否启用官方 REST API。面板初始化配置时会为监控和管理功能启用它。"),
		intField("RESTAPIPort", "server_management", "8212", 1, 65535, "官方 REST API 的监听端口。"),
		enumField("LogFormatType", "server_management", "Text", []string{"Text", "Json"}, "服务端日志格式，可选纯文本或 JSON。"),
		boolField("bIsShowJoinLeftMessage", "server_management", "True", "是否在专用服务器中显示玩家加入和离开消息。"),
		boolField("bAllowClientMod", "server_management", "True", "是否允许启用客户端 Mod 的玩家加入服务器。"),
		boolField("bEnableBuildingPlayerUIdDisplay", "server_management", "False", "是否在建筑上显示创建者的玩家 ID。"),
		intField("ChatPostLimitPerMinute", "server_management", "30", 1, 120, "每名玩家每分钟最多可发送的聊天消息数。"),
		strField("CrossplayPlatforms", "server_management", "(Steam,Xbox,PS5,Mac)", "允许连接服务器的平台。默认值为 (Steam,Xbox,PS5,Mac)。"),

		intField("BaseCampMaxNum", "performance", "128", 0, 0, "服务器中可存在的据点总数。"),
		intField("BaseCampMaxNumInGuild", "performance", "4", 0, 10, "每个公会可拥有的最大据点数。官方上限为 10；提高该值会增加服务器负载。"),
		intField("BaseCampWorkerMaxNum", "performance", "15", 0, 50, "每个据点可工作的最大帕鲁数。官方上限为 50；提高该值会增加服务器负载。"),
		intField("MaxBuildingLimitNum", "performance", "0", 0, 1000000, "每名玩家可建造的建筑数量上限；0 表示不限制。"),
		intField("PhysicsActiveDropItemMaxNum", "performance", "-1", 0, 0, "可使用物理行为的掉落物最大数量；官方默认值为 -1。"),
		intField("ServerReplicatePawnCullDistance", "performance", "15000", 5000, 15000, "玩家周围同步帕鲁的距离，单位为厘米，范围 5000 到 15000。"),
		floatField("ItemContainerForceMarkDirtyInterval", "performance", "1.0", 0, 3600, "打开容器界面时强制重新同步的间隔，单位为秒。"),

		floatField("DayTimeSpeedRate", "game_balance", "1.0", 0.1, 10, "白天时间流逝速度倍率。"),
		floatField("NightTimeSpeedRate", "game_balance", "1.0", 0.1, 10, "夜晚时间流逝速度倍率。"),
		floatField("ExpRate", "game_balance", "1.0", 0.1, 20, "经验值获取倍率。"),
		floatField("PalCaptureRate", "game_balance", "1.0", 0.1, 20, "帕鲁捕获率倍率。"),
		floatField("PalSpawnNumRate", "game_balance", "1.0", 0.1, 10, "帕鲁刷新数量倍率。较高数值会影响服务器性能。"),
		floatField("PalDamageRateAttack", "game_balance", "1.0", 0.1, 20, "帕鲁造成伤害倍率。"),
		floatField("PalDamageRateDefense", "game_balance", "1.0", 0.1, 20, "帕鲁受到伤害倍率。"),
		floatField("PlayerDamageRateAttack", "game_balance", "1.0", 0.1, 20, "玩家造成伤害倍率。"),
		floatField("PlayerDamageRateDefense", "game_balance", "1.0", 0.1, 20, "玩家受到伤害倍率。"),
		floatField("BuildObjectDamageRate", "game_balance", "1.0", 0, 0, "建筑受到伤害的倍率。"),
		floatField("BuildObjectDeteriorationDamageRate", "game_balance", "1.0", 0, 0, "建筑劣化速度倍率。"),
		floatField("CollectionDropRate", "game_balance", "1.0", 0.1, 20, "可采集物品掉落倍率。"),
		floatField("CollectionObjectHpRate", "game_balance", "1.0", 0, 0, "可采集物体的生命值倍率。"),
		floatField("CollectionObjectRespawnSpeedRate", "game_balance", "1.0", 0, 0, "可采集物体的重生间隔倍率。"),
		floatField("EnemyDropItemRate", "game_balance", "1.0", 0.1, 20, "敌人掉落物品数量倍率。"),
		floatField("EquipmentDurabilityDamageRate", "game_balance", "1.0", 0, 0, "装备耐久度损耗倍率。"),
		intField("GuildPlayerMaxNum", "game_balance", "20", 0, 0, "每个公会允许的最大玩家数。"),
		floatField("GuildRejoinCooldownMinutes", "game_balance", "0", 0, 0, "退出公会后再次加入公会的冷却时间，单位为分钟。"),
		floatField("ItemCorruptionMultiplier", "game_balance", "1.0", 0, 0, "物品腐坏速度倍率。"),
		floatField("ItemWeightRate", "game_balance", "1.0", 0, 0, "物品重量倍率。"),
		floatField("MonsterFarmActionSpeedRate", "game_balance", "1.0", 0, 0, "牧场放牧产物的生产速度倍率。"),
		floatField("PalEggDefaultHatchingTime", "game_balance", "1.0", 0, 240, "巨大蛋孵化所需时间，单位为小时；其他蛋也需要孵化时间。"),
		enumField("DeathPenalty", "game_balance", "Item", []string{"None", "Item", "ItemAndEquipment", "All"}, "死亡惩罚。可控制死亡时是否掉落物品、装备和队伍中的帕鲁。"),
		floatField("PalStaminaDecreaceRate", "game_balance", "1.0", 0.1, 20, "帕鲁耐力消耗倍率。"),
		floatField("PalStomachDecreaceRate", "game_balance", "1.0", 0.1, 20, "帕鲁饱食度消耗倍率。"),
		floatField("PlayerStaminaDecreaceRate", "game_balance", "1.0", 0.1, 20, "玩家耐力消耗倍率。"),
		floatField("PlayerStomachDecreaceRate", "game_balance", "1.0", 0.1, 20, "玩家饱食度消耗倍率。"),
		floatField("PalAutoHPRegeneRate", "game_balance", "1.0", 0, 0, "帕鲁自然生命恢复倍率。"),
		floatField("PalAutoHpRegeneRateInSleep", "game_balance", "1.0", 0, 0, "帕鲁在帕鲁终端中睡眠时的生命恢复倍率。"),
		floatField("PlayerAutoHPRegeneRate", "game_balance", "1.0", 0, 0, "玩家自然生命恢复倍率。"),
		floatField("PlayerAutoHpRegeneRateInSleep", "game_balance", "1.0", 0, 0, "玩家睡眠时的生命恢复倍率。"),
		intField("SupplyDropSpan", "game_balance", "180", 1, 10080, "陨石和补给投放间隔，单位为分钟。"),
		floatField("BlockRespawnTime", "game_balance", "5.0", 0, 3600, "死亡后可再次复活前的冷却时间，单位为秒。"),
		floatField("RespawnPenaltyDurationThreshold", "game_balance", "0.0", 0, 0, "再次死亡时应用复活冷却倍率的生存时间阈值，单位为秒。"),
		floatField("RespawnPenaltyTimeScale", "game_balance", "2.0", 0, 0, "连续死亡时应用到复活冷却的倍率。"),

		boolField("bEnableFastTravel", "features", "True", "是否启用快速传送。"),
		boolField("bEnableFastTravelOnlyBaseCamp", "features", "False", "是否将快速传送限制为仅在据点之间移动。"),
		boolField("bExistPlayerAfterLogout", "features", "False", "是否让玩家下线后在原地进入睡眠状态，避免通过下线脱离战斗。"),
		boolField("bIsStartLocationSelectByMap", "features", "False", "是否允许玩家开始游戏时在地图上选择初始位置。"),
		boolField("bHardcore", "features", "False", "是否启用硬核模式。启用后角色死亡将无法复活。"),
		boolField("bCharacterRecreateInHardcore", "features", "False", "硬核模式中死亡后是否允许重新创建角色。"),
		boolField("bPalLost", "features", "False", "死亡时是否永久失去帕鲁。"),
		boolField("bEnableInvaderEnemy", "features", "True", "是否启用入侵敌人事件。"),
		boolField("bShowPlayerList", "features", "False", "是否在 ESC 菜单显示玩家列表。"),
		boolField("bIsUseBackupSaveData", "features", "True", "是否启用世界存档备份。启用后会增加磁盘负载。"),
		boolField("bAllowGlobalPalboxExport", "features", "True", "是否允许保存到全局帕鲁终端。"),
		boolField("bAllowGlobalPalboxImport", "features", "False", "是否允许从全局帕鲁终端读取。"),
		boolField("bAutoResetGuildNoOnlinePlayers", "features", "False", "公会成员长期不在线时，是否自动删除该公会建筑和据点帕鲁。"),
		floatField("AutoResetGuildTimeNoOnlinePlayers", "features", "72.0", 0, 10000, "触发自动清理前的离线时长；bAutoResetGuildNoOnlinePlayers 为 False 时无效。"),
		enumField("RandomizerType", "features", "None", []string{"None", "Region", "All"}, "帕鲁刷新随机化模式：None 为不随机，Region 为按区域随机，All 为完全随机。"),
		strField("RandomizerSeed", "features", "", "帕鲁刷新随机化使用的种子值。"),
		boolField("bIsRandomizerPalLevelRandom", "features", "False", "随机化时是否让野生帕鲁等级完全随机；关闭时按区域等级范围随机。"),
		boolField("bEnableVoiceChat", "features", "False", "是否启用游戏内语音聊天。"),
		floatField("VoiceChatMaxVolumeDistance", "features", "3000.0", 0, 0, "语音聊天保持最大音量的距离。"),
		floatField("VoiceChatZeroVolumeDistance", "features", "15000.0", 0, 0, "语音聊天音量衰减为零的距离。"),
		boolField("bInvisibleOtherGuildBaseCampAreaFX", "features", "False", "是否隐藏其他公会的据点区域边界。"),

		boolField("bIsPvP", "pvp", "False", "是否启用 PvP。官方说明该功能仍为试用功能。"),
		boolField("bEnablePlayerToPlayerDamage", "pvp", "False", "是否允许玩家之间互相造成伤害。启用 PvP 时官方建议一并开启。"),
		boolField("bEnableDefenseOtherGuildPlayer", "pvp", "False", "是否让据点帕鲁攻击进入据点的敌对玩家。启用 PvP 时官方建议一并开启。"),
		boolField("bEnableAimAssistPad", "pvp", "True", "是否为使用手柄的玩家启用瞄准辅助。"),
		boolField("bAllowEnhanceStat_Health", "pvp", "True", "是否允许玩家将属性点分配到生命值。"),
		boolField("bAllowEnhanceStat_Attack", "pvp", "True", "是否允许玩家将属性点分配到攻击力。"),
		boolField("bAllowEnhanceStat_Stamina", "pvp", "True", "是否允许玩家将属性点分配到耐力。"),
		boolField("bAllowEnhanceStat_Weight", "pvp", "True", "是否允许玩家将属性点分配到负重。"),
		boolField("bAllowEnhanceStat_WorkSpeed", "pvp", "True", "是否允许玩家将属性点分配到工作速度。"),
		boolField("bBuildAreaLimit", "pvp", "False", "是否限制在快速传送点、出生点等重要位置附近建造。"),
		boolField("bCanPickupOtherGuildDeathPenaltyDrop", "pvp", "False", "是否允许拾取其他公会玩家因死亡惩罚掉落的物品和帕鲁。"),
		boolField("bAdditionalDropItemWhenPlayerKillingInPvPMode", "pvp", "False", "PvP 中击杀玩家时，是否额外掉落指定物品。"),
		strField("AdditionalDropItemWhenPlayerKillingInPvPMode", "pvp", "PlayerDropItem", "启用 PvP 击杀额外掉落后，要掉落的物品 ID。"),
		floatField("AdditionalDropItemNumWhenPlayerKillingInPvPMode", "pvp", "1.0", 0, 1000, "启用 PvP 击杀额外掉落后，掉落物品的数量。"),
		boolField("bDisplayPvPItemNumOnWorldMap_BaseCamp", "pvp", "False", "是否在地图上显示各据点持有的 PvP 专属物品数量。"),
		boolField("bDisplayPvPItemNumOnWorldMap_Player", "pvp", "False", "是否在地图上显示玩家位置及其持有的 PvP 专属物品数量。"),

		listField("DenyTechnologyList", "technology", "", "要禁用的科技 ID 列表，例如 DenyTechnologyList=(\"GrapplingGun\")。"),
	})
}

func Defaults() Settings {
	out := Settings{}
	for _, f := range Schema() {
		if f.Default != nil {
			out[f.Key] = *f.Default
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
		issues = append(issues, ValidationIssue{Field: "AdminPassword", Severity: "warning", Message: "启用 REST API 时建议设置管理员密码，否则认证访问不可用"})
	}
	if parseFloat(settings["PalSpawnNumRate"]) > 3 {
		issues = append(issues, ValidationIssue{Field: "PalSpawnNumRate", Severity: "warning", Message: "帕鲁刷新倍率过高可能增加服务器负载"})
	}
	if parseFloat(settings["BaseCampWorkerMaxNum"]) > 30 {
		issues = append(issues, ValidationIssue{Field: "BaseCampWorkerMaxNum", Severity: "warning", Message: "据点工作帕鲁数量过高可能增加服务器负载"})
	}
	if strings.EqualFold(settings["bIsPvP"], "True") {
		for _, key := range []string{"bEnablePlayerToPlayerDamage", "bEnableDefenseOtherGuildPlayer"} {
			if !strings.EqualFold(settings[key], "True") {
				issues = append(issues, ValidationIssue{Field: key, Severity: "warning", Message: "官方 PvP 配置建议与 bIsPvP 一起启用此项"})
			}
		}
	}
	return issues
}

func validateField(schema FieldSchema, value string) *ValidationIssue {
	switch schema.Type {
	case TypeBool:
		if !isBool(value) {
			return &ValidationIssue{Field: schema.Key, Severity: "error", Message: "必须是 True 或 False"}
		}
	case TypeInt:
		i, err := strconv.Atoi(value)
		if err != nil {
			f, floatErr := strconv.ParseFloat(value, 64)
			if floatErr != nil || math.Trunc(f) != f {
				return &ValidationIssue{Field: schema.Key, Severity: "error", Message: "必须是整数"}
			}
			return validateNumber(schema, f)
		}
		return validateNumber(schema, float64(i))
	case TypeFloat:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return &ValidationIssue{Field: schema.Key, Severity: "error", Message: "必须是数字"}
		}
		return validateNumber(schema, f)
	case TypeEnum:
		for _, item := range schema.Enum {
			if strings.EqualFold(item, value) {
				return nil
			}
		}
		return &ValidationIssue{Field: schema.Key, Severity: "error", Message: fmt.Sprintf("必须是以下值之一：%s", strings.Join(schema.Enum, ", "))}
	}
	return nil
}

func validateNumber(schema FieldSchema, value float64) *ValidationIssue {
	if schema.Min != nil && value < *schema.Min {
		return &ValidationIssue{Field: schema.Key, Severity: "error", Message: fmt.Sprintf("数值必须大于等于 %v", *schema.Min)}
	}
	if schema.Max != nil && value > *schema.Max {
		return &ValidationIssue{Field: schema.Key, Severity: "error", Message: fmt.Sprintf("数值必须小于等于 %v", *schema.Max)}
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
	return FieldSchema{Key: key, Group: group, Type: TypeString, Default: stringPointer(def), RequiresRestart: true, Description: desc}
}

func boolField(key, group, def, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeBool, Default: stringPointer(def), RequiresRestart: true, Description: desc}
}

func intField(key, group, def string, min, max float64, desc string) FieldSchema {
	field := FieldSchema{Key: key, Group: group, Type: TypeInt, Default: stringPointer(def), RequiresRestart: true, Description: desc}
	switch key {
	case "PublicPort", "RCONPort", "RESTAPIPort":
		field.Min, field.Max = stringNumberPointer(1), stringNumberPointer(65535)
	case "BaseCampMaxNumInGuild", "BaseCampWorkerMaxNum":
		field.Max = stringNumberPointer(max)
	case "ServerReplicatePawnCullDistance":
		field.Min, field.Max = stringNumberPointer(min), stringNumberPointer(max)
	}
	return field
}

func floatField(key, group, def string, _, _ float64, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeFloat, Default: stringPointer(def), RequiresRestart: true, Description: desc}
}

func enumField(key, group, def string, enum []string, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeEnum, Default: stringPointer(def), Enum: enum, RequiresRestart: true, Description: desc}
}

func listField(key, group, def, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeList, Default: stringPointer(def), RequiresRestart: true, Description: desc}
}

func optionalBoolField(key, group, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeBool, RequiresRestart: true, Description: desc}
}

func optionalIntField(key, group, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeInt, RequiresRestart: true, Description: desc}
}

func optionalFloatField(key, group, desc string) FieldSchema {
	return FieldSchema{Key: key, Group: group, Type: TypeFloat, RequiresRestart: true, Description: desc}
}

func stringPointer(value string) *string {
	return &value
}

func stringNumberPointer(value float64) *float64 {
	return &value
}

func localizeSchema(fields []FieldSchema) []FieldSchema {
	for i := range fields {
		if label, ok := fieldLabels[fields[i].Key]; ok {
			fields[i].Label = label
		} else {
			fields[i].Label = fields[i].Key
		}
		if labels, ok := enumLabels[fields[i].Key]; ok {
			fields[i].EnumLabels = labels
		}
	}
	return fields
}

var fieldLabels = map[string]string{
	"ServerName":                                      "服务器名称",
	"ServerDescription":                               "服务器简介",
	"AdminPassword":                                   "管理员密码",
	"ServerPassword":                                  "加入密码",
	"PublicPort":                                      "公网端口",
	"PublicIP":                                        "公网 IP",
	"ServerPlayerMaxNum":                              "最大玩家数",
	"RCONEnabled":                                     "启用 RCON",
	"RCONPort":                                        "RCON 端口",
	"RESTAPIEnabled":                                  "启用 REST API",
	"RESTAPIPort":                                     "REST API 端口",
	"LogFormatType":                                   "日志格式",
	"bIsShowJoinLeftMessage":                          "显示加入和离开消息",
	"bAllowClientMod":                                 "允许客户端 Mod",
	"bEnableBuildingPlayerUIdDisplay":                 "显示建筑创建者 ID",
	"ChatPostLimitPerMinute":                          "每分钟聊天上限",
	"CrossplayPlatforms":                              "跨平台连接范围",
	"BaseCampMaxNumInGuild":                           "每公会据点上限",
	"BaseCampMaxNum":                                  "服务器据点总数上限",
	"BaseCampWorkerMaxNum":                            "每据点工作帕鲁上限",
	"MaxBuildingLimitNum":                             "玩家建筑数量上限",
	"PhysicsActiveDropItemMaxNum":                     "物理掉落物数量上限",
	"ServerReplicatePawnCullDistance":                 "帕鲁同步距离",
	"ItemContainerForceMarkDirtyInterval":             "容器强制同步间隔",
	"DayTimeSpeedRate":                                "白天速度倍率",
	"NightTimeSpeedRate":                              "夜晚速度倍率",
	"ExpRate":                                         "经验倍率",
	"PalCaptureRate":                                  "捕获率倍率",
	"PalSpawnNumRate":                                 "帕鲁刷新倍率",
	"PalDamageRateAttack":                             "帕鲁攻击倍率",
	"PalDamageRateDefense":                            "帕鲁受伤倍率",
	"PlayerDamageRateAttack":                          "玩家攻击倍率",
	"PlayerDamageRateDefense":                         "玩家受伤倍率",
	"BuildObjectDamageRate":                           "建筑受伤倍率",
	"BuildObjectDeteriorationDamageRate":              "建筑劣化倍率",
	"CollectionDropRate":                              "采集掉落倍率",
	"CollectionObjectHpRate":                          "采集物生命倍率",
	"CollectionObjectRespawnSpeedRate":                "采集物重生间隔倍率",
	"EnemyDropItemRate":                               "敌人掉落倍率",
	"EquipmentDurabilityDamageRate":                   "装备耐久损耗倍率",
	"GuildPlayerMaxNum":                               "公会玩家上限",
	"GuildRejoinCooldownMinutes":                      "公会重新加入冷却",
	"ItemCorruptionMultiplier":                        "物品腐坏速度倍率",
	"ItemWeightRate":                                  "物品重量倍率",
	"MonsterFarmActionSpeedRate":                      "牧场生产速度倍率",
	"PalEggDefaultHatchingTime":                       "帕鲁蛋孵化时间",
	"DeathPenalty":                                    "死亡惩罚",
	"PalStaminaDecreaceRate":                          "帕鲁耐力消耗倍率",
	"PalStomachDecreaceRate":                          "帕鲁饱食度消耗倍率",
	"PlayerStaminaDecreaceRate":                       "玩家耐力消耗倍率",
	"PlayerStomachDecreaceRate":                       "玩家饱食度消耗倍率",
	"PalAutoHPRegeneRate":                             "帕鲁自然回血倍率",
	"PalAutoHpRegeneRateInSleep":                      "帕鲁睡眠回血倍率",
	"PlayerAutoHPRegeneRate":                          "玩家自然回血倍率",
	"PlayerAutoHpRegeneRateInSleep":                   "玩家睡眠回血倍率",
	"SupplyDropSpan":                                  "补给投放间隔",
	"BlockRespawnTime":                                "复活冷却时间",
	"RespawnPenaltyDurationThreshold":                 "复活惩罚重置阈值",
	"RespawnPenaltyTimeScale":                         "复活冷却倍率",
	"bEnableFastTravel":                               "启用快速传送",
	"bEnableFastTravelOnlyBaseCamp":                   "仅允许据点间快速传送",
	"bExistPlayerAfterLogout":                         "下线后保留角色",
	"bIsStartLocationSelectByMap":                     "允许选择出生位置",
	"bHardcore":                                       "启用硬核模式",
	"bCharacterRecreateInHardcore":                    "硬核死亡后重建角色",
	"bPalLost":                                        "死亡永久失去帕鲁",
	"bEnableInvaderEnemy":                             "启用入侵事件",
	"bShowPlayerList":                                 "显示玩家列表",
	"bIsUseBackupSaveData":                            "启用存档备份",
	"bAllowGlobalPalboxExport":                        "允许导出到全局帕鲁终端",
	"bAllowGlobalPalboxImport":                        "允许从全局帕鲁终端导入",
	"bAutoResetGuildNoOnlinePlayers":                  "自动清理离线公会",
	"AutoResetGuildTimeNoOnlinePlayers":               "离线公会清理时长",
	"RandomizerType":                                  "帕鲁随机化模式",
	"RandomizerSeed":                                  "帕鲁随机化种子",
	"bIsRandomizerPalLevelRandom":                     "完全随机帕鲁等级",
	"bEnableVoiceChat":                                "启用语音聊天",
	"VoiceChatMaxVolumeDistance":                      "语音最大音量距离",
	"VoiceChatZeroVolumeDistance":                     "语音静音距离",
	"bInvisibleOtherGuildBaseCampAreaFX":              "隐藏其他公会据点边界",
	"bIsPvP":                                          "启用 PvP",
	"bEnablePlayerToPlayerDamage":                     "允许玩家互相伤害",
	"bEnableDefenseOtherGuildPlayer":                  "据点帕鲁攻击敌对玩家",
	"bEnableAimAssistPad":                             "启用手柄瞄准辅助",
	"bAllowEnhanceStat_Health":                        "允许加点生命值",
	"bAllowEnhanceStat_Attack":                        "允许加点攻击力",
	"bAllowEnhanceStat_Stamina":                       "允许加点耐力",
	"bAllowEnhanceStat_Weight":                        "允许加点负重",
	"bAllowEnhanceStat_WorkSpeed":                     "允许加点工作速度",
	"bBuildAreaLimit":                                 "限制关键区域建造",
	"bCanPickupOtherGuildDeathPenaltyDrop":            "允许拾取其他公会死亡掉落",
	"bAdditionalDropItemWhenPlayerKillingInPvPMode":   "PvP 击杀额外掉落",
	"AdditionalDropItemWhenPlayerKillingInPvPMode":    "PvP 击杀掉落物品 ID",
	"AdditionalDropItemNumWhenPlayerKillingInPvPMode": "PvP 击杀掉落数量",
	"bDisplayPvPItemNumOnWorldMap_BaseCamp":           "地图显示据点 PvP 物品数",
	"bDisplayPvPItemNumOnWorldMap_Player":             "地图显示玩家 PvP 物品数",
	"DenyTechnologyList":                              "禁用科技列表",
}

var enumLabels = map[string]map[string]string{
	"DeathPenalty": {
		"None":             "不掉落",
		"Item":             "掉落物品（不含装备）",
		"ItemAndEquipment": "掉落物品和装备",
		"All":              "全部掉落（物品、装备和队伍帕鲁）",
	},
	"LogFormatType": {
		"Text": "文本",
		"Json": "JSON",
	},
	"RandomizerType": {
		"None":   "不随机",
		"Region": "按区域随机",
		"All":    "完全随机",
	},
}
