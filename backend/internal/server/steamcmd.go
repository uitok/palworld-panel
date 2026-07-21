package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	for _, path := range []string{m.cfg.SteamCMDDir, m.cfg.ServerDirectory()} {
		if err := m.cfg.ValidateManagedPath(path, false); err != nil {
			return err
		}
	}
	if err := m.nativeSteamCMD().InstallOrUpdate(ctx, palworldServerAppID, m.cfg.ServerDirectory()); err != nil {
		return err
	}
	return m.validateWindowsServerInstall()
}

func (m Manager) installOrUpdateLinux(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("linux_steamcmd runtime requires Linux host")
	}
	for _, path := range []string{m.cfg.SteamCMDDir, m.cfg.ServerDirectory()} {
		if err := m.cfg.ValidateManagedPath(path, false); err != nil {
			return err
		}
	}
	if err := m.nativeSteamCMD().InstallOrUpdate(ctx, palworldServerAppID, m.cfg.ServerDirectory()); err != nil {
		return err
	}
	return m.validateLinuxServerInstall()
}

func (m Manager) validateLinuxServerInstall() error {
	path := m.cfg.PalServerLinuxPath()
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Palworld Linux server installation is incomplete: %w", err)
	}
	if info.IsDir() || info.Mode()&0111 == 0 {
		return fmt.Errorf("Palworld Linux server launcher is not executable: %s", path)
	}
	if _, err := m.linuxServerBinaryPath(); err != nil {
		return err
	}
	if _, err := readAppManifestBuildID(appManifestPathForRoot(m.cfg.ServerDirectory())); err != nil {
		return fmt.Errorf("Palworld Steam appmanifest is invalid: %w", err)
	}
	return nil
}

func (m Manager) linuxServerBinaryPath() (string, error) {
	for _, name := range []string{"PalServer-Linux-Shipping", "PalServer-Linux-Test"} {
		path := filepath.Join(m.cfg.LinuxBinariesDir(), name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("Palworld Linux server binary is missing under %s", m.cfg.LinuxBinariesDir())
}

func (m Manager) validateWindowsServerInstall() error {
	return validateWindowsServerDirectory(m.cfg.ServerDirectory())
}

func validateWindowsServerDirectory(serverRoot string) error {
	palServerPath := filepath.Join(serverRoot, "PalServer.exe")
	if err := validatePEExecutable(palServerPath); err != nil {
		return fmt.Errorf("Palworld server installation is incomplete: %w", err)
	}
	commandCandidates := []string{
		filepath.Join(serverRoot, "Pal", "Binaries", "Win64", "PalServer-Win64-Shipping-Cmd.exe"),
		filepath.Join(serverRoot, "Pal", "Binaries", "Win64", "PalServer-Win64-Test-Cmd.exe"),
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
	manifest := appManifestPathForRoot(serverRoot)
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

func validateHostExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() || info.Mode()&0111 == 0 {
		return fmt.Errorf("not executable: %s", path)
	}
	return nil
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
