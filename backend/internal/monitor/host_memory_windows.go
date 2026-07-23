//go:build windows

package monitor

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var globalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

type memoryStatusEx struct {
	Length                   uint32
	MemoryLoad               uint32
	TotalPhysical            uint64
	AvailablePhysical        uint64
	TotalPageFile            uint64
	AvailablePageFile        uint64
	TotalVirtual             uint64
	AvailableVirtual         uint64
	AvailableExtendedVirtual uint64
}

type windowsHostMemoryCollector struct{}

func newPlatformHostMemoryCollector() HostMemoryCollector {
	return windowsHostMemoryCollector{}
}

func (windowsHostMemoryCollector) Collect(context.Context) (HostMemoryStats, error) {
	status := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	result, _, callErr := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if result == 0 {
		return HostMemoryStats{}, fmt.Errorf("GlobalMemoryStatusEx: %w", callErr)
	}
	return mapWindowsMemoryStatus(windowsMemoryStatus{
		TotalPhysicalBytes: status.TotalPhysical, AvailablePhysicalBytes: status.AvailablePhysical,
		TotalPageFileBytes: status.TotalPageFile, AvailablePageFileBytes: status.AvailablePageFile,
	}), nil
}
