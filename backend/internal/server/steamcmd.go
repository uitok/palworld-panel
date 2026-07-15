package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"palpanel/internal/steamcmd"
)

func (m Manager) nativeSteamCMD() *steamcmd.Client {
	client := steamcmd.New(m.cfg)
	client.SetHTTPClient(m.downloadClient)
	return client
}

func (m Manager) ensureSteamCMD(ctx context.Context) error {
	return m.nativeSteamCMD().Ensure(ctx)
}

func (m Manager) installOrUpdateWindows(ctx context.Context) error {
	for _, path := range []string{m.cfg.SteamCMDDir, m.cfg.ServerDir} {
		if err := m.cfg.ValidateManagedPath(path, false); err != nil {
			return err
		}
	}
	if err := m.nativeSteamCMD().InstallOrUpdate(ctx, palworldServerAppID, m.cfg.ServerDir); err != nil {
		return err
	}
	return m.validateWindowsServerInstall()
}

func (m Manager) validateWindowsServerInstall() error {
	if err := validatePEExecutable(m.cfg.PalServerExePath()); err != nil {
		return fmt.Errorf("Palworld server installation is incomplete: %w", err)
	}
	commandCandidates := []string{
		filepath.Join(m.cfg.Win64Dir(), "PalServer-Win64-Shipping-Cmd.exe"),
		filepath.Join(m.cfg.Win64Dir(), "PalServer-Win64-Test-Cmd.exe"),
	}
	commandFound := false
	for _, path := range commandCandidates {
		if err := validatePEExecutable(path); err == nil {
			commandFound = true
			break
		}
	}
	if !commandFound {
		return fmt.Errorf(
			"Palworld server installation is incomplete: expected a valid command executable at %s or %s",
			commandCandidates[0], commandCandidates[1],
		)
	}
	manifest := m.appManifestPath()
	if info, err := os.Stat(manifest); err != nil || info.IsDir() || info.Size() == 0 {
		if err == nil {
			err = fmt.Errorf("manifest is not a non-empty file")
		}
		return fmt.Errorf("Palworld server installation is incomplete: %s: %w", manifest, err)
	}
	buildID, err := readAppManifestBuildID(manifest)
	if err != nil || strings.TrimSpace(buildID) == "" {
		return fmt.Errorf("Palworld Steam appmanifest is invalid: %w", err)
	}
	return nil
}

func validatePEExecutable(path string) error {
	return steamcmd.ValidatePEExecutable(path)
}

func (m Manager) removeManagedDirectory(path string) error {
	if err := m.cfg.ValidateManagedPath(path, false); err != nil {
		return err
	}
	if err := m.cfg.ValidateManagedPath(path, false); err != nil {
		return err
	}
	return os.RemoveAll(path)
}
