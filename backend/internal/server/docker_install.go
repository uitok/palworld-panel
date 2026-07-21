package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"palpanel/internal/db"
)

const dockerInstallScriptName = "install-docker.sh"

var (
	dockerSourceProbe = probeDockerSource
)

type HostCapabilities struct {
	OS                       string           `json:"os"`
	Arch                     string           `json:"arch"`
	DistroID                 string           `json:"distro_id,omitempty"`
	DistroName               string           `json:"distro_name,omitempty"`
	DistroVersion            string           `json:"distro_version,omitempty"`
	DistroCodename           string           `json:"distro_codename,omitempty"`
	PackageManager           string           `json:"package_manager,omitempty"`
	Systemd                  bool             `json:"systemd"`
	Supported                bool             `json:"supported"`
	UnsupportedReason        string           `json:"unsupported_reason,omitempty"`
	RecommendedRuntime       string           `json:"recommended_runtime"`
	Docker                   DockerCapability `json:"docker"`
	Sudo                     SudoCapability   `json:"sudo"`
	CurrentUser              string           `json:"current_user,omitempty"`
	CurrentUserInDockerGroup bool             `json:"current_user_in_docker_group"`
	Warnings                 []string         `json:"warnings,omitempty"`
}

type DockerCapability struct {
	CLIInstalled    bool   `json:"cli_installed"`
	CLIPath         string `json:"cli_path,omitempty"`
	DaemonReachable bool   `json:"daemon_reachable"`
	Version         string `json:"version,omitempty"`
	Error           string `json:"error,omitempty"`
}

type SudoCapability struct {
	IsRoot        bool   `json:"is_root"`
	SudoInstalled bool   `json:"sudo_installed"`
	Passwordless  bool   `json:"passwordless"`
	CanElevate    bool   `json:"can_elevate"`
	NeedsPassword bool   `json:"needs_password"`
	Message       string `json:"message,omitempty"`
}

type DockerInstallSource struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	ProbeURL  string `json:"probe_url"`
	Available bool   `json:"available"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
	Selected  bool   `json:"selected"`
}

type DockerInstallPlan struct {
	Host            HostCapabilities      `json:"host"`
	Source          string                `json:"source"`
	SourceURL       string                `json:"source_url,omitempty"`
	Sources         []DockerInstallSource `json:"sources"`
	Supported       bool                  `json:"supported"`
	CanAutoInstall  bool                  `json:"can_auto_install"`
	RequiresManual  bool                  `json:"requires_manual"`
	DockerInstalled bool                  `json:"docker_installed"`
	DockerReady     bool                  `json:"docker_ready"`
	ErrorCode       string                `json:"error_code,omitempty"`
	Message         string                `json:"message,omitempty"`
	ManualCommand   string                `json:"manual_command,omitempty"`
	Script          string                `json:"script,omitempty"`
	ScriptPath      string                `json:"script_path,omitempty"`
	Warnings        []string              `json:"warnings,omitempty"`
}

type DockerInstallRequest struct {
	Source                      string `json:"source"`
	AddCurrentUserToDockerGroup bool   `json:"add_current_user_to_docker_group"`
}

type DockerInstallError struct {
	Code   string
	Status int
	Msg    string
}

func (e *DockerInstallError) Error() string {
	return e.Msg
}

func dockerInstallErr(status int, code, msg string) *DockerInstallError {
	return &DockerInstallError{Code: code, Status: status, Msg: msg}
}

func RecommendedRuntimeForOS(goos string) string {
	if goos == "windows" {
		return RuntimeWindowsSteamCMD
	}
	if goos == "linux" {
		return RuntimeLinuxSteamCMD
	}
	return RuntimeWineDocker
}

func (m Manager) HostCapabilities(ctx context.Context) HostCapabilities {
	host := HostCapabilities{
		OS:                 runtime.GOOS,
		Arch:               runtime.GOARCH,
		RecommendedRuntime: RecommendedRuntimeForOS(runtime.GOOS),
		CurrentUser:        currentUsername(),
	}
	host.Supported = host.OS == "linux" || host.OS == "windows"
	if !host.Supported {
		host.UnsupportedReason = "unsupported_os"
	}
	if host.OS == "linux" {
		release := readOSRelease("/etc/os-release")
		host.DistroID = release["ID"]
		host.DistroName = release["NAME"]
		host.DistroVersion = release["VERSION_ID"]
		host.DistroCodename = release["VERSION_CODENAME"]
		if host.DistroCodename == "" {
			host.DistroCodename = release["UBUNTU_CODENAME"]
		}
		host.PackageManager = detectPackageManager(host.DistroID)
		host.Systemd = detectSystemd()
		host.CurrentUserInDockerGroup = currentUserInGroup("docker")
	} else if host.OS == "windows" {
		host.Systemd = false
	}
	host.Docker = detectDocker(ctx, m.cfg.DockerBinary)
	host.Sudo = detectSudo(ctx, host.OS)
	if host.OS == "linux" && host.PackageManager == "" {
		host.Warnings = append(host.Warnings, "unsupported Linux package manager")
	}
	if host.OS == "linux" && !host.Systemd {
		host.Warnings = append(host.Warnings, "systemd is not available; Docker service startup must be handled manually")
	}
	if host.OS == "linux" && host.Docker.CLIInstalled && !host.Docker.DaemonReachable && host.Docker.Error != "" {
		host.Warnings = append(host.Warnings, host.Docker.Error)
	}
	return host
}

func (m Manager) DockerInstallPlan(ctx context.Context, source string) (DockerInstallPlan, error) {
	host := m.HostCapabilities(ctx)
	selected, sources := selectDockerInstallSource(ctx, host, source)
	supported, code, msg := dockerInstallSupport(host)
	warnings := append([]string{}, host.Warnings...)
	if selected.Error != "" && normalizeDockerSource(source) == "auto" {
		warnings = append(warnings, "Docker source probe fell back to "+selected.ID+": "+selected.Error)
	}
	if host.OS == "linux" {
		warnings = append(warnings, "Adding a user to the docker group grants root-equivalent host control. Re-login or restart the PalPanel backend for new group membership to take effect.")
	}

	script := ""
	scriptPath := ""
	manualCommand := ""
	if host.OS == "linux" && selected.ID != "" {
		script = buildDockerInstallScript(host, selected)
		path, err := m.writeDockerInstallScript(script)
		if err == nil {
			scriptPath = path
		} else {
			warnings = append(warnings, "failed to write manual script: "+err.Error())
		}
		if scriptPath != "" {
			manualCommand = "sudo bash " + shellQuote(scriptPath)
		}
	}

	plan := DockerInstallPlan{
		Host:            host,
		Source:          selected.ID,
		SourceURL:       selected.URL,
		Sources:         sources,
		Supported:       supported,
		CanAutoInstall:  supported && host.Sudo.CanElevate,
		RequiresManual:  supported && !host.Sudo.CanElevate,
		DockerInstalled: host.Docker.CLIInstalled,
		DockerReady:     host.Docker.CLIInstalled && host.Docker.DaemonReachable,
		ErrorCode:       code,
		Message:         msg,
		ManualCommand:   manualCommand,
		Script:          script,
		ScriptPath:      scriptPath,
		Warnings:        warnings,
	}
	if plan.DockerReady {
		plan.Supported = true
		plan.ErrorCode = ""
		plan.Message = "Docker Engine is ready"
		plan.CanAutoInstall = false
		plan.RequiresManual = false
	}
	if plan.RequiresManual && plan.Message == "" {
		plan.Message = "root or passwordless sudo is required for automatic Docker installation"
		plan.ErrorCode = "sudo_required"
	}
	return plan, nil
}

func (m Manager) InstallDocker(ctx context.Context, req DockerInstallRequest) (db.Job, error) {
	plan, err := m.DockerInstallPlan(ctx, req.Source)
	if err != nil {
		return db.Job{}, err
	}
	if plan.DockerReady {
		return db.Job{}, dockerInstallErr(http.StatusConflict, "docker_ready", "Docker Engine is already reachable")
	}
	if !plan.Supported {
		if plan.ErrorCode == "" {
			plan.ErrorCode = "docker_install_unsupported"
		}
		if plan.Message == "" {
			plan.Message = "Docker automatic installation is not supported on this host"
		}
		return db.Job{}, dockerInstallErr(http.StatusBadRequest, plan.ErrorCode, plan.Message)
	}
	if !plan.Host.Sudo.CanElevate {
		return db.Job{}, dockerInstallErr(http.StatusConflict, "sudo_required", "Docker installation requires root or passwordless sudo; run the manual script as an administrator")
	}

	return m.startJob(ctx, "docker_install", "queued Docker Engine install", func(jobCtx context.Context, jobID string) {
		m.update(jobID, "running", 10, "probing Docker package source", "")
		plan, err := m.DockerInstallPlan(jobCtx, req.Source)
		if err != nil {
			m.update(jobID, "failed", 10, "Docker install planning failed", err.Error())
			return
		}
		if !plan.Supported {
			m.update(jobID, "failed", 10, "Docker install unsupported", plan.Message)
			return
		}
		if !plan.Host.Sudo.CanElevate {
			m.update(jobID, "failed", 10, "Docker install requires manual sudo", "run the manual script as root or with sudo")
			return
		}

		m.update(jobID, "running", 20, "writing Docker install script", "")
		scriptPath, err := m.writeDockerInstallScript(plan.Script)
		if err != nil {
			m.update(jobID, "failed", 20, "script write failed", err.Error())
			return
		}

		m.update(jobID, "running", 35, "configuring repository and installing Docker packages", "")
		if err := runDockerInstallScript(jobCtx, scriptPath, plan.Host.Sudo.IsRoot, req.AddCurrentUserToDockerGroup); err != nil {
			m.update(jobID, "failed", 35, "Docker package installation failed", err.Error())
			return
		}

		m.update(jobID, "running", 80, "starting Docker service", "")
		_ = tryStartDockerService(jobCtx, plan.Host.Sudo.IsRoot)

		m.update(jobID, "running", 90, "verifying Docker daemon", "")
		host := m.HostCapabilities(jobCtx)
		if host.Docker.DaemonReachable {
			m.update(jobID, "completed", 100, "Docker Engine installed and reachable", "")
			return
		}
		if sudoDockerVersion(jobCtx) == nil {
			m.update(jobID, "completed", 100, "Docker Engine installed; restart PalPanel backend or re-login to refresh docker group access", "")
			return
		}
		errText := host.Docker.Error
		if errText == "" {
			errText = "docker daemon is not reachable"
		}
		m.update(jobID, "failed", 90, "Docker verification failed", errText)
	})
}

func (m Manager) writeDockerInstallScript(script string) (string, error) {
	if strings.TrimSpace(script) == "" {
		return "", errors.New("empty Docker install script")
	}
	if err := os.MkdirAll(m.cfg.ToolsDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(m.cfg.ToolsDir, dockerInstallScriptName)
	if err := os.WriteFile(path, []byte(script), 0o750); err != nil {
		return "", err
	}
	return path, nil
}

type dockerSourceDefinition struct {
	ID      string
	Name    string
	BaseURL string
}

var dockerSourceDefinitions = []dockerSourceDefinition{
	{ID: "official", Name: "Docker Official", BaseURL: "https://download.docker.com/linux"},
	{ID: "aliyun", Name: "Aliyun Docker CE Mirror", BaseURL: "https://mirrors.aliyun.com/docker-ce/linux"},
	{ID: "azurecn", Name: "Azure China Docker CE Mirror", BaseURL: "https://mirror.azure.cn/docker-ce/linux"},
}

func selectDockerInstallSource(ctx context.Context, host HostCapabilities, requested string) (DockerInstallSource, []DockerInstallSource) {
	requested = normalizeDockerSource(requested)
	repoDistro, ok := dockerRepoDistro(host)
	if !ok {
		return DockerInstallSource{}, nil
	}
	sources := make([]DockerInstallSource, 0, len(dockerSourceDefinitions))
	for _, def := range dockerSourceDefinitions {
		repoURL := strings.TrimRight(def.BaseURL, "/") + "/" + repoDistro
		sources = append(sources, DockerInstallSource{
			ID:       def.ID,
			Name:     def.Name,
			URL:      repoURL,
			ProbeURL: dockerProbeURL(host.PackageManager, repoURL),
		})
	}
	var wg sync.WaitGroup
	for i := range sources {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result := dockerSourceProbe(ctx, sources[index].ProbeURL)
			sources[index].Available = result.Available
			sources[index].LatencyMS = result.Latency.Milliseconds()
			sources[index].Error = result.Error
		}(i)
	}
	wg.Wait()

	selectedIndex := -1
	if requested != "auto" {
		for i := range sources {
			if sources[i].ID == requested {
				selectedIndex = i
				break
			}
		}
	}
	if selectedIndex == -1 {
		available := make([]int, 0, len(sources))
		for i, source := range sources {
			if source.Available {
				available = append(available, i)
			}
		}
		if len(available) > 0 {
			sort.SliceStable(available, func(i, j int) bool {
				return sources[available[i]].LatencyMS < sources[available[j]].LatencyMS
			})
			selectedIndex = available[0]
		}
	}
	if selectedIndex == -1 {
		for i := range sources {
			if sources[i].ID == "official" {
				selectedIndex = i
				break
			}
		}
	}
	if selectedIndex >= 0 {
		sources[selectedIndex].Selected = true
		return sources[selectedIndex], sources
	}
	return DockerInstallSource{}, sources
}

func normalizeDockerSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	switch source {
	case "official", "aliyun", "azurecn":
		return source
	default:
		return "auto"
	}
}

type sourceProbeResult struct {
	Available bool
	Latency   time.Duration
	Error     string
}

func probeDockerSource(ctx context.Context, rawURL string) sourceProbeResult {
	if rawURL == "" {
		return sourceProbeResult{Error: "empty probe URL"}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	started := time.Now()
	status, err := probeURL(probeCtx, http.MethodHead, rawURL)
	if err == nil && status == http.StatusMethodNotAllowed {
		status, err = probeURL(probeCtx, http.MethodGet, rawURL)
	}
	latency := time.Since(started)
	if err != nil {
		return sourceProbeResult{Latency: latency, Error: err.Error()}
	}
	if status >= 200 && status < 400 {
		return sourceProbeResult{Available: true, Latency: latency}
	}
	return sourceProbeResult{Latency: latency, Error: fmt.Sprintf("HTTP %d", status)}
}

func probeURL(ctx context.Context, method, rawURL string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return 0, err
	}
	if method == http.MethodGet {
		req.Header.Set("Range", "bytes=0-0")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 256))
	return resp.StatusCode, nil
}

func dockerProbeURL(packageManager, repoURL string) string {
	if packageManager == "apt" {
		return strings.TrimRight(repoURL, "/") + "/gpg"
	}
	return strings.TrimRight(repoURL, "/") + "/docker-ce.repo"
}

func dockerRepoDistro(host HostCapabilities) (string, bool) {
	if host.OS != "linux" {
		return "", false
	}
	switch strings.ToLower(host.DistroID) {
	case "debian", "ubuntu", "fedora", "rhel", "centos":
		return strings.ToLower(host.DistroID), true
	case "rocky", "almalinux", "alma":
		return "centos", true
	default:
		return "", false
	}
}

func dockerInstallSupport(host HostCapabilities) (bool, string, string) {
	if host.OS != "linux" {
		return false, "unsupported_os", "Docker one-click installation is only supported on Linux hosts"
	}
	if _, ok := dockerRepoDistro(host); !ok {
		return false, "unsupported_distribution", "this Linux distribution is not supported by the Docker installer"
	}
	if host.PackageManager != "apt" && host.PackageManager != "dnf" && host.PackageManager != "yum" {
		return false, "unsupported_package_manager", "apt, dnf, or yum is required for Docker package installation"
	}
	if !host.Systemd {
		return false, "no_systemd", "systemd is required so PalPanel can enable and start the Docker service"
	}
	return true, "", ""
}

func buildDockerInstallScript(host HostCapabilities, source DockerInstallSource) string {
	repoDistro, _ := dockerRepoDistro(host)
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("set -euo pipefail\n\n")
	b.WriteString("DOCKER_REPO_URL=" + shellQuote(source.URL) + "\n")
	b.WriteString("DOCKER_REPO_DISTRO=" + shellQuote(repoDistro) + "\n")
	b.WriteString("DOCKER_SOURCE_ID=" + shellQuote(source.ID) + "\n")
	b.WriteString("ADD_CURRENT_USER_TO_DOCKER_GROUP=\"${ADD_CURRENT_USER_TO_DOCKER_GROUP:-0}\"\n")
	b.WriteString("TARGET_DOCKER_USER=\"${TARGET_DOCKER_USER:-${SUDO_USER:-${USER:-}}}\"\n\n")
	b.WriteString("log() { printf '\\n[palpanel] %s\\n' \"$*\"; }\n")
	b.WriteString("require_root() {\n")
	b.WriteString("  if [ \"$(id -u)\" -ne 0 ]; then\n")
	b.WriteString("    echo \"Run this script as root or with sudo. PalPanel does not store sudo passwords.\" >&2\n")
	b.WriteString("    exit 1\n")
	b.WriteString("  fi\n")
	b.WriteString("}\n\n")
	b.WriteString("detect_os() {\n")
	b.WriteString("  if [ ! -r /etc/os-release ]; then echo \"/etc/os-release not found\" >&2; exit 2; fi\n")
	b.WriteString("  . /etc/os-release\n")
	b.WriteString("  OS_ID=\"${ID:-}\"\n")
	b.WriteString("  OS_CODENAME=\"${VERSION_CODENAME:-${UBUNTU_CODENAME:-}}\"\n")
	b.WriteString("}\n\n")
	b.WriteString("start_docker_service() {\n")
	b.WriteString("  log \"Starting Docker service\"\n")
	b.WriteString("  if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then\n")
	b.WriteString("    systemctl enable --now docker\n")
	b.WriteString("  elif command -v service >/dev/null 2>&1; then\n")
	b.WriteString("    service docker start\n")
	b.WriteString("  else\n")
	b.WriteString("    echo \"No supported service manager found for Docker\" >&2\n")
	b.WriteString("    exit 3\n")
	b.WriteString("  fi\n")
	b.WriteString("}\n\n")
	b.WriteString("maybe_add_docker_group() {\n")
	b.WriteString("  if [ \"$ADD_CURRENT_USER_TO_DOCKER_GROUP\" != \"1\" ]; then return 0; fi\n")
	b.WriteString("  log \"Adding user to docker group\"\n")
	b.WriteString("  groupadd -f docker\n")
	b.WriteString("  if [ -n \"$TARGET_DOCKER_USER\" ] && id \"$TARGET_DOCKER_USER\" >/dev/null 2>&1; then\n")
	b.WriteString("    usermod -aG docker \"$TARGET_DOCKER_USER\"\n")
	b.WriteString("    echo \"User $TARGET_DOCKER_USER was added to docker group. Re-login or restart PalPanel backend before using Docker without sudo.\"\n")
	b.WriteString("  else\n")
	b.WriteString("    echo \"TARGET_DOCKER_USER is empty or does not exist; skipped docker group membership.\"\n")
	b.WriteString("  fi\n")
	b.WriteString("}\n\n")
	b.WriteString("verify_docker() {\n")
	b.WriteString("  log \"Verifying Docker\"\n")
	b.WriteString("  docker version\n")
	b.WriteString("}\n\n")
	b.WriteString("install_apt() {\n")
	b.WriteString("  detect_os\n")
	b.WriteString("  if [ \"$OS_ID\" != \"debian\" ] && [ \"$OS_ID\" != \"ubuntu\" ]; then echo \"apt path only supports Debian/Ubuntu\" >&2; exit 4; fi\n")
	b.WriteString("  if [ -z \"$OS_CODENAME\" ]; then echo \"VERSION_CODENAME is required for Docker apt repository\" >&2; exit 4; fi\n")
	b.WriteString("  log \"Installing apt prerequisites\"\n")
	b.WriteString("  apt-get update\n")
	b.WriteString("  apt-get install -y ca-certificates curl gnupg\n")
	b.WriteString("  install -m 0755 -d /etc/apt/keyrings\n")
	b.WriteString("  curl -fsSL \"$DOCKER_REPO_URL/gpg\" -o /etc/apt/keyrings/docker.asc\n")
	b.WriteString("  chmod a+r /etc/apt/keyrings/docker.asc\n")
	b.WriteString("  echo \"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] $DOCKER_REPO_URL $OS_CODENAME stable\" > /etc/apt/sources.list.d/docker.list\n")
	b.WriteString("  log \"Installing Docker Engine packages\"\n")
	b.WriteString("  apt-get update\n")
	b.WriteString("  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin\n")
	b.WriteString("}\n\n")
	b.WriteString("install_rpm() {\n")
	b.WriteString("  local pm=\"\"\n")
	b.WriteString("  if command -v dnf >/dev/null 2>&1; then pm=dnf; elif command -v yum >/dev/null 2>&1; then pm=yum; else echo \"dnf or yum is required\" >&2; exit 5; fi\n")
	b.WriteString("  log \"Installing rpm prerequisites\"\n")
	b.WriteString("  \"$pm\" -y install ca-certificates curl\n")
	b.WriteString("  install -m 0755 -d /etc/yum.repos.d\n")
	b.WriteString("  curl -fsSL \"$DOCKER_REPO_URL/docker-ce.repo\" -o /etc/yum.repos.d/docker-ce.repo\n")
	b.WriteString("  log \"Installing Docker Engine packages\"\n")
	b.WriteString("  \"$pm\" -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin\n")
	b.WriteString("}\n\n")
	b.WriteString("main() {\n")
	b.WriteString("  require_root\n")
	b.WriteString("  log \"Using Docker source: $DOCKER_SOURCE_ID ($DOCKER_REPO_URL)\"\n")
	b.WriteString("  if command -v docker >/dev/null 2>&1; then\n")
	b.WriteString("    log \"Docker CLI already exists; attempting service start\"\n")
	b.WriteString("    start_docker_service || true\n")
	b.WriteString("    maybe_add_docker_group\n")
	b.WriteString("    verify_docker\n")
	b.WriteString("    exit 0\n")
	b.WriteString("  fi\n")
	if host.PackageManager == "apt" {
		b.WriteString("  install_apt\n")
	} else {
		b.WriteString("  install_rpm\n")
	}
	b.WriteString("  start_docker_service\n")
	b.WriteString("  maybe_add_docker_group\n")
	b.WriteString("  verify_docker\n")
	b.WriteString("}\n\n")
	b.WriteString("main \"$@\"\n")
	return b.String()
}

func runDockerInstallScript(ctx context.Context, scriptPath string, isRoot bool, addUser bool) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	args := []string{scriptPath}
	binary := "bash"
	if !isRoot {
		args = []string{"-n", "bash", scriptPath}
		binary = "sudo"
	}
	cmd := exec.CommandContext(timeoutCtx, binary, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("ADD_CURRENT_USER_TO_DOCKER_GROUP=%d", boolInt(addUser)))
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w: %s", binary, err, limitString(strings.TrimSpace(out.String()), 4000))
	}
	return nil
}

func tryStartDockerService(ctx context.Context, isRoot bool) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	script := "if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then systemctl enable --now docker; elif command -v service >/dev/null 2>&1; then service docker start; else exit 1; fi"
	binary := "bash"
	args := []string{"-lc", script}
	if !isRoot {
		binary = "sudo"
		args = []string{"-n", "bash", "-lc", script}
	}
	cmd := exec.CommandContext(timeoutCtx, binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("start docker service failed: %w: %s", err, limitString(strings.TrimSpace(string(out)), 1000))
	}
	return nil
}

func sudoDockerVersion(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, "sudo", "-n", "docker", "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo docker version failed: %w: %s", err, limitString(strings.TrimSpace(string(out)), 1000))
	}
	return nil
}

func detectDocker(ctx context.Context, configured string) DockerCapability {
	path, ok := resolveDockerBinary(configured)
	docker := DockerCapability{CLIInstalled: ok, CLIPath: path}
	if !ok {
		docker.Error = "Docker CLI not found"
		return docker
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, path, "version", "--format", "{{.Server.Version}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if dockerDetectPermissionDenied(err, out) && currentUserInGroup("docker") && commandExists("sg") {
			sgCmd := exec.CommandContext(timeoutCtx, "sg", "docker", "-c", shellQuote(path)+" version --format '{{.Server.Version}}'")
			sgOut, sgErr := sgCmd.CombinedOutput()
			if sgErr == nil {
				docker.DaemonReachable = true
				docker.Version = strings.TrimSpace(string(sgOut))
				return docker
			}
		}
		docker.Error = "docker version failed: " + limitString(strings.TrimSpace(string(out)), 1000)
		if docker.Error == "docker version failed: " {
			docker.Error = err.Error()
		}
		return docker
	}
	docker.DaemonReachable = true
	docker.Version = strings.TrimSpace(string(out))
	return docker
}

func dockerDetectPermissionDenied(err error, out []byte) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error() + " " + string(out))
	return strings.Contains(msg, "permission denied") &&
		(strings.Contains(msg, "docker.sock") || strings.Contains(msg, "docker api") || strings.Contains(msg, "var/run/docker"))
}

func resolveDockerBinary(configured string) (string, bool) {
	if configured == "" {
		configured = "docker"
	}
	if resolved, err := exec.LookPath(configured); err == nil {
		return resolved, true
	}
	if strings.ContainsAny(configured, `\/`) {
		if fileExists(configured) {
			return configured, true
		}
		return configured, false
	}
	defaultWindowsPath := `C:\Program Files\Docker\Docker\resources\bin\docker.exe`
	if runtime.GOOS == "windows" && fileExists(defaultWindowsPath) {
		return defaultWindowsPath, true
	}
	return configured, false
}

func detectSudo(ctx context.Context, goos string) SudoCapability {
	if goos != "linux" {
		return SudoCapability{Message: "sudo is not used on this host"}
	}
	isRoot := os.Geteuid() == 0
	if isRoot {
		return sudoCapabilityFromProbe(true, true, nil, "")
	}
	_, err := exec.LookPath("sudo")
	if err != nil {
		return sudoCapabilityFromProbe(false, false, err, "")
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, "sudo", "-n", "true")
	out, runErr := cmd.CombinedOutput()
	return sudoCapabilityFromProbe(false, true, runErr, string(out))
}

func sudoCapabilityFromProbe(isRoot bool, sudoInstalled bool, probeErr error, output string) SudoCapability {
	if isRoot {
		return SudoCapability{IsRoot: true, CanElevate: true, Message: "running as root"}
	}
	if !sudoInstalled {
		return SudoCapability{SudoInstalled: false, Message: "sudo is not installed"}
	}
	if probeErr == nil {
		return SudoCapability{SudoInstalled: true, Passwordless: true, CanElevate: true, Message: "passwordless sudo is available"}
	}
	lower := strings.ToLower(output + " " + probeErr.Error())
	needsPassword := strings.Contains(lower, "password") || strings.Contains(lower, "sudo")
	message := strings.TrimSpace(output)
	if message == "" {
		message = probeErr.Error()
	}
	return SudoCapability{
		SudoInstalled: true,
		NeedsPassword: needsPassword,
		Message:       "sudo -n true failed: " + limitString(message, 1000),
	}
}

func detectPackageManager(distroID string) string {
	distroID = strings.ToLower(distroID)
	if (distroID == "debian" || distroID == "ubuntu") && commandExists("apt-get") {
		return "apt"
	}
	if commandExists("dnf") {
		return "dnf"
	}
	if commandExists("yum") {
		return "yum"
	}
	if commandExists("apt-get") {
		return "apt"
	}
	return ""
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func detectSystemd() bool {
	return commandExists("systemctl") && dirExists("/run/systemd/system")
}

func readOSRelease(path string) map[string]string {
	body, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		} else {
			value = strings.Trim(value, `"'`)
		}
		out[strings.TrimSpace(key)] = value
	}
	return out
}

func currentUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if v := os.Getenv("SUDO_USER"); v != "" {
		return v
	}
	if v := os.Getenv("USER"); v != "" {
		return v
	}
	return ""
}

func currentUserInGroup(groupName string) bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	group, err := user.LookupGroup(groupName)
	if err != nil {
		return false
	}
	ids, err := u.GroupIds()
	if err != nil {
		return false
	}
	for _, gid := range ids {
		if gid == group.Gid {
			return true
		}
	}
	return false
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func limitString(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}
