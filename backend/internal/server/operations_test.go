package server

import (
	"archive/zip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/palconfig"
)

func TestManagerLifecycleConfigurationAndBackupWorkflow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is for POSIX CI")
	}
	manager, cleanup := newOperationsManager(t)
	defer cleanup()
	ctx := t.Context()

	if err := manager.SetRuntimeMode(ctx, RuntimeWineDocker); err != nil {
		t.Fatalf("SetRuntimeMode returned error: %v", err)
	}
	if mode, err := manager.RuntimeMode(ctx); err != nil || mode != RuntimeWineDocker {
		t.Fatalf("RuntimeMode = %q, %v", mode, err)
	}
	startup := StartupConfig{Port: 8211, Players: 16, LogFormat: "text"}
	if _, err := manager.SetStartupConfig(ctx, startup); err != nil {
		t.Fatalf("SetStartupConfig returned error: %v", err)
	}
	if got, err := manager.StartupConfig(ctx); err != nil || got.Port != 8211 || got.Players != 16 {
		t.Fatalf("StartupConfig = %#v, %v", got, err)
	}
	if encoded, err := EncodeStartupConfig(startup, manager.cfg); err != nil {
		t.Fatalf("EncodeStartupConfig returned error: %v", err)
	} else if decoded := DecodeStartupConfig(encoded, manager.cfg); decoded.Port != startup.Port {
		t.Fatalf("DecodeStartupConfig = %#v", decoded)
	}
	if decoded := DecodeStartupConfig("{", manager.cfg); decoded.Port != manager.cfg.GamePort {
		t.Fatalf("invalid startup JSON should use defaults: %#v", decoded)
	}

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := manager.Restart(ctx); err != nil {
		t.Fatalf("Restart returned error: %v", err)
	}
	if err := manager.Stop(ctx); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if err := manager.MarkPendingRestart(ctx); err != nil {
		t.Fatalf("MarkPendingRestart returned error: %v", err)
	}
	prerequisites, err := manager.Prerequisites(ctx)
	if err != nil || len(prerequisites) == 0 {
		t.Fatalf("Prerequisites = %#v, %v", prerequisites, err)
	}

	writeFile(t, filepath.Join(manager.cfg.ServerDir, "Pal", "Saved", "SaveGames", "0", "world", "Level.sav"), "world-data")
	job, err := manager.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup returned error: %v", err)
	}
	completed := waitForJob(t, manager.store, job.ID)
	if completed.Status != "completed" {
		t.Fatalf("backup job = %#v", completed)
	}
	backups, err := manager.ListBackups()
	if err != nil || len(backups) != 1 {
		t.Fatalf("ListBackups = %#v, %v", backups, err)
	}
	name := backups[0].Name
	if result, err := manager.VerifyBackup(name); err != nil || !result.Valid || result.CheckedFiles == 0 {
		t.Fatalf("VerifyBackup = %#v, %v", result, err)
	}
	if path, err := manager.BackupDownloadPath(name); err != nil || path == "" {
		t.Fatalf("BackupDownloadPath = %q, %v", path, err)
	}
	writeFile(t, filepath.Join(manager.cfg.ServerDir, "Pal", "Saved", "SaveGames", "0", "world", "Level.sav"), "changed")
	restoreJob, err := manager.RestoreBackup(ctx, name)
	if err != nil {
		t.Fatalf("RestoreBackup returned error: %v", err)
	}
	restored := waitForJob(t, manager.store, restoreJob.ID)
	if restored.Status != "completed" {
		t.Fatalf("restore job = %#v", restored)
	}
	body, err := os.ReadFile(filepath.Join(manager.cfg.ServerDir, "Pal", "Saved", "SaveGames", "0", "world", "Level.sav"))
	if err != nil || string(body) != "world-data" {
		t.Fatalf("restored world = %q, %v", body, err)
	}
	if err := manager.DeleteBackup(name); err != nil {
		t.Fatalf("DeleteBackup returned error: %v", err)
	}
	if err := manager.DeleteBackup(name); err == nil {
		t.Fatal("expected missing backup delete to fail")
	}
	if _, err := manager.BackupDownloadPath("../unsafe.zip"); err == nil {
		t.Fatal("expected unsafe backup path to fail")
	}
}

func TestDockerHostPlansAndHelpers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is for POSIX CI")
	}
	manager, cleanup := newOperationsManager(t)
	defer cleanup()
	restoreSource := stubDockerProbe(func(_ context.Context, rawURL string) sourceProbeResult {
		return sourceProbeResult{Available: strings.Contains(rawURL, "download.docker.com"), Latency: time.Millisecond}
	})
	defer restoreSource()
	restoreMirror := stubDockerMirrorProbe(func(_ context.Context, rawURL string) sourceProbeResult {
		return sourceProbeResult{Available: strings.Contains(rawURL, "1ms"), Latency: time.Millisecond}
	})
	defer restoreMirror()

	host := manager.HostCapabilities(t.Context())
	if host.OS == "" || host.Arch == "" || host.Docker.CLIPath == "" {
		t.Fatalf("HostCapabilities = %#v", host)
	}
	plan, err := manager.DockerInstallPlan(t.Context(), "official")
	if err != nil || plan.Source != "official" || plan.Script == "" || plan.ScriptPath == "" {
		t.Fatalf("DockerInstallPlan = %#v, %v", plan, err)
	}
	if _, err := manager.InstallDocker(t.Context(), DockerInstallRequest{Source: "official"}); err == nil {
		t.Fatal("expected automatic Docker install to be unavailable in test host")
	} else {
		var installErr *DockerInstallError
		if !errors.As(err, &installErr) || installErr.Error() == "" || dockerInstallErr(400, "test", "message").Error() != "message" {
			t.Fatalf("unexpected install error: %v", err)
		}
	}
	mirrorPlan, err := manager.DockerMirrorPlan(t.Context(), "one_ms")
	if err != nil || mirrorPlan.Mirror != "one_ms" {
		t.Fatalf("DockerMirrorPlan = %#v, %v", mirrorPlan, err)
	}
	// The stub marks this specific mirror unavailable, so configuration must
	// fail before any host mutation even when the test itself runs as root.
	if _, err := manager.ConfigureDockerMirrors(t.Context(), DockerMirrorRequest{Mirror: "daocloud"}); err == nil {
		t.Fatal("expected mirror configuration to be unavailable in test host")
	}

	if got := normalizeDockerMirror("unknown"); got != "auto" {
		t.Fatalf("normalizeDockerMirror = %q", got)
	}
	if ok, code, _ := dockerMirrorSupport(HostCapabilities{OS: "windows"}); ok || code != "unsupported_os" {
		t.Fatalf("unexpected mirror support: %v %q", ok, code)
	}
	if ok, code, _ := dockerMirrorSupport(HostCapabilities{OS: "linux"}); ok || code != "docker_not_installed" {
		t.Fatalf("unexpected mirror support: %v %q", ok, code)
	}
	if got := shellQuote("a'b"); got != `'a'"'"'b'` || boolInt(true) != 1 || boolInt(false) != 0 {
		t.Fatalf("unexpected shell helpers: %q", got)
	}
	if got := limitString("  abcdef  ", 3); got != "abc...(truncated)" {
		t.Fatalf("limitString = %q", got)
	}
}

func TestDockerMirrorParsingAndNetworkProbes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "daemon.json")
	if mirrors, err := readDockerDaemonMirrors(path); err != nil || mirrors != nil {
		t.Fatalf("missing mirrors = %#v, %v", mirrors, err)
	}
	if err := os.WriteFile(path, []byte(`{"registry-mirrors":["https://one.example/",12," "]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	mirrors, err := readDockerDaemonMirrors(path)
	if err != nil || len(mirrors) != 1 || mirrors[0] != "https://one.example" {
		t.Fatalf("readDockerDaemonMirrors = %#v, %v", mirrors, err)
	}
	if _, err := parseDockerDaemonMirrors([]byte("{")); err == nil {
		t.Fatal("expected invalid daemon JSON")
	}
	if mirrors, err := parseDockerDaemonMirrors(nil); err != nil || mirrors != nil {
		t.Fatalf("empty daemon config = %#v, %v", mirrors, err)
	}
	if _, _, err := mergeDockerDaemonMirrors([]byte("{"), nil); err == nil {
		t.Fatal("expected invalid daemon merge")
	}
	merged := mergeMirrorLists([]string{"https://one/", ""}, []string{"https://one", "https://two"})
	if len(merged) != 2 || minInt(1, 2) != 1 {
		t.Fatalf("mergeMirrorLists = %#v", merged)
	}

	probeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/" {
			t.Fatalf("probe path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer probeServer.Close()
	if result := probeDockerRegistryMirror(t.Context(), probeServer.URL); !result.Available {
		t.Fatalf("mirror probe = %#v", result)
	}
	if result := probeDockerRegistryMirror(t.Context(), ""); result.Error == "" {
		t.Fatal("expected empty mirror error")
	}
	if status, err := probeURL(t.Context(), http.MethodGet, probeServer.URL+"/v2/"); err != nil || status != http.StatusUnauthorized {
		t.Fatalf("probeURL = %d, %v", status, err)
	}
}

func TestArchiveAndFileHelpersRejectUnsafeInput(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "unsafe.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, _ := writer.Create("../escape.txt")
	_, _ = entry.Write([]byte("bad"))
	_ = writer.Close()
	_ = file.Close()
	if err := extractZipSafe(archive, filepath.Join(root, "out")); err == nil {
		t.Fatal("expected unsafe archive to fail")
	}
	if !unsafeZipName("../x") || !unsafeZipName("/absolute") || unsafeZipName("safe/file") {
		t.Fatal("unexpected unsafeZipName result")
	}
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "nested", "destination")
	if err := os.WriteFile(source, []byte("copy"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(source, destination); err != nil {
		t.Fatalf("copyFile returned error: %v", err)
	}
	if body, err := os.ReadFile(destination); err != nil || string(body) != "copy" {
		t.Fatalf("copied file = %q, %v", body, err)
	}
	logPath := filepath.Join(root, "log")
	if err := os.WriteFile(logPath, []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if tail, err := tailFile(logPath, 2); err != nil || tail != "three\n" {
		t.Fatalf("tailFile = %q, %v", tail, err)
	}
}

func newOperationsManager(t *testing.T) (Manager, func()) {
	t.Helper()
	root := t.TempDir()
	fakeDocker := filepath.Join(root, "docker")
	script := `#!/bin/sh
case "$1" in
  --version) echo "Docker version 28.0.0" ;;
  version) echo "28.0.0" ;;
  info) echo "Server Version: 28.0.0" ;;
  inspect) echo "running" ;;
  *) exit 0 ;;
esac`
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := appconfig.Config{
		DataDir: root, ServerDir: filepath.Join(root, "server"), WinePrefixDir: filepath.Join(root, "wine"),
		ToolsDir: filepath.Join(root, "tools"), SteamCMDDir: filepath.Join(root, "steamcmd"), UploadsDir: filepath.Join(root, "uploads"),
		BackupsDir: filepath.Join(root, "backups"), LogsDir: filepath.Join(root, "logs"), DBPath: filepath.Join(root, "test.db"),
		SaveIndexCacheDir: filepath.Join(root, "save-index"), DockerBinary: fakeDocker, DockerImage: "image", DockerContainer: "container",
		GamePort: 8211, QueryPort: 27015, RESTPort: 8212,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, cfg.PalServerExePath(), "binary")
	if err := os.MkdirAll(filepath.Dir(cfg.PalWorldSettingsPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := palconfig.Defaults()
	settings["RESTAPIEnabled"] = "True"
	settings["AdminPassword"] = "secret"
	if err := palconfig.Write(cfg.PalWorldSettingsPath(), settings); err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(cfg, store, docker.NewRunner(cfg))
	return manager, func() { _ = store.Close() }
}
