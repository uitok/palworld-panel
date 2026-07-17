package indexer

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFinalizeAndJSONNeverExposeRawSavePayloads(t *testing.T) {
	index := EmptyIndex("fixture", "test")
	index.Warnings = []string{"decode failed: \\x00\\xff\x00"}
	index.Players = []Player{{PlayerUID: "uid_1", Raw: map[string]any{"values": []int{0, 255}}}}
	index.Guilds = []Guild{{ID: "guild_1", Raw: []byte{0, 255}}}
	index.Bases = []Base{{ID: "base_1", Raw: []byte{0, 255}}}
	index.Pals = []Pal{{InstanceID: "pal_1", Raw: []byte{0, 255}}}
	index.Finalize()

	body, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	text := string(body)
	if strings.Contains(text, `"raw"`) || strings.Contains(text, `\\x`) || strings.ContainsRune(text, '\x00') {
		t.Fatalf("raw save content leaked into index JSON: %s", text)
	}
	if !strings.Contains(index.Warnings[0], "[byte]") {
		t.Fatalf("warning was not sanitized: %q", index.Warnings[0])
	}
}

func TestMapObjectPalLocationTypes(t *testing.T) {
	tests := map[string]string{
		"DisplayCharacter":    "viewing_cage",
		"DimensionPalStorage": "dimensional_pal_storage",
		"GlobalPalStorage":    "global_pal_storage",
		"Unknown":             "",
	}
	for input, expected := range tests {
		if actual := mapObjectPalLocationType(input); actual != expected {
			t.Fatalf("mapObjectPalLocationType(%q) = %q, want %q", input, actual, expected)
		}
	}
}
