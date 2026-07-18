//go:build windows

package monitor

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"golang.org/x/sys/windows"
)

const windowsCPUSampleInterval = 250 * time.Millisecond

func platformProcessCPUPercent(ctx context.Context, pid int) (float64, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, fmt.Errorf("open PID %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	before, err := windowsProcessCPUTime(handle)
	if err != nil {
		return 0, err
	}
	started := time.Now()
	timer := time.NewTimer(windowsCPUSampleInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-timer.C:
	}
	after, err := windowsProcessCPUTime(handle)
	if err != nil {
		return 0, err
	}
	elapsed := time.Since(started)
	if after < before || elapsed <= 0 {
		return 0, fmt.Errorf("invalid process time sample")
	}
	processorCount := runtime.NumCPU()
	if processorCount < 1 {
		processorCount = 1
	}
	processDuration := time.Duration(after-before) * 100 * time.Nanosecond
	percent := float64(processDuration) / float64(elapsed) / float64(processorCount) * 100
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return percent, nil
}

func windowsProcessCPUTime(handle windows.Handle) (uint64, error) {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return 0, fmt.Errorf("read process times: %w", err)
	}
	return filetimeTicks(kernel) + filetimeTicks(user), nil
}

func filetimeTicks(value windows.Filetime) uint64 {
	return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
}
