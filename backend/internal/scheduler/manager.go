package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"palpanel/internal/db"
	"palpanel/internal/id"
	"palpanel/internal/palrest"
	"palpanel/internal/server"
)

type Manager struct {
	store   *db.Store
	server  server.Manager
	palrest palrest.Client
}

func New(store *db.Store, serverManager server.Manager, restClient palrest.Client) Manager {
	return Manager{store: store, server: serverManager, palrest: restClient}
}

func (m Manager) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = m.RunDue(context.Background())
			}
		}
	}()
}

func (m Manager) List(ctx context.Context) ([]db.Schedule, error) {
	return m.store.ListSchedules(ctx)
}

func (m Manager) Create(ctx context.Context, item db.Schedule) (db.Schedule, error) {
	item.ID = id.New("sched")
	item.Enabled = true
	item.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return m.save(ctx, item)
}

func (m Manager) Update(ctx context.Context, id string, item db.Schedule) (db.Schedule, error) {
	current, err := m.store.GetSchedule(ctx, id)
	if err != nil {
		return db.Schedule{}, err
	}
	item.ID = id
	item.CreatedAt = current.CreatedAt
	if item.LastRunAt == "" {
		item.LastRunAt = current.LastRunAt
	}
	return m.save(ctx, item)
}

func (m Manager) Delete(ctx context.Context, id string) error {
	return m.store.DeleteSchedule(ctx, id)
}

func (m Manager) RunNow(ctx context.Context, id string) (db.Job, error) {
	item, err := m.store.GetSchedule(ctx, id)
	if err != nil {
		return db.Job{}, err
	}
	return m.run(ctx, item)
}

func (m Manager) RunDue(ctx context.Context) error {
	items, err := m.store.ListSchedules(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		next, err := time.Parse(time.RFC3339Nano, item.NextRunAt)
		if err != nil || next.After(now) {
			continue
		}
		if _, err := m.run(ctx, item); err != nil {
			_ = m.alert(ctx, "error", "计划任务失败", err.Error(), item.ID)
		}
	}
	return nil
}

func (m Manager) save(ctx context.Context, item db.Schedule) (db.Schedule, error) {
	item.Type = strings.TrimSpace(item.Type)
	if !supportedType(item.Type) {
		return db.Schedule{}, fmt.Errorf("unsupported schedule type: %s", item.Type)
	}
	if item.IntervalMinutes <= 0 && strings.TrimSpace(item.TimeOfDay) == "" {
		return db.Schedule{}, fmt.Errorf("interval_minutes or time_of_day is required")
	}
	if item.WaitTime == 0 {
		item.WaitTime = 30
	}
	if item.Type == "safe_restart" && (item.WaitTime < 5 || item.WaitTime > 300) {
		return db.Schedule{}, fmt.Errorf("safe_restart waittime must be between 5 and 300 seconds")
	}
	next, err := nextRun(item, time.Now().UTC())
	if err != nil {
		return db.Schedule{}, err
	}
	item.NextRunAt = next.Format(time.RFC3339Nano)
	if err := m.store.UpsertSchedule(ctx, item); err != nil {
		return db.Schedule{}, err
	}
	return m.store.GetSchedule(ctx, item.ID)
}

func (m Manager) run(ctx context.Context, item db.Schedule) (db.Job, error) {
	if running, err := m.hasRunningJob(ctx, item.Type); err == nil && running {
		_ = m.alert(ctx, "info", "计划任务已跳过", "已有同类型任务正在执行: "+item.Type, item.ID)
		next, _ := nextRun(item, time.Now().UTC())
		item.NextRunAt = next.Format(time.RFC3339Nano)
		_ = m.store.UpsertSchedule(ctx, item)
		return db.Job{}, fmt.Errorf("same schedule type is already running")
	}
	var job db.Job
	var err error
	switch item.Type {
	case "save":
		job, err = m.saveJob(ctx)
	case "backup":
		job, err = m.server.Backup(ctx)
	case "safe_restart":
		job, err = m.server.SafeRestart(ctx, item.WaitTime, item.Message, func(ctx context.Context, wait int, message string) error {
			if _, err := m.palrest.Do(ctx, http.MethodPost, "save", nil); err != nil {
				return err
			}
			_, err := m.palrest.Do(ctx, http.MethodPost, "shutdown", gin.H{"waittime": wait, "message": message})
			return err
		})
	case "update":
		job, err = m.server.UpdateWithPreUpdate(ctx, m.preUpdateHook())
	case "version_check":
		job, err = m.server.CheckVersion(ctx)
	default:
		err = fmt.Errorf("unsupported schedule type: %s", item.Type)
	}
	if err != nil {
		if next, nextErr := nextRun(item, time.Now().UTC()); nextErr == nil {
			item.NextRunAt = next.Format(time.RFC3339Nano)
			_ = m.store.UpsertSchedule(ctx, item)
		}
		return db.Job{}, err
	}
	now := time.Now().UTC()
	item.LastRunAt = now.Format(time.RFC3339Nano)
	next, nextErr := nextRun(item, now)
	if nextErr == nil {
		item.NextRunAt = next.Format(time.RFC3339Nano)
	}
	_ = m.store.UpsertSchedule(ctx, item)
	return job, nil
}

func (m Manager) saveJob(ctx context.Context) (db.Job, error) {
	job, err := m.store.CreateJob(ctx, id.New("job"), "save", "queued scheduled save")
	if err != nil {
		return db.Job{}, err
	}
	go func(jobID string) {
		_ = m.store.UpdateJob(context.Background(), jobID, "running", 20, "saving world", "")
		if _, err := m.palrest.Do(context.Background(), http.MethodPost, "save", nil); err != nil {
			_ = m.store.UpdateJob(context.Background(), jobID, "failed", 20, "save failed", err.Error())
			_ = m.alert(context.Background(), "error", "计划保存失败", err.Error(), jobID)
			return
		}
		_ = m.store.UpdateJob(context.Background(), jobID, "completed", 100, "save completed", "")
	}(job.ID)
	return job, nil
}

func (m Manager) preUpdateHook() func(context.Context) error {
	return func(ctx context.Context) error {
		if _, err := m.palrest.Do(ctx, http.MethodPost, "announce", gin.H{"message": "Server update starting soon. Saving world and stopping server."}); err != nil {
			return fmt.Errorf("announce before update failed: %w", err)
		}
		if _, err := m.palrest.Do(ctx, http.MethodPost, "save", nil); err != nil {
			return fmt.Errorf("save before update failed: %w", err)
		}
		return nil
	}
}

func (m Manager) hasRunningJob(ctx context.Context, typ string) (bool, error) {
	jobs, err := m.store.ListJobs(ctx, 100)
	if err != nil {
		return false, err
	}
	for _, job := range jobs {
		if job.Type == typ && (job.Status == "queued" || job.Status == "waiting" || job.Status == "running") {
			return true, nil
		}
	}
	return false, nil
}

func (m Manager) alert(ctx context.Context, severity, title, message, source string) error {
	return m.store.CreateAlert(ctx, db.Alert{
		ID:       id.New("alert"),
		Severity: severity,
		Title:    title,
		Message:  message,
		Source:   source,
		Status:   "open",
	})
}

func supportedType(typ string) bool {
	switch typ {
	case "save", "backup", "safe_restart", "update", "version_check":
		return true
	default:
		return false
	}
}

func nextRun(item db.Schedule, from time.Time) (time.Time, error) {
	if item.IntervalMinutes > 0 {
		return from.Add(time.Duration(item.IntervalMinutes) * time.Minute), nil
	}
	parts := strings.Split(strings.TrimSpace(item.TimeOfDay), ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("time_of_day must be HH:mm")
	}
	hour, err := parseClockPart(parts[0], 0, 23)
	if err != nil {
		return time.Time{}, err
	}
	minute, err := parseClockPart(parts[1], 0, 59)
	if err != nil {
		return time.Time{}, err
	}
	next := time.Date(from.Year(), from.Month(), from.Day(), hour, minute, 0, 0, time.UTC)
	if !next.After(from) {
		next = next.Add(24 * time.Hour)
	}
	return next, nil
}

func parseClockPart(raw string, min, max int) (int, error) {
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil {
		return 0, err
	}
	if value < min || value > max {
		return 0, fmt.Errorf("time value out of range")
	}
	return value, nil
}
