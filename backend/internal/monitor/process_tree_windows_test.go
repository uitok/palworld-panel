//go:build windows

package monitor

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

func TestWindowsProcessTreeStatsAggregatesChildCPUAndMemory(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ready := t.TempDir() + `\ready`
	root := exec.Command(executable, "-test.run=TestWindowsProcessTreeHelperProcess")
	root.Env = append(os.Environ(), "PALPANEL_PROCESS_TREE_HELPER=root", "PALPANEL_PROCESS_TREE_READY="+ready)
	if err := root.Start(); err != nil {
		t.Fatal(err)
	}
	defer exec.Command("taskkill", "/PID", strconv.Itoa(root.Process.Pid), "/T", "/F").Run()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("process-tree helper did not become ready")
		}
		time.Sleep(25 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stats, err := platformProcessTreeStats(ctx, root.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if stats.ProcessCount < 2 {
		t.Fatalf("ProcessCount = %d, want at least root and child", stats.ProcessCount)
	}
	_, rootMemory, rootCount := windowsAggregateProcessStats([]uint32{uint32(root.Process.Pid)})
	if rootCount != 1 || stats.MemoryUsageBytes <= rootMemory+4*1024*1024 {
		t.Fatalf("tree memory = %d, root memory = %d; child working set was not aggregated", stats.MemoryUsageBytes, rootMemory)
	}
	if stats.CPUPercent <= 0 {
		t.Fatalf("CPUPercent = %f, busy child CPU time was not aggregated", stats.CPUPercent)
	}
}

func TestWindowsProcessTreeHelperProcess(t *testing.T) {
	role := os.Getenv("PALPANEL_PROCESS_TREE_HELPER")
	if role == "" {
		return
	}
	if role == "root" {
		executable, _ := os.Executable()
		child := exec.Command(executable, "-test.run=TestWindowsProcessTreeHelperProcess")
		child.Env = append(os.Environ(), "PALPANEL_PROCESS_TREE_HELPER=child")
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		if err := os.WriteFile(os.Getenv("PALPANEL_PROCESS_TREE_READY"), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			os.Exit(3)
		}
		_ = child.Wait()
		return
	}
	memory := make([]byte, 64*1024*1024)
	for index := range memory {
		memory[index] = byte(index)
	}
	deadline := time.Now().Add(30 * time.Second)
	var value uint64
	for time.Now().Before(deadline) {
		for index := 0; index < 1_000_000; index++ {
			value += uint64(index) ^ uint64(memory[index%len(memory)])
		}
	}
	if value == 0 {
		os.Exit(4)
	}
}
