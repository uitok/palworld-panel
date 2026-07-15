//go:build windows

package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestManagedWindowsHelperProcess(t *testing.T) {
	if os.Getenv("PALPANEL_WINDOWS_PROCESS_HELPER") != "1" {
		return
	}
	if childPIDPath := os.Getenv("PALPANEL_WINDOWS_PROCESS_HELPER_CHILD_PID_FILE"); childPIDPath != "" {
		child := exec.Command(os.Args[0], "-test.run=^TestManagedWindowsHelperChildProcess$", "-test.v")
		child.Env = append(os.Environ(), "PALPANEL_WINDOWS_PROCESS_HELPER_CHILD=1")
		if err := child.Start(); err != nil {
			panic(fmt.Sprintf("start managed helper child: %v", err))
		}
		if err := os.WriteFile(childPIDPath, []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			_ = child.Process.Kill()
			panic(fmt.Sprintf("write managed helper child PID: %v", err))
		}
	}
	fmt.Println("managed helper ready")
	time.Sleep(30 * time.Second)
}

func TestManagedWindowsHelperChildProcess(t *testing.T) {
	if os.Getenv("PALPANEL_WINDOWS_PROCESS_HELPER_CHILD") != "1" {
		return
	}
	fmt.Println("managed helper child ready")
	time.Sleep(30 * time.Second)
}

func TestStartWindowsSurvivesCanceledRequestContext(t *testing.T) {
	manager, cleanup := newWindowsProcessTestManager(t)
	defer cleanup()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(manager.cfg.PalServerExePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(self, manager.cfg.PalServerExePath()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PALPANEL_WINDOWS_PROCESS_HELPER", "1")
	childPIDPath := filepath.Join(t.TempDir(), "managed-child.pid")
	t.Setenv("PALPANEL_WINDOWS_PROCESS_HELPER_CHILD_PID_FILE", childPIDPath)

	requestCtx, cancel := context.WithCancel(t.Context())
	if err := manager.startWindows(requestCtx, []string{"-test.run=^TestManagedWindowsHelperProcess$", "-test.v"}); err != nil {
		cancel()
		t.Fatalf("startWindows returned error: %v", err)
	}
	cancel()
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		_ = manager.stopWindows(stopCtx)
	})

	time.Sleep(300 * time.Millisecond)
	status, err := manager.windowsStatus(t.Context())
	if err != nil || !status.Exists || status.Status != "running" {
		t.Fatalf("process did not survive request cancellation: status=%#v err=%v", status, err)
	}
	childPID := waitForManagedWindowsHelperPID(t, childPIDPath)
	childBeforeStop, err := inspectWindowsProcess(childPID)
	if err != nil || !childBeforeStop.Running {
		t.Fatalf("managed helper child is not running before stop: pid=%d info=%#v err=%v", childPID, childBeforeStop, err)
	}
	logContent, err := os.ReadFile(manager.cfg.ServerLogPath())
	if err != nil || !strings.Contains(string(logContent), "[palpanel] starting PalServer.exe") {
		t.Fatalf("server lifecycle log was not initialized: content=%q err=%v", logContent, err)
	}

	stopCtx, stopCancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer stopCancel()
	if err := manager.stopWindows(stopCtx); err != nil {
		t.Fatalf("stopWindows returned error: %v", err)
	}
	status, err = manager.windowsStatus(t.Context())
	if err != nil || status.Exists {
		t.Fatalf("managed helper still appears to be running: status=%#v err=%v", status, err)
	}
	childAfterStop, childErr := inspectWindowsProcess(childPID)
	if childErr == nil && childAfterStop.Running && childAfterStop.CreationTime == childBeforeStop.CreationTime {
		t.Fatalf("managed helper child still appears to be running after stop: pid=%d info=%#v", childPID, childAfterStop)
	}
	if childErr != nil && !isWindowsProcessMissing(childErr) {
		t.Fatalf("inspect managed helper child after stop: %v", childErr)
	}
	if err := os.Remove(manager.cfg.ServerLogPath()); err != nil {
		t.Fatalf("server log is still held open after stop: %v", err)
	}
}

func TestTerminateWindowsProcessTreeNative(t *testing.T) {
	childPIDPath := filepath.Join(t.TempDir(), "native-child.pid")
	cmd := exec.Command(os.Args[0], "-test.run=^TestManagedWindowsHelperProcess$", "-test.v")
	cmd.Env = append(
		os.Environ(),
		"PALPANEL_WINDOWS_PROCESS_HELPER=1",
		"PALPANEL_WINDOWS_PROCESS_HELPER_CHILD_PID_FILE="+childPIDPath,
	)
	prepareWindowsProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start unmanaged helper: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = terminateWindowsProcessTree(cleanupCtx, cmd.Process.Pid)
		_ = cmd.Wait()
	})

	childPID := waitForManagedWindowsHelperPID(t, childPIDPath)
	childBeforeStop, err := inspectWindowsProcess(childPID)
	if err != nil || !childBeforeStop.Running {
		t.Fatalf("untracked helper child is not running before stop: pid=%d info=%#v err=%v", childPID, childBeforeStop, err)
	}

	stopCtx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := terminateWindowsProcessTree(stopCtx, cmd.Process.Pid); err != nil {
		t.Fatalf("terminateWindowsProcessTree returned error: %v", err)
	}
	_ = cmd.Wait()
	childAfterStop, childErr := inspectWindowsProcess(childPID)
	if childErr == nil && childAfterStop.Running && childAfterStop.CreationTime == childBeforeStop.CreationTime {
		t.Fatalf("untracked helper child still appears to be running after stop: pid=%d info=%#v", childPID, childAfterStop)
	}
	if childErr != nil && !isWindowsProcessMissing(childErr) {
		t.Fatalf("inspect untracked helper child after stop: %v", childErr)
	}
}

func waitForManagedWindowsHelperPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
			if parseErr == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("managed helper did not write child PID to %s", path)
	return 0
}
