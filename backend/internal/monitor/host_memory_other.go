//go:build !linux && !windows

package monitor

import (
	"context"
	"errors"
)

type unsupportedHostMemoryCollector struct{}

func newPlatformHostMemoryCollector() HostMemoryCollector {
	return unsupportedHostMemoryCollector{}
}

func (unsupportedHostMemoryCollector) Collect(context.Context) (HostMemoryStats, error) {
	return HostMemoryStats{}, errors.New("host memory collection is unavailable on this platform")
}
