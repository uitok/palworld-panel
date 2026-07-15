//go:build !windows

package mods

import (
	"io/fs"
	"os"
)

func localScanPathIsLink(_ string, info fs.FileInfo) (bool, error) {
	return info.Mode()&os.ModeSymlink != 0, nil
}
