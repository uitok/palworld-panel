package monitor

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"palpanel/internal/db"
	"palpanel/internal/server"
)

func TestDeriveRiskReasonsUsesStableCodesAndMessages(t *testing.T) {
	reasons := deriveRiskReasons(db.MonitorSample{
		HostMemoryAvailable: true, HostMemoryTotalBytes: 1000, HostMemoryAvailableBytes: 99,
		HostSwapTotalBytes: 1000, HostSwapFreeBytes: 50,
		WorkloadMemoryAvailable: true, WorkloadMemoryUsageBytes: 900, WorkloadMemoryLimitBytes: 1000,
		LifecycleAvailable: true, OOMKilled: true, ExitCode: 137, FinishedAt: "2026-07-22T01:00:00Z",
	})
	want := []db.MonitorRiskReason{
		{Code: "host_memory_pressure", Message: "主机可用内存低于 10%", Severity: "warning"},
		{Code: "swap_exhaustion", Message: "主机交换空间剩余低于 10%", Severity: "warning"},
		{Code: "workload_memory_pressure", Message: "工作负载内存用量达到限制的 90%", Severity: "warning"},
		{Code: "oom_killed", Message: "工作负载被 OOM 终止", Severity: "critical"},
		{Code: "abnormal_exit", Message: "工作负载异常退出，退出码 137", Severity: "critical"},
	}
	if len(reasons) != len(want) {
		t.Fatalf("reasons = %#v", reasons)
	}
	for index := range want {
		if reasons[index] != want[index] {
			t.Fatalf("reason[%d] = %#v, want %#v", index, reasons[index], want[index])
		}
	}
}

type restartTrackingServer struct {
	restarts int
}

func (*restartTrackingServer) Status(context.Context) (server.Status, error) {
	return server.Status{}, nil
}

func (s *restartTrackingServer) Restart(context.Context) error {
	s.restarts++
	return nil
}

func TestSustainedAlertTriggerDedupRestartPersistenceAndRecovery(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "alerts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	trackedServer := &restartTrackingServer{}
	manager := Manager{store: store, server: trackedServer}
	reason := db.MonitorRiskReason{Code: "host_memory_pressure", Message: "主机可用内存低于 10%", Severity: "warning"}

	for sample := 1; sample <= 2; sample++ {
		if err := manager.processRiskAlerts(t.Context(), []db.MonitorRiskReason{reason}); err != nil {
			t.Fatal(err)
		}
		if alerts, _ := store.ListAlerts(t.Context(), 10); len(alerts) != 0 {
			t.Fatalf("sample %d triggered early alert: %#v", sample, alerts)
		}
	}
	if err := manager.processRiskAlerts(t.Context(), []db.MonitorRiskReason{reason}); err != nil {
		t.Fatal(err)
	}
	alerts, err := store.ListAlerts(t.Context(), 10)
	if err != nil || len(alerts) != 1 || alerts[0].Status != "open" {
		t.Fatalf("third sample alerts = %#v, %v", alerts, err)
	}

	managerAfterRestart := Manager{store: store, server: trackedServer}
	if err := managerAfterRestart.processRiskAlerts(t.Context(), []db.MonitorRiskReason{reason}); err != nil {
		t.Fatal(err)
	}
	alerts, _ = store.ListAlerts(t.Context(), 10)
	if len(alerts) != 1 {
		t.Fatalf("open round was duplicated after restart: %#v", alerts)
	}

	for healthy := 1; healthy <= 2; healthy++ {
		if err := managerAfterRestart.processRiskAlerts(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		alerts, _ = store.ListAlerts(t.Context(), 10)
		if alerts[0].Status != "open" {
			t.Fatalf("healthy sample %d resolved early: %#v", healthy, alerts[0])
		}
	}
	if err := managerAfterRestart.processRiskAlerts(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	alerts, _ = store.ListAlerts(t.Context(), 10)
	if len(alerts) != 1 || alerts[0].Status != "resolved" {
		t.Fatalf("three healthy samples did not resolve round: %#v", alerts)
	}
	state, found, err := store.GetMonitorAlertState(t.Context(), reason.Code)
	if err != nil || !found || state.Open || state.UnhealthyCount != 0 || state.HealthyCount != 0 {
		t.Fatalf("resolved state = %#v, %v, %v", state, found, err)
	}
	if trackedServer.restarts != 0 {
		t.Fatalf("monitoring must never restart the server, got %d calls", trackedServer.restarts)
	}
}

func TestOOMAlertIsImmediateCriticalAndDeduplicated(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "oom-alert.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := Manager{store: store}
	reason := db.MonitorRiskReason{Code: "oom_killed", Message: "工作负载被 OOM 终止", Severity: "critical"}
	if err := manager.processRiskAlerts(t.Context(), []db.MonitorRiskReason{reason}); err != nil {
		t.Fatal(err)
	}
	if err := manager.processRiskAlerts(t.Context(), []db.MonitorRiskReason{reason}); err != nil {
		t.Fatal(err)
	}
	alerts, err := store.ListAlerts(t.Context(), 10)
	if err != nil || len(alerts) != 1 || alerts[0].Severity != "error" || alerts[0].Status != "open" {
		t.Fatalf("OOM alerts = %#v, %v", alerts, err)
	}
}

func TestConcurrentThirdUnhealthySamplesOpenOneAlertRound(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "concurrent-alert.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := Manager{store: store}
	reason := db.MonitorRiskReason{Code: "host_memory_pressure", Message: "主机可用内存低于 10%", Severity: "warning"}
	for index := 0; index < 2; index++ {
		if err := manager.processRiskAlerts(t.Context(), []db.MonitorRiskReason{reason}); err != nil {
			t.Fatal(err)
		}
	}

	const overlappingSamples = 8
	start := make(chan struct{})
	errorsBySample := make(chan error, overlappingSamples)
	var wait sync.WaitGroup
	for index := 0; index < overlappingSamples; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			errorsBySample <- manager.processRiskAlerts(t.Context(), []db.MonitorRiskReason{reason})
		}()
	}
	close(start)
	wait.Wait()
	close(errorsBySample)
	for err := range errorsBySample {
		if err != nil {
			t.Fatalf("concurrent sample returned error: %v", err)
		}
	}
	alerts, err := store.ListAlerts(t.Context(), 100)
	if err != nil || len(alerts) != 1 || alerts[0].Status != "open" {
		t.Fatalf("alerts = %#v, %v", alerts, err)
	}
	state, found, err := store.GetMonitorAlertState(t.Context(), reason.Code)
	if err != nil || !found || !state.Open || state.UnhealthyCount != 3 || state.AlertID != alerts[0].ID {
		t.Fatalf("state = %#v, %v, %v", state, found, err)
	}
}
