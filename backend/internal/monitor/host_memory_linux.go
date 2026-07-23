//go:build linux

package monitor

import (
	"context"
	"os"
)

type linuxHostMemoryCollector struct{}

func newPlatformHostMemoryCollector() HostMemoryCollector {
	return linuxHostMemoryCollector{}
}

func (linuxHostMemoryCollector) Collect(context.Context) (HostMemoryStats, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return HostMemoryStats{}, err
	}
	defer file.Close()
	return parseProcMeminfo(file)
}
