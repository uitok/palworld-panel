package monitor

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
)

func TestDockerStatsReusesStructuredStatusWithoutInspect(t *testing.T) {
	manager := Manager{cfg: appconfig.Config{DockerBinary: "docker", DockerContainer: "palworld"}}
	var calls [][]string
	manager.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte("5%|1GiB / 4GiB"), nil
	}
	sample := db.MonitorSample{}
	manager.fillDockerStats(t.Context(), &sample, docker.ContainerStatus{
		Exists: true, Status: "running", LifecycleAvailable: true, RestartCount: 2, StartedAt: "2026-07-22T01:00:00Z",
	})
	inspectCalls := 0
	for _, call := range calls {
		if len(call) > 1 && call[1] == "inspect" {
			inspectCalls++
		}
	}
	if inspectCalls != 0 {
		t.Fatalf("monitor issued %d inspect calls", inspectCalls)
	}
	if sample.WorkloadMemoryUsageBytes != 1<<30 || sample.WorkloadMemoryLimitBytes != 4<<30 || !sample.LifecycleAvailable || sample.RestartCount != 2 || sample.StartedAt == "" {
		t.Fatalf("unexpected sample: %#v", sample)
	}
}

func TestStoppedDockerLifecycleSurvivesStatsFailureWithoutAnotherInspect(t *testing.T) {
	manager := Manager{cfg: appconfig.Config{DockerBinary: "docker", DockerContainer: "palworld"}}
	inspectCalls := 0
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "inspect" {
			inspectCalls++
		}
		return nil, errors.New("container is not running")
	}
	sample := db.MonitorSample{}
	manager.fillDockerStats(t.Context(), &sample, docker.ContainerStatus{
		Exists: true, Status: "exited", LifecycleAvailable: true, OOMKilled: true,
		ExitCode: 137, RestartCount: 4, StartedAt: "2026-07-22T01:00:00Z", FinishedAt: "2026-07-22T02:00:00Z",
	})
	if inspectCalls != 0 {
		t.Fatalf("monitor issued %d additional inspect calls", inspectCalls)
	}
	if !sample.LifecycleAvailable || !sample.OOMKilled || sample.ExitCode != 137 || sample.RestartCount != 4 || sample.FinishedAt == "" {
		t.Fatalf("sample = %#v", sample)
	}
}

func TestDockerLifecycleSurvivesMalformedStatsWithoutAnotherInspect(t *testing.T) {
	manager := Manager{cfg: appconfig.Config{DockerBinary: "docker", DockerContainer: "palworld"}}
	inspectCalls := 0
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "inspect" {
			inspectCalls++
		}
		return []byte("malformed stats"), nil
	}
	sample := db.MonitorSample{}
	manager.fillDockerStats(t.Context(), &sample, docker.ContainerStatus{
		Exists: true, Status: "running", LifecycleAvailable: true, RestartCount: 2, StartedAt: "2026-07-22T01:00:00Z",
	})
	if inspectCalls != 0 || !sample.LifecycleAvailable || sample.RestartCount != 2 || sample.StartedAt == "" {
		t.Fatalf("inspect=%d sample=%#v", inspectCalls, sample)
	}
}

func TestWindowsProcessAndHostMemoryRemainSeparate(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "monitor.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SetKV(t.Context(), "windows_pid", "321"); err != nil {
		t.Fatal(err)
	}
	manager := Manager{
		store: store,
		hostMemory: fixedHostMemoryCollector{stats: HostMemoryStats{
			TotalBytes: 32 << 30, AvailableBytes: 6 << 30, SwapTotalBytes: 8 << 30, SwapFreeBytes: 2 << 30,
		}},
		processStats: func(context.Context, int) (ProcessStats, error) {
			return ProcessStats{MemoryUsageBytes: 3 << 30, ProcessCount: 5}, nil
		},
	}
	sample := db.MonitorSample{}
	manager.fillHostMemory(t.Context(), &sample)
	manager.fillWindowsProcessStats(t.Context(), &sample)
	if sample.HostMemoryTotalBytes != 32<<30 || sample.HostMemoryAvailableBytes != 6<<30 || sample.HostSwapTotalBytes != 8<<30 || sample.HostSwapFreeBytes != 2<<30 {
		t.Fatalf("unexpected host memory: %#v", sample)
	}
	if sample.WorkloadMemoryUsageBytes != 3<<30 || sample.WorkloadMemoryLimitBytes != 0 {
		t.Fatalf("unexpected workload memory: %#v", sample)
	}
	if !sample.MemoryAvailable || sample.MemoryUsageBytes != sample.WorkloadMemoryUsageBytes || strings.Contains(sample.UnavailableReason, "host memory") {
		t.Fatalf("unexpected compatibility mapping: %#v", sample)
	}
}

func TestWindowsProcessUsesPersistedCreationTimeWithoutInventingExitMetadata(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "windows-lifecycle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SetKV(t.Context(), "windows_pid", "321"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetKV(t.Context(), "windows_process", `{"pid":321,"executable":"PalServer.exe","creation_time":116444736000000000}`); err != nil {
		t.Fatal(err)
	}
	manager := Manager{store: store, processStats: func(context.Context, int) (ProcessStats, error) {
		return ProcessStats{MemoryUsageBytes: 1}, nil
	}}
	sample := db.MonitorSample{}
	manager.fillWindowsProcessStats(t.Context(), &sample)
	if sample.StartedAt != "1970-01-01T00:00:00Z" {
		t.Fatalf("started_at = %q", sample.StartedAt)
	}
	if sample.FinishedAt != "" || sample.ExitCode != 0 || sample.RestartCount != 0 || sample.OOMKilled {
		t.Fatalf("invented Windows exit metadata: %#v", sample)
	}
}
