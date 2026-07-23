package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestTypeForAnyOfIncludingNull(t *testing.T) {
	value := schema{AnyOf: []schema{{Type: "string"}, {Type: "null"}}}
	if got, want := typeFor(value, 2), "string | null"; got != want {
		t.Fatalf("typeFor() = %q, want %q", got, want)
	}
}

func TestOpenAPIGeneratesMonitorDiagnosticContracts(t *testing.T) {
	output := filepath.Join(t.TempDir(), "contracts.ts")
	if err := run(filepath.Join("..", "..", "..", "docs", "openapi.yaml"), output); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	contract := string(body)
	for _, want := range []string{
		`"MonitorRiskReason":`,
		`"MonitorSample":`,
		`"host_memory_total_bytes": number`,
		`"workload_memory_usage_bytes": number`,
		`"oom_killed": boolean`,
		`"lifecycle_available": boolean`,
		`"risk_reasons": Array<components["schemas"]["MonitorRiskReason"]>`,
		`"MonitorSnapshot":`,
	} {
		if !strings.Contains(contract, want) {
			t.Fatalf("generated monitor contract does not contain %q", want)
		}
	}
}
