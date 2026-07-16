package api

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zipEntry struct {
	name string
	body string
	mode os.FileMode
}

func writeTestZIP(t *testing.T, entries []zipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "save.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		part, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractSaveArchiveAcceptsStandardWorld(t *testing.T) {
	archive := writeTestZIP(t, []zipEntry{
		{name: "Saved/SaveGames/world/Level.sav", body: "level"},
		{name: "Saved/SaveGames/world/Players/0001.sav", body: "player"},
	})
	destination := filepath.Join(t.TempDir(), "files")
	if err := extractSaveArchive(archive, destination, 1024); err != nil {
		t.Fatalf("extractSaveArchive returned error: %v", err)
	}
	world, err := findImportedWorld(destination)
	if err != nil || filepath.Base(world) != "world" {
		t.Fatalf("findImportedWorld = %q, %v", world, err)
	}
}

func TestExtractSaveArchiveRejectsUnsafeEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []zipEntry
		limit   int64
		want    string
	}{
		{name: "path traversal", entries: []zipEntry{{name: "../Level.sav", body: "x"}}, limit: 1024, want: "unsafe archive path"},
		{name: "symlink", entries: []zipEntry{{name: "Level.sav", body: "target", mode: os.ModeSymlink | 0o777}}, limit: 1024, want: "symbolic links"},
		{name: "expanded size", entries: []zipEntry{{name: "Level.sav", body: strings.Repeat("x", 32)}}, limit: 8, want: "exceeds limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive := writeTestZIP(t, test.entries)
			err := extractSaveArchive(archive, filepath.Join(t.TempDir(), "files"), test.limit)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestFindImportedWorldRejectsArchiveWithoutLevel(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "LocalData.sav"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := findImportedWorld(root); err == nil {
		t.Fatal("expected missing Level.sav error")
	}
}
