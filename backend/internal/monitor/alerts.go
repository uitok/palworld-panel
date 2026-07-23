package monitor

import (
	"context"
	"fmt"

	"palpanel/internal/db"
	"palpanel/internal/id"
)

var monitorRiskCodes = []string{
	"host_memory_pressure",
	"swap_exhaustion",
	"workload_memory_pressure",
	"oom_killed",
	"abnormal_exit",
}

func deriveRiskReasons(sample db.MonitorSample) []db.MonitorRiskReason {
	reasons := make([]db.MonitorRiskReason, 0, len(monitorRiskCodes))
	if sample.HostMemoryAvailable && ratioAtLeast(sample.HostMemoryTotalBytes-sample.HostMemoryAvailableBytes, sample.HostMemoryTotalBytes, 90) {
		reasons = append(reasons, db.MonitorRiskReason{Code: "host_memory_pressure", Message: "主机可用内存低于 10%", Severity: "warning"})
	}
	if sample.HostSwapTotalBytes > 0 && ratioAtLeast(sample.HostSwapTotalBytes-sample.HostSwapFreeBytes, sample.HostSwapTotalBytes, 90) {
		reasons = append(reasons, db.MonitorRiskReason{Code: "swap_exhaustion", Message: "主机交换空间剩余低于 10%", Severity: "warning"})
	}
	if sample.WorkloadMemoryAvailable && ratioAtLeast(sample.WorkloadMemoryUsageBytes, sample.WorkloadMemoryLimitBytes, 90) {
		reasons = append(reasons, db.MonitorRiskReason{Code: "workload_memory_pressure", Message: "工作负载内存用量达到限制的 90%", Severity: "warning"})
	}
	if sample.LifecycleAvailable && sample.OOMKilled {
		reasons = append(reasons, db.MonitorRiskReason{Code: "oom_killed", Message: "工作负载被 OOM 终止", Severity: "critical"})
	}
	if sample.LifecycleAvailable && sample.FinishedAt != "" && sample.ExitCode != 0 {
		reasons = append(reasons, db.MonitorRiskReason{Code: "abnormal_exit", Message: fmt.Sprintf("工作负载异常退出，退出码 %d", sample.ExitCode), Severity: "critical"})
	}
	return reasons
}

func ratioAtLeast(value, total int64, percent int64) bool {
	return total > 0 && value >= 0 && value*100 >= total*percent
}

func (m Manager) processRiskAlerts(ctx context.Context, reasons []db.MonitorRiskReason) error {
	active := make(map[string]db.MonitorRiskReason, len(reasons))
	for _, reason := range reasons {
		active[reason.Code] = reason
	}
	for _, code := range monitorRiskCodes {
		reason, unhealthy := active[code]
		alert := db.Alert{}
		if unhealthy {
			alert = monitorAlert(id.New("alert"), reason)
		}
		if err := m.store.ApplyMonitorAlertSample(ctx, code, unhealthy, code == "oom_killed", alert); err != nil {
			return err
		}
	}
	return nil
}

func monitorAlert(alertID string, reason db.MonitorRiskReason) db.Alert {
	severity := "warning"
	if reason.Severity == "critical" {
		severity = "error"
	}
	return db.Alert{
		ID: alertID, Severity: severity, Title: "监控风险：" + reason.Code,
		Message: reason.Message, Source: "monitor:" + reason.Code, Status: "open",
	}
}
