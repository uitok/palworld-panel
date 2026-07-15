//go:build !windows

package appconfig

import "path/filepath"

func resolveFinalPath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
