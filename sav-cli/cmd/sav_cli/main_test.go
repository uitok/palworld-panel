package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectWritesStructuredJSONInsteadOfRawSaveBytes(t *testing.T) {
	root := t.TempDir()
	savePath := filepath.Join(root, "Level.sav")
	outputPath := filepath.Join(root, "inspect.json")
	rawSave := append([]byte("GVAS"), bytes.Repeat([]byte{0x00, 0xff, 0x81, 0x10}, 256)...)
	if err := os.WriteFile(savePath, rawSave, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"inspect", "--file", savePath, "--output", outputPath}); err != nil {
		t.Fatalf("inspect returned an error: %v", err)
	}
	body, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(body) {
		t.Fatalf("inspect output was not JSON: %q", body)
	}
	if bytes.ContainsRune(body, '\x00') || bytes.Contains(body, rawSave) || strings.Contains(string(body), "\\x") || strings.Contains(string(body), "\\u0000") || strings.Contains(string(body), `"raw"`) {
		t.Fatalf("raw binary save bytes leaked into inspect output: %q", body)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["file"] != savePath || int(payload["size"].(float64)) != len(rawSave) {
		t.Fatalf("unexpected inspect metadata: %#v", payload)
	}
}
