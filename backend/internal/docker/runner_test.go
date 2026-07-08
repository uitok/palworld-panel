package docker

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"palpanel/internal/appconfig"
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
