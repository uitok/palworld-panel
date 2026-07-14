package webui

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestSelectPrefersEmbeddedReadyFilesystem(t *testing.T) {
	embedded := testFilesystem("embedded")
	external := testFilesystem("external")
	selected, ok := Select(embedded, external)
	if !ok {
		t.Fatal("expected a ready filesystem")
	}
	body, err := fs.ReadFile(selected, "index.html")
	if err != nil || string(body) != "embedded" {
		t.Fatalf("selected index = %q, %v", body, err)
	}
}

func TestSelectFallsBackAndRejectsIncompleteFilesystems(t *testing.T) {
	incomplete := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("missing assets")}}
	external := testFilesystem("external")
	selected, ok := Select(incomplete, external)
	if !ok || selected == nil {
		t.Fatal("expected the fallback filesystem")
	}
	if _, ok := Select(incomplete, nil); ok {
		t.Fatal("incomplete filesystem should not be selected")
	}
}

func TestLoadExternalDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	selected, ok := Load(root)
	if !ok || selected == nil {
		t.Fatal("external directory should be ready")
	}
	if selected, ok := Load(filepath.Join(root, "missing")); ok || selected != nil {
		t.Fatal("missing directory should not be selected")
	}
}

func testFilesystem(index string) fstest.MapFS {
	return fstest.MapFS{
		"index.html":      &fstest.MapFile{Data: []byte(index)},
		"assets/index.js": &fstest.MapFile{Data: []byte("asset")},
	}
}
