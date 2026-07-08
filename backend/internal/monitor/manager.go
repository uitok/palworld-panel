package monitor

import (
	"context"
	"encoding/csv"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/id"
	"palpanel/internal/palconfig"
	"palpanel/internal/palrest"
	"palpanel/internal/server"
)

type Manager struct {
	cfg     appconfig.Config
	store   *db.Store
	server  server.Manager
	palrest palrest.Client
}

type Snapshot struct {
	Sample db.MonitorSample `json:"sample"`
}

func New(cfg appconfig.Config, store *db.Store, serverManager server.Manager, restClient palrest.Client) Manager {
	return Manager{cfg: cfg, store: store, server: serverManager, palrest: restClient}
}

func (m Manager) Start(ctx context.Context) {
	go func() {
		_, _ = m.Sample(context.Background())
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = m.Sample(context.Background())
			}
		}
	}()
}

func (m Manager) Snapshot(ctx context.Context) (Snapshot, error) {
	samples, err := m.store.ListMonitorSamples(ctx, 1)
	if err != nil {
		return Snapshot{}, err
	}
	if len(samples) == 0 {
		sample, err := m.Sample(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		return Snapshot{Sample: sample}, nil
	}
	return Snapshot{Sample: samples[len(samples)-1]}, nil
}

func (m Manager) History(ctx context.Context, limit int) ([]db.MonitorSample, error) {
	return m.store.ListMonitorSamples(ctx, limit)
}

func (m Manager) Sample(ctx context.Context) (db.MonitorSample, error) {
	sample := db.MonitorSample{ID: id.New("mon"), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	status, err := m.server.Status(ctx)
	if err != nil {
		sample.UnavailableReason = err.Error()
	} else {
		sample.GamePortHealthy = status.Container.Status == "running"
		sample.QueryPortHealthy = status.Container.Status == "running"
		if status.RuntimeMode == server.RuntimeWineDocker {
			m.fillDockerStats(ctx, &sample)
		} else {
			m.fillWindowsProcessStats(ctx, &sample)
		}
	}
	m.fillDiskStats(ctx, &sample)
	m.fillRESTStats(ctx, &sample)
	m.fillRCONHealth(ctx, &sample)
	if err := m.store.InsertMonitorSample(ctx, sample); err != nil {
		return sample, err
	}
	return sample, nil
}

func (m Manager) fillRESTStats(ctx context.Context, sample *db.MonitorSample) {
	resp, err := m.palrest.Do(ctx, "GET", "metrics", nil)
	if err != nil {
		sample.RESTHealthy = false
		appendReason(sample, "REST: "+err.Error())
		return
	}
	sample.RESTHealthy = resp.Status < 400
	data := mapFromAny(resp.Body)
	sample.CurrentPlayers = int(numberValue(data, "current_players", "currentPlayerNum", "currentplayernum", "players"))
	sample.MaxPlayers = int(numberValue(data, "max_players", "maxPlayerNum", "maxplayernum"))
}

func (m Manager) fillRCONHealth(ctx context.Context, sample *db.MonitorSample) {
	settings, err := palconfig.Read(m.cfg.PalWorldSettingsPath())
	if err != nil || !strings.EqualFold(settings["RCONEnabled"], "True") {
		sample.RCONHealthy = false
		return
	}
	port := settings["RCONPort"]
	if strings.TrimSpace(port) == "" {
		port = "25575"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 2*time.Second)
	if err != nil {
		sample.RCONHealthy = false
		appendReason(sample, "RCON: "+err.Error())
		return
	}
	_ = conn.Close()
	sample.RCONHealthy = true
}

func (m Manager) fillDockerStats(ctx context.Context, sample *db.MonitorSample) {
	cmd := exec.CommandContext(ctx, m.cfg.DockerBinary, "stats", "--no-stream", "--format", "{{.CPUPerc}}|{{.MemUsage}}", m.cfg.DockerContainer)
	out, err := cmd.CombinedOutput()
	if err != nil {
		appendReason(sample, "docker stats: "+strings.TrimSpace(string(out)))
		return
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) != 2 {
		appendReason(sample, "docker stats: unexpected output")
		return
	}
	cpu := strings.TrimSuffix(strings.TrimSpace(parts[0]), "%")
	if v, err := strconv.ParseFloat(cpu, 64); err == nil {
		sample.CPUAvailable = true
		sample.CPUPercent = v
	}
	memParts := strings.Split(parts[1], "/")
	if len(memParts) == 2 {
		if usage, ok := parseDockerBytes(strings.TrimSpace(memParts[0])); ok {
			sample.MemoryAvailable = true
			sample.MemoryUsageBytes = usage
		}
		if limit, ok := parseDockerBytes(strings.TrimSpace(memParts[1])); ok {
			sample.MemoryLimitBytes = limit
		}
	}
}

func (m Manager) fillWindowsProcessStats(ctx context.Context, sample *db.MonitorSample) {
	pid, _, err := m.store.GetKV(ctx, "windows_pid")
	if err != nil || strings.TrimSpace(pid) == "" {
		appendReason(sample, "windows process: pid unavailable")
		return
	}
	cmd := exec.CommandContext(ctx, "tasklist", "/FI", "PID eq "+pid, "/FO", "CSV", "/NH")
	out, err := cmd.CombinedOutput()
	if err != nil {
		appendReason(sample, "tasklist: "+strings.TrimSpace(string(out)))
		return
	}
	rows, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	if err != nil || len(rows) == 0 || len(rows[0]) < 5 {
		appendReason(sample, "tasklist: process not found")
		return
	}
	kb := strings.NewReplacer(",", "", " K", "", " ", "").Replace(rows[0][4])
	if value, err := strconv.ParseInt(kb, 10, 64); err == nil {
		sample.MemoryAvailable = true
		sample.MemoryUsageBytes = value * 1024
	}
	sample.CPUAvailable = false
}

func (m Manager) fillDiskStats(ctx context.Context, sample *db.MonitorSample) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		drive := "C:"
		if len(m.cfg.DataDir) >= 2 && m.cfg.DataDir[1] == ':' {
			drive = m.cfg.DataDir[:2]
		}
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", fmt.Sprintf("(Get-CimInstance Win32_LogicalDisk -Filter \"DeviceID='%s'\").FreeSpace; (Get-CimInstance Win32_LogicalDisk -Filter \"DeviceID='%s'\").Size", drive, drive))
	} else {
		cmd = exec.CommandContext(ctx, "df", "-k", m.cfg.DataDir)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		appendReason(sample, "disk: "+strings.TrimSpace(string(out)))
		return
	}
	lines := nonEmptyLines(string(out))
	if runtime.GOOS == "windows" {
		if len(lines) >= 2 {
			free, _ := strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64)
			total, _ := strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64)
			if total > 0 {
				sample.DiskAvailable = true
				sample.DiskFreeBytes = free
				sample.DiskTotalBytes = total
			}
		}
		return
	}
	if len(lines) >= 2 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 4 {
			total, _ := strconv.ParseInt(fields[1], 10, 64)
			free, _ := strconv.ParseInt(fields[3], 10, 64)
			if total > 0 {
				sample.DiskAvailable = true
				sample.DiskTotalBytes = total * 1024
				sample.DiskFreeBytes = free * 1024
			}
		}
	}
}

func appendReason(sample *db.MonitorSample, reason string) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return
	}
	if sample.UnavailableReason != "" {
		sample.UnavailableReason += " | "
	}
	sample.UnavailableReason += reason
}

func nonEmptyLines(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func parseDockerBytes(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	units := []struct {
		suffix string
		mul    float64
	}{
		{"GiB", 1024 * 1024 * 1024},
		{"MiB", 1024 * 1024},
		{"KiB", 1024},
		{"GB", 1000 * 1000 * 1000},
		{"MB", 1000 * 1000},
		{"KB", 1000},
		{"B", 1},
	}
	for _, unit := range units {
		if strings.HasSuffix(raw, unit.suffix) {
			v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(raw, unit.suffix)), 64)
			if err != nil {
				return 0, false
			}
			return int64(v * unit.mul), true
		}
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	return v, err == nil
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func numberValue(data map[string]any, keys ...string) float64 {
	for _, key := range keys {
		value, ok := data[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case string:
			n, _ := strconv.ParseFloat(v, 64)
			return n
		}
	}
	return 0
}
