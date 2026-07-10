package gvas

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestDecodeCurrentGuildRawWithRoles(t *testing.T) {
	var body bytes.Buffer
	groupID := writeFixtureGUID(&body, 1)
	fstring(&body, "fixture-internal")
	u32(&body, 0) // character handles
	body.WriteByte(0)
	u32(&body, 0) // leading bytes
	u32(&body, 1)
	baseID := writeFixtureGUID(&body, 17)
	i32(&body, 0)
	i32(&body, 1)
	u32(&body, 1)
	pointID := writeFixtureGUID(&body, 33)
	fstring(&body, "Fixture Guild")
	writeZeroGUID(&body)
	u32(&body, 0) // guild markers
	u32(&body, 2)
	body.Write([]byte{2, 3})
	i32(&body, 0)
	playerID := writeFixtureGUID(&body, 49)
	u32(&body, 1)
	body.Write(fixtureGUIDBytes(49))
	writeI64(&body, 123456)
	fstring(&body, "Fixture Player")
	body.WriteByte(1)
	u32(&body, 1)
	body.WriteByte(2)
	u32(&body, 2)
	body.Write([]byte{3, 7})
	u32(&body, 0)

	raw, err := DecodeGroupRaw(body.Bytes(), "EPalGroupType::Guild")
	if err != nil {
		t.Fatal(err)
	}
	if raw["group_id"] != groupID || raw["guild_name"] != "Fixture Guild" || raw["admin_player_uid"] != playerID {
		t.Fatalf("unexpected guild identity fields: %#v", raw)
	}
	bases := raw["base_ids"].([]any)
	points := raw["map_object_instance_ids_base_camp_points"].([]any)
	players := raw["players"].([]any)
	if len(bases) != 1 || bases[0] != baseID || len(points) != 1 || points[0] != pointID || len(players) != 1 {
		t.Fatalf("current guild relationships were not decoded: %#v", raw)
	}
	player := players[0].(map[string]any)
	if player["role"] != 1 {
		t.Fatalf("expected guild role 1, got %#v", player)
	}
}

func TestDecodeOneDotZeroContainerSlots(t *testing.T) {
	var character bytes.Buffer
	writeZeroGUID(&character)
	instanceID := writeFixtureGUID(&character, 65)
	character.WriteByte(0)
	character.Write(make([]byte, 5))

	characterRaw, err := DecodeCharacterContainerSlotRaw(character.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if characterRaw["instance_id"] != instanceID {
		t.Fatalf("unexpected character instance: %#v", characterRaw)
	}

	var item bytes.Buffer
	i32(&item, 2)
	i32(&item, 7)
	fstring(&item, "Stone")
	writeFixtureGUID(&item, 81)
	writeFixtureGUID(&item, 97)
	item.Write([]byte{1, 2, 3, 4})

	itemRaw, err := DecodeItemContainerSlotRaw(item.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	itemValue := itemRaw["item"].(map[string]any)
	if itemRaw["slot_index"] != 2 || itemRaw["count"] != 7 || itemValue["static_id"] != "Stone" {
		t.Fatalf("unexpected item slot: %#v", itemRaw)
	}
}

func TestDecodeOneDotZeroMapObjectModelPrefix(t *testing.T) {
	var body bytes.Buffer
	instanceID := writeFixtureGUID(&body, 113)
	writeFixtureGUID(&body, 129)
	baseID := writeFixtureGUID(&body, 145)
	groupID := writeFixtureGUID(&body, 161)
	i32(&body, 20000)
	i32(&body, 20000)
	for _, value := range []float64{0, 0, 0, 1, 100, 200, 300, 1, 1, 1} {
		writeF64(&body, value)
	}
	body.Write(make([]byte, 32))

	raw, err := DecodeMapObjectModelRaw(body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if raw["instance_id"] != instanceID || raw["base_camp_id_belong_to"] != baseID || raw["group_id_belong_to"] != groupID {
		t.Fatalf("unexpected map object IDs: %#v", raw)
	}
	transform := raw["initial_transform_cache"].(map[string]any)
	translation := transform["translation"].(map[string]any)
	if translation["x"] != float64(100) || translation["y"] != float64(200) || translation["z"] != float64(300) {
		t.Fatalf("unexpected map object transform: %#v", transform)
	}
}

func fixtureGUIDBytes(seed byte) []byte {
	out := make([]byte, 16)
	for i := range out {
		out[i] = seed + byte(i)
	}
	return out
}

func writeFixtureGUID(body *bytes.Buffer, seed byte) string {
	value := fixtureGUIDBytes(seed)
	body.Write(value)
	return formatGUID(value)
}

func writeZeroGUID(body *bytes.Buffer) {
	body.Write(make([]byte, 16))
}

func writeI64(body *bytes.Buffer, value int64) {
	_ = binary.Write(body, binary.LittleEndian, value)
}

func writeF64(body *bytes.Buffer, value float64) {
	_ = binary.Write(body, binary.LittleEndian, value)
}
