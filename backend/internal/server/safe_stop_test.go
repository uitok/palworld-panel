package server

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"palpanel/internal/docker"
)

func TestSafeStopRequestsGracefulShutdownThenFallsBackToManagedStop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake Docker lifecycle fixture is POSIX-only")
	}
	manager, cleanup := newOperationsManager(t)
	defer cleanup()
	if err := manager.SetRuntimeMode(t.Context(), RuntimeWineDocker); err != nil {
		t.Fatal(err)
	}
	manager.lifecycleWait = func(context.Context, time.Duration) error { return nil }
	manager.gracefulStopTimeout = 0
	manager.gracefulStopPoll = 0

	var notified bool
	job, err := manager.SafeStop(t.Context(), 60, "maintenance", func(_ context.Context, wait int, message string) error {
		notified = true
		if wait != 60 || message != "maintenance" {
			t.Fatalf("notification = %d %q", wait, message)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("SafeStop returned error: %v", err)
	}
	completed := waitForJob(t, manager.store, job.ID)
	if !notified {
		t.Fatal("graceful shutdown notifier was not called")
	}
	if completed.Status != "completed" || !strings.Contains(completed.Message, "fallback") {
		t.Fatalf("safe stop job = %#v", completed)
	}
}

func TestSafeStopValidatesCountdown(t *testing.T) {
	manager, cleanup := newOperationsManager(t)
	defer cleanup()
	for _, wait := range []int{0, 4, 301} {
		if _, err := manager.SafeStop(t.Context(), wait, "", nil); err == nil {
			t.Fatalf("SafeStop(%d) should reject invalid countdown", wait)
		}
	}
}

func TestServerStatusRunning(t *testing.T) {
	if serverStatusRunning(Status{}) {
		t.Fatal("missing process must not be running")
	}
	if !serverStatusRunning(Status{Container: docker.ContainerStatus{Exists: true, Status: "running"}}) {
		t.Fatal("running process was not recognized")
	}
	if !serverStatusRunning(Status{Container: docker.ContainerStatus{Exists: true, Status: "paused"}}) {
		t.Fatal("paused process must remain managed as running")
	}
	if serverStatusRunning(Status{Container: docker.ContainerStatus{Exists: true, Status: "exited"}}) {
		t.Fatal("exited process must not be running")
	}
}
