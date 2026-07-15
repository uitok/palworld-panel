//go:build windows

package monitor

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

func platformDiskUsage(path string) (int64, int64, error) {
	pathPtr, err := windows.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return 0, 0, err
	}
	var freeAvailable, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeAvailable, &total, &totalFree); err != nil {
		return 0, 0, err
	}
	return int64(freeAvailable), int64(total), nil
}
