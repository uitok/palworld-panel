//go:build windows

package appconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidateManagedPathRejectsJunctionEscape(t *testing.T) {
	root := t.TempDir()
	runtimeRoot := filepath.Join(root, "runtime")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(runtimeRoot, "escaped")
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, outside).CombinedOutput(); err != nil {
		t.Skipf("cannot create test junction: %v: %s", err, output)
	}
	t.Cleanup(func() { _ = os.Remove(junction) })

	cfg := Config{RuntimeRoot: runtimeRoot, DataDir: filepath.Join(runtimeRoot, "data")}
	if err := cfg.ValidateManagedPath(filepath.Join(junction, "payload"), false); err == nil {
		t.Fatal("junction escape was accepted")
	}
}
