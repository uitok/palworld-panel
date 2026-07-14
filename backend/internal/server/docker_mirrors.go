package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"palpanel/internal/db"
)

const (
	dockerMirrorScriptName = "configure-docker-mirrors.sh"
	dockerDaemonConfigPath = "/etc/docker/daemon.json"
)

var dockerRegistryMirrorProbe = probeDockerRegistryMirror

type DockerRegistryMirror struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	ProbeURL  string `json:"probe_url"`
	Available bool   `json:"available"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
	Selected  bool   `json:"selected"`
}

type DockerMirrorPlan struct {
	Host             HostCapabilities       `json:"host"`
	Mirror           string                 `json:"mirror"`
	Mirrors          []DockerRegistryMirror `json:"mirrors"`
	SelectedMirrors  []string               `json:"selected_mirrors"`
	ExistingMirrors  []string               `json:"existing_mirrors,omitempty"`
	ConfigPath       string                 `json:"config_path"`
	Supported        bool                   `json:"supported"`
	CanAutoConfigure bool                   `json:"can_auto_configure"`
	RequiresManual   bool                   `json:"requires_manual"`
	DockerInstalled  bool                   `json:"docker_installed"`
	DockerReady      bool                   `json:"docker_ready"`
	ErrorCode        string                 `json:"error_code,omitempty"`
	Message          string                 `json:"message,omitempty"`
	ManualCommand    string                 `json:"manual_command,omitempty"`
	Script           string                 `json:"script,omitempty"`
	ScriptPath       string                 `json:"script_path,omitempty"`
	Warnings         []string               `json:"warnings,omitempty"`
}

type DockerMirrorRequest struct {
	Mirror string `json:"mirror"`
}

type dockerRegistryMirrorDefinition struct {
	ID   string
	Name string
	URL  string
}

var dockerRegistryMirrorDefinitions = []dockerRegistryMirrorDefinition{
	{ID: "daocloud", Name: "DaoCloud Docker Hub Mirror", URL: "https://docker.m.daocloud.io"},
	{ID: "one_ms", Name: "1ms Docker Hub Mirror", URL: "https://docker.1ms.run"},
	{ID: "registry_cyou", Name: "registry.cyou Docker Hub Mirror", URL: "https://registry.cyou"},
	{ID: "dockerproxy_net", Name: "dockerproxy.net Docker Hub Mirror", URL: "https://dockerproxy.net"},
	{ID: "dockerproxy_link", Name: "dockerproxy.link Docker Hub Mirror", URL: "https://dockerproxy.link"},
	{ID: "docker_jiaxin", Name: "docker.jiaxin.site Docker Hub Mirror", URL: "https://docker.jiaxin.site"},
	{ID: "docker_xuanyuan", Name: "docker.xuanyuan.me Docker Hub Mirror", URL: "https://docker.xuanyuan.me"},
	{ID: "free_hubfast", Name: "free.hubfast.cn Docker Hub Mirror", URL: "https://free.hubfast.cn"},
}

func (m Manager) DockerMirrorPlan(ctx context.Context, mirror string) (DockerMirrorPlan, error) {
	host := m.HostCapabilities(ctx)
	mirrors, selectedMirrors := selectDockerRegistryMirrors(ctx, mirror)
	supported, code, msg := dockerMirrorSupport(host)
	warnings := append([]string{}, host.Warnings...)
	if host.OS == "linux" {
		warnings = append(warnings, "Docker registry mirrors are third-party or organization-provided services; use trusted mirrors for production hosts.")
	}

	existingMirrors, err := readDockerDaemonMirrors(dockerDaemonConfigPath)
	if err != nil {
		warnings = append(warnings, "failed to read existing Docker daemon config: "+err.Error())
	}
	if supported && len(selectedMirrors) == 0 {
		code = "no_mirror_available"
		msg = "no Docker Hub mirror responded from this host"
	}

	script := ""
	scriptPath := ""
	manualCommand := ""
	if host.OS == "linux" && len(selectedMirrors) > 0 {
		script = buildDockerMirrorScript(selectedMirrors)
		path, err := m.writeDockerMirrorScript(script)
		if err == nil {
			scriptPath = path
		} else {
			warnings = append(warnings, "failed to write mirror script: "+err.Error())
		}
		if scriptPath != "" {
			manualCommand = "sudo bash " + shellQuote(scriptPath)
		}
	}

	plan := DockerMirrorPlan{
		Host:             host,
		Mirror:           normalizeDockerMirror(mirror),
		Mirrors:          mirrors,
		SelectedMirrors:  selectedMirrors,
		ExistingMirrors:  existingMirrors,
		ConfigPath:       dockerDaemonConfigPath,
		Supported:        supported,
		CanAutoConfigure: supported && host.Sudo.CanElevate && len(selectedMirrors) > 0,
		RequiresManual:   supported && !host.Sudo.CanElevate && len(selectedMirrors) > 0,
		DockerInstalled:  host.Docker.CLIInstalled,
		DockerReady:      host.Docker.CLIInstalled && host.Docker.DaemonReachable,
		ErrorCode:        code,
		Message:          msg,
		ManualCommand:    manualCommand,
		Script:           script,
		ScriptPath:       scriptPath,
		Warnings:         warnings,
	}
	if plan.RequiresManual && plan.Message == "" {
		plan.Message = "root or passwordless sudo is required for automatic Docker mirror configuration"
		plan.ErrorCode = "sudo_required"
	}
	if plan.CanAutoConfigure && plan.Message == "" {
		plan.Message = "Docker Hub mirror acceleration can be configured automatically"
	}
	return plan, nil
}

func (m Manager) ConfigureDockerMirrors(ctx context.Context, req DockerMirrorRequest) (db.Job, error) {
	plan, err := m.DockerMirrorPlan(ctx, req.Mirror)
	if err != nil {
		return db.Job{}, err
	}
	if !plan.Supported {
		if plan.ErrorCode == "" {
			plan.ErrorCode = "docker_mirror_unsupported"
		}
		if plan.Message == "" {
			plan.Message = "Docker mirror configuration is not supported on this host"
		}
		return db.Job{}, dockerInstallErr(http.StatusBadRequest, plan.ErrorCode, plan.Message)
	}
	if len(plan.SelectedMirrors) == 0 {
		return db.Job{}, dockerInstallErr(http.StatusConflict, "no_mirror_available", "no Docker Hub mirror responded from this host")
	}
	if !plan.Host.Sudo.CanElevate {
		return db.Job{}, dockerInstallErr(http.StatusConflict, "sudo_required", "Docker mirror configuration requires root or passwordless sudo; run the manual script as an administrator")
	}

	return m.startJob(ctx, "docker_mirror_configure", "queued Docker mirror configuration", func(jobCtx context.Context, jobID string) {
		m.update(jobID, "running", 10, "probing Docker Hub mirrors", "")
		plan, err := m.DockerMirrorPlan(jobCtx, req.Mirror)
		if err != nil {
			m.update(jobID, "failed", 10, "Docker mirror planning failed", err.Error())
			return
		}
		if !plan.Supported {
			m.update(jobID, "failed", 10, "Docker mirror configuration unsupported", plan.Message)
			return
		}
		if len(plan.SelectedMirrors) == 0 {
			m.update(jobID, "failed", 10, "no Docker Hub mirror available", "all configured mirror probes failed")
			return
		}
		if !plan.Host.Sudo.CanElevate {
			m.update(jobID, "failed", 10, "Docker mirror configuration requires manual sudo", "run the manual script as root or with sudo")
			return
		}

		m.update(jobID, "running", 35, "writing Docker mirror configuration script", "")
		scriptPath, err := m.writeDockerMirrorScript(plan.Script)
		if err != nil {
			m.update(jobID, "failed", 35, "script write failed", err.Error())
			return
		}

		m.update(jobID, "running", 60, "updating Docker daemon registry mirrors", "")
		if err := runDockerMirrorScript(jobCtx, scriptPath, plan.Host.Sudo.IsRoot); err != nil {
			m.update(jobID, "failed", 60, "Docker mirror configuration failed", err.Error())
			return
		}

		m.update(jobID, "running", 90, "verifying Docker daemon", "")
		host := m.HostCapabilities(jobCtx)
		if host.Docker.DaemonReachable {
			m.update(jobID, "completed", 100, "Docker mirror acceleration configured", "")
			return
		}
		errText := host.Docker.Error
		if errText == "" {
			errText = "docker daemon is not reachable after mirror configuration"
		}
		m.update(jobID, "failed", 90, "Docker verification failed", errText)
	})
}

func dockerMirrorSupport(host HostCapabilities) (bool, string, string) {
	if host.OS != "linux" {
		return false, "unsupported_os", "Docker mirror configuration is only supported on Linux hosts"
	}
	if !host.Docker.CLIInstalled {
		return false, "docker_not_installed", "Docker Engine must be installed before configuring registry mirrors"
	}
	if !host.Systemd && !commandExists("service") {
		return false, "no_service_manager", "systemd or service is required to restart Docker after mirror configuration"
	}
	return true, "", ""
}

func selectDockerRegistryMirrors(ctx context.Context, requested string) ([]DockerRegistryMirror, []string) {
	requested = normalizeDockerMirror(requested)
	mirrors := make([]DockerRegistryMirror, 0, len(dockerRegistryMirrorDefinitions))
	for _, def := range dockerRegistryMirrorDefinitions {
		mirrors = append(mirrors, DockerRegistryMirror{
			ID:       def.ID,
			Name:     def.Name,
			URL:      def.URL,
			ProbeURL: strings.TrimRight(def.URL, "/") + "/v2/",
		})
	}
	var wg sync.WaitGroup
	for i := range mirrors {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result := dockerRegistryMirrorProbe(ctx, mirrors[index].URL)
			mirrors[index].Available = result.Available
			mirrors[index].LatencyMS = result.Latency.Milliseconds()
			mirrors[index].Error = result.Error
		}(i)
	}
	wg.Wait()

	if requested != "auto" {
		for i := range mirrors {
			if mirrors[i].ID == requested {
				mirrors[i].Selected = mirrors[i].Available
				if mirrors[i].Available {
					return mirrors, []string{mirrors[i].URL}
				}
				return mirrors, nil
			}
		}
	}

	available := make([]int, 0, len(mirrors))
	for i, mirror := range mirrors {
		if mirror.Available {
			available = append(available, i)
		}
	}
	sort.SliceStable(available, func(i, j int) bool {
		return mirrors[available[i]].LatencyMS < mirrors[available[j]].LatencyMS
	})
	selected := make([]string, 0, minInt(len(available), 4))
	for _, index := range available {
		if len(selected) >= 4 {
			break
		}
		mirrors[index].Selected = true
		selected = append(selected, mirrors[index].URL)
	}
	return mirrors, selected
}

func normalizeDockerMirror(mirror string) string {
	mirror = strings.ToLower(strings.TrimSpace(mirror))
	if mirror == "" || mirror == "auto" {
		return "auto"
	}
	for _, def := range dockerRegistryMirrorDefinitions {
		if mirror == def.ID {
			return mirror
		}
	}
	return "auto"
}

func probeDockerRegistryMirror(ctx context.Context, rawURL string) sourceProbeResult {
	if rawURL == "" {
		return sourceProbeResult{Error: "empty mirror URL"}
	}
	probeURL := strings.TrimRight(rawURL, "/") + "/v2/"
	probeCtx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
	defer cancel()
	started := time.Now()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, probeURL, nil)
	if err != nil {
		return sourceProbeResult{Error: err.Error()}
	}
	resp, err := http.DefaultClient.Do(req)
	latency := time.Since(started)
	if err != nil {
		return sourceProbeResult{Latency: latency, Error: err.Error()}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
	if (resp.StatusCode >= 200 && resp.StatusCode < 400) || resp.StatusCode == http.StatusUnauthorized {
		return sourceProbeResult{Available: true, Latency: latency}
	}
	return sourceProbeResult{Latency: latency, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

func readDockerDaemonMirrors(path string) ([]string, error) {
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseDockerDaemonMirrors(body)
}

func parseDockerDaemonMirrors(body []byte) ([]string, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, err
	}
	return stringListFromAny(cfg["registry-mirrors"]), nil
}

func mergeDockerDaemonMirrors(body []byte, selected []string) ([]byte, []string, error) {
	body = bytes.TrimSpace(body)
	cfg := map[string]any{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &cfg); err != nil {
			return nil, nil, err
		}
	}
	merged := mergeMirrorLists(selected, stringListFromAny(cfg["registry-mirrors"]))
	cfg["registry-mirrors"] = merged
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return append(out, '\n'), merged, nil
}

func stringListFromAny(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			continue
		}
		text = strings.TrimRight(strings.TrimSpace(text), "/")
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func mergeMirrorLists(primary, existing []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(primary)+len(existing))
	for _, item := range append(append([]string{}, primary...), existing...) {
		item = strings.TrimRight(strings.TrimSpace(item), "/")
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func buildDockerMirrorScript(mirrors []string) string {
	mirrorsJSON, _ := json.Marshal(mergeMirrorLists(mirrors, nil))
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("set -euo pipefail\n\n")
	b.WriteString("DOCKER_DAEMON_CONFIG=\"${DOCKER_DAEMON_CONFIG:-/etc/docker/daemon.json}\"\n")
	b.WriteString("MIRRORS_JSON=" + shellQuote(string(mirrorsJSON)) + "\n\n")
	b.WriteString("log() { printf '\\n[palpanel] %s\\n' \"$*\"; }\n")
	b.WriteString("require_root() {\n")
	b.WriteString("  if [ \"$(id -u)\" -ne 0 ]; then\n")
	b.WriteString("    echo \"Run this script as root or with sudo. PalPanel does not store sudo passwords.\" >&2\n")
	b.WriteString("    exit 1\n")
	b.WriteString("  fi\n")
	b.WriteString("}\n\n")
	b.WriteString("find_python() {\n")
	b.WriteString("  if command -v python3 >/dev/null 2>&1; then echo python3; return 0; fi\n")
	b.WriteString("  if command -v python >/dev/null 2>&1; then echo python; return 0; fi\n")
	b.WriteString("  echo \"python3 is required to safely merge /etc/docker/daemon.json\" >&2\n")
	b.WriteString("  exit 2\n")
	b.WriteString("}\n\n")
	b.WriteString("restart_docker() {\n")
	b.WriteString("  log \"Restarting Docker\"\n")
	b.WriteString("  if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then\n")
	b.WriteString("    systemctl daemon-reload || true\n")
	b.WriteString("    systemctl restart docker\n")
	b.WriteString("  elif command -v service >/dev/null 2>&1; then\n")
	b.WriteString("    service docker restart\n")
	b.WriteString("  else\n")
	b.WriteString("    echo \"No supported service manager found for Docker\" >&2\n")
	b.WriteString("    exit 3\n")
	b.WriteString("  fi\n")
	b.WriteString("}\n\n")
	b.WriteString("configure_mirrors() {\n")
	b.WriteString("  local py\n")
	b.WriteString("  py=\"$(find_python)\"\n")
	b.WriteString("  install -m 0755 -d \"$(dirname \"$DOCKER_DAEMON_CONFIG\")\"\n")
	b.WriteString("  if [ -f \"$DOCKER_DAEMON_CONFIG\" ]; then\n")
	b.WriteString("    cp -a \"$DOCKER_DAEMON_CONFIG\" \"$DOCKER_DAEMON_CONFIG.bak.$(date +%Y%m%d%H%M%S)\"\n")
	b.WriteString("  fi\n")
	b.WriteString("  log \"Writing Docker registry mirrors to $DOCKER_DAEMON_CONFIG\"\n")
	b.WriteString("  \"$py\" - \"$DOCKER_DAEMON_CONFIG\" \"$MIRRORS_JSON\" <<'PY'\n")
	b.WriteString("import json\n")
	b.WriteString("import os\n")
	b.WriteString("import sys\n")
	b.WriteString("import tempfile\n\n")
	b.WriteString("path = sys.argv[1]\n")
	b.WriteString("selected = [item.rstrip('/') for item in json.loads(sys.argv[2]) if isinstance(item, str) and item.strip()]\n")
	b.WriteString("data = {}\n")
	b.WriteString("if os.path.exists(path):\n")
	b.WriteString("    with open(path, 'r', encoding='utf-8') as fh:\n")
	b.WriteString("        raw = fh.read().strip()\n")
	b.WriteString("    if raw:\n")
	b.WriteString("        data = json.loads(raw)\n")
	b.WriteString("existing = data.get('registry-mirrors', [])\n")
	b.WriteString("if not isinstance(existing, list):\n")
	b.WriteString("    existing = []\n")
	b.WriteString("merged = []\n")
	b.WriteString("for item in selected + existing:\n")
	b.WriteString("    if isinstance(item, str):\n")
	b.WriteString("        item = item.strip().rstrip('/')\n")
	b.WriteString("        if item and item not in merged:\n")
	b.WriteString("            merged.append(item)\n")
	b.WriteString("data['registry-mirrors'] = merged\n")
	b.WriteString("directory = os.path.dirname(path) or '.'\n")
	b.WriteString("fd, tmp = tempfile.mkstemp(prefix='.daemon.', dir=directory)\n")
	b.WriteString("try:\n")
	b.WriteString("    with os.fdopen(fd, 'w', encoding='utf-8') as fh:\n")
	b.WriteString("        json.dump(data, fh, indent=2, ensure_ascii=False)\n")
	b.WriteString("        fh.write('\\n')\n")
	b.WriteString("    os.chmod(tmp, 0o644)\n")
	b.WriteString("    os.replace(tmp, path)\n")
	b.WriteString("finally:\n")
	b.WriteString("    if os.path.exists(tmp):\n")
	b.WriteString("        os.unlink(tmp)\n")
	b.WriteString("PY\n")
	b.WriteString("}\n\n")
	b.WriteString("main() {\n")
	b.WriteString("  require_root\n")
	b.WriteString("  log \"Selected Docker mirrors: $MIRRORS_JSON\"\n")
	b.WriteString("  configure_mirrors\n")
	b.WriteString("  restart_docker\n")
	b.WriteString("  log \"Verifying Docker\"\n")
	b.WriteString("  docker info >/dev/null\n")
	b.WriteString("  docker info --format '{{json .RegistryConfig.Mirrors}}' 2>/dev/null || true\n")
	b.WriteString("}\n\n")
	b.WriteString("main \"$@\"\n")
	return b.String()
}

func (m Manager) writeDockerMirrorScript(script string) (string, error) {
	if strings.TrimSpace(script) == "" {
		return "", errors.New("empty Docker mirror script")
	}
	if err := os.MkdirAll(m.cfg.ToolsDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(m.cfg.ToolsDir, dockerMirrorScriptName)
	if err := os.WriteFile(path, []byte(script), 0o750); err != nil {
		return "", err
	}
	return path, nil
}

func runDockerMirrorScript(ctx context.Context, scriptPath string, isRoot bool) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	args := []string{scriptPath}
	binary := "bash"
	if !isRoot {
		args = []string{"-n", "bash", scriptPath}
		binary = "sudo"
	}
	cmd := exec.CommandContext(timeoutCtx, binary, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w: %s", binary, err, limitString(strings.TrimSpace(out.String()), 4000))
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
