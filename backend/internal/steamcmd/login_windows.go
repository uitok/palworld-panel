//go:build windows

package steamcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func hardenCredentialTree(ctx context.Context, steamCMDDir string) error {
	return hardenCredentialTreeWithRunner(ctx, steamCMDDir, func(ctx context.Context, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, "icacls.exe", args...).CombinedOutput()
	})
}

func hardenCredentialTreeWithRunner(ctx context.Context, steamCMDDir string, run func(context.Context, ...string) ([]byte, error)) error {
	configDir := filepath.Join(steamCMDDir, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("create SteamCMD config directory: %w", err)
	}
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("read current Windows identity: %w", err)
	}
	sid := user.User.Sid.String()
	args := []string{
		configDir,
		"/inheritance:r",
		"/grant:r",
		"*" + sid + ":(OI)(CI)F",
		"*S-1-5-18:(OI)(CI)F",
		"*S-1-5-32-544:(OI)(CI)F",
		"/T",
		"/C",
	}
	output, err := run(ctx, args...)
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return fmt.Errorf("apply SteamCMD config ACL: %w: %s", err, detail)
		}
		return fmt.Errorf("apply SteamCMD config ACL: %w", err)
	}
	return nil
}

func launchInteractiveSteamCMD(binary, directory, accountName string) error {
	cmd, err := interactiveSteamCMDCommand(binary, directory, accountName)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch SteamCMD: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release SteamCMD process handle: %w", err)
	}
	return nil
}

func interactiveSteamCMDCommand(binary, directory, accountName string) (*exec.Cmd, error) {
	if err := ValidateAccountName(accountName); err != nil {
		return nil, err
	}
	cmd := exec.Command(binary, "+login", accountName)
	cmd.Dir = directory
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE | windows.CREATE_NEW_PROCESS_GROUP}
	return cmd, nil
}
