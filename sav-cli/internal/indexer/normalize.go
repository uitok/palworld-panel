package indexer

import (
	"fmt"
	"strconv"
	"strings"

	"palpanel/sav-cli/internal/gvas"
)

func Normalize(file *gvas.File, sourcePath string) Index {
	index := EmptyIndex(sourcePath, ParserName)

	world, ok := asMap(getField(file.Properties, "worldSaveData"))
	if !ok {
		index.Warnings = append(index.Warnings, "worldSaveData is missing or has an unexpected shape")
		index.Finalize()
		return index
	}

	memberGuild := normalizeGuilds(&index, world)
	normalizeCharacters(&index, world, memberGuild)
	normalizeBases(&index, world)
	applyGuildNames(&index)
	rebuildMapEntities(&index)
	index.Finalize()
	return index
}

func normalizeGuilds(index *Index, world map[string]any) map[string]Guild {
	memberGuild := map[string]Guild{}
	entries := asList(getField(world, "GroupSaveDataMap"))
	rawDecoded := 0
	for _, entryAny := range entries {
		entry, ok := rawMap(entryAny)
		if !ok {
			continue
		}
		value, _ := asMap(entry["value"])
		groupType := asString(getField(value, "GroupType"))
		if groupType != "EPalGroupType::Guild" {
			continue
		}
		raw, err := rawDataMap(getField(value, "RawData"), func(data []byte) (map[string]any, error) {
			return gvas.DecodeGroupRaw(data, groupType)
		})
		if err != nil {
			index.Warnings = append(index.Warnings, "group RawData decode failed: "+err.Error())
			continue
		}
		if msg := asString(raw["decode_error"]); msg != "" {
			index.Warnings = append(index.Warnings, "group RawData decode failed: "+msg)
			continue
		}
		if len(raw) > 0 {
			rawDecoded++
		}
		gid := firstNonEmpty(asString(raw["group_id"]), asString(entry["key"]))
		if gid == "" {
			continue
		}
		members := []GuildMember{}
		for _, memberAny := range asList(raw["players"]) {
			member, ok := asMap(memberAny)
			if !ok {
				continue
			}
			info, _ := asMap(member["player_info"])
			uid := asString(member["player_uid"])
			members = append(members, GuildMember{
				PlayerUID:      uid,
				Nickname:       asString(info["player_name"]),
				LastOnlineTime: formatUnixLike(asInt64(info["last_online_real_time"])),
			})
		}
		baseIDs := []string{}
		for _, id := range asList(raw["base_ids"]) {
			baseIDs = append(baseIDs, asString(id))
		}
		guild := Guild{
			ID:             gid,
			Name:           firstNonEmpty(asString(raw["guild_name"]), asString(raw["group_name"]), gid),
			OwnerPlayerUID: asString(raw["admin_player_uid"]),
			Members:        members,
			BaseIDs:        baseIDs,
			Raw:            raw,
		}
		index.Guilds = append(index.Guilds, guild)
		for _, member := range members {
			memberGuild[member.PlayerUID] = guild
		}
	}
	return memberGuild
}

func normalizeCharacters(index *Index, world map[string]any, memberGuild map[string]Guild) {
	entries := asList(getField(world, "CharacterSaveParameterMap"))
	rawDecoded := 0
	saveParams := 0
	for _, entryAny := range entries {
		entry, ok := rawMap(entryAny)
		if !ok {
			continue
		}
		key, _ := asMap(entry["key"])
		value, _ := asMap(entry["value"])
		raw, err := rawDataMap(getField(value, "RawData"), gvas.DecodeCharacterRaw)
		if err != nil {
			index.Warnings = append(index.Warnings, "character RawData decode failed: "+err.Error())
			continue
		}
		if msg := asString(raw["decode_error"]); msg != "" {
			index.Warnings = append(index.Warnings, "character RawData decode failed: "+msg)
			continue
		}
		if len(raw) > 0 {
			rawDecoded++
		}
		object, _ := asMap(raw["object"])
		saveParam, _ := asMap(getField(object, "SaveParameter"))
		if len(saveParam) == 0 {
			continue
		}
		saveParams++
		playerUID := firstNonEmpty(asString(getField(key, "PlayerUId")), asString(getField(key, "PlayerUID")))
		instanceID := firstNonEmpty(asString(getField(key, "InstanceId")), asString(getField(key, "InstanceID")))
		if instanceID == "" {
			instanceID = firstNonEmpty(playerUID, fmt.Sprintf("character-%d", len(index.Pals)+len(index.Players)+1))
		}
		location := locationFromAny(firstNonEmptyAny(getField(saveParam, "Transform"), getField(saveParam, "Location"), getField(saveParam, "LastJumpedLocation")))
		if asBool(getField(saveParam, "IsPlayer")) {
			guild := memberGuild[playerUID]
			player := Player{
				PlayerUID:        playerUID,
				SteamID:          asString(firstNonEmptyAny(getField(saveParam, "PlatformUserId"), getField(saveParam, "SteamId"))),
				Nickname:         firstNonEmpty(asString(getField(saveParam, "NickName")), playerUID),
				Level:            asInt(getField(saveParam, "Level")),
				GuildID:          guild.ID,
				GuildName:        guild.Name,
				IsOnline:         false,
				LastOnlineTime:   "",
				Location:         location,
				InventorySummary: map[string]any{},
				Raw:              map[string]any{"key": key, "save_parameter": saveParam},
			}
			index.Players = append(index.Players, player)
			continue
		}
		ownerUID := asString(getField(saveParam, "OwnerPlayerUId"))
		if ownerUID == "" {
			continue
		}
		guild := memberGuild[ownerUID]
		pal := Pal{
			InstanceID:     instanceID,
			CharacterID:    asString(firstNonEmptyAny(getField(saveParam, "CharacterID"), getField(saveParam, "CharacterId"))),
			Nickname:       asString(getField(saveParam, "NickName")),
			Level:          asIntDefault(getField(saveParam, "Level"), 1),
			OwnerPlayerUID: ownerUID,
			GuildID:        guild.ID,
			ContainerID:    containerIDFromAny(firstNonEmptyAny(getField(saveParam, "SlotID"), getField(saveParam, "EquipItemContainerId"))),
			Location:       location,
			Skills:         stringSlice(firstNonEmptyAny(getField(saveParam, "SkillList"), getField(saveParam, "EquipWaza"), getField(saveParam, "MasteredWaza"))),
			Passives:       stringSlice(getField(saveParam, "PassiveSkillList")),
			Status:         "Healthy",
			Raw:            map[string]any{"key": key, "save_parameter": saveParam},
		}
		index.Pals = append(index.Pals, pal)
	}
	_, _ = rawDecoded, saveParams
}

func normalizeBases(index *Index, world map[string]any) {
	entries := asList(getField(world, "BaseCampSaveData"))
	rawDecoded := 0
	for i, entryAny := range entries {
		entry, ok := rawMap(entryAny)
		if !ok {
			continue
		}
		value, _ := asMap(entry["value"])
		raw, err := rawDataMap(getField(value, "RawData"), gvas.DecodeBaseCampRaw)
		if err != nil {
			index.Warnings = append(index.Warnings, "base RawData decode failed: "+err.Error())
			continue
		}
		if msg := asString(raw["decode_error"]); msg != "" {
			index.Warnings = append(index.Warnings, "base RawData decode failed: "+msg)
			continue
		}
		if len(raw) > 0 {
			rawDecoded++
		}
		if len(raw) == 0 {
			continue
		}
		base := Base{
			ID:              firstNonEmpty(asString(raw["id"]), asString(entry["key"]), fmt.Sprintf("base-%d", i+1)),
			Name:            firstNonEmpty(asString(raw["name"]), fmt.Sprintf("Base %d", i+1)),
			GuildID:         asString(raw["group_id_belong_to"]),
			Location:        locationFromAny(raw["transform"]),
			StructuresCount: 0,
			Workers:         []Worker{},
			Containers:      []string{},
			Status:          "Safe",
			Raw:             raw,
		}
		index.Bases = append(index.Bases, base)
	}
	_ = rawDecoded
}

func applyGuildNames(index *Index) {
	guildNames := map[string]string{}
	for _, guild := range index.Guilds {
		guildNames[guild.ID] = guild.Name
	}
	for i := range index.Bases {
		index.Bases[i].GuildName = guildNames[index.Bases[i].GuildID]
	}
	for i := range index.Players {
		if index.Players[i].GuildName == "" {
			index.Players[i].GuildName = guildNames[index.Players[i].GuildID]
		}
	}
}

func rebuildMapEntities(index *Index) {
	index.MapEntities = []MapEntity{}
	for _, player := range index.Players {
		index.MapEntities = append(index.MapEntities, MapEntity{Type: "player", ID: player.PlayerUID, Label: player.Nickname, Location: player.Location})
	}
	for _, base := range index.Bases {
		index.MapEntities = append(index.MapEntities, MapEntity{Type: "base", ID: base.ID, Label: base.Name, Location: base.Location})
	}
	for _, pal := range index.Pals {
		if pal.Location.X != 0 || pal.Location.Y != 0 || pal.Location.Z != 0 {
			label := firstNonEmpty(pal.Nickname, pal.CharacterID)
			index.MapEntities = append(index.MapEntities, MapEntity{Type: "pal", ID: pal.InstanceID, Label: label, Location: pal.Location})
		}
	}
}

func getField(data any, key string) any {
	m, ok := asMap(data)
	if !ok {
		return nil
	}
	return unwrap(m[key])
}

func rawDataMap(value any, decode func([]byte) (map[string]any, error)) (map[string]any, error) {
	if raw, ok := asMap(value); ok {
		return raw, nil
	}
	rawBytes := bytesFromAny(value)
	if len(rawBytes) == 0 {
		return nil, nil
	}
	return decode(rawBytes)
}

func unwrap(value any) any {
	for i := 0; i < 12; i++ {
		m, ok := value.(map[string]any)
		if !ok {
			return value
		}
		next, ok := m["value"]
		if !ok {
			return value
		}
		value = next
	}
	return value
}

func asMap(value any) (map[string]any, bool) {
	value = unwrap(value)
	m, ok := value.(map[string]any)
	return m, ok
}

func rawMap(value any) (map[string]any, bool) {
	m, ok := value.(map[string]any)
	return m, ok
}

func asList(value any) []any {
	value = unwrap(value)
	switch v := value.(type) {
	case []any:
		return v
	case map[string]any:
		if values, ok := v["values"].([]any); ok {
			return values
		}
		return nil
	default:
		return nil
	}
}

func asString(value any) string {
	value = unwrap(value)
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func asInt(value any) int {
	return asIntDefault(value, 0)
}

func asIntDefault(value any, fallback int) int {
	value = unwrap(value)
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}

func asInt64(value any) int64 {
	value = unwrap(value)
	switch v := value.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return n
		}
	}
	return 0
}

func asFloat(value any) float64 {
	value = unwrap(value)
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		n, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return n
		}
	}
	return 0
}

func asBool(value any) bool {
	value = unwrap(value)
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true") || v == "1"
	case int:
		return v != 0
	default:
		return false
	}
}

func locationFromAny(value any) Coordinates {
	value = unwrap(value)
	m, ok := value.(map[string]any)
	if !ok {
		return Coordinates{}
	}
	if t, ok := m["translation"]; ok {
		m, _ = asMap(t)
	} else if t, ok := m["Translation"]; ok {
		m, _ = asMap(t)
	}
	return Coordinates{X: asFloat(m["x"]), Y: asFloat(m["y"]), Z: asFloat(m["z"])}
}

func stringSlice(value any) []string {
	out := []string{}
	for _, item := range asList(value) {
		s := asString(item)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func bytesFromAny(value any) []byte {
	value = unwrap(value)
	switch v := value.(type) {
	case []byte:
		return v
	case map[string]any:
		return bytesFromAny(v["values"])
	case []int:
		out := make([]byte, 0, len(v))
		for _, item := range v {
			out = append(out, byte(item))
		}
		return out
	case []any:
		out := make([]byte, 0, len(v))
		for _, item := range v {
			out = append(out, byte(asInt(item)))
		}
		return out
	default:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyAny(values ...any) any {
	for _, value := range values {
		if asString(value) != "" {
			return value
		}
	}
	return nil
}

func containerIDFromAny(value any) string {
	m, ok := asMap(value)
	if !ok {
		return asString(value)
	}
	if id := asString(getField(m, "ID")); id != "" {
		return id
	}
	if id := containerIDFromAny(getField(m, "ContainerId")); id != "" {
		return id
	}
	return asString(value)
}

func formatUnixLike(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}
