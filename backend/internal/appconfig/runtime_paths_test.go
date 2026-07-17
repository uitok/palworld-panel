package appconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelativeRuntimeRootUsesRepositoryInsteadOfWorkingDirectory(t *testing.T) {
	repository, ok := discoverRepositoryRoot()
	if !ok {
		t.Fatal("test repository root was not discovered")
	}
	workingDirectory := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	layout, err := ResolveRuntimeLayout(filepath.Join("dev-runtime", "windows path", "中文"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(repository, "dev-runtime", "windows path", "中文")
	if !samePath(layout.RuntimeRoot, want) {
		t.Fatalf("RuntimeRoot = %q, want %q", layout.RuntimeRoot, want)
	}
}

func TestRuntimeRootRejectsRepositoryAndSourceOverlap(t *testing.T) {
	repository, ok := discoverRepositoryRoot()
	if !ok {
		t.Fatal("test repository root was not discovered")
	}
	for _, root := range []string{repository, filepath.Join(repository, "backend"), filepath.Join(repository, "frontend", "generated-runtime")} {
		if _, err := ResolveRuntimeLayout(root); err == nil {
			t.Fatalf("expected protected runtime root %q to be rejected", root)
		}
	}
}

func TestEnsureDirsRejectsRuntimeEscape(t *testing.T) {
	runtimeRoot := filepath.Join(t.TempDir(), "runtime")
	cfg := Config{
		RuntimeRoot: runtimeRoot,
		DataDir:     filepath.Join(runtimeRoot, "data"),
		ServerDir:   filepath.Join(runtimeRoot, "..", "outside"),
		DBPath:      filepath.Join(runtimeRoot, "data", "database", "panel.db"),
	}
	if err := cfg.EnsureDirs(); err == nil || !strings.Contains(err.Error(), "escapes runtime root") {
		t.Fatalf("EnsureDirs error = %v", err)
	}
}

func TestValidateManagedPathRejectsRuntimeRootAsDeleteTarget(t *testing.T) {
	runtimeRoot := filepath.Join(t.TempDir(), "runtime")
	cfg := Config{RuntimeRoot: runtimeRoot, DataDir: filepath.Join(runtimeRoot, "data")}
	if err := cfg.ValidateManagedPath(runtimeRoot, false); err == nil {
		t.Fatal("runtime root must not be accepted as a destructive-operation target")
	}
	if err := cfg.ValidateManagedPath(filepath.Join(runtimeRoot, "mods", "one"), false); err != nil {
		t.Fatalf("managed child path was rejected: %v", err)
	}
}

func TestValidateManagedPathAllowsOnlyBoundExternalServerTree(t *testing.T) {
	base := t.TempDir()
	runtimeRoot := filepath.Join(base, "runtime")
	externalServer := filepath.Join(base, "PalServer")
	if err := os.MkdirAll(filepath.Join(externalServer, "Pal", "Saved"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := (Config{
		RuntimeRoot: runtimeRoot,
		DataDir:     filepath.Join(runtimeRoot, "data"),
		ServerDir:   filepath.Join(runtimeRoot, "palworld"),
	}).WithServerDirectoryState()
	if err := cfg.BindImportedServerDirectory(externalServer); err != nil {
		t.Fatalf("BindImportedServerDirectory returned error: %v", err)
	}
	if err := cfg.ValidateManagedPath(filepath.Join(externalServer, "Pal", "Saved"), false); err != nil {
		t.Fatalf("bound server path was rejected: %v", err)
	}
	if err := cfg.ValidateManagedPath(filepath.Join(base, "unrelated"), false); err == nil {
		t.Fatal("unrelated external path was accepted")
	}
}
