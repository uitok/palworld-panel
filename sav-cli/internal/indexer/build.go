package indexer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"palpanel/sav-cli/internal/gvas"
	"palpanel/sav-cli/internal/sav"
)

const ParserName = "palpanel-sav-cli-go"

func Build(savePath string) (Index, error) {
	started := time.Now()
	worldDir, err := FindWorldDir(savePath)
	if err != nil {
		return EmptyIndex(savePath, ParserName), err
	}
	snapshot, err := CopySnapshot(worldDir)
	if err != nil {
		return EmptyIndex(worldDir, ParserName), err
	}
	defer os.RemoveAll(snapshot)

	manifest, err := SnapshotManifest(snapshot)
	if err != nil {
		return EmptyIndex(worldDir, ParserName), err
	}
	levelData, err := os.ReadFile(filepath.Join(snapshot, "Level.sav"))
	if err != nil {
		return EmptyIndex(worldDir, ParserName), NewError(CodeLevelSavNotFound, "read Level.sav: %v", err)
	}
	gvasData, info, err := sav.DecodeToGVAS(levelData)
	if err != nil {
		var incompatible *sav.IncompatibleError
		if errors.As(err, &incompatible) {
			return EmptyIndex(worldDir, ParserName), ParserIncompatible("%s; magic=%s save_type=%q data_offset=%d gvas_offset=%d", incompatible.Message, info.Magic, info.SaveType, info.DataOffset, info.GVASOffset)
		}
		return EmptyIndex(worldDir, ParserName), ParserIncompatible("decode Level.sav: %v", err)
	}
	parsed, err := gvas.Read(gvasData)
	if err != nil {
		return EmptyIndex(worldDir, ParserName), ParserIncompatible("parse GVAS: %v", err)
	}

	index := Normalize(parsed, worldDir)
	index.Snapshot = manifest
	index.DurationMS = int(time.Since(started).Milliseconds())
	index.Finalize()
	return index, nil
}

func FindWorldDir(path string) (string, error) {
	src, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	st, err := os.Stat(src)
	if err == nil && !st.IsDir() && filepath.Base(src) == "Level.sav" {
		return filepath.Dir(src), nil
	}
	if err != nil {
		return "", NewError(CodeSavePathNotFound, "save path does not exist: %s", src)
	}
	if _, err := os.Stat(filepath.Join(src, "Level.sav")); err == nil {
		return src, nil
	}

	var candidates []string
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := strings.ToLower(d.Name())
			if name == "backup" || name == "backups" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "Level.sav" {
			candidates = append(candidates, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", NewError(CodeLevelSavNotFound, "Level.sav was not found under: %s", src)
	}
	sort.Slice(candidates, func(i, j int) bool {
		ii, _ := os.Stat(filepath.Join(candidates[i], "Level.sav"))
		jj, _ := os.Stat(filepath.Join(candidates[j], "Level.sav"))
		if ii == nil || jj == nil {
			return candidates[i] < candidates[j]
		}
		return ii.ModTime().After(jj.ModTime())
	})
	return filepath.Abs(candidates[0])
}

func CopySnapshot(worldDir string) (string, error) {
	snapshot, err := os.MkdirTemp("", "palpanel-sav-cli-")
	if err != nil {
		return "", err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(snapshot)
		}
	}()

	savs, err := filepath.Glob(filepath.Join(worldDir, "*.sav"))
	if err != nil {
		return "", err
	}
	for _, src := range savs {
		if err := copyFile(src, filepath.Join(snapshot, filepath.Base(src))); err != nil {
			return "", err
		}
	}
	playerDir := filepath.Join(worldDir, "Players")
	if st, err := os.Stat(playerDir); err == nil && st.IsDir() {
		dest := filepath.Join(snapshot, "Players")
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return "", err
		}
		players, _ := filepath.Glob(filepath.Join(playerDir, "*.sav"))
		for _, src := range players {
			if err := copyFile(src, filepath.Join(dest, filepath.Base(src))); err != nil {
				return "", err
			}
		}
	}
	if _, err := os.Stat(filepath.Join(snapshot, "Level.sav")); err != nil {
		return "", NewError(CodeLevelSavNotFound, "Level.sav was not copied from: %s", worldDir)
	}
	cleanup = false
	return snapshot, nil
}

func SnapshotManifest(snapshot string) (Snapshot, error) {
	var files []string
	top, _ := filepath.Glob(filepath.Join(snapshot, "*.sav"))
	files = append(files, top...)
	players, _ := filepath.Glob(filepath.Join(snapshot, "Players", "*.sav"))
	files = append(files, players...)
	sort.Strings(files)
	h := sha256.New()
	out := Snapshot{Files: []SnapshotFile{}}
	for _, path := range files {
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(snapshot, path)
		_, _ = h.Write([]byte(rel))
		_, _ = h.Write([]byte(fmt.Sprintf("%d:%d:", st.Size(), st.ModTime().UnixNano())))
		out.Files = append(out.Files, SnapshotFile{
			Path:  filepath.ToSlash(rel),
			Size:  st.Size(),
			MTime: st.ModTime().Unix(),
		})
	}
	out.Fingerprint = hex.EncodeToString(h.Sum(nil))
	return out, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
