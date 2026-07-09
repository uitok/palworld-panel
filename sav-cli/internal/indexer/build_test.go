package indexer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildReturnsParserIncompatibleForPlM(t *testing.T) {
	dir := t.TempDir()
	level := make([]byte, 24)
	level[0] = 100
	level[4] = 12
	copy(level[8:12], []byte("PlM1"))
	copy(level[20:24], []byte("GVAS"))
	if err := os.WriteFile(filepath.Join(dir, "Level.sav"), level, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Build(dir)
	var indexed *Error
	if !errors.As(err, &indexed) {
		t.Fatalf("expected indexer error, got %T %v", err, err)
	}
	if indexed.Code != CodeParserIncompatible {
		t.Fatalf("expected %s, got %s", CodeParserIncompatible, indexed.Code)
	}
}
