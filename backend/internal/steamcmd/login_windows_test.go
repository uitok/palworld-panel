//go:build windows

package steamcmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestHardenPrivatePathRestrictsCredentialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "steam-workshop-credentials.json")
	var captured []string
	err := hardenPrivatePathWithRunner(t.Context(), path, func(_ context.Context, args ...string) ([]byte, error) {
		captured = append([]string(nil), args...)
		return []byte("success"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(captured) < 7 || captured[0] != path || !containsACLGrant(captured, ":F") {
		t.Fatalf("icacls arguments = %#v", captured)
	}
}

func containsACLGrant(args []string, suffix string) bool {
	for _, value := range args {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}
