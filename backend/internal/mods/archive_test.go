package mods

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractArchiveRejectsUnsafePathsAndSpecialFiles(t *testing.T) {
	tests := []struct {
		name     string
		entry    string
		mode     os.FileMode
		contents string
	}{
		{name: "parent traversal", entry: "../outside.txt", contents: "bad"},
		{name: "absolute", entry: "/absolute.txt", contents: "bad"},
		{name: "windows absolute", entry: `C:\absolute.txt`, contents: "bad"},
		{name: "backslash traversal", entry: `..\outside.txt`, contents: "bad"},
		{name: "windows alternate data stream", entry: "nested/file:stream", contents: "bad"},
		{name: "windows device", entry: "nested/NUL.txt", contents: "bad"},
		{name: "windows trailing dot", entry: "nested/file.", contents: "bad"},
		{name: "symlink", entry: "link", mode: os.ModeSymlink | 0o777, contents: "target"},
		{name: "named pipe", entry: "pipe", mode: os.ModeNamedPipe | 0o600},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive := createZip(t, []zipTestEntry{{name: test.entry, mode: test.mode, body: test.contents}})
			path := filepath.Join(t.TempDir(), "mod.zip")
			if err := os.WriteFile(path, archive, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := extractArchive(path, filepath.Join(t.TempDir(), "out")); err == nil {
				t.Fatal("expected unsafe archive to fail")
			}
		})
	}
}

func TestArchiveRequiresOneValidInfoJSON(t *testing.T) {
	tests := []struct {
		name    string
		entries []zipTestEntry
		want    string
	}{
		{name: "missing", entries: []zipTestEntry{{name: "README.txt", body: "none"}}, want: "not found"},
		{name: "duplicate", entries: []zipTestEntry{
			{name: "A/Info.json", body: `{"PackageName":"A"}`},
			{name: "B/info.JSON", body: `{"PackageName":"B"}`},
		}, want: "exactly one"},
		{name: "missing package", entries: []zipTestEntry{{name: "Mod/Info.json", body: `{"Name":"No package"}`}}, want: "PackageName"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archivePath := filepath.Join(t.TempDir(), "mod.zip")
			if err := os.WriteFile(archivePath, createZip(t, test.entries), 0o600); err != nil {
				t.Fatal(err)
			}
			extracted := filepath.Join(t.TempDir(), "out")
			if err := extractArchive(archivePath, extracted); err != nil {
				t.Fatal(err)
			}
			_, _, err := inspectModDirectory(extracted)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("inspection error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestExtractArchiveAcceptsRegularMod(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "mod.zip")
	archive := createZip(t, []zipTestEntry{
		{name: "Example/Info.json", body: `{"Name":"Example","PackageName":"ExamplePackage","Version":"2.0"}`},
		{name: "Example/Binaries/mod.dll", body: "binary"},
	})
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "out")
	if err := extractArchive(archivePath, destination); err != nil {
		t.Fatal(err)
	}
	root, info, err := inspectModDirectory(destination)
	if err != nil || info.PackageName != "ExamplePackage" || filepath.Base(root) != "Example" {
		t.Fatalf("inspection = %s, %#v, %v", root, info, err)
	}
}

type zipTestEntry struct {
	name string
	mode os.FileMode
	body string
}

func createZip(t *testing.T, entries []zipTestEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
