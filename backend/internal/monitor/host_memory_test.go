package monitor

import (
	"context"
	"strings"
	"testing"
)

func TestParseProcMeminfoMapsHostMemoryAndSwap(t *testing.T) {
	stats, err := parseProcMeminfo(strings.NewReader(`MemTotal:       16384000 kB
MemFree:         1024000 kB
MemAvailable:    4096000 kB
Buffers:          128000 kB
Cached:           256000 kB
SwapTotal:       2097152 kB
SwapFree:         524288 kB
`))
	if err != nil {
		t.Fatalf("parseProcMeminfo returned error: %v", err)
	}
	if stats.TotalBytes != 16384000*1024 || stats.AvailableBytes != 4096000*1024 {
		t.Fatalf("unexpected host memory: %#v", stats)
	}
	if stats.SwapTotalBytes != 2097152*1024 || stats.SwapFreeBytes != 524288*1024 {
		t.Fatalf("unexpected host swap: %#v", stats)
	}
}

func TestParseProcMeminfoRequiresTotalAndAvailable(t *testing.T) {
	if _, err := parseProcMeminfo(strings.NewReader("MemTotal: 10 kB\n")); err == nil {
		t.Fatal("expected missing MemAvailable to fail")
	}
}

func TestMapWindowsMemoryStatusSeparatesPhysicalMemoryAndSwap(t *testing.T) {
	stats := mapWindowsMemoryStatus(windowsMemoryStatus{
		TotalPhysicalBytes:     16 << 30,
		AvailablePhysicalBytes: 4 << 30,
		TotalPageFileBytes:     24 << 30,
		AvailablePageFileBytes: 7 << 30,
	})
	if stats.TotalBytes != 16<<30 || stats.AvailableBytes != 4<<30 {
		t.Fatalf("unexpected physical memory mapping: %#v", stats)
	}
	if stats.SwapTotalBytes != 8<<30 || stats.SwapFreeBytes != 3<<30 {
		t.Fatalf("unexpected swap mapping: %#v", stats)
	}
}

type fixedHostMemoryCollector struct {
	stats HostMemoryStats
	err   error
}

func (f fixedHostMemoryCollector) Collect(context.Context) (HostMemoryStats, error) {
	return f.stats, f.err
}
