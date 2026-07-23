//go:build windows

package monitor

import (
	"context"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsCPUSampleInterval = 250 * time.Millisecond

var getProcessMemoryInfo = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

func platformProcessTreeStats(ctx context.Context, rootPID int) (ProcessStats, error) {
	beforePIDs, err := windowsProcessTree(uint32(rootPID))
	if err != nil {
		return ProcessStats{}, err
	}
	before, _, _ := windowsAggregateProcessStats(beforePIDs)
	started := time.Now()
	timer := time.NewTimer(windowsCPUSampleInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ProcessStats{}, ctx.Err()
	case <-timer.C:
	}
	afterPIDs, err := windowsProcessTree(uint32(rootPID))
	if err != nil {
		return ProcessStats{}, err
	}
	after, memory, count := windowsAggregateProcessStats(afterPIDs)
	if count == 0 {
		return ProcessStats{}, fmt.Errorf("PID %d and its children are unavailable", rootPID)
	}
	elapsed := time.Since(started)
	if after < before || elapsed <= 0 {
		return ProcessStats{}, fmt.Errorf("invalid process-tree time sample")
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
	return ProcessStats{CPUPercent: percent, MemoryUsageBytes: memory, ProcessCount: count}, nil
}

func windowsProcessTree(rootPID uint32) ([]uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("enumerate processes: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	children := map[uint32][]uint32{}
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, fmt.Errorf("read process snapshot: %w", err)
	}
	for {
		children[entry.ParentProcessID] = append(children[entry.ParentProcessID], entry.ProcessID)
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if err == syscall.ERROR_NO_MORE_FILES {
				break
			}
			return nil, fmt.Errorf("read process snapshot: %w", err)
		}
	}
	result := []uint32{rootPID}
	seen := map[uint32]bool{rootPID: true}
	for index := 0; index < len(result); index++ {
		for _, child := range children[result[index]] {
			if child == 0 || seen[child] {
				continue
			}
			seen[child] = true
			result = append(result, child)
		}
	}
	return result, nil
}

func windowsAggregateProcessStats(pids []uint32) (cpuTicks uint64, workingSet int64, count int) {
	for _, pid := range pids {
		handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, pid)
		if err != nil {
			continue
		}
		cpu, cpuErr := windowsProcessCPUTime(handle)
		memory, memoryErr := windowsProcessWorkingSet(handle)
		windows.CloseHandle(handle)
		if cpuErr != nil || memoryErr != nil {
			continue
		}
		cpuTicks += cpu
		workingSet += memory
		count++
	}
	return cpuTicks, workingSet, count
}

func windowsProcessCPUTime(handle windows.Handle) (uint64, error) {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return 0, fmt.Errorf("read process times: %w", err)
	}
	return filetimeTicks(kernel) + filetimeTicks(user), nil
}

func windowsProcessWorkingSet(handle windows.Handle) (int64, error) {
	counters := processMemoryCounters{CB: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	result, _, callErr := getProcessMemoryInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&counters)), uintptr(counters.CB))
	if result == 0 {
		return 0, fmt.Errorf("read process memory: %w", callErr)
	}
	return int64(counters.WorkingSetSize), nil
}

func filetimeTicks(value windows.Filetime) uint64 {
	return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
}
