//go:build linux

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"palpanel/internal/docker"
)

const kvLinuxProcess = "linux_process"

type linuxProcessRecord struct {
	PID        int    `json:"pid"`
	Executable string `json:"executable"`
	StartTime  string `json:"start_time"`
}

func (m Manager) startLinux(ctx context.Context, args []string) error {
	if err := m.validateLinuxServerInstall(); err != nil {
		return err
	}
	if status, _ := m.linuxStatus(ctx); status.Status == "running" {
		return fmt.Errorf("PalServer is already running")
	}
	if err := os.MkdirAll(filepath.Dir(m.cfg.ServerLogPath()), 0o755); err != nil {
		return err
	}
	logFile, err := newRollingLogWriter(m.cfg.ServerLogPath(), 20*1024*1024, 5)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(logFile, "%s [palpanel] starting native Linux PalServer\n", time.Now().UTC().Format(time.RFC3339Nano))
	command := m.cfg.PalServerLinuxPath()
	commandArgs := args
	ue4ssPath := m.cfg.LinuxUE4SSPath()
	if info, statErr := os.Stat(ue4ssPath); statErr == nil && !info.IsDir() {
		command, err = m.linuxServerBinaryPath()
		if err != nil {
			_ = logFile.Close()
			return err
		}
		commandArgs = append([]string{"Pal"}, args...)
	}
	cmd := exec.Command(command, commandArgs...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = m.cfg.ServerDirectory(), logFile, logFile
	if command != m.cfg.PalServerLinuxPath() {
		cmd.Env = append(os.Environ(), "LD_PRELOAD="+ue4ssPath)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	record := linuxProcessRecord{PID: cmd.Process.Pid, Executable: m.cfg.PalServerLinuxPath(), StartTime: linuxProcessStartTime(cmd.Process.Pid)}
	raw, _ := json.Marshal(record)
	if err := m.store.SetKV(ctx, kvLinuxProcess, string(raw)); err != nil {
		_ = syscall.Kill(-record.PID, syscall.SIGKILL)
		_ = logFile.Close()
		return err
	}
	go func() {
		err := cmd.Wait()
		_, _ = fmt.Fprintf(logFile, "%s [palpanel] native Linux PalServer exited: %v\n", time.Now().UTC().Format(time.RFC3339Nano), err)
		_ = logFile.Close()
		current, ok := m.loadLinuxProcess(context.Background())
		if ok && current.PID == record.PID {
			_ = m.store.SetKV(context.Background(), kvLinuxProcess, "")
		}
	}()
	return nil
}

func (m Manager) loadLinuxProcess(ctx context.Context) (linuxProcessRecord, bool) {
	raw, ok, err := m.store.GetKV(ctx, kvLinuxProcess)
	if err != nil || !ok || raw == "" {
		return linuxProcessRecord{}, false
	}
	var record linuxProcessRecord
	if json.Unmarshal([]byte(raw), &record) != nil || record.PID <= 0 {
		return linuxProcessRecord{}, false
	}
	return record, true
}

func linuxProcessRunning(record linuxProcessRecord) bool {
	if syscall.Kill(record.PID, 0) != nil {
		return false
	}
	return record.StartTime == "" || linuxProcessStartTime(record.PID) == record.StartTime
}

func linuxProcessStartTime(pid int) string {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return ""
	}
	text := string(raw)
	end := strings.LastIndex(text, ")")
	if end < 0 {
		return ""
	}
	fields := strings.Fields(text[end+1:])
	if len(fields) <= 19 {
		return ""
	}
	return fields[19]
}

func (m Manager) stopLinux(ctx context.Context) error {
	record, ok := m.loadLinuxProcess(ctx)
	if !ok {
		return nil
	}
	if linuxProcessRunning(record) {
		_ = syscall.Kill(-record.PID, syscall.SIGTERM)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		for linuxProcessRunning(record) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				_ = syscall.Kill(-record.PID, syscall.SIGKILL)
			case <-ticker.C:
			}
		}
	}
	return m.store.SetKV(ctx, kvLinuxProcess, "")
}

func (m Manager) linuxStatus(ctx context.Context) (docker.ContainerStatus, error) {
	record, ok := m.loadLinuxProcess(ctx)
	if !ok {
		return docker.ContainerStatus{Exists: false, Status: "missing"}, nil
	}
	if !linuxProcessRunning(record) {
		_ = m.store.SetKV(ctx, kvLinuxProcess, "")
		return docker.ContainerStatus{Exists: false, Status: "missing"}, nil
	}
	return docker.ContainerStatus{Exists: true, Status: "running"}, nil
}
