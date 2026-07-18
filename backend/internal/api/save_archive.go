package api

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type saveArchiveFormat string

const (
	saveArchiveZIP      saveArchiveFormat = "zip"
	saveArchiveTAR      saveArchiveFormat = "tar"
	saveArchiveTARGzip  saveArchiveFormat = "tar_gzip"
	maxSaveArchiveFiles                   = 4096
)

func saveArchiveFormatForName(name string) (saveArchiveFormat, error) {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return saveArchiveZIP, nil
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return saveArchiveTARGzip, nil
	case strings.HasSuffix(lower, ".tar"):
		return saveArchiveTAR, nil
	default:
		return "", errors.New("only .zip, .tar, .tar.gz, and .tgz save archives are supported")
	}
}

func defaultSaveSourceName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	lower := strings.ToLower(base)
	for _, suffix := range []string{".tar.gz", ".tgz", ".zip", ".tar"} {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSpace(base[:len(base)-len(suffix)])
		}
	}
	return strings.TrimSpace(base)
}

func extractSaveArchive(archivePath, destination string, maxBytes int64, format saveArchiveFormat) error {
	switch format {
	case saveArchiveZIP:
		return extractSaveZIP(archivePath, destination, maxBytes)
	case saveArchiveTAR:
		file, err := os.Open(archivePath)
		if err != nil {
			return err
		}
		defer file.Close()
		return extractSaveTAR(tar.NewReader(file), destination, maxBytes)
	case saveArchiveTARGzip:
		file, err := os.Open(archivePath)
		if err != nil {
			return err
		}
		defer file.Close()
		compressed, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer compressed.Close()
		return extractSaveTAR(tar.NewReader(compressed), destination, maxBytes)
	default:
		return errors.New("unsupported save archive format")
	}
}

func extractSaveZIP(archivePath, destination string, maxBytes int64) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	if len(reader.File) == 0 || len(reader.File) > maxSaveArchiveFiles {
		return errors.New("archive file count is invalid")
	}
	var total int64
	for _, entry := range reader.File {
		clean, err := cleanSaveArchivePath(entry.Name)
		if err != nil {
			return err
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not allowed: %s", entry.Name)
		}
		size := int64(entry.UncompressedSize64)
		if size < 0 || total > maxBytes-size {
			return errors.New("uncompressed save archive exceeds limit")
		}
		total += size
		target, err := saveArchiveTarget(destination, clean)
		if err != nil {
			return err
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		if err := writeSaveArchiveFile(target, source, size); err != nil {
			_ = source.Close()
			return err
		}
		if err := source.Close(); err != nil {
			return err
		}
	}
	return nil
}

func extractSaveTAR(reader *tar.Reader, destination string, maxBytes int64) error {
	count := 0
	var total int64
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		count++
		if count > maxSaveArchiveFiles {
			return errors.New("archive file count is invalid")
		}
		clean, err := cleanSaveArchivePath(header.Name)
		if err != nil {
			return err
		}
		target, err := saveArchiveTarget(destination, clean)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || total > maxBytes-header.Size {
				return errors.New("uncompressed save archive exceeds limit")
			}
			total += header.Size
			if err := writeSaveArchiveFile(target, reader, header.Size); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("links are not allowed: %s", header.Name)
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			continue
		default:
			return fmt.Errorf("unsupported archive entry type for %s", header.Name)
		}
	}
	if count == 0 {
		return errors.New("archive file count is invalid")
	}
	return nil
}

func cleanSaveArchivePath(name string) (string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	normalized = strings.TrimSuffix(normalized, "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") || strings.ContainsRune(normalized, '\x00') {
		return "", fmt.Errorf("unsafe archive path: %s", name)
	}
	clean := path.Clean(normalized)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != normalized {
		return "", fmt.Errorf("unsafe archive path: %s", name)
	}
	for _, component := range strings.Split(clean, "/") {
		if component == "" || strings.Contains(component, ":") {
			return "", fmt.Errorf("unsafe archive path: %s", name)
		}
	}
	return filepath.FromSlash(clean), nil
}

func saveArchiveTarget(destination, clean string) (string, error) {
	target := filepath.Join(destination, clean)
	if !pathWithin(destination, target) {
		return "", fmt.Errorf("unsafe archive target: %s", clean)
	}
	return target, nil
}

func writeSaveArchiveFile(target string, source io.Reader, size int64) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	destination, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.CopyN(destination, source, size)
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	if written != size {
		return io.ErrUnexpectedEOF
	}
	return closeErr
}

func findImportedWorld(root string) (string, error) {
	var candidates []string
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if name == "backup" || name == "backups" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(entry.Name(), "Level.sav") {
			candidates = append(candidates, filepath.Dir(current))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", errors.New("Level.sav was not found in the archive")
	}
	return candidates[0], nil
}
