package jobs

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"palpanel/internal/db"
)

func TestExecutorSerializesLifecycleJobs(t *testing.T) {
	store := openStore(t)
	executor := New(store, 2)
	var active atomic.Int32
	var maximum atomic.Int32
	release := make(chan struct{})
	for range 2 {
		_, err := executor.Submit(t.Context(), ClassLifecycle, "test", "queued", func(_ context.Context, id string) {
			current := active.Add(1)
			for {
				previous := maximum.Load()
				if current <= previous || maximum.CompareAndSwap(previous, current) {
					break
				}
			}
			<-release
			active.Add(-1)
			_ = executor.Update(id, "completed", 100, "done", "")
		})
		if err != nil {
			t.Fatalf("Submit returned error: %v", err)
		}
	}
	close(release)
	shutdownCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := executor.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
	if maximum.Load() != 1 {
		t.Fatalf("maximum concurrent lifecycle jobs = %d", maximum.Load())
	}
}

func TestExecutorReconcilesAndRejectsAfterShutdown(t *testing.T) {
	store := openStore(t)
	job, err := store.CreateJob(t.Context(), "job_old", "backup", "queued")
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	executor := New(store, 1)
	if count, err := executor.Reconcile(t.Context()); err != nil || count != 1 {
		t.Fatalf("Reconcile = %d, %v", count, err)
	}
	job, err = store.GetJob(t.Context(), job.ID)
	if err != nil || job.Status != "failed" || job.ErrorCode != ErrorInterruptedByRestart {
		t.Fatalf("unexpected reconciled job: %#v, %v", job, err)
	}
	if err := executor.Shutdown(t.Context()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
	if _, err := executor.Submit(t.Context(), ClassGeneral, "test", "queued", func(context.Context, string) {}); err != ErrShuttingDown {
		t.Fatalf("Submit error = %v, want ErrShuttingDown", err)
	}
}

func openStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
