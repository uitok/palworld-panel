package docker

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/networkproxy"
	"palpanel/internal/steamcmd"
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

func TestWorkshopUsesPersistedSteamCMDCacheWithoutPasswordArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Docker runner")
	}
	root := t.TempDir()
	commandLog := filepath.Join(root, "commands.log")
	environmentLog := filepath.Join(root, "environment.log")
	fakeDocker := filepath.Join(root, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(commandLog) + "\n" +
		"env > " + shellQuote(environmentLog) + "\n" +
		"exit 0\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(appconfig.Config{RuntimeRoot: root, DataDir: filepath.Join(root, "data"), DockerBinary: fakeDocker, DockerImage: "image", WorkshopAppID: "1623730"})
	if err := runner.DownloadWorkshopTo(t.Context(), "3625364851", filepath.Join(root, "download"), "fixture_user"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(commandLog)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(body)), "\n")
	if !containsExact(args, "fixture_user") || !containsExact(args, "steam-auth-verify") && !containsExact(args, "workshop") {
		t.Errorf("Docker arguments do not use the account with the Workshop command:\n%s", string(body))
	}
	for _, forbidden := range []string{"STEAM_USERNAME", "STEAM_PASSWORD", "never-log-this-password"} {
		if strings.Contains(strings.ToLower(string(body)), strings.ToLower(forbidden)) {
			t.Fatalf("Docker arguments contain forbidden credential material %q:\n%s", forbidden, string(body))
		}
	}
	wantMount := volume(runner.cfg.WorkshopSteamCMDConfigDir(), workshopSteamHomePath)
	if !containsExact(args, wantMount) {
		t.Fatalf("Docker arguments do not mount the persistent SteamCMD cache %q:\n%s", wantMount, string(body))
	}
	if containsExact(args, volume(runner.cfg.WorkshopSteamCMDConfigDir(), "/opt/steamcmd/config")) {
		t.Fatalf("Docker arguments still mount the cache beside the SteamCMD installation:\n%s", string(body))
	}
}

func TestWorkshopAuthenticationUsesTemporaryRunscriptWithoutSecretArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Docker runner")
	}
	root := t.TempDir()
	commandLog := filepath.Join(root, "commands.log")
	scriptLog := filepath.Join(root, "script.log")
	fakeDocker := filepath.Join(root, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(commandLog) + "\n" +
		"if [ \"${1:-}\" = image ]; then exit 0; fi\n" +
		"for arg in \"$@\"; do case \"$arg\" in *:/run/palpanel/steam-login.txt:ro) host=${arg%:/run/palpanel/steam-login.txt:ro}; cp \"$host\" " + shellQuote(scriptLog) + ";; esac; done\n" +
		"printf 'Logged in OK\\n'\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := appconfig.Config{RuntimeRoot: root, DataDir: filepath.Join(root, "data"), DockerBinary: fakeDocker, DockerImage: "image"}
	runner := NewRunner(cfg)
	password := `space quote" slash\linux-secret`
	guard := "654321"
	out, err := runner.AuthenticateWorkshop(t.Context(), steamcmd.LoginRequest{AccountName: "fixture_user", Password: password, SteamGuardCode: guard})
	if err != nil || !strings.Contains(string(out), "Logged in OK") {
		t.Fatalf("AuthenticateWorkshop output = %q, error = %v", out, err)
	}
	arguments, err := os.ReadFile(commandLog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(arguments), password) || strings.Contains(string(arguments), guard) {
		t.Fatalf("Docker arguments exposed Steam secrets: %s", arguments)
	}
	if !strings.Contains(string(arguments), volume(cfg.WorkshopSteamCMDConfigDir(), workshopSteamHomePath)) {
		t.Fatalf("Docker login does not persist the Linux Steam home: %s", arguments)
	}
	runscript, err := os.ReadFile(scriptLog)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(runscript), steamScriptArgument(password)) || !strings.Contains(string(runscript), steamScriptArgument(guard)) {
		t.Fatalf("temporary runscript did not safely quote credentials: %s", runscript)
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(cfg.SteamWorkshopCredentialsPath()), "steamcmd-docker-login-*.txt"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary Docker login scripts remain: %#v, %v", leftovers, err)
	}
}

func TestWorkshopAuthenticationCleansTemporaryRunscriptAfterDockerFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Docker runner")
	}
	root := t.TempDir()
	fakeDocker := filepath.Join(root, "docker")
	script := "#!/bin/sh\n" +
		"if [ \"${1:-}\" = image ]; then exit 0; fi\n" +
		"printf 'transport failed\\n'\n" +
		"exit 7\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := appconfig.Config{RuntimeRoot: root, DataDir: filepath.Join(root, "data"), DockerBinary: fakeDocker, DockerImage: "image"}
	runner := NewRunner(cfg)
	if _, err := runner.AuthenticateWorkshop(t.Context(), steamcmd.LoginRequest{AccountName: "fixture_user", Password: "failure secret"}); err == nil {
		t.Fatal("AuthenticateWorkshop unexpectedly succeeded")
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(cfg.SteamWorkshopCredentialsPath()), "steamcmd-docker-login-*.txt"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary Docker login scripts remain after failure: %#v, %v", leftovers, err)
	}
}

func TestWorkshopDownloadClassifiesExpiredDockerCache(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Docker runner")
	}
	root := t.TempDir()
	fakeDocker := filepath.Join(root, "docker")
	script := "#!/bin/sh\n" +
		"printf 'FAILED (No cached credentials and @NoPromptForPassword is set)\\n'\n" +
		"exit 3\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(appconfig.Config{RuntimeRoot: root, DataDir: filepath.Join(root, "data"), DockerBinary: fakeDocker, DockerImage: "image", WorkshopAppID: "1623730"})
	err := runner.DownloadWorkshopTo(t.Context(), "3625364851", filepath.Join(root, "download"), "fixture_user")
	if !errors.Is(err, steamcmd.ErrLoginRequired) {
		t.Fatalf("expired Docker cache error = %v", err)
	}
}

func TestManagedInstallProxyUsesLoopbackBridgeAndHostNetworkWithoutCredentialArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Docker runner")
	}
	root := t.TempDir()
	commandLog := filepath.Join(root, "commands.log")
	environmentLog := filepath.Join(root, "environment.log")
	fakeDocker := filepath.Join(root, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(commandLog) + "\n" +
		"env > " + shellQuote(environmentLog) + "\n" +
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
	if !containsAdjacent(args, "--network", "host") {
		t.Fatalf("Docker arguments do not enable host networking:\n%s", body)
	}
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		found := false
		for _, arg := range args {
			if strings.HasPrefix(arg, name+"=http://127.0.0.1:") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Docker arguments do not pass bridged %s explicitly:\n%s", name, body)
		}
	}
	if strings.Contains(string(body), "never-log-proxy-password") || strings.Contains(string(body), "proxy-user") {
		t.Fatalf("Docker arguments exposed proxy credentials:\n%s", body)
	}
	environment, err := os.ReadFile(environmentLog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(environment), "never-log-proxy-password") || strings.Contains(string(environment), "proxy-user") {
		t.Fatalf("Docker process environment exposed upstream proxy credentials:\n%s", environment)
	}
	bridgeAddress := ""
	for _, arg := range args {
		if strings.HasPrefix(arg, "HTTP_PROXY=http://127.0.0.1:") {
			bridgeAddress = strings.TrimPrefix(arg, "HTTP_PROXY=http://")
			break
		}
	}
	if bridgeAddress == "" {
		t.Fatal("bridged proxy address was not captured")
	}
	if connection, err := net.DialTimeout("tcp", bridgeAddress, 200*time.Millisecond); err == nil {
		_ = connection.Close()
		t.Fatalf("temporary proxy bridge %s remained open after Docker command", bridgeAddress)
	}
}

func TestWineImageBuildUsesProxyAwareRunDownload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Docker runner")
	}
	root := t.TempDir()
	commandLog := filepath.Join(root, "commands.log")
	fakeDocker := filepath.Join(root, "docker")
	if err := os.WriteFile(fakeDocker, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+shellQuote(commandLog)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runnerDir := filepath.Join(root, "runner")
	if err := os.MkdirAll(runnerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runnerDir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := appconfig.Config{RuntimeRoot: root, DataDir: filepath.Join(root, "data"), DockerBinary: fakeDocker, DockerImage: "image", RunnerDir: runnerDir}
	enabled := true
	proxyURL := "socks5://proxy-user:never-log-proxy-password@127.0.0.1:10808"
	if _, err := networkproxy.New(cfg).Update(networkproxy.ConfigUpdate{InstallEnabled: &enabled, InstallProxyURL: &proxyURL}); err != nil {
		t.Fatal(err)
	}
	if err := NewRunner(cfg).BuildImage(t.Context()); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(commandLog)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(body)), "\n")
	if !containsAdjacent(args, "--network", "host") {
		t.Fatalf("build did not enable host networking:\n%s", body)
	}
	if strings.Contains(string(body), "never-log-proxy-password") || strings.Contains(string(body), "proxy-user") {
		t.Fatalf("build arguments exposed proxy credentials:\n%s", body)
	}
	foundHTTPProxy := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "HTTP_PROXY=http://127.0.0.1:") {
			foundHTTPProxy = true
		}
	}
	if !foundHTTPProxy {
		t.Fatalf("build did not receive the local HTTP bridge:\n%s", body)
	}

	dockerfile, err := os.ReadFile(filepath.Join("..", "..", "deployments", "wine-runner", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(dockerfile), "ADD https://") || !strings.Contains(string(dockerfile), "curl --fail") {
		t.Fatalf("Wine runner Dockerfile must use a proxy-aware RUN download:\n%s", dockerfile)
	}
}

func TestCanceledDockerDownloadClosesProxyBridge(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture exercises the Linux Docker runner")
	}
	root := t.TempDir()
	commandLog := filepath.Join(root, "commands.log")
	fakeDocker := filepath.Join(root, "docker")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellQuote(commandLog) + "\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := appconfig.Config{RuntimeRoot: root, DataDir: filepath.Join(root, "data"), DockerBinary: fakeDocker, DockerImage: "image", ServerDir: filepath.Join(root, "server"), WinePrefixDir: filepath.Join(root, "wine")}
	enabled := true
	proxyURL := "socks5h://proxy-user:never-log-proxy-password@127.0.0.1:10808"
	if _, err := networkproxy.New(cfg).Update(networkproxy.ConfigUpdate{InstallEnabled: &enabled, InstallProxyURL: &proxyURL}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- NewRunner(cfg).InstallOrUpdate(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(commandLog); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("fake Docker command did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if err := <-done; err == nil {
		t.Fatal("canceled Docker command unexpectedly succeeded")
	}
	body, err := os.ReadFile(commandLog)
	if err != nil {
		t.Fatal(err)
	}
	bridgeAddress := ""
	for _, arg := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if strings.HasPrefix(arg, "HTTP_PROXY=http://127.0.0.1:") {
			bridgeAddress = strings.TrimPrefix(arg, "HTTP_PROXY=http://")
			break
		}
	}
	if bridgeAddress == "" {
		t.Fatalf("bridge address missing from canceled command:\n%s", body)
	}
	if connection, err := net.DialTimeout("tcp", bridgeAddress, 200*time.Millisecond); err == nil {
		_ = connection.Close()
		t.Fatalf("temporary proxy bridge %s remained open after cancellation", bridgeAddress)
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
	if strings.Contains(string(body), "/opt/steamcmd/config") {
		t.Fatal("Wine runner persists login state beside the SteamCMD installation instead of under $HOME/Steam")
	}
	if !strings.Contains(string(body), `steam_home="${HOME:-/root}/Steam"`) {
		t.Fatal("Wine runner does not define the persistent Linux Steam home")
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
