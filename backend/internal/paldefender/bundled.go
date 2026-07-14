package paldefender

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const (
	BundledPalDefenderVersion = "1.8.1"
	BundledPalDefenderSHA256  = "18b9f63eea2dd407f29b77a262f9d33b1dcd4b744328892c13d5822701418d03"
)

//go:embed assets/PalDefender.dll
var bundledPalDefenderDLL []byte

type BundledAssetInfo struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Size    int    `json:"size"`
}

func BundledInfo() BundledAssetInfo {
	return BundledAssetInfo{
		Version: BundledPalDefenderVersion,
		SHA256:  BundledPalDefenderSHA256,
		Size:    len(bundledPalDefenderDLL),
	}
}

func validateBundledDLL() error {
	if len(bundledPalDefenderDLL) == 0 {
		return fmt.Errorf("bundled PalDefender.dll is empty")
	}
	sum := sha256.Sum256(bundledPalDefenderDLL)
	if hex.EncodeToString(sum[:]) != BundledPalDefenderSHA256 {
		return fmt.Errorf("bundled PalDefender.dll sha256 mismatch")
	}
	return nil
}

func (m Manager) installBundledDLL() error {
	if err := validateBundledDLL(); err != nil {
		return err
	}
	destination := filepath.Join(m.cfg.Win64Dir(), "PalDefender.dll")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".PalDefender.dll.*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(bundledPalDefenderDLL); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}

	previous := destination + ".palpanel-replaced"
	_ = os.Remove(previous)
	hadPrevious := fileExists(destination)
	if hadPrevious {
		if err := os.Rename(destination, previous); err != nil {
			return err
		}
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		if hadPrevious {
			_ = os.Rename(previous, destination)
		}
		return err
	}
	_ = os.Remove(previous)
	return nil
}
