package main

import "testing"

func TestTypeForTypedAdditionalProperties(t *testing.T) {
	value := schema{
		Type: "object",
		AdditionalProperties: map[string]any{
			"$ref": "#/components/schemas/PalDefenderInventorySlot",
		},
	}
	want := `Record<string, components["schemas"]["PalDefenderInventorySlot"]>`
	if got := typeFor(value, 2); got != want {
		t.Fatalf("typeFor() = %q, want %q", got, want)
	}
}
