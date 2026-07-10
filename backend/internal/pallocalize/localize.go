package pallocalize

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed catalog.zh-CN.json
var catalogJSON []byte

type catalog struct {
	Pals     map[string]string `json:"pals"`
	Items    map[string]string `json:"items"`
	Passives map[string]string `json:"passives"`
}

type normalizedCatalog struct {
	pals     map[string]string
	items    map[string]string
	passives map[string]string
}

var names = loadCatalog()

func loadCatalog() normalizedCatalog {
	var source catalog
	if err := json.Unmarshal(catalogJSON, &source); err != nil {
		panic("decode embedded Palworld localization catalog: " + err.Error())
	}
	return normalizedCatalog{
		pals:     normalizeKeys(source.Pals),
		items:    normalizeKeys(source.Items),
		passives: normalizeKeys(source.Passives),
	}
}

func normalizeKeys(source map[string]string) map[string]string {
	out := make(map[string]string, len(source))
	for key, value := range source {
		key = normalize(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	return out
}

func lookup(values map[string]string, id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if translated := values[normalize(id)]; translated != "" {
		return translated
	}
	return id
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func PalName(characterID string) string {
	return lookup(names.pals, characterID)
}

func ItemName(itemID string) string {
	return lookup(names.items, itemID)
}

func PassiveName(passiveID string) string {
	return lookup(names.passives, passiveID)
}

func GuildName(name string) string {
	trimmed := strings.TrimSpace(name)
	switch normalize(trimmed) {
	case "unnamed guild":
		return "未命名公会"
	case "":
		return ""
	default:
		return trimmed
	}
}

func BaseName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "新規生成拠点テンプレート名") {
		return "未命名据点"
	}
	if strings.HasPrefix(trimmed, "Base ") {
		return "据点 " + strings.TrimSpace(strings.TrimPrefix(trimmed, "Base "))
	}
	return trimmed
}

func MapObjectName(objectID string) string {
	trimmed := strings.TrimSpace(objectID)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "damagablerock") {
		return "可采集岩石"
	}
	switch normalize(trimmed) {
	case "palboxv2":
		return "帕鲁终端"
	case "workbench":
		return "原始作业台"
	case "treasurebox":
		return "宝箱"
	case "treasurebox_requiredlonghold":
		return "宝箱（需长按）"
	case "commondropitem3d":
		return "掉落物品"
	default:
		return trimmed
	}
}
