//go:build windows

package steamcmd

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/sys/windows"
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

func TestInteractiveSteamCMDCommandUsesDirectNewConsole(t *testing.T) {
	cmd, err := interactiveSteamCMDCommand(`C:\tools\steamcmd.exe`, `C:\tools`, "fixture_user")
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{`C:\tools\steamcmd.exe`, "+login", "fixture_user"}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Fatalf("command args = %#v, want %#v", cmd.Args, wantArgs)
	}
	if cmd.Dir != `C:\tools` {
		t.Fatalf("command directory = %q", cmd.Dir)
	}
	wantFlags := uint32(windows.CREATE_NEW_CONSOLE | windows.CREATE_NEW_PROCESS_GROUP)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&wantFlags != wantFlags {
		t.Fatalf("creation flags = %#v", cmd.SysProcAttr)
	}
}
