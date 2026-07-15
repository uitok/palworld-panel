//go:build !windows

package server

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

type trackedWindowsProcess struct{}

func prepareWindowsProcess(*exec.Cmd) {}

func trackWindowsProcess(*os.Process) (*trackedWindowsProcess, error) {
	return &trackedWindowsProcess{}, nil
}

func finishTrackedWindowsProcess(*trackedWindowsProcess) {}

func inspectWindowsProcess(int) (windowsProcessInfo, error) {
	return windowsProcessInfo{}, errors.New("Windows process inspection is unavailable on this host")
}

func terminateWindowsProcessTree(context.Context, int) error {
	return errors.New("Windows process termination is unavailable on this host")
}

func sameWindowsExecutable(left, right string) bool {
	leftAbs, _ := filepath.Abs(left)
	rightAbs, _ := filepath.Abs(right)
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func isWindowsProcessMissing(error) bool { return false }
