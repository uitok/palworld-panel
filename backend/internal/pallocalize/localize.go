package pallocalize

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
)

//go:embed catalog.zh-CN.json
var catalogJSON []byte

type catalog struct {
	Pals      map[string]string `json:"pals"`
	Items     map[string]string `json:"items"`
	ItemIcons map[string]string `json:"item_icons"`
	Passives  map[string]string `json:"passives"`
}

type normalizedCatalog struct {
	pals     map[string]string
	items    map[string]string
	passives map[string]string
	itemList []ItemEntry
}

type ItemEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
}

var names = loadCatalog()

func loadCatalog() normalizedCatalog {
	var source catalog
	if err := json.Unmarshal(catalogJSON, &source); err != nil {
		panic("decode embedded Palworld localization catalog: " + err.Error())
	}
	itemList := make([]ItemEntry, 0, len(source.Items))
	for id, name := range source.Items {
		id = strings.TrimSpace(id)
		name = strings.TrimSpace(name)
		if id != "" && name != "" {
			itemList = append(itemList, ItemEntry{ID: id, Name: name, Icon: strings.TrimSpace(source.ItemIcons[id])})
		}
	}
	sort.Slice(itemList, func(i, j int) bool {
		return strings.ToLower(itemList[i].ID) < strings.ToLower(itemList[j].ID)
	})
	return normalizedCatalog{
		pals:     normalizeKeys(source.Pals),
		items:    normalizeKeys(source.Items),
		passives: normalizeKeys(source.Passives),
		itemList: itemList,
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

func SearchItems(query string, limit int) []ItemEntry {
	if limit <= 0 {
		limit = 100
	}
	if limit > 5000 {
		limit = 5000
	}
	query = normalize(query)
	results := make([]ItemEntry, 0, min(limit, len(names.itemList)))
	for _, item := range names.itemList {
		if query != "" && !strings.Contains(normalize(item.ID), query) && !strings.Contains(normalize(item.Name), query) {
			continue
		}
		results = append(results, item)
		if len(results) == limit {
			break
		}
	}
	return results
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
