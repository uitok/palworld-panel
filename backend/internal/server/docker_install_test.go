package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRecommendedRuntimeForOS(t *testing.T) {
	cases := []struct {
		goos string
		want string
	}{
		{goos: "windows", want: RuntimeWindowsSteamCMD},
		{goos: "linux", want: RuntimeLinuxSteamCMD},
		{goos: "darwin", want: RuntimeWineDocker},
	}
	for _, tc := range cases {
		if got := RecommendedRuntimeForOS(tc.goos); got != tc.want {
			t.Fatalf("RecommendedRuntimeForOS(%q) = %q, want %q", tc.goos, got, tc.want)
		}
	}
}

func TestSelectDockerInstallSourceAutoPicksFastestAvailable(t *testing.T) {
	restore := stubDockerProbe(func(_ context.Context, rawURL string) sourceProbeResult {
		switch {
		case strings.Contains(rawURL, "aliyun"):
			return sourceProbeResult{Available: true, Latency: 10 * time.Millisecond}
		case strings.Contains(rawURL, "download.docker.com"):
			return sourceProbeResult{Available: true, Latency: 80 * time.Millisecond}
		default:
			return sourceProbeResult{Error: "unreachable", Latency: 2 * time.Second}
		}
	})
	defer restore()

	selected, sources := selectDockerInstallSource(context.Background(), debianHost(), "auto")
	if selected.ID != "aliyun" {
		t.Fatalf("expected aliyun source, got %#v", selected)
	}
	if len(sources) != 3 {
		t.Fatalf("expected three source candidates, got %d", len(sources))
	}
}

func TestSelectDockerInstallSourceFallsBackToOfficialWhenAllFail(t *testing.T) {
	restore := stubDockerProbe(func(_ context.Context, _ string) sourceProbeResult {
		return sourceProbeResult{Error: "timeout", Latency: 2 * time.Second}
	})
	defer restore()

	selected, _ := selectDockerInstallSource(context.Background(), debianHost(), "auto")
	if selected.ID != "official" {
		t.Fatalf("expected official fallback, got %#v", selected)
	}
	if selected.Error == "" {
		t.Fatalf("expected fallback source to carry probe error")
	}
}

func TestSelectDockerInstallSourceManualOverride(t *testing.T) {
	restore := stubDockerProbe(func(_ context.Context, rawURL string) sourceProbeResult {
		if strings.Contains(rawURL, "aliyun") {
			return sourceProbeResult{Available: true, Latency: time.Millisecond}
		}
		return sourceProbeResult{Available: true, Latency: 100 * time.Millisecond}
	})
	defer restore()

	selected, _ := selectDockerInstallSource(context.Background(), debianHost(), "official")
	if selected.ID != "official" {
		t.Fatalf("expected manual official source, got %#v", selected)
	}
}

func TestSudoCapabilityFromProbe(t *testing.T) {
	root := sudoCapabilityFromProbe(true, true, nil, "")
	if !root.CanElevate || !root.IsRoot {
		t.Fatalf("expected root to be able to elevate: %#v", root)
	}
	passwordless := sudoCapabilityFromProbe(false, true, nil, "")
	if !passwordless.CanElevate || !passwordless.Passwordless {
		t.Fatalf("expected passwordless sudo: %#v", passwordless)
	}
	needsPassword := sudoCapabilityFromProbe(false, true, errors.New("exit status 1"), "sudo: a password is required")
	if !needsPassword.NeedsPassword || needsPassword.CanElevate {
		t.Fatalf("expected sudo password requirement: %#v", needsPassword)
	}
	noSudo := sudoCapabilityFromProbe(false, false, errors.New("not found"), "")
	if noSudo.SudoInstalled || noSudo.CanElevate {
		t.Fatalf("expected no sudo capability: %#v", noSudo)
	}
}

func TestDockerInstallSupport(t *testing.T) {
	if ok, code, _ := dockerInstallSupport(debianHost()); !ok || code != "" {
		t.Fatalf("expected supported Debian host, got ok=%v code=%q", ok, code)
	}
	host := debianHost()
	host.Systemd = false
	if ok, code, _ := dockerInstallSupport(host); ok || code != "no_systemd" {
		t.Fatalf("expected no_systemd, got ok=%v code=%q", ok, code)
	}
	host = debianHost()
	host.DistroID = "arch"
	if ok, code, _ := dockerInstallSupport(host); ok || code != "unsupported_distribution" {
		t.Fatalf("expected unsupported_distribution, got ok=%v code=%q", ok, code)
	}
	host = debianHost()
	host.PackageManager = ""
	if ok, code, _ := dockerInstallSupport(host); ok || code != "unsupported_package_manager" {
		t.Fatalf("expected unsupported_package_manager, got ok=%v code=%q", ok, code)
	}
}

func TestBuildDockerInstallScriptForDebianApt(t *testing.T) {
	script := buildDockerInstallScript(debianHost(), DockerInstallSource{
		ID:  "official",
		URL: "https://download.docker.com/linux/debian",
	})
	for _, want := range []string{
		"apt-get install -y ca-certificates curl gnupg",
		"docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",
		"https://download.docker.com/linux/debian",
		"VERSION_CODENAME",
		"systemctl enable --now docker",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("apt script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildDockerInstallScriptForRpmManagers(t *testing.T) {
	for _, host := range []HostCapabilities{
		{OS: "linux", DistroID: "fedora", PackageManager: "dnf", Systemd: true},
		{OS: "linux", DistroID: "centos", PackageManager: "yum", Systemd: true},
	} {
		script := buildDockerInstallScript(host, DockerInstallSource{
			ID:  "aliyun",
			URL: "https://mirrors.aliyun.com/docker-ce/linux/" + host.DistroID,
		})
		for _, want := range []string{
			"curl -fsSL \"$DOCKER_REPO_URL/docker-ce.repo\"",
			"\"$pm\" -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",
			"ADD_CURRENT_USER_TO_DOCKER_GROUP",
		} {
			if !strings.Contains(script, want) {
				t.Fatalf("rpm script for %s missing %q:\n%s", host.PackageManager, want, script)
			}
		}
	}
}

func TestSelectDockerRegistryMirrorsAutoPicksFastestAvailable(t *testing.T) {
	restore := stubDockerMirrorProbe(func(_ context.Context, rawURL string) sourceProbeResult {
		switch {
		case strings.Contains(rawURL, "docker.m.daocloud.io"):
			return sourceProbeResult{Available: true, Latency: 20 * time.Millisecond}
		case strings.Contains(rawURL, "docker.1ms.run"):
			return sourceProbeResult{Available: true, Latency: 10 * time.Millisecond}
		case strings.Contains(rawURL, "registry.cyou"):
			return sourceProbeResult{Available: true, Latency: 30 * time.Millisecond}
		default:
			return sourceProbeResult{Error: "timeout", Latency: 2 * time.Second}
		}
	})
	defer restore()

	mirrors, selected := selectDockerRegistryMirrors(context.Background(), "auto")
	if len(mirrors) != len(dockerRegistryMirrorDefinitions) {
		t.Fatalf("expected mirror candidates, got %d", len(mirrors))
	}
	if len(selected) != 3 {
		t.Fatalf("expected three selected mirrors, got %#v", selected)
	}
	if selected[0] != "https://docker.1ms.run" {
		t.Fatalf("expected fastest mirror first, got %#v", selected)
	}
}

func TestSelectDockerRegistryMirrorsManualOverride(t *testing.T) {
	restore := stubDockerMirrorProbe(func(_ context.Context, rawURL string) sourceProbeResult {
		if strings.Contains(rawURL, "dockerproxy.net") {
			return sourceProbeResult{Available: true, Latency: 100 * time.Millisecond}
		}
		return sourceProbeResult{Available: true, Latency: time.Millisecond}
	})
	defer restore()

	_, selected := selectDockerRegistryMirrors(context.Background(), "dockerproxy_net")
	if len(selected) != 1 || selected[0] != "https://dockerproxy.net" {
		t.Fatalf("expected manual dockerproxy.net mirror, got %#v", selected)
	}
}

func TestMergeDockerDaemonMirrorsPreservesExistingConfig(t *testing.T) {
	body := []byte(`{"log-driver":"json-file","registry-mirrors":["https://old.example/"]}`)
	out, mirrors, err := mergeDockerDaemonMirrors(body, []string{"https://docker.1ms.run", "https://old.example"})
	if err != nil {
		t.Fatalf("mergeDockerDaemonMirrors returned error: %v", err)
	}
	if len(mirrors) != 2 || mirrors[0] != "https://docker.1ms.run" || mirrors[1] != "https://old.example" {
		t.Fatalf("unexpected merged mirrors: %#v", mirrors)
	}
	if !strings.Contains(string(out), `"log-driver": "json-file"`) {
		t.Fatalf("expected existing daemon key to be preserved: %s", out)
	}
}

func TestBuildDockerMirrorScript(t *testing.T) {
	script := buildDockerMirrorScript([]string{"https://docker.1ms.run", "https://registry.cyou"})
	for _, want := range []string{
		"/etc/docker/daemon.json",
		"https://docker.1ms.run",
		"https://registry.cyou",
		"registry-mirrors",
		"systemctl restart docker",
		"docker info",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("mirror script missing %q:\n%s", want, script)
		}
	}
}

func debianHost() HostCapabilities {
	return HostCapabilities{
		OS:                 "linux",
		DistroID:           "debian",
		DistroCodename:     "trixie",
		PackageManager:     "apt",
		Systemd:            true,
		RecommendedRuntime: RuntimeWineDocker,
	}
}

func stubDockerProbe(fn func(context.Context, string) sourceProbeResult) func() {
	previous := dockerSourceProbe
	dockerSourceProbe = fn
	return func() {
		dockerSourceProbe = previous
	}
}

func stubDockerMirrorProbe(fn func(context.Context, string) sourceProbeResult) func() {
	previous := dockerRegistryMirrorProbe
	dockerRegistryMirrorProbe = fn
	return func() {
		dockerRegistryMirrorProbe = previous
	}
}
