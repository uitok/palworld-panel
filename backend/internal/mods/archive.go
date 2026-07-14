package mods

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxExtractedBytes int64 = 1 << 30
	maxArchiveFiles         = 10_000
)

func extractArchive(zipPath, destination string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > maxArchiveFiles {
		return fmt.Errorf("zip contains more than %d entries", maxArchiveFiles)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(reader.File))
	var declaredBytes int64
	var extractedBytes int64
	for _, entry := range reader.File {
		name, err := safeArchivePath(entry.Name)
		if err != nil {
			return err
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("zip contains duplicate path: %s", entry.Name)
		}
		seen[key] = struct{}{}
		if entry.UncompressedSize64 > uint64(maxExtractedBytes) {
			return fmt.Errorf("zip entry exceeds the extracted size limit: %s", entry.Name)
		}
		declaredBytes += int64(entry.UncompressedSize64)
		if declaredBytes > maxExtractedBytes {
			return fmt.Errorf("zip exceeds the %d byte extracted size limit", maxExtractedBytes)
		}

		mode := entry.Mode()
		isDirectory := entry.FileInfo().IsDir()
		if mode&os.ModeSymlink != 0 || (!isDirectory && mode.Type() != 0) {
			return fmt.Errorf("zip contains unsupported file type: %s", entry.Name)
		}
		target := filepath.Join(destination, filepath.FromSlash(name))
		targetAbsolute, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if targetAbsolute != destination && !strings.HasPrefix(targetAbsolute, destination+string(os.PathSeparator)) {
			return fmt.Errorf("zip contains unsafe path: %s", entry.Name)
		}
		if isDirectory {
			if err := os.MkdirAll(targetAbsolute, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetAbsolute), 0o755); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		permissions := mode.Perm()
		if permissions == 0 {
			permissions = 0o644
		}
		destinationFile, err := os.OpenFile(targetAbsolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, permissions)
		if err != nil {
			_ = source.Close()
			return err
		}
		remaining := maxExtractedBytes - extractedBytes
		written, copyErr := io.Copy(destinationFile, io.LimitReader(source, remaining+1))
		extractedBytes += written
		closeErr := destinationFile.Close()
		_ = source.Close()
		if copyErr != nil {
			return copyErr
		}
		if extractedBytes > maxExtractedBytes {
			return fmt.Errorf("zip exceeds the %d byte extracted size limit", maxExtractedBytes)
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func safeArchivePath(raw string) (string, error) {
	normalized := strings.ReplaceAll(raw, "\\", "/")
	normalized = strings.TrimSuffix(normalized, "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") || strings.Contains(normalized, ":") {
		return "", fmt.Errorf("zip contains unsafe path: %s", raw)
	}
	cleaned := path.Clean(normalized)
	if cleaned != normalized || !fs.ValidPath(cleaned) {
		return "", fmt.Errorf("zip contains unsafe path: %s", raw)
	}
	for _, component := range strings.Split(cleaned, "/") {
		if !safeWindowsPathComponent(component) {
			return "", fmt.Errorf("zip contains a path unsupported on Windows: %s", raw)
		}
	}
	return cleaned, nil
}

func safeWindowsPathComponent(component string) bool {
	if component == "" || strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") || strings.ContainsAny(component, `<>"|?*`) {
		return false
	}
	for _, character := range component {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	base := component
	if index := strings.IndexByte(base, '.'); index >= 0 {
		base = base[:index]
	}
	base = strings.ToUpper(base)
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CLOCK$" {
		return false
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return false
	}
	return true
}

func inspectModDirectory(root string) (string, Info, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", Info{}, err
	}
	infoPaths := make([]string, 0, 1)
	entries := 0
	var totalBytes int64
	err = filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > maxArchiveFiles+1 {
			return fmt.Errorf("mod directory contains more than %d entries", maxArchiveFiles)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!entry.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("mod directory contains unsupported file type: %s", current)
		}
		if !entry.IsDir() {
			totalBytes += info.Size()
			if totalBytes > maxExtractedBytes {
				return fmt.Errorf("mod directory exceeds the %d byte size limit", maxExtractedBytes)
			}
			if strings.EqualFold(entry.Name(), "Info.json") {
				infoPaths = append(infoPaths, current)
			}
		}
		return nil
	})
	if err != nil {
		return "", Info{}, err
	}
	if len(infoPaths) == 0 {
		return "", Info{}, fmt.Errorf("Info.json not found")
	}
	if len(infoPaths) != 1 {
		return "", Info{}, fmt.Errorf("mod archive must contain exactly one Info.json")
	}
	metadata, err := ReadInfo(infoPaths[0])
	if err != nil {
		return "", Info{}, err
	}
	return filepath.Dir(infoPaths[0]), metadata, nil
}
