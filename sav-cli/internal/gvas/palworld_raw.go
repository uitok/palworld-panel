package gvas

import "fmt"

type rawDecoder func([]byte) (any, error)

func (r *Reader) byteArrayRawData(typeName string, size int64, path string, decoder rawDecoder) (map[string]any, error) {
	value, err := r.Property(typeName, size, path, true)
	if err != nil {
		return nil, err
	}
	rawBytes := bytesFromProperty(value)
	decoded, err := decoder(rawBytes)
	if err != nil {
		value["value"] = map[string]any{
			"decode_error": err.Error(),
			"values":       intsFromBytes(rawBytes),
		}
	} else {
		value["value"] = decoded
	}
	value["custom_type"] = path
	return value, nil
}

func (r *Reader) groupSaveDataMap(typeName string, size int64, path string) (map[string]any, error) {
	value, err := r.Property(typeName, size, path, true)
	if err != nil {
		return nil, err
	}
	entries, _ := value["value"].([]any)
	for _, entryAny := range entries {
		entry, ok := entryAny.(map[string]any)
		if !ok {
			continue
		}
		groupValue, ok := entry["value"].(map[string]any)
		if !ok {
			continue
		}
		groupType := asString(getNested(groupValue, "GroupType", "value", "value"))
		rawBytes := bytesFromProperty(getMap(groupValue, "RawData"))
		if len(rawBytes) == 0 {
			continue
		}
		decoded, err := decodeGroupRaw(rawBytes, groupType)
		if err != nil {
			groupValue["RawData"] = map[string]any{
				"type": "ArrayProperty",
				"value": map[string]any{
					"decode_error": err.Error(),
					"values":       intsFromBytes(rawBytes),
				},
			}
			continue
		}
		if raw, ok := groupValue["RawData"].(map[string]any); ok {
			raw["value"] = decoded
			raw["custom_type"] = path
		} else {
			groupValue["RawData"] = map[string]any{"value": decoded, "custom_type": path}
		}
	}
	value["custom_type"] = path
	return value, nil
}

func decodeCharacterRaw(data []byte) (any, error) {
	return DecodeCharacterRaw(data)
}

func DecodeCharacterRaw(data []byte) (map[string]any, error) {
	reader := NewReader(data)
	props, err := reader.PropertiesUntilEnd("")
	if err != nil {
		return nil, err
	}
	unknown := []any{}
	for i := 0; i < 4 && !reader.EOF(); i++ {
		b, err := reader.Byte()
		if err != nil {
			return nil, err
		}
		unknown = append(unknown, int(b))
	}
	groupID := ""
	if reader.Remaining() >= 16 {
		id, err := reader.GUID()
		if err != nil {
			return nil, err
		}
		groupID = id
	}
	out := map[string]any{
		"object":        props,
		"unknown_bytes": unknown,
		"group_id":      groupID,
	}
	if !reader.EOF() {
		out["trailing_unparsed_data"] = intsFromBytes(mustReadRest(reader))
	}
	return out, nil
}

func decodeBaseCampRaw(data []byte) (any, error) {
	return DecodeBaseCampRaw(data)
}

func DecodeBaseCampRaw(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return nil, nil
	}
	reader := NewReader(data)
	id, err := reader.GUID()
	if err != nil {
		return nil, err
	}
	name, err := reader.FString()
	if err != nil {
		return nil, err
	}
	state, err := reader.Byte()
	if err != nil {
		return nil, err
	}
	transform, err := reader.Transform()
	if err != nil {
		return nil, err
	}
	areaRange, err := reader.Float()
	if err != nil {
		return nil, err
	}
	groupID, err := reader.GUID()
	if err != nil {
		return nil, err
	}
	fastTravel, err := reader.Transform()
	if err != nil {
		return nil, err
	}
	ownerMapObjectID, err := reader.GUID()
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"id":                           id,
		"name":                         name,
		"state":                        int(state),
		"transform":                    transform,
		"area_range":                   areaRange,
		"group_id_belong_to":           groupID,
		"fast_travel_local_transform":  fastTravel,
		"owner_map_object_instance_id": ownerMapObjectID,
	}
	if !reader.EOF() {
		out["trailing_unparsed_data"] = intsFromBytes(mustReadRest(reader))
	}
	return out, nil
}

func decodeGroupRaw(data []byte, groupType string) (any, error) {
	return DecodeGroupRaw(data, groupType)
}

func DecodeGroupRaw(data []byte, groupType string) (map[string]any, error) {
	reader := NewReader(data)
	groupID, err := reader.GUID()
	if err != nil {
		return nil, err
	}
	groupName, err := reader.FString()
	if err != nil {
		return nil, err
	}
	handles, err := reader.TArray(readInstanceID)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"group_type":                      groupType,
		"group_id":                        groupID,
		"group_name":                      groupName,
		"individual_character_handle_ids": handles,
	}
	if groupType == "EPalGroupType::Guild" || groupType == "EPalGroupType::IndependentGuild" || groupType == "EPalGroupType::Organization" {
		orgType, err := reader.Byte()
		if err != nil {
			return nil, err
		}
		baseIDs, err := reader.TArray(readGUIDAny)
		if err != nil {
			return nil, err
		}
		out["org_type"] = int(orgType)
		out["base_ids"] = baseIDs
	}
	if groupType == "EPalGroupType::Guild" || groupType == "EPalGroupType::IndependentGuild" {
		baseLevel, err := reader.I32()
		if err != nil {
			return nil, err
		}
		points, err := reader.TArray(readGUIDAny)
		if err != nil {
			return nil, err
		}
		guildName, err := reader.FString()
		if err != nil {
			return nil, err
		}
		out["base_camp_level"] = int(baseLevel)
		out["map_object_instance_ids_base_camp_points"] = points
		out["guild_name"] = guildName
	}
	if groupType == "EPalGroupType::IndependentGuild" {
		playerUID, err := reader.GUID()
		if err != nil {
			return nil, err
		}
		guildName2, err := reader.FString()
		if err != nil {
			return nil, err
		}
		lastOnline, err := reader.I64()
		if err != nil {
			return nil, err
		}
		playerName, err := reader.FString()
		if err != nil {
			return nil, err
		}
		out["player_uid"] = playerUID
		out["guild_name_2"] = guildName2
		out["player_info"] = map[string]any{"last_online_real_time": lastOnline, "player_name": playerName}
	}
	if groupType == "EPalGroupType::Guild" {
		adminUID, err := reader.GUID()
		if err != nil {
			return nil, err
		}
		count, err := reader.I32()
		if err != nil {
			return nil, err
		}
		if count < 0 || count > 1_000_000 {
			return nil, fmt.Errorf("guild player count is unreasonable: %d", count)
		}
		players := make([]any, 0, count)
		for range count {
			playerUID, err := reader.GUID()
			if err != nil {
				return nil, err
			}
			lastOnline, err := reader.I64()
			if err != nil {
				return nil, err
			}
			playerName, err := reader.FString()
			if err != nil {
				return nil, err
			}
			players = append(players, map[string]any{
				"player_uid":  playerUID,
				"player_info": map[string]any{"last_online_real_time": lastOnline, "player_name": playerName},
			})
		}
		out["admin_player_uid"] = adminUID
		out["players"] = players
	}
	if !reader.EOF() {
		out["trailing_unparsed_data"] = intsFromBytes(mustReadRest(reader))
	}
	return out, nil
}

func readInstanceID(r *Reader) (any, error) {
	guid, err := r.GUID()
	if err != nil {
		return nil, err
	}
	instanceID, err := r.GUID()
	if err != nil {
		return nil, err
	}
	return map[string]any{"guid": guid, "instance_id": instanceID}, nil
}

func readGUIDAny(r *Reader) (any, error) {
	return r.GUID()
}

func bytesFromProperty(value any) []byte {
	raw := getNested(value, "value", "values")
	if raw == nil {
		raw = getNested(value, "values")
	}
	switch v := raw.(type) {
	case []byte:
		return v
	case []int:
		out := make([]byte, 0, len(v))
		for _, n := range v {
			out = append(out, byte(n))
		}
		return out
	case []any:
		out := make([]byte, 0, len(v))
		for _, item := range v {
			switch n := item.(type) {
			case int:
				out = append(out, byte(n))
			case uint:
				out = append(out, byte(n))
			case uint8:
				out = append(out, n)
			case int32:
				out = append(out, byte(n))
			case float64:
				out = append(out, byte(n))
			}
		}
		return out
	default:
		return nil
	}
}

func intsFromBytes(data []byte) []any {
	out := make([]any, 0, len(data))
	for _, b := range data {
		out = append(out, int(b))
	}
	return out
}

func mustReadRest(r *Reader) []byte {
	b, err := r.Read(r.Remaining())
	if err != nil {
		return nil
	}
	return b
}

func getMap(data any, key string) map[string]any {
	m, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	v, ok := m[key].(map[string]any)
	if !ok {
		return nil
	}
	return v
}

func getNested(data any, keys ...string) any {
	current := data
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[key]
	}
	return current
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}
