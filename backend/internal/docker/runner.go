package docker

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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

func NewRunner(cfg appconfig.Config) Runner {
	return Runner{cfg: cfg}
}

func (r Runner) BuildImage(ctx context.Context) error {
	args := []string{"build", "-t", r.cfg.DockerImage, "-f", filepath.Join(r.cfg.RunnerDir, "Dockerfile")}
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		if v := os.Getenv(key); v != "" {
			args = append(args, "--build-arg", key+"="+proxyForContainer(v))
		}
	}
	args = append(args, r.cfg.RunnerDir)
	_, err := r.run(ctx, args...)
	return err
}

func (r Runner) InstallOrUpdate(ctx context.Context) error {
	args := []string{
		"run", "--rm",
		"--add-host", "host.docker.internal:host-gateway",
		"-v", volume(r.cfg.ServerDir, "/data/server"),
		"-v", volume(r.cfg.WinePrefixDir, "/data/wineprefix"),
	}
	args = append(args, containerProxyEnvArgs()...)
	args = append(args, r.cfg.DockerImage, "install")
	_, err := r.run(ctx, args...)
	return err
}

func (r Runner) DownloadWorkshop(ctx context.Context, itemID string) error {
	args := []string{
		"run", "--rm",
		"--add-host", "host.docker.internal:host-gateway",
		"-e", "STEAM_USERNAME=" + os.Getenv("STEAM_USERNAME"),
		"-e", "STEAM_PASSWORD=" + os.Getenv("STEAM_PASSWORD"),
		"-v", volume(r.cfg.WorkshopModsDir(), "/data/workshop"),
	}
	args = append(args, containerProxyEnvArgs()...)
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
		"-v", volume(r.cfg.ServerDir, "/data/server"),
		"-v", volume(r.cfg.WinePrefixDir, "/data/wineprefix"),
		"-p", fmt.Sprintf("%d:%d/udp", gamePort, gamePort),
		"-p", fmt.Sprintf("%d:27015/udp", r.cfg.QueryPort),
		"-p", fmt.Sprintf("%d:8212/tcp", r.cfg.RESTPort),
		r.cfg.DockerImage,
		"start",
	}
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
	binary := dockerBinary(r.cfg.DockerBinary)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = dockerEnv(os.Environ())
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return out.Bytes(), fmt.Errorf("docker %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return out.Bytes(), nil
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
