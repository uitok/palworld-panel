//go:build windows

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const stillActive = 259

const (
	windowsWaitTimeout     = 258
	windowsTerminationCode = 1
)

// trackedWindowsProcess keeps the launcher-owned process handle alive until
// cmd.Wait has released its output files. The Job Object is the normal tree
// ownership boundary; native tree termination is used only on hosts that do
// not allow assigning this child to another Job Object.
type trackedWindowsProcess struct {
	process *os.Process
	done    chan struct{}

	mu  sync.Mutex
	job windows.Handle

	finishOnce sync.Once
}

var windowsProcessTracker = struct {
	sync.Mutex
	byPID map[int]*trackedWindowsProcess
}{byPID: make(map[int]*trackedWindowsProcess)}

func prepareWindowsProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}
}

// trackWindowsProcess tries to create a kill-on-close Job Object. A job
// assignment warning is non-fatal because hosts can put the launcher in a
// non-breakaway job; that case has a verified native process-tree fallback.
func trackWindowsProcess(process *os.Process) (*trackedWindowsProcess, error) {
	if process == nil || process.Pid <= 0 {
		return nil, errors.New("cannot track an invalid Windows process")
	}
	tracked := &trackedWindowsProcess{process: process, done: make(chan struct{})}
	job, jobErr := createWindowsProcessJob(process.Pid)
	if jobErr == nil {
		tracked.job = job
	}

	windowsProcessTracker.Lock()
	if previous := windowsProcessTracker.byPID[process.Pid]; previous != nil {
		select {
		case <-previous.done:
			// A completed process can no longer own this PID.
		default:
			windowsProcessTracker.Unlock()
			if job != 0 {
				_ = windows.CloseHandle(job)
			}
			return nil, fmt.Errorf("Windows process PID %d is already tracked", process.Pid)
		}
	}
	windowsProcessTracker.byPID[process.Pid] = tracked
	windowsProcessTracker.Unlock()
	return tracked, jobErr
}

func finishTrackedWindowsProcess(tracked *trackedWindowsProcess) {
	if tracked == nil {
		return
	}
	tracked.finishOnce.Do(func() {
		windowsProcessTracker.Lock()
		if windowsProcessTracker.byPID[tracked.process.Pid] == tracked {
			delete(windowsProcessTracker.byPID, tracked.process.Pid)
		}
		windowsProcessTracker.Unlock()

		tracked.mu.Lock()
		if tracked.job != 0 {
			_ = windows.CloseHandle(tracked.job)
			tracked.job = 0
		}
		tracked.mu.Unlock()
		close(tracked.done)
	})
}

func trackedWindowsProcessForPID(pid int) *trackedWindowsProcess {
	windowsProcessTracker.Lock()
	defer windowsProcessTracker.Unlock()
	return windowsProcessTracker.byPID[pid]
}

func waitForTrackedWindowsProcess(ctx context.Context, tracked *trackedWindowsProcess) error {
	if tracked == nil {
		return nil
	}
	select {
	case <-tracked.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func createWindowsProcessJob(pid int) (windows.Handle, error) {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(process)

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func inspectWindowsProcess(pid int) (windowsProcessInfo, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return windowsProcessInfo{}, err
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return windowsProcessInfo{}, err
	}
	if exitCode != stillActive {
		return windowsProcessInfo{Running: false}, nil
	}

	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return windowsProcessInfo{}, err
	}
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return windowsProcessInfo{}, err
	}
	return windowsProcessInfo{
		Running:      true,
		Executable:   windows.UTF16ToString(buffer[:size]),
		CreationTime: uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime),
	}, nil
}

func terminateWindowsProcessTree(ctx context.Context, pid int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tracked := trackedWindowsProcessForPID(pid)
	if tracked == nil {
		return terminateWindowsProcessTreeNative(ctx, pid, nil)
	}

	tracked.mu.Lock()
	job := tracked.job
	if job != 0 {
		err := windows.TerminateJobObject(job, windowsTerminationCode)
		tracked.mu.Unlock()
		if err == nil {
			return waitForTrackedWindowsProcess(ctx, tracked)
		}
		select {
		case <-tracked.done:
			return nil
		default:
			// The process can be launched from a non-breakaway host job. Fall
			// through to the creation-time-checked native tree termination.
		}
	} else {
		tracked.mu.Unlock()
	}
	if err := terminateWindowsProcessTreeNative(ctx, pid, tracked.process); err != nil {
		return err
	}
	return waitForTrackedWindowsProcess(ctx, tracked)
}

type windowsTreeProcess struct {
	pid          int
	creationTime uint64
}

func terminateWindowsProcessTreeNative(ctx context.Context, rootPID int, rootProcess *os.Process) error {
	tree, err := snapshotWindowsProcessTree(rootPID)
	if err != nil {
		return err
	}
	for _, process := range tree {
		if process.pid == rootPID && rootProcess != nil {
			if err := rootProcess.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return fmt.Errorf("terminate tracked PalServer process %d: %w", rootPID, err)
			}
			continue
		}
		if err := terminateWindowsSnapshotProcess(ctx, process); err != nil {
			return err
		}
	}
	return nil
}

// snapshotWindowsProcessTree returns descendants before their parent. A
// creation-time snapshot prevents a recycled PID from killing another process.
func snapshotWindowsProcessTree(rootPID int) ([]windowsTreeProcess, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	children := make(map[uint32][]uint32)
	present := false
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		if isWindowsProcessMissing(err) || errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return nil, nil
		}
		return nil, err
	}
	for {
		if int(entry.ProcessID) == rootPID {
			present = true
		}
		children[entry.ParentProcessID] = append(children[entry.ParentProcessID], entry.ProcessID)
		err = windows.Process32Next(snapshot, &entry)
		if err == nil {
			continue
		}
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			break
		}
		return nil, err
	}
	if !present {
		return nil, nil
	}

	orderedPIDs := make([]uint32, 0)
	visited := make(map[uint32]bool)
	var visit func(uint32)
	visit = func(pid uint32) {
		if visited[pid] {
			return
		}
		visited[pid] = true
		for _, child := range children[pid] {
			visit(child)
		}
		orderedPIDs = append(orderedPIDs, pid)
	}
	visit(uint32(rootPID))

	tree := make([]windowsTreeProcess, 0, len(orderedPIDs))
	for _, pid := range orderedPIDs {
		info, err := inspectWindowsProcess(int(pid))
		if err != nil {
			if isWindowsProcessMissing(err) {
				continue
			}
			return nil, fmt.Errorf("inspect managed process-tree PID %d: %w", pid, err)
		}
		if !info.Running {
			continue
		}
		tree = append(tree, windowsTreeProcess{pid: int(pid), creationTime: info.CreationTime})
	}
	return tree, nil
}

func terminateWindowsSnapshotProcess(ctx context.Context, process windowsTreeProcess) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(process.pid))
	if err != nil {
		if isWindowsProcessMissing(err) {
			return nil
		}
		return fmt.Errorf("open managed process-tree PID %d for termination: %w", process.pid, err)
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return err
	}
	if exitCode != stillActive {
		return nil
	}
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return err
	}
	creationTime := uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime)
	if creationTime != process.creationTime {
		return fmt.Errorf("refusing to terminate PID %d because its creation time changed", process.pid)
	}
	if err := windows.TerminateProcess(handle, windowsTerminationCode); err != nil {
		return fmt.Errorf("terminate managed process-tree PID %d: %w", process.pid, err)
	}
	return waitForWindowsProcessExit(ctx, handle, process.pid)
}

func waitForWindowsProcessExit(ctx context.Context, handle windows.Handle, pid int) error {
	for {
		status, err := windows.WaitForSingleObject(handle, 50)
		if err != nil {
			return err
		}
		switch status {
		case windows.WAIT_OBJECT_0:
			return nil
		case windowsWaitTimeout:
			select {
			case <-ctx.Done():
				return fmt.Errorf("wait for managed process-tree PID %d: %w", pid, ctx.Err())
			default:
			}
		default:
			return fmt.Errorf("wait for managed process-tree PID %d returned status %d", pid, status)
		}
	}
}

func sameWindowsExecutable(left, right string) bool {
	return strings.EqualFold(canonicalWindowsPath(left), canonicalWindowsPath(right))
}

func canonicalWindowsPath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if strings.HasPrefix(path, `\\?\UNC\`) {
		path = `\\` + strings.TrimPrefix(path, `\\?\UNC\`)
	} else {
		path = strings.TrimPrefix(path, `\\?\`)
	}
	return filepath.Clean(path)
}

func isWindowsProcessMissing(err error) bool {
	return errors.Is(err, windows.ERROR_INVALID_PARAMETER) || errors.Is(err, windows.ERROR_NOT_FOUND)
}
