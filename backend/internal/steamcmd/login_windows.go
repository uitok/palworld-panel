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
	"unsafe"

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

func launchInteractiveSteamCMD(binary, directory, accountName string) (<-chan struct{}, error) {
	if err := ValidateAccountName(accountName); err != nil {
		return nil, err
	}
	applicationName, err := windows.UTF16PtrFromString(binary)
	if err != nil {
		return nil, fmt.Errorf("encode SteamCMD path: %w", err)
	}
	commandLine, err := windows.UTF16PtrFromString(interactiveSteamCMDCommandLine(binary, accountName))
	if err != nil {
		return nil, fmt.Errorf("encode SteamCMD command line: %w", err)
	}
	currentDirectory, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return nil, fmt.Errorf("encode SteamCMD directory: %w", err)
	}
	startup := &windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	process := &windows.ProcessInformation{}
	flags := uint32(windows.CREATE_NEW_CONSOLE | windows.CREATE_NEW_PROCESS_GROUP)
	if err := windows.CreateProcess(applicationName, commandLine, nil, nil, false, flags, nil, currentDirectory, startup, process); err != nil {
		return nil, fmt.Errorf("launch SteamCMD with an interactive console: %w", err)
	}
	_ = windows.CloseHandle(process.Thread)
	done := make(chan struct{})
	go func() {
		_, _ = windows.WaitForSingleObject(process.Process, windows.INFINITE)
		_ = windows.CloseHandle(process.Process)
		close(done)
	}()
	return done, nil
}

func interactiveSteamCMDCommandLine(binary, accountName string) string {
	return strings.Join([]string{syscall.EscapeArg(binary), "+login", syscall.EscapeArg(accountName)}, " ")
}
