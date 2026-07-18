//go:build windows

package steamcmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestHardenCredentialTreeTargetsOnlySteamCMDConfig(t *testing.T) {
	root := t.TempDir()
	var captured []string
	err := hardenCredentialTreeWithRunner(t.Context(), root, func(_ context.Context, args ...string) ([]byte, error) {
		captured = append([]string(nil), args...)
		return []byte("success"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(captured) == 0 || captured[0] != filepath.Join(root, "config") {
		t.Fatalf("icacls target = %#v", captured)
	}
	for _, forbidden := range []string{root, filepath.Join(root, "steamapps"), filepath.Join(root, "steamcmd.exe")} {
		if captured[0] == forbidden {
			t.Fatalf("credential ACL unexpectedly targets %q", forbidden)
		}
	}
}

func TestInteractiveSteamCMDCommandLineQuotesBinaryAndUsesOnlyAccountName(t *testing.T) {
	commandLine := interactiveSteamCMDCommandLine(`C:\Program Files\SteamCMD\steamcmd.exe`, "fixture_user")
	if !strings.Contains(commandLine, `"C:\Program Files\SteamCMD\steamcmd.exe"`) || !strings.HasSuffix(commandLine, "+login fixture_user") {
		t.Fatalf("command line = %q", commandLine)
	}
}
