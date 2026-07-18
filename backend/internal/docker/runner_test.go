package docker

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/networkproxy"
)

func TestPalServerPortUsesStartupArg(t *testing.T) {
	got := palServerPort([]string{"-players=24", "-port=9001"}, 8211)
	if got != 9001 {
		t.Fatalf("expected startup port, got %d", got)
	}
}

func TestPalServerPortFallsBackOnInvalidArg(t *testing.T) {
	for _, args := range [][]string{
		{"-port=0"},
		{"-port=70000"},
		{"-port=not-a-port"},
		nil,
	} {
		if got := palServerPort(args, 8211); got != 8211 {
			t.Fatalf("expected fallback for %#v, got %d", args, got)
		}
	}
}

func TestDockerPermissionDeniedDetectsSocketErrors(t *testing.T) {
	out := []byte("permission denied while trying to connect to the docker API at unix:///var/run/docker.sock")
	if !dockerPermissionDenied(errors.New("exit status 1"), out) {
		t.Fatal("expected docker socket permission error to be detected")
	}
	if dockerPermissionDenied(errors.New("exit status 1"), []byte("network timeout")) {
		t.Fatal("did not expect unrelated docker error to be treated as permission denied")
	}
}

func TestDockerShellCommandQuotesArgs(t *testing.T) {
	got := dockerShellCommand("/usr/bin/docker", []string{"inspect", "-f", "{{.State.Status}}", "name with spaces", "a'b"})
	for _, want := range []string{
		"'/usr/bin/docker'",
		"'{{.State.Status}}'",
		"'name with spaces'",
		"'a'\"'\"'b'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("command %q missing quoted part %q", got, want)
		}
	}
}

func TestHostOwnerEnvArgsIncludesCurrentUserOnLinux(t *testing.T) {
	got := strings.Join(hostOwnerEnvArgs(), " ")
	if got == "" {
		t.Skip("host owner env args are not used on this platform")
	}
	for _, want := range []string{
		"-e",
		"PALPANEL_HOST_UID=",
		"PALPANEL_HOST_GID=",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("host owner args %q missing %q", got, want)
		}
	}
}

func TestHostUserRunArgsRunsContainerAsCurrentUserOnLinux(t *testing.T) {
	got := strings.Join(hostUserRunArgs(), " ")
	if got == "" {
		t.Skip("host user run args are not used on this platform")
	}
	for _, want := range []string{
		"--user",
		"-e HOME=/data/wineprefix",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("host user run args %q missing %q", got, want)
		}
	}
}

func TestRunnerBaseImageCandidatesUsesMirrorPrefixes(t *testing.T) {
	got := runnerBaseImageCandidates(
		"docker.io/scottyhardy/docker-wine:latest@sha256:test",
		[]string{"https://docker.1ms.run/", "docker.1ms.run", "registry.cyou"},
	)
	want := []string{
		"docker.io/scottyhardy/docker-wine:latest@sha256:test",
		"docker.1ms.run/scottyhardy/docker-wine:latest@sha256:test",
		"registry.cyou/scottyhardy/docker-wine:latest@sha256:test",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected candidates:\n%v", got)
	}
}

func TestBuildImageRetriesMirrorCandidateOnDockerHubTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Wine runner")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "docker.log")
	fakeDocker := filepath.Join(dir, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> " + shellQuote(logPath) + "\n" +
		"case \"$*\" in\n" +
		"  *'" + wineBaseImageBuildArg + "=scottyhardy/docker-wine:latest@sha256:test'*) echo 'failed to resolve source metadata: dial tcp: i/o timeout' >&2; exit 1 ;;\n" +
		"  *'" + wineBaseImageBuildArg + "=docker.1ms.run/scottyhardy/docker-wine:latest@sha256:test'*) exit 0 ;;\n" +
		"  *) echo \"unexpected args: $*\" >&2; exit 2 ;;\n" +
		"esac\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(appconfig.Config{
		DockerBinary:                 fakeDocker,
		DockerImage:                  "test-runner:local",
		DockerRunnerBaseImage:        "scottyhardy/docker-wine:latest@sha256:test",
		DockerRunnerBaseImageMirrors: []string{"docker.1ms.run"},
		RunnerDir:                    dir,
	})
	if err := runner.BuildImage(t.Context()); err != nil {
		t.Fatalf("BuildImage returned error: %v", err)
	}
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(body)
	if strings.Count(log, "docker-wine:latest@sha256:test") != 2 {
		t.Fatalf("expected direct and mirror build attempts, got log:\n%s", log)
	}
	if !strings.Contains(log, "docker.1ms.run/scottyhardy/docker-wine") {
		t.Fatalf("expected mirror candidate in log:\n%s", log)
	}
}

func TestBuildImageDoesNotRetryDockerfileSyntaxError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Wine runner")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "docker.log")
	fakeDocker := filepath.Join(dir, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> " + shellQuote(logPath) + "\n" +
		"echo 'Dockerfile parse error' >&2\n" +
		"exit 1\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("not a dockerfile\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(appconfig.Config{
		DockerBinary:                 fakeDocker,
		DockerImage:                  "test-runner:local",
		DockerRunnerBaseImage:        "scottyhardy/docker-wine:latest@sha256:test",
		DockerRunnerBaseImageMirrors: []string{"docker.1ms.run"},
		RunnerDir:                    dir,
	})
	if err := runner.BuildImage(t.Context()); err == nil {
		t.Fatal("expected BuildImage to fail")
	}
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(strings.TrimSpace(string(body)), "\n") + 1; lines != 1 {
		t.Fatalf("expected one build attempt for non-pull error, got %d:\n%s", lines, string(body))
	}
}

func TestStartPublishesManagementPortsOnLoopback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Wine runner")
	}
	root := t.TempDir()
	commandLog := filepath.Join(root, "commands.log")
	fakeDocker := filepath.Join(root, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" >> " + shellQuote(commandLog) + "\n" +
		"if [ \"$1\" = inspect ]; then echo 'No such object' >&2; exit 1; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := appconfig.Config{
		DockerBinary: fakeDocker, DockerImage: "image", DockerContainer: "container",
		ServerDir: filepath.Join(root, "server"), WinePrefixDir: filepath.Join(root, "wineprefix"), LogsDir: filepath.Join(root, "logs"),
		GamePort: 8211, QueryPort: 27015, RCONPort: 25585, RESTPort: 18212, PalDefenderRESTPort: 18080,
	}
	if err := NewRunner(cfg).StartWithArgs(t.Context(), []string{"-port=8211", "-log"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(commandLog)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(body)), "\n")
	for _, expected := range []string{
		"max-size=20m",
		"max-file=5",
		cfg.LogsDir + ":/data/logs",
		"-log",
	} {
		if !containsExact(args, expected) {
			t.Errorf("Docker start arguments missing %q:\n%s", expected, string(body))
		}
	}
	if !containsAdjacent(args, "-e", "WINEDLLOVERRIDES="+wineDLLOverrides) {
		t.Errorf("Docker start arguments do not pass the Wine DLL override as an environment variable:\n%s", string(body))
	}
	for _, binding := range []string{
		"8211:8211/udp",
		"27015:27015/udp",
		"127.0.0.1:18212:18212/tcp",
		"127.0.0.1:25585:25585/tcp",
		"127.0.0.1:18080:18080/tcp",
	} {
		if !containsAdjacent(args, "-p", binding) {
			t.Errorf("Docker start arguments do not publish %q:\n%s", binding, string(body))
		}
	}
	for _, forbidden := range []string{"18212:18212/tcp", "25585:25585/tcp", "18080:18080/tcp"} {
		if containsExact(args, forbidden) {
			t.Errorf("management port was published on all host interfaces as %q:\n%s", forbidden, string(body))
		}
	}
}

func TestWorkshopCredentialsAreNotPlacedInDockerArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Docker runner")
	}
	root := t.TempDir()
	commandLog := filepath.Join(root, "commands.log")
	fakeDocker := filepath.Join(root, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(commandLog) + "\n" +
		"exit 0\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STEAM_USERNAME", "fixture_user")
	t.Setenv("STEAM_PASSWORD", "never-log-this-password")
	runner := NewRunner(appconfig.Config{DockerBinary: fakeDocker, DockerImage: "image", WorkshopAppID: "1623730"})
	if err := runner.DownloadWorkshopTo(t.Context(), "3625364851", filepath.Join(root, "download")); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(commandLog)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(body)), "\n")
	for _, name := range []string{"STEAM_USERNAME", "STEAM_PASSWORD"} {
		if !containsAdjacent(args, "-e", name) {
			t.Errorf("Docker arguments do not pass %s by environment name:\n%s", name, string(body))
		}
	}
	if strings.Contains(string(body), "never-log-this-password") || strings.Contains(string(body), "STEAM_PASSWORD=") {
		t.Fatalf("Docker arguments exposed the Workshop password:\n%s", string(body))
	}
}

func TestManagedInstallProxyIsPassedByEnvironmentNameWithoutCredentialArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Docker runner")
	}
	root := t.TempDir()
	commandLog := filepath.Join(root, "commands.log")
	environmentLog := filepath.Join(root, "environment.log")
	fakeDocker := filepath.Join(root, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(commandLog) + "\n" +
		"if [ -n \"${HTTP_PROXY:-}\" ]; then printf 'configured' > " + shellQuote(environmentLog) + "; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := appconfig.Config{RuntimeRoot: root, DataDir: filepath.Join(root, "data"), DockerBinary: fakeDocker, DockerImage: "image", ServerDir: filepath.Join(root, "server"), WinePrefixDir: filepath.Join(root, "wine")}
	enabled := true
	proxyURL := "http://proxy-user:never-log-proxy-password@127.0.0.1:7890"
	if _, err := networkproxy.New(cfg).Update(networkproxy.ConfigUpdate{InstallEnabled: &enabled, InstallProxyURL: &proxyURL}); err != nil {
		t.Fatal(err)
	}
	if err := NewRunner(cfg).InstallOrUpdate(t.Context()); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(commandLog)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(body)), "\n")
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"} {
		if !containsAdjacent(args, "-e", name) {
			t.Fatalf("Docker arguments do not pass %s by environment name:\n%s", name, body)
		}
	}
	if strings.Contains(string(body), "never-log-proxy-password") || strings.Contains(string(body), "proxy-user") {
		t.Fatalf("Docker arguments exposed proxy credentials:\n%s", body)
	}
	if _, err := os.Stat(environmentLog); err != nil {
		t.Fatalf("managed proxy was not present in Docker command environment: %v", err)
	}
}

func TestWineDLLOverridesPreferNativePalDefenderLoader(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bash fixture exercises the Linux Wine runner")
	}
	entrypoint, err := filepath.Abs(filepath.Join("..", "..", "deployments", "wine-runner", "entrypoint.sh"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		configured string
		want       string
	}{
		{name: "empty", want: wineDLLOverrides},
		{name: "runner value is stable", configured: wineDLLOverrides, want: wineDLLOverrides},
		{name: "preserves unrelated", configured: "xaudio2_7=b", want: "xaudio2_7=b;" + wineDLLOverrides},
		{name: "normalizes separator", configured: "xaudio2_7=b;", want: "xaudio2_7=b;" + wineDLLOverrides},
		{name: "upgrades old default", configured: "dwmapi=n,b", want: wineDLLOverrides},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", `source "$1" source-only; palpanel_wine_dll_overrides "$2"`, "bash", entrypoint, tt.configured)
			cmd.Env = append(os.Environ(), "PALPANEL_ENTRYPOINT_SOURCE_ONLY=1")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("source entrypoint: %v: %s", err, out)
			}
			if got := strings.TrimSpace(string(out)); got != tt.want {
				t.Fatalf("overrides = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWineRunnerEntrypointUsesUnixLineEndings(t *testing.T) {
	entrypoint, err := filepath.Abs(filepath.Join("..", "..", "deployments", "wine-runner", "entrypoint.sh"))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(entrypoint)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "\r\n") {
		t.Fatal("Wine runner entrypoint contains CRLF line endings")
	}
}

func containsExact(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsAdjacent(values []string, first, second string) bool {
	for index := 0; index+1 < len(values); index++ {
		if values[index] == first && values[index+1] == second {
			return true
		}
	}
	return false
}
