//go:build windows

package steamcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows"
)

func securePrivatePath(ctx context.Context, path string) error {
	return hardenPrivatePathWithRunner(ctx, path, func(ctx context.Context, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, "icacls.exe", args...).CombinedOutput()
	})
}

func SecurePrivatePath(ctx context.Context, path string) error {
	return securePrivatePath(ctx, path)
}

func hardenPrivatePathWithRunner(ctx context.Context, path string, run func(context.Context, ...string) ([]byte, error)) error {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("read current Windows identity: %w", err)
	}
	sid := user.User.Sid.String()
	grant := "*" + sid + ":F"
	systemGrant := "*S-1-5-18:F"
	adminGrant := "*S-1-5-32-544:F"
	if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
		grant = "*" + sid + ":(OI)(CI)F"
		systemGrant = "*S-1-5-18:(OI)(CI)F"
		adminGrant = "*S-1-5-32-544:(OI)(CI)F"
	}
	args := []string{
		path,
		"/inheritance:r",
		"/grant:r",
		grant,
		systemGrant,
		adminGrant,
		"/T",
		"/C",
	}
	output, err := run(ctx, args...)
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return fmt.Errorf("apply private path ACL: %w: %s", err, detail)
		}
		return fmt.Errorf("apply private path ACL: %w", err)
	}
	return nil
}
