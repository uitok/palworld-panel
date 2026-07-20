//go:build !windows

package steamcmd

import (
	"context"
	"os"
)

func securePrivatePath(_ context.Context, path string) error {
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		mode = 0o700
	}
	return os.Chmod(path, mode)
}
