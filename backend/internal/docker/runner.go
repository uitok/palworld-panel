package docker

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"palpanel/internal/appconfig"
)

type Runner struct {
	cfg appconfig.Config
}

type ContainerStatus struct {
	Exists bool   `json:"exists"`
	Status string `json:"status"`
}

const (
	wineBaseImageBuildArg = "PALPANEL_WINE_BASE_IMAGE"
	wineDLLOverrides      = "dwmapi=n,b;d3d9=n,b"
)

func NewRunner(cfg appconfig.Config) Runner {
	return Runner{cfg: cfg}
}

func (r Runner) BuildImage(ctx context.Context) error {
	candidates := runnerBaseImageCandidates(r.cfg.DockerRunnerBaseImage, r.cfg.DockerRunnerBaseImageMirrors)
	attemptErrors := make([]string, 0, len(candidates))
	for index, baseImage := range candidates {
		err := r.buildImageWithBase(ctx, baseImage)
		if err == nil {
			return nil
		}
		attemptErrors = append(attemptErrors, fmt.Sprintf("%s: %s", baseImage, compactDockerError(err.Error())))
		if index == 0 && !dockerBaseImagePullFailure(err) {
			return err
		}
	}
	return fmt.Errorf(
		"build wine runner image failed: unable to pull Docker base image after trying %d source(s). "+
			"Try configuring Docker Hub mirror acceleration in Setup > Advanced settings, or set PALPANEL_DOCKER_RUNNER_BASE_IMAGE_MIRRORS. Attempts: %s",
		len(attemptErrors),
		strings.Join(attemptErrors, " | "),
	)
}

func (r Runner) buildImageWithBase(ctx context.Context, baseImage string) error {
	args := []string{"build", "-t", r.cfg.DockerImage, "-f", filepath.Join(r.cfg.RunnerDir, "Dockerfile")}
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		if v := os.Getenv(key); v != "" {
			args = append(args, "--build-arg", key+"="+proxyForContainer(v))
		}
	}
	args = append(args, "--build-arg", wineBaseImageBuildArg+"="+baseImage)
	args = append(args, r.cfg.RunnerDir)
	_, err := r.run(ctx, args...)
	return err
}

func runnerBaseImageCandidates(baseImage string, mirrors []string) []string {
	baseImage = strings.TrimSpace(baseImage)
	if baseImage == "" {
		baseImage = appconfig.DefaultDockerRunnerBaseImage
	}
	out := []string{baseImage}
	dockerHubRef := dockerHubImageRef(baseImage)
	if dockerHubRef == "" {
		return dedupeStrings(out)
	}
	for _, mirror := range mirrors {
		mirror = normalizeImageMirrorPrefix(mirror)
		if mirror == "" {
			continue
		}
		if strings.Contains(mirror, dockerHubRef) {
			out = append(out, mirror)
			continue
		}
		out = append(out, mirror+"/"+dockerHubRef)
	}
	return dedupeStrings(out)
}

func dockerHubImageRef(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	parts := strings.Split(image, "/")
	if len(parts) == 1 {
		return "library/" + image
	}
	first := parts[0]
	if first == "docker.io" || first == "index.docker.io" || first == "registry-1.docker.io" {
		if len(parts) == 2 {
			return "library/" + parts[1]
		}
		return strings.Join(parts[1:], "/")
	}
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return ""
	}
	return image
}

func normalizeImageMirrorPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	if parsed, err := url.Parse(prefix); err == nil && parsed.Host != "" {
		prefix = parsed.Host + strings.TrimRight(parsed.Path, "/")
	}
	prefix = strings.Trim(prefix, "/")
	return prefix
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func dockerBaseImagePullFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	needles := []string{
		"load metadata for",
		"failed to resolve source metadata",
		"failed to do request",
		"registry-1.docker.io",
		"docker.io",
		"deadlineexceeded",
		"context deadline exceeded",
		"i/o timeout",
		"tls handshake timeout",
		"client.timeout",
		"dial tcp",
		"no route to host",
		"connection refused",
		"connection reset",
		"temporary failure",
		"lookup ",
		"unexpected eof",
	}
	for _, needle := range needles {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

func compactDockerError(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	const limit = 900
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}

func (r Runner) InstallOrUpdate(ctx context.Context) error {
	args := []string{
		"run", "--rm",
		"--add-host", "host.docker.internal:host-gateway",
		"-v", volume(r.cfg.ServerDir, "/data/server"),
		"-v", volume(r.cfg.WinePrefixDir, "/data/wineprefix"),
	}
	args = append(args, containerProxyEnvArgs()...)
	args = append(args, hostOwnerEnvArgs()...)
	args = append(args, r.cfg.DockerImage, "install")
	_, err := r.run(ctx, args...)
	return err
}

func (r Runner) ImageExists(ctx context.Context) (bool, error) {
	_, err := r.run(ctx, "image", "inspect", r.cfg.DockerImage)
	if err == nil {
		return true, nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such image") || strings.Contains(msg, "not found") {
		return false, nil
	}
	return false, err
}

func (r Runner) AppInfo(ctx context.Context) (string, error) {
	exists, err := r.ImageExists(ctx)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("wine runner image is not built; run install or bootstrap before checking remote version")
	}
	args := []string{
		"run", "--rm",
		"--add-host", "host.docker.internal:host-gateway",
	}
	args = append(args, containerProxyEnvArgs()...)
	args = append(args, r.cfg.DockerImage, "appinfo")
	out, err := r.run(ctx, args...)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (r Runner) DownloadWorkshop(ctx context.Context, itemID string) error {
	return r.DownloadWorkshopTo(ctx, itemID, r.cfg.WorkshopModsDir())
}

func (r Runner) DownloadWorkshopTo(ctx context.Context, itemID, destinationRoot string) error {
	args := []string{
		"run", "--rm",
		"--add-host", "host.docker.internal:host-gateway",
		"-e", "STEAM_USERNAME=" + os.Getenv("STEAM_USERNAME"),
		"-e", "STEAM_PASSWORD=" + os.Getenv("STEAM_PASSWORD"),
		"-e", "PALPANEL_WORKSHOP_APP_ID=" + r.cfg.WorkshopAppID,
		"-v", volume(destinationRoot, "/data/workshop"),
	}
	args = append(args, containerProxyEnvArgs()...)
	args = append(args, hostOwnerEnvArgs()...)
	args = append(args, r.cfg.DockerImage, "workshop", itemID)
	_, err := r.run(ctx, args...)
	return err
}

func (r Runner) Start(ctx context.Context) error {
	return r.StartWithArgs(ctx, nil)
}

func (r Runner) StartWithArgs(ctx context.Context, serverArgs []string) error {
	status, err := r.Status(ctx)
	if err != nil {
		return err
	}
	if status.Exists {
		if status.Status == "running" {
			return nil
		}
		if err := r.removeContainer(ctx); err != nil {
			return err
		}
	}

	gamePort := palServerPort(serverArgs, r.cfg.GamePort)
	args := []string{
		"run", "-d",
		"--name", r.cfg.DockerContainer,
		"--restart", "unless-stopped",
		"--log-opt", "max-size=20m",
		"--log-opt", "max-file=5",
		"-e", "WINEDLLOVERRIDES=" + wineDLLOverrides,
		"-v", volume(r.cfg.ServerDir, "/data/server"),
		"-v", volume(r.cfg.WinePrefixDir, "/data/wineprefix"),
		"-v", volume(r.cfg.LogsDir, "/data/logs"),
		"-p", fmt.Sprintf("%d:%d/udp", gamePort, gamePort),
		"-p", fmt.Sprintf("%d:27015/udp", r.cfg.QueryPort),
		"-p", fmt.Sprintf("127.0.0.1:%d:8212/tcp", r.cfg.RESTPort),
		"-p", fmt.Sprintf("127.0.0.1:%d:%d/tcp", r.cfg.EffectivePalDefenderRESTPort(), r.cfg.EffectivePalDefenderRESTPort()),
	}
	args = append(args, hostUserRunArgs()...)
	args = append(args, r.cfg.DockerImage, "start")
	args = append(args, serverArgs...)
	_, err = r.run(ctx, args...)
	return err
}

func (r Runner) Stop(ctx context.Context) error {
	status, err := r.Status(ctx)
	if err != nil {
		return err
	}
	if !status.Exists {
		return nil
	}
	_, err = r.run(ctx, "stop", r.cfg.DockerContainer)
	return err
}

func (r Runner) Restart(ctx context.Context) error {
	return r.RestartWithArgs(ctx, nil)
}

func (r Runner) RestartWithArgs(ctx context.Context, serverArgs []string) error {
	status, err := r.Status(ctx)
	if err != nil {
		return err
	}
	if !status.Exists {
		return r.StartWithArgs(ctx, serverArgs)
	}
	if status.Status == "running" {
		if _, err := r.run(ctx, "stop", r.cfg.DockerContainer); err != nil {
			return err
		}
	}
	if err := r.removeContainer(ctx); err != nil {
		return err
	}
	return r.StartWithArgs(ctx, serverArgs)
}

func (r Runner) Logs(ctx context.Context, tail int) (string, error) {
	if tail <= 0 || tail > 5000 {
		tail = 200
	}
	out, err := r.run(ctx, "logs", "--tail", fmt.Sprintf("%d", tail), r.cfg.DockerContainer)
	return string(out), err
}

func (r Runner) Status(ctx context.Context) (ContainerStatus, error) {
	out, err := r.run(ctx, "inspect", "-f", "{{.State.Status}}", r.cfg.DockerContainer)
	if err != nil {
		msg := strings.ToLower(err.Error() + string(out))
		if strings.Contains(msg, "no such object") {
			return ContainerStatus{Exists: false, Status: "missing"}, nil
		}
		return ContainerStatus{}, err
	}
	return ContainerStatus{Exists: true, Status: strings.TrimSpace(string(out))}, nil
}

func (r Runner) run(ctx context.Context, args ...string) ([]byte, error) {
	return RunCommand(ctx, r.cfg.DockerBinary, args...)
}

func RunCommand(ctx context.Context, binary string, args ...string) ([]byte, error) {
	binary = dockerBinary(binary)
	out, err := runDockerCommand(ctx, binary, args, false)
	if err == nil {
		return out, nil
	}
	if dockerPermissionDenied(err, out) && canUseDockerGroupShell() {
		sgOut, sgErr := runDockerCommand(ctx, binary, args, true)
		if sgErr == nil {
			return sgOut, nil
		}
		return sgOut, fmt.Errorf("docker %s via sg docker failed after direct Docker socket permission was denied: %w: %s", strings.Join(args, " "), sgErr, compactDockerError(strings.TrimSpace(string(sgOut))))
	}
	return out, fmt.Errorf("docker %s failed: %w: %s", strings.Join(args, " "), err, compactDockerError(strings.TrimSpace(string(out))))
}

func runDockerCommand(ctx context.Context, binary string, args []string, useDockerGroup bool) ([]byte, error) {
	command := binary
	commandArgs := args
	if useDockerGroup {
		command = "sg"
		commandArgs = []string{"docker", "-c", dockerShellCommand(binary, args)}
	}
	cmd := exec.CommandContext(ctx, command, commandArgs...)
	cmd.Env = dockerEnv(os.Environ())
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.Bytes(), err
}

func dockerPermissionDenied(err error, out []byte) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error() + " " + string(out))
	return strings.Contains(msg, "permission denied") &&
		(strings.Contains(msg, "docker.sock") || strings.Contains(msg, "docker api") || strings.Contains(msg, "var/run/docker"))
}

func canUseDockerGroupShell() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if _, err := exec.LookPath("sg"); err != nil {
		return false
	}
	current, err := user.Current()
	if err != nil {
		return false
	}
	group, err := user.LookupGroup("docker")
	if err != nil {
		return false
	}
	ids, err := current.GroupIds()
	if err != nil {
		return false
	}
	for _, id := range ids {
		if id == group.Gid {
			return true
		}
	}
	return false
}

func dockerShellCommand(binary string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(binary))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (r Runner) removeContainer(ctx context.Context) error {
	_, err := r.run(ctx, "rm", r.cfg.DockerContainer)
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such container") || strings.Contains(msg, "no such object") {
		return nil
	}
	return err
}

func dockerEnv(base []string) []string {
	pathExtra := `C:\Program Files\Docker\Docker\resources\bin`
	foundPath := false
	for i, item := range base {
		if strings.HasPrefix(strings.ToUpper(item), "PATH=") {
			foundPath = true
			if !strings.Contains(strings.ToLower(item), strings.ToLower(pathExtra)) {
				base[i] = item + ";" + pathExtra
			}
		}
	}
	if !foundPath {
		base = append(base, "PATH="+pathExtra)
	}
	return base
}

func volume(host, container string) string {
	return filepath.Clean(host) + ":" + container
}

func dockerBinary(configured string) string {
	if configured != "" {
		if resolved, err := exec.LookPath(configured); err == nil {
			return resolved
		}
		if strings.ContainsAny(configured, `\/`) {
			return configured
		}
	}
	defaultPath := `C:\Program Files\Docker\Docker\resources\bin\docker.exe`
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	return configured
}

func containerProxyEnvArgs() []string {
	var args []string
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		if v := os.Getenv(key); v != "" {
			args = append(args, "-e", key+"="+proxyForContainer(v))
		}
	}
	return args
}

func hostOwnerEnvArgs() []string {
	if runtime.GOOS == "windows" {
		return nil
	}
	uid := os.Getuid()
	gid := os.Getgid()
	if uid < 0 || gid < 0 {
		return nil
	}
	return []string{
		"-e", fmt.Sprintf("PALPANEL_HOST_UID=%d", uid),
		"-e", fmt.Sprintf("PALPANEL_HOST_GID=%d", gid),
	}
}

func hostUserRunArgs() []string {
	if runtime.GOOS == "windows" {
		return nil
	}
	uid := os.Getuid()
	gid := os.Getgid()
	if uid < 0 || gid < 0 {
		return nil
	}
	return []string{
		"--user", fmt.Sprintf("%d:%d", uid, gid),
		"-e", "HOME=/data/wineprefix",
	}
}

func proxyForContainer(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Hostname()
		port = u.Port()
	}
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		if u.Scheme == "socks5" {
			u.Scheme = "socks5h"
		}
		if port != "" {
			u.Host = net.JoinHostPort("host.docker.internal", port)
		} else {
			u.Host = "host.docker.internal"
		}
	}
	return u.String()
}

func palServerPort(serverArgs []string, fallback int) int {
	for _, arg := range serverArgs {
		value, ok := strings.CutPrefix(arg, "-port=")
		if !ok {
			continue
		}
		var port int
		if _, err := fmt.Sscanf(value, "%d", &port); err == nil && port > 0 && port <= 65535 {
			return port
		}
	}
	return fallback
}
