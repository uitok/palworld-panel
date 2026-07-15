package steamcmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"palpanel/internal/appconfig"
)

func TestDownloadWorkshopToActivatesOnlyVerifiedResult(t *testing.T) {
	client, cfg := newTestClient(t)
	var captured []string
	client.runCommand = func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		captured = append([]string(nil), args...)
		stage := argumentAfter(t, args, "+force_install_dir")
		item := filepath.Join(stage, "steamapps", "workshop", "content", "1623730", "123456789")
		if err := os.MkdirAll(item, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(item, "Info.json"), []byte(`{"Name":"Fixture","PackageName":"Fixture"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		return []byte("Success. Downloaded item 123456789"), nil
	}

	destination := filepath.Join(cfg.RuntimeRoot, "mods", "staging", "download with space")
	if err := client.DownloadWorkshopTo(t.Context(), "1623730", "123456789", destination, "fixture_user"); err != nil {
		t.Fatalf("DownloadWorkshopTo returned error: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(destination, "123456789", "Info.json")); err != nil || !strings.Contains(string(body), "Fixture") {
		t.Fatalf("activated result = %q, %v", body, err)
	}
	if argumentAfter(t, captured, "+workshop_download_item") != "1623730" {
		t.Fatalf("command args = %#v", captured)
	}
	stages, err := filepath.Glob(filepath.Join(destination, ".steamcmd-workshop-*"))
	if err != nil || len(stages) != 0 {
		t.Fatalf("command staging was retained: %#v, %v", stages, err)
	}
}

func TestDownloadWorkshopToRejectsMissingResultWithoutReplacingPrevious(t *testing.T) {
	client, cfg := newTestClient(t)
	client.runCommand = func(context.Context, string, string, ...string) ([]byte, error) {
		return []byte("Success. Downloaded item 123456789"), nil
	}
	destination := filepath.Join(cfg.RuntimeRoot, "mods", "staging", "missing")
	previous := filepath.Join(destination, "123456789")
	if err := os.MkdirAll(previous, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(previous, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := client.DownloadWorkshopTo(t.Context(), "1623730", "123456789", destination, "fixture_user")
	if err == nil || !strings.Contains(err.Error(), "did not produce a complete Workshop item") {
		t.Fatalf("DownloadWorkshopTo error = %v", err)
	}
	if body, readErr := os.ReadFile(filepath.Join(previous, "keep.txt")); readErr != nil || string(body) != "keep" {
		t.Fatalf("previous item changed after incomplete download: %q, %v", body, readErr)
	}
}

func TestDownloadWorkshopToRejectsZeroExitSteamCMDFailure(t *testing.T) {
	client, cfg := newTestClient(t)
	client.runCommand = func(context.Context, string, string, ...string) ([]byte, error) {
		return []byte("ERROR! Download item 123456789 failed (Failure)."), nil
	}
	destination := filepath.Join(cfg.RuntimeRoot, "mods", "staging", "zero-exit-failure")

	err := client.DownloadWorkshopTo(t.Context(), "1623730", "123456789", destination, "fixture_user")
	if err == nil || !strings.Contains(err.Error(), "failed to download") || !strings.Contains(err.Error(), "decryption key") {
		t.Fatalf("DownloadWorkshopTo error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(destination, "123456789")); !os.IsNotExist(statErr) {
		t.Fatalf("failed item became visible: %v", statErr)
	}
}

func TestDownloadWorkshopToCancellationLeavesNoVisibleItem(t *testing.T) {
	client, cfg := newTestClient(t)
	started := make(chan struct{})
	client.runCommand = func(ctx context.Context, _, _ string, _ ...string) ([]byte, error) {
		close(started)
		<-ctx.Done()
		return []byte("partial"), ctx.Err()
	}
	destination := filepath.Join(cfg.RuntimeRoot, "mods", "staging", "cancel")
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- client.DownloadWorkshopTo(ctx, "1623730", "123456789", destination, "fixture_user")
	}()
	<-started
	cancel()
	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DownloadWorkshopTo error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(destination, "123456789")); !os.IsNotExist(statErr) {
		t.Fatalf("cancelled item became visible: %v", statErr)
	}
}

func TestDownloadWorkshopToAppliesCommandTimeout(t *testing.T) {
	client, cfg := newTestClient(t)
	client.timeout = 30 * time.Millisecond
	client.runCommand = func(ctx context.Context, _, _ string, _ ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	err := client.DownloadWorkshopTo(t.Context(), "1623730", "123456789", filepath.Join(cfg.RuntimeRoot, "mods", "staging", "timeout"), "fixture_user")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("DownloadWorkshopTo error = %v", err)
	}
}

func TestDownloadWorkshopToSerializesClientsSharingSteamCMD(t *testing.T) {
	first, cfg := newTestClient(t)
	second := New(cfg)
	second.goos = "windows"
	second.login = loginState{account: "fixture_user", verifiedAt: time.Now(), credentialsSecure: true}
	var active atomic.Int32
	var maximum atomic.Int32
	run := func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		current := active.Add(1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		defer active.Add(-1)
		time.Sleep(40 * time.Millisecond)
		stage := argumentAfter(t, args, "+force_install_dir")
		values := valuesAfter(args, "+workshop_download_item", 2)
		item := filepath.Join(stage, "steamapps", "workshop", "content", values[0], values[1])
		if err := os.MkdirAll(item, 0o755); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(filepath.Join(item, "Info.json"), []byte(`{"PackageName":"Fixture"}`), 0o644)
	}
	first.runCommand = run
	second.runCommand = run

	var wait sync.WaitGroup
	errorsFound := make(chan error, 2)
	for index, client := range []*Client{first, second} {
		wait.Add(1)
		go func(index int, client *Client) {
			defer wait.Done()
			errorsFound <- client.DownloadWorkshopTo(t.Context(), "1623730", "12345678"+string(rune('0'+index)), filepath.Join(cfg.RuntimeRoot, "mods", "staging", string(rune('a'+index))), "fixture_user")
		}(index, client)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	if maximum.Load() != 1 {
		t.Fatalf("maximum concurrent SteamCMD commands = %d, want 1", maximum.Load())
	}
}

func TestDownloadWorkshopToRejectsEscapingDestinationBeforeCommand(t *testing.T) {
	client, cfg := newTestClient(t)
	called := false
	client.runCommand = func(context.Context, string, string, ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	err := client.DownloadWorkshopTo(t.Context(), "1623730", "123456789", filepath.Join(cfg.RuntimeRoot, "..", "outside"), "fixture_user")
	if err == nil || !strings.Contains(err.Error(), "escapes runtime root") {
		t.Fatalf("DownloadWorkshopTo error = %v", err)
	}
	if called {
		t.Fatal("SteamCMD ran for an escaping destination")
	}
}

func TestDownloadWorkshopToRedactsAccountNameFromErrors(t *testing.T) {
	client, cfg := newTestClient(t)
	client.runCommand = func(context.Context, string, string, ...string) ([]byte, error) {
		return []byte("login failed for fixture_user"), errors.New("exit status 1")
	}
	err := client.DownloadWorkshopTo(t.Context(), "1623730", "123456789", filepath.Join(cfg.RuntimeRoot, "mods", "staging", "secret"), "fixture_user")
	if err == nil || strings.Contains(err.Error(), "fixture_user") || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("redacted error = %v", err)
	}
}

// TestLiveDownloadWorkshop exercises the official SteamCMD binary only when a
// developer explicitly opts in. It keeps the downloaded SteamCMD installation
// in the configured runtime root and removes just its uniquely named fixture.
func TestLiveDownloadWorkshop(t *testing.T) {
	if os.Getenv("PALPANEL_LIVE_STEAMCMD") != "1" {
		t.Skip("set PALPANEL_LIVE_STEAMCMD=1 to run the live SteamCMD Workshop check")
	}
	if runtime.GOOS != "windows" {
		t.Skip("native SteamCMD live check requires Windows")
	}
	cfg, err := appconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	client := New(cfg)
	fixtureRoot := filepath.Join(cfg.RuntimeRoot, "mods", "fixtures", "live-steamcmd-"+time.Now().UTC().Format("20060102T150405"))
	if err := client.validateManaged(fixtureRoot); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.removeManagedDirectory(fixtureRoot) }()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	accountName := strings.TrimSpace(os.Getenv("PALPANEL_LIVE_STEAM_ACCOUNT"))
	if err := ValidateAccountName(accountName); err != nil {
		t.Fatalf("set PALPANEL_LIVE_STEAM_ACCOUNT to the account cached by SteamCMD: %v", err)
	}
	if err := client.DownloadWorkshopTo(ctx, cfg.WorkshopAppID, "3625364851", fixtureRoot, accountName); err != nil {
		t.Fatal(err)
	}
	if err := client.validateDownloadedTree(filepath.Join(fixtureRoot, "3625364851")); err != nil {
		t.Fatalf("live Workshop result validation failed: %v", err)
	}
}

func newTestClient(t *testing.T) (*Client, appconfig.Config) {
	t.Helper()
	runtimeRoot := filepath.Join(t.TempDir(), "runtime with space \u4e2d\u6587")
	cfg := appconfig.Config{
		RuntimeRoot: runtimeRoot,
		DataDir:     filepath.Join(runtimeRoot, "data"), ServerDir: filepath.Join(runtimeRoot, "palworld"),
		ToolsDir: filepath.Join(runtimeRoot, "temp"), SteamCMDDir: filepath.Join(runtimeRoot, "steamcmd"),
		UploadsDir: filepath.Join(runtimeRoot, "mods", "staging"), BackupsDir: filepath.Join(runtimeRoot, "data", "backups"),
		LogsDir: filepath.Join(runtimeRoot, "data", "logs"), DBPath: filepath.Join(runtimeRoot, "data", "database", "panel.db"),
		SaveIndexCacheDir: filepath.Join(runtimeRoot, "data", "save-index"), SteamCMDDownloadMaxBytes: 4 << 20,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.SteamCMDBinaryPath(), []byte("MZ-fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	client := New(cfg)
	client.goos = "windows"
	client.login = loginState{account: "fixture_user", verifiedAt: time.Now(), credentialsSecure: true}
	return client, cfg
}

func argumentAfter(t *testing.T, args []string, name string) string {
	t.Helper()
	values := valuesAfter(args, name, 1)
	if len(values) != 1 {
		t.Fatalf("argument %s not found in %#v", name, args)
	}
	return values[0]
}

func valuesAfter(args []string, name string, count int) []string {
	for index, value := range args {
		if value == name && index+count < len(args) {
			return args[index+1 : index+1+count]
		}
	}
	return nil
}
