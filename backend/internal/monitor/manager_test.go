package monitor

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/palrest"
	"palpanel/internal/server"
)

type fakeStatusServer struct {
	status server.Status
	err    error
}

func (f fakeStatusServer) Status(context.Context) (server.Status, error) {
	return f.status, f.err
}

func TestPruneRemovesExpiredSamples(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "monitor.db"))
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
	defer store.Close()
	for _, sample := range []db.MonitorSample{
		{ID: "expired", CreatedAt: time.Now().UTC().Add(-8 * 24 * time.Hour).Format(time.RFC3339Nano)},
		{ID: "current", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)},
	} {
		if err := store.InsertMonitorSample(t.Context(), sample); err != nil {
			t.Fatalf("InsertMonitorSample returned error: %v", err)
		}
	}
	manager := Manager{cfg: appconfig.Config{MonitorRetentionDays: 7}, store: store}
	if err := manager.Prune(t.Context()); err != nil {
		t.Fatalf("Prune returned error: %v", err)
	}
	samples, err := store.ListMonitorSamples(t.Context(), 10)
	if err != nil || len(samples) != 1 || samples[0].ID != "current" {
		t.Fatalf("unexpected samples: %#v, %v", samples, err)
	}
}

func TestSampleCollectsDockerDiskRESTAndHistory(t *testing.T) {
	root := t.TempDir()
	store, err := db.Open(filepath.Join(root, "monitor.db"))
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
	defer store.Close()
	restServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, password, ok := r.BasicAuth(); !ok || user != "admin" || password != "secret" {
			t.Fatalf("unexpected auth: %q %q %v", user, password, ok)
		}
		_, _ = w.Write([]byte(`{"current_players":3,"max_players":32}`))
	}))
	defer restServer.Close()
	cfg := appconfig.Config{
		DataDir: root, ServerDir: filepath.Join(root, "server"), DockerBinary: "docker",
		DockerContainer: "palworld", MonitorRetentionDays: 7,
	}
	if err := os.MkdirAll(filepath.Dir(cfg.PalWorldSettingsPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `[/Script/Pal.PalGameWorldSettings]
OptionSettings=(AdminPassword="secret",RCONEnabled=False)`
	if err := os.WriteFile(cfg.PalWorldSettingsPath(), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(cfg, store, fakeStatusServer{status: server.Status{
		RuntimeMode: server.RuntimeWineDocker,
		Container:   docker.ContainerStatus{Exists: true, Status: "running"},
	}}, palrest.New(restServer.URL, "admin", ""))
	manager.diskUsage = func(string) (int64, int64, error) {
		return 800 * 1024, 1000 * 1024, nil
	}
	manager.run = func(_ context.Context, name string, _ ...string) ([]byte, error) {
		switch name {
		case "docker":
			return []byte("12.5%|1.5GiB / 4GiB\n"), nil
		default:
			return nil, errors.New("unexpected command: " + name)
		}
	}
	fixed := time.Date(2026, 7, 14, 1, 2, 3, 0, time.UTC)
	manager.now = func() time.Time { return fixed }

	sample, err := manager.Sample(t.Context())
	if err != nil {
		t.Fatalf("Sample returned error: %v", err)
	}
	if !sample.CPUAvailable || sample.CPUPercent != 12.5 || sample.MemoryUsageBytes != 1610612736 || sample.MemoryLimitBytes != 4294967296 {
		t.Fatalf("unexpected process stats: %#v", sample)
	}
	if !sample.DiskAvailable || sample.DiskFreeBytes != 800*1024 || sample.CurrentPlayers != 3 || sample.MaxPlayers != 32 || !sample.RESTHealthy {
		t.Fatalf("unexpected sample: %#v", sample)
	}
	snapshot, err := manager.Snapshot(t.Context())
	if err != nil || snapshot.Sample.ID != sample.ID {
		t.Fatalf("Snapshot = %#v, %v", snapshot, err)
	}
	history, err := manager.History(t.Context(), 10)
	if err != nil || len(history) != 1 {
		t.Fatalf("History = %#v, %v", history, err)
	}
}

func TestSampleCollectsWindowsStatsAndCanDisableHistory(t *testing.T) {
	root := t.TempDir()
	store, err := db.Open(filepath.Join(root, "monitor.db"))
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
	defer store.Close()
	if err := store.SetKV(t.Context(), "windows_pid", "123"); err != nil {
		t.Fatal(err)
	}
	restServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"currentPlayerNum":"2","maxPlayerNum":16}`))
	}))
	defer restServer.Close()
	cfg := appconfig.Config{DataDir: `C:\data`, ServerDir: filepath.Join(root, "server"), MonitorRetentionDays: 0}
	manager := New(cfg, store, fakeStatusServer{status: server.Status{
		RuntimeMode: server.RuntimeWindowsSteamCMD,
		Container:   docker.ContainerStatus{Exists: true, Status: "running"},
	}}, palrest.New(restServer.URL, "", ""))
	manager.goos = "windows"
	manager.diskUsage = func(string) (int64, int64, error) { return 100, 1000, nil }
	manager.run = func(_ context.Context, name string, _ ...string) ([]byte, error) {
		switch name {
		case "tasklist":
			return []byte(`"PalServer.exe","123","Console","1","1,024 K"` + "\n"), nil
		default:
			return nil, errors.New("unexpected command")
		}
	}
	sample, err := manager.Sample(t.Context())
	if err != nil || sample.MemoryUsageBytes != 1024*1024 || !sample.DiskAvailable || sample.DiskFreeBytes != 100 {
		t.Fatalf("Sample = %#v, %v", sample, err)
	}
	history, err := manager.History(t.Context(), 10)
	if err != nil || len(history) != 0 {
		t.Fatalf("history should be disabled: %#v, %v", history, err)
	}
	snapshot, err := manager.Snapshot(t.Context())
	if err != nil || snapshot.Sample.ID == "" {
		t.Fatalf("Snapshot = %#v, %v", snapshot, err)
	}
}

func TestRCONAndFailureReasons(t *testing.T) {
	root := t.TempDir()
	store, err := db.Open(filepath.Join(root, "monitor.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := appconfig.Config{DataDir: root, ServerDir: filepath.Join(root, "server")}
	if err := os.MkdirAll(filepath.Dir(cfg.PalWorldSettingsPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `[/Script/Pal.PalGameWorldSettings]
OptionSettings=(RCONEnabled=True,RCONPort=25575)`
	if err := os.WriteFile(cfg.PalWorldSettingsPath(), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(cfg, store, fakeStatusServer{err: errors.New("status unavailable")}, palrest.New("http://127.0.0.1:1", "", ""))
	manager.diskUsage = func(string) (int64, int64, error) { return 0, 0, errors.New("failed") }
	manager.run = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("df failed"), errors.New("failed")
	}
	manager.dial = func(_, _ string, _ time.Duration) (net.Conn, error) {
		client, peer := net.Pipe()
		go func() { _ = peer.Close() }()
		return client, nil
	}
	sample, err := manager.Sample(t.Context())
	if err != nil || !sample.RCONHealthy || !strings.Contains(sample.UnavailableReason, "status unavailable") || !strings.Contains(sample.UnavailableReason, "disk:") || !strings.Contains(sample.UnavailableReason, "REST:") {
		t.Fatalf("Sample = %#v, %v", sample, err)
	}

	sample = db.MonitorSample{}
	manager.run = func(_ context.Context, _ string, _ ...string) ([]byte, error) { return []byte("bad output"), nil }
	manager.fillDockerStats(t.Context(), &sample)
	if !strings.Contains(sample.UnavailableReason, "unexpected output") {
		t.Fatalf("unexpected reason: %q", sample.UnavailableReason)
	}
}

func TestHelpersAndBackgroundLoop(t *testing.T) {
	for raw, want := range map[string]int64{
		"1KiB":  1024,
		"1.5MB": 1500000,
		"42":    42,
	} {
		got, ok := parseDockerBytes(raw)
		if !ok || got != want {
			t.Fatalf("parseDockerBytes(%q) = %d, %v", raw, got, ok)
		}
	}
	if _, ok := parseDockerBytes("bad"); ok {
		t.Fatal("expected invalid byte value")
	}
	data := map[string]any{"float": float64(2), "int": 3, "string": "4.5"}
	if numberValue(data, "float") != 2 || numberValue(data, "int") != 3 || numberValue(data, "string") != 4.5 || numberValue(nil, "missing") != 0 {
		t.Fatalf("unexpected number conversion")
	}
	if len(nonEmptyLines(" a \n\n b ")) != 2 || len(mapFromAny(nil)) != 0 || len(mapFromAny("bad")) != 0 {
		t.Fatal("unexpected helper output")
	}

	root := t.TempDir()
	store, err := db.Open(filepath.Join(root, "loop.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := New(appconfig.Config{DataDir: root, ServerDir: filepath.Join(root, "server"), MonitorRetentionDays: 0}, store, fakeStatusServer{}, palrest.New("http://127.0.0.1:1", "", ""))
	manager.diskUsage = func(string) (int64, int64, error) { return 0, 0, errors.New("unavailable") }
	manager.run = func(_ context.Context, _ string, _ ...string) ([]byte, error) { return nil, errors.New("unavailable") }
	ctx, cancel := context.WithCancel(t.Context())
	done := manager.Start(ctx)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("monitor loop did not stop")
	}
}
