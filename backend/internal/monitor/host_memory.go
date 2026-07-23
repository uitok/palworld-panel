package monitor

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type HostMemoryStats struct {
	TotalBytes     int64
	AvailableBytes int64
	SwapTotalBytes int64
	SwapFreeBytes  int64
}

type HostMemoryCollector interface {
	Collect(context.Context) (HostMemoryStats, error)
}

type windowsMemoryStatus struct {
	TotalPhysicalBytes     uint64
	AvailablePhysicalBytes uint64
	TotalPageFileBytes     uint64
	AvailablePageFileBytes uint64
}

func mapWindowsMemoryStatus(status windowsMemoryStatus) HostMemoryStats {
	swapTotal := subtractUint64(status.TotalPageFileBytes, status.TotalPhysicalBytes)
	swapFree := subtractUint64(status.AvailablePageFileBytes, status.AvailablePhysicalBytes)
	if swapFree > swapTotal {
		swapFree = swapTotal
	}
	return HostMemoryStats{
		TotalBytes:     clampUint64ToInt64(status.TotalPhysicalBytes),
		AvailableBytes: clampUint64ToInt64(status.AvailablePhysicalBytes),
		SwapTotalBytes: clampUint64ToInt64(swapTotal),
		SwapFreeBytes:  clampUint64ToInt64(swapFree),
	}
}

func parseProcMeminfo(reader io.Reader) (HostMemoryStats, error) {
	body, err := io.ReadAll(reader)
	if err != nil {
		return HostMemoryStats{}, err
	}
	values := make(map[string]int64)
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr != nil || value < 0 {
			continue
		}
		if len(fields) >= 3 && !strings.EqualFold(fields[2], "kB") {
			continue
		}
		values[key] = value * 1024
	}
	if values["MemTotal"] <= 0 || values["MemAvailable"] < 0 {
		return HostMemoryStats{}, fmt.Errorf("/proc/meminfo is missing MemTotal or MemAvailable")
	}
	if _, ok := values["MemAvailable"]; !ok {
		return HostMemoryStats{}, fmt.Errorf("/proc/meminfo is missing MemAvailable")
	}
	return HostMemoryStats{
		TotalBytes: values["MemTotal"], AvailableBytes: values["MemAvailable"],
		SwapTotalBytes: values["SwapTotal"], SwapFreeBytes: values["SwapFree"],
	}, nil
}

func subtractUint64(value, deduction uint64) uint64 {
	if value <= deduction {
		return 0
	}
	return value - deduction
}

func clampUint64ToInt64(value uint64) int64 {
	const maxInt64 = uint64(^uint64(0) >> 1)
	if value > maxInt64 {
		return int64(maxInt64)
	}
	return int64(value)
}
