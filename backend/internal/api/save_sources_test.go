package api

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
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

func writeTestTAR(t *testing.T, entries []zipEntry, compressed bool) string {
	t.Helper()
	extension := ".tar"
	if compressed {
		extension = ".tar.gz"
	}
	archivePath := filepath.Join(t.TempDir(), "save"+extension)
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	var writer *tar.Writer
	var compressedWriter *gzip.Writer
	if compressed {
		compressedWriter = gzip.NewWriter(file)
		writer = tar.NewWriter(compressedWriter)
	} else {
		writer = tar.NewWriter(file)
	}
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o600, Size: int64(len(entry.body)), Typeflag: tar.TypeReg}
		if entry.mode&os.ModeSymlink != 0 {
			header.Typeflag = tar.TypeSymlink
			header.Linkname = "Level.sav"
			header.Size = 0
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Size > 0 {
			if _, err := writer.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if compressedWriter != nil {
		if err := compressedWriter.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return archivePath
}

func TestExtractSaveArchiveAcceptsStandardWorld(t *testing.T) {
	archive := writeTestZIP(t, []zipEntry{
		{name: "Saved/SaveGames/world/Level.sav", body: "level"},
		{name: "Saved/SaveGames/world/Players/0001.sav", body: "player"},
	})
	destination := filepath.Join(t.TempDir(), "files")
	if err := extractSaveArchive(archive, destination, 1024, saveArchiveZIP); err != nil {
		t.Fatalf("extractSaveArchive returned error: %v", err)
	}
	world, err := findImportedWorld(destination)
	if err != nil || filepath.Base(world) != "world" {
		t.Fatalf("findImportedWorld = %q, %v", world, err)
	}
}

func TestExtractSaveArchiveAcceptsTARVariantsAndWindowsSeparators(t *testing.T) {
	for _, test := range []struct {
		name       string
		compressed bool
		format     saveArchiveFormat
	}{
		{name: "tar", format: saveArchiveTAR},
		{name: "tar gzip", compressed: true, format: saveArchiveTARGzip},
	} {
		t.Run(test.name, func(t *testing.T) {
			archive := writeTestTAR(t, []zipEntry{
				{name: `Saved\SaveGames\world\Level.sav`, body: "level"},
				{name: `Saved\SaveGames\world\Players\0001.sav`, body: "player"},
			}, test.compressed)
			destination := filepath.Join(t.TempDir(), "files")
			if err := extractSaveArchive(archive, destination, 1024, test.format); err != nil {
				t.Fatalf("extractSaveArchive returned error: %v", err)
			}
			world, err := findImportedWorld(destination)
			if err != nil || filepath.Base(world) != "world" {
				t.Fatalf("findImportedWorld = %q, %v", world, err)
			}
		})
	}
}

func TestSaveArchiveFormatAndDefaultName(t *testing.T) {
	for name, want := range map[string]saveArchiveFormat{
		"world.zip": saveArchiveZIP, "world.tar": saveArchiveTAR,
		"world.tar.gz": saveArchiveTARGzip, "world.TGZ": saveArchiveTARGzip,
	} {
		got, err := saveArchiveFormatForName(name)
		if err != nil || got != want {
			t.Errorf("saveArchiveFormatForName(%q) = %q, %v", name, got, err)
		}
	}
	if _, err := saveArchiveFormatForName("world.7z"); err == nil {
		t.Fatal("unsupported archive extension was accepted")
	}
	if got := defaultSaveSourceName("world.tar.gz"); got != "world" {
		t.Fatalf("defaultSaveSourceName = %q", got)
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
			err := extractSaveArchive(archive, filepath.Join(t.TempDir(), "files"), test.limit, saveArchiveZIP)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestExtractSaveTARRejectsUnsafeEntries(t *testing.T) {
	archive := writeTestTAR(t, []zipEntry{{name: "link", body: "target", mode: os.ModeSymlink | 0o777}}, false)
	err := extractSaveArchive(archive, filepath.Join(t.TempDir(), "files"), 1024, saveArchiveTAR)
	if err == nil || !strings.Contains(err.Error(), "links are not allowed") {
		t.Fatalf("error = %v", err)
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
