//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"palpanel/internal/appconfig"
	"palpanel/internal/buildinfo"
)

const (
	messageBoxOK             = 0x00000000
	messageBoxYesNo          = 0x00000004
	messageBoxIconInfo       = 0x00000040
	messageBoxIconQuestion   = 0x00000020
	messageBoxIconError      = 0x00000010
	messageBoxSetForeground  = 0x00010000
	messageBoxResultYes      = 6
	processAssignPermissions = windows.PROCESS_SET_QUOTA | windows.PROCESS_TERMINATE
)

var (
	user32         = windows.NewLazySystemDLL("user32.dll")
	messageBoxProc = user32.NewProc("MessageBoxW")
)

type options struct {
	noBrowser       bool
	noPrompt        bool
	exitAfterHealth bool
	showVersion     bool
}

type childProcess struct {
	cmd *exec.Cmd
	log *os.File
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = messageBox("PalPanel failed to start", err.Error(), messageBoxOK|messageBoxIconError|messageBoxSetForeground)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("PalPanel", flag.ContinueOnError)
	opts := options{}
	fs.BoolVar(&opts.noBrowser, "no-browser", false, "do not open the browser")
	fs.BoolVar(&opts.noPrompt, "no-prompt", false, "do not display interactive prompts")
	fs.BoolVar(&opts.exitAfterHealth, "exit-after-health", false, "stop children after smoke-test health checks")
	fs.BoolVar(&opts.showVersion, "version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if opts.showVersion {
		info := buildinfo.Current()
		fmt.Printf("PalPanel Launcher %s (commit %s, built %s)\n", info.Version, info.Commit, info.BuildTime)
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	root := filepath.Dir(executable)
	mutex, alreadyRunning, err := acquireInstanceMutex(root)
	if err != nil {
		return fmt.Errorf("create instance lock: %w", err)
	}
	if alreadyRunning {
		_ = windows.CloseHandle(mutex)
		return errors.New("PalPanel is already running from this directory")
	}
	defer windows.CloseHandle(mutex)

	serverPath := filepath.Join(root, "palpanel-server.exe")
	savPath := filepath.Join(root, "sav-cli.exe")
	for _, path := range []string{serverPath, savPath} {
		if info, statErr := os.Stat(path); statErr != nil || info.IsDir() {
			return fmt.Errorf("required executable is missing: %s", path)
		}
	}
	configPath := filepath.Join(root, "config", "palpanel.env")
	tokenMarkerPath := filepath.Join(root, "config", ".show-admin-token")
	dataPath := filepath.Join(root, "data")
	logsPath := filepath.Join(root, "logs")
	firstRun := false
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		firstRun = true
		if !opts.noPrompt {
			result, boxErr := messageBox(
				"PalPanel portable setup",
				"PalPanel will store configuration and server data next to this program. Continue?",
				messageBoxYesNo|messageBoxIconQuestion|messageBoxSetForeground,
			)
			if boxErr != nil {
				return boxErr
			}
			if result != messageBoxResultYes {
				return nil
			}
		}
		if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
			return err
		}
		output, initErr := exec.Command(serverPath, "--config", configPath, "--init-config").CombinedOutput()
		if initErr != nil {
			return fmt.Errorf("initialize config: %w: %s", initErr, strings.TrimSpace(string(output)))
		}
		if err := os.WriteFile(tokenMarkerPath, []byte("pending\n"), 0o600); err != nil {
			return fmt.Errorf("record first-run token state: %w", err)
		}
	} else if err != nil {
		return err
	}
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(logsPath, 0o700); err != nil {
		return err
	}
	config, err := appconfig.ParseEnvFile(configPath)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	job, err := createKillOnCloseJob()
	if err != nil {
		return fmt.Errorf("create child process job: %w", err)
	}
	defer windows.CloseHandle(job)

	savLog := filepath.Join(logsPath, "sav-cli.log")
	serverLog := filepath.Join(logsPath, "palpanel.log")
	if err := rotateLog(savLog, 10*1024*1024, 5); err != nil {
		return err
	}
	if err := rotateLog(serverLog, 10*1024*1024, 5); err != nil {
		return err
	}
	savChild, err := startChild(job, savPath, []string{"serve", "--host", "127.0.0.1", "--port", "8090"}, nil, savLog)
	if err != nil {
		return fmt.Errorf("start sav-cli: %w", err)
	}
	defer savChild.closeLog()
	if err := waitForHealth("http://127.0.0.1:8090/health", savChild.cmd, 30*time.Second); err != nil {
		return fmt.Errorf("sav-cli health check: %w", err)
	}

	childEnv := map[string]string{
		"PALPANEL_FRONTEND_DIST":        filepath.Join(root, "frontend", "dist"),
		"PALPANEL_BACKEND_DIR":          filepath.Join(root, "backend"),
		"PALPANEL_RUNNER_DIR":           filepath.Join(root, "backend", "deployments", "wine-runner"),
		"PALPANEL_DATA_DIR":             dataPath,
		"PALPANEL_SAVE_INDEXER_ENABLED": "true",
		"PALPANEL_SAVE_INDEXER_URL":     "http://127.0.0.1:8090",
	}
	serverChild, err := startChild(job, serverPath, []string{"--config", configPath}, childEnv, serverLog)
	if err != nil {
		return fmt.Errorf("start palpanel server: %w", err)
	}
	defer serverChild.closeLog()
	healthURL, dashboardURL := panelURLs(config)
	if err := waitForHealth(healthURL, serverChild.cmd, 45*time.Second); err != nil {
		return fmt.Errorf("palpanel health check: %w", err)
	}

	stopRotation := make(chan struct{})
	defer close(stopRotation)
	go rotateWhileRunning(stopRotation, []string{savLog, serverLog})
	if !opts.noBrowser {
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", dashboardURL).Start()
	}
	showFirstToken := firstRun
	if _, err := os.Stat(tokenMarkerPath); err == nil {
		showFirstToken = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if showFirstToken && !opts.noPrompt {
		token := strings.TrimSpace(os.Getenv("PANEL_TOKEN"))
		if token == "" {
			token = strings.TrimSpace(config["PANEL_TOKEN"])
		}
		if _, err := messageBox(
			"PalPanel is ready",
			fmt.Sprintf("Dashboard: %s\n\nAdmin token (shown on first launch):\n%s\n\nChoose OK to stop PalPanel.", dashboardURL, token),
			messageBoxOK|messageBoxIconInfo|messageBoxSetForeground,
		); err != nil {
			return err
		}
		_ = os.Remove(tokenMarkerPath)
		return nil
	}
	if opts.exitAfterHealth {
		return nil
	}
	if !opts.noPrompt {
		if _, err := messageBox("PalPanel is running", "The dashboard is ready. Choose OK to stop PalPanel.", messageBoxOK|messageBoxIconInfo|messageBoxSetForeground); err != nil {
			return err
		}
		return nil
	}
	return waitForEitherChild(savChild.cmd, serverChild.cmd)
}

func acquireInstanceMutex(root string) (windows.Handle, bool, error) {
	hash := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(root))))
	name, err := windows.UTF16PtrFromString(fmt.Sprintf("Local\\PalPanelLauncher_%x", hash[:12]))
	if err != nil {
		return 0, false, err
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return handle, true, nil
	}
	return handle, false, err
}

func createKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func startChild(job windows.Handle, path string, args []string, overrides map[string]string, logPath string) (*childProcess, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = filepath.Dir(path)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = mergeEnvironment(os.Environ(), overrides)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	process, err := windows.OpenProcess(processAssignPermissions, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = cmd.Process.Kill()
		_ = logFile.Close()
		return nil, err
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		_ = cmd.Process.Kill()
		_ = logFile.Close()
		return nil, err
	}
	return &childProcess{cmd: cmd, log: logFile}, nil
}

func (c *childProcess) closeLog() {
	if c != nil && c.log != nil {
		_ = c.log.Close()
	}
}

func mergeEnvironment(current []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return current
	}
	result := make([]string, 0, len(current)+len(overrides))
	for _, item := range current {
		name, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		matched := false
		for override := range overrides {
			if strings.EqualFold(name, override) {
				matched = true
				break
			}
		}
		if !matched {
			result = append(result, item)
		}
	}
	for name, value := range overrides {
		result = append(result, name+"="+value)
	}
	return result
}

func waitForHealth(url string, command *exec.Cmd, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Second}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if command.ProcessState != nil && command.ProcessState.Exited() {
				return errors.New("process exited before becoming healthy")
			}
			request, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			response, err := client.Do(request)
			if err == nil {
				_ = response.Body.Close()
				if response.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
	}
}

func panelURLs(config map[string]string) (string, string) {
	listen := strings.TrimSpace(os.Getenv("PALPANEL_LISTEN_ADDR"))
	if listen == "" {
		listen = strings.TrimSpace(config["PALPANEL_LISTEN_ADDR"])
	}
	if listen == "" {
		listen = "127.0.0.1:8080"
	}
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		port = "8080"
	}
	if _, err := strconv.Atoi(port); err != nil {
		port = "8080"
	}
	base := "http://127.0.0.1:" + port
	return base + "/api/health", base + "/dashboard"
}

func rotateWhileRunning(stop <-chan struct{}, paths []string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			for _, path := range paths {
				_ = rotateLog(path, 10*1024*1024, 5)
			}
		}
	}
}

func rotateLog(path string, maxBytes int64, backups int) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) || (err == nil && info.Size() < maxBytes) {
		return nil
	}
	if err != nil {
		return err
	}
	for index := backups; index >= 2; index-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", path, index-1), fmt.Sprintf("%s.%d", path, index))
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".1", content, 0o600); err != nil {
		return err
	}
	return os.Truncate(path, 0)
}

func waitForEitherChild(commands ...*exec.Cmd) error {
	result := make(chan error, len(commands))
	for _, command := range commands {
		go func(cmd *exec.Cmd) { result <- cmd.Wait() }(command)
	}
	err := <-result
	if err == nil {
		return errors.New("a managed process exited")
	}
	return fmt.Errorf("a managed process exited: %w", err)
}

func messageBox(title, message string, flags uintptr) (int, error) {
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return 0, err
	}
	messagePtr, err := windows.UTF16PtrFromString(message)
	if err != nil {
		return 0, err
	}
	result, _, callErr := messageBoxProc.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), flags)
	if result == 0 {
		return 0, callErr
	}
	return int(result), nil
}
