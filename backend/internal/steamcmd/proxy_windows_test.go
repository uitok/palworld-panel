//go:build windows

package steamcmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteProxyRestoreMarkerDoesNotOverwritePendingRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "steamcmd-proxy-restore.json")
	first := proxyRegistrySnapshot{Version: 1, ProxyServer: optionalString{Exists: true, Value: "original:7890"}}
	if err := writeProxyRestoreMarker(path, first); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second := proxyRegistrySnapshot{Version: 1, ProxyServer: optionalString{Exists: true, Value: "must-not-replace:9999"}}
	if err := writeProxyRestoreMarker(path, second); err == nil {
		t.Fatal("expected an existing recovery marker to be preserved")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(original) {
		t.Fatalf("pending recovery marker changed: %v", err)
	}
}
