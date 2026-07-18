//go:build !windows

package mods

import "os"

func replaceConfigFile(temporaryPath, destination string) error {
	return os.Rename(temporaryPath, destination)
}
