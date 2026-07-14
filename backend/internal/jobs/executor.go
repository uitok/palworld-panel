package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sync"

	"palpanel/internal/db"
	"palpanel/internal/id"
)

type Class uint8

const (
	ClassGeneral Class = iota
	ClassLifecycle
)

const (
	ErrorInterruptedByRestart  = "interrupted_by_restart"
	ErrorInterruptedByShutdown = "interrupted_by_shutdown"
	ErrorWorkerPanic           = "worker_panic"
	ErrorMissingTerminalState  = "missing_terminal_state"
)

var ErrShuttingDown = errors.New("job executor is shutting down")

type Store interface {
	CreateJob(context.Context, string, string, string) (db.Job, error)
	GetJob(context.Context, string) (db.Job, error)
	UpdateJobWithCode(context.Context, string, string, int, string, string, string) error
	FailIncompleteJobs(context.Context, string, string) (int64, error)
}

type Work func(context.Context, string)

type Executor struct {
	store Store

	ctx       context.Context
	cancel    context.CancelFunc
	general   chan struct{}
	lifecycle sync.Mutex

	mu        sync.Mutex
	accepting bool
	wg        sync.WaitGroup
}

func New(store Store, maxConcurrent int) *Executor {
	if maxConcurrent < 1 {
		maxConcurrent = 4
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Executor{
		store:     store,
		ctx:       ctx,
		cancel:    cancel,
		general:   make(chan struct{}, maxConcurrent),
		accepting: true,
	}
}

func (e *Executor) Reconcile(ctx context.Context) (int64, error) {
	return e.store.FailIncompleteJobs(ctx, ErrorInterruptedByRestart, "job interrupted by process restart")
}

func (e *Executor) Submit(ctx context.Context, class Class, typ, message string, work Work) (db.Job, error) {
	if work == nil {
		return db.Job{}, errors.New("job work is required")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.accepting {
		return db.Job{}, ErrShuttingDown
	}
	job, err := e.store.CreateJob(ctx, id.New("job"), typ, message)
	if err != nil {
		return db.Job{}, err
	}
	e.wg.Add(1)
	go e.run(class, job.ID, work)
	return job, nil
}

func (e *Executor) Update(jobID, status string, progress int, message, detail string) error {
	return e.store.UpdateJobWithCode(context.Background(), jobID, status, progress, message, detail, "")
}

func (e *Executor) UpdateWithCode(jobID, status string, progress int, message, detail, errorCode string) error {
	return e.store.UpdateJobWithCode(context.Background(), jobID, status, progress, message, detail, errorCode)
}

func (e *Executor) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	e.accepting = false
	e.mu.Unlock()

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		e.cancel()
		return nil
	case <-ctx.Done():
		e.cancel()
		return ctx.Err()
	}
}

func (e *Executor) run(class Class, jobID string, work Work) {
	defer e.wg.Done()
	if !e.acquire(class) {
		e.updateFailure(jobID, ErrorInterruptedByShutdown, "job interrupted during shutdown", e.ctx.Err())
		return
	}
	defer e.release(class)
	defer func() {
		if recovered := recover(); recovered != nil {
			detail := fmt.Sprintf("job worker panic: %v\n%s", recovered, debug.Stack())
			e.updateFailure(jobID, ErrorWorkerPanic, "job worker panicked", errors.New(detail))
			return
		}
		e.ensureTerminal(jobID)
	}()
	work(e.ctx, jobID)
}

func (e *Executor) acquire(class Class) bool {
	if class == ClassLifecycle {
		e.lifecycle.Lock()
		if e.ctx.Err() != nil {
			e.lifecycle.Unlock()
			return false
		}
		return true
	}
	select {
	case e.general <- struct{}{}:
		return true
	case <-e.ctx.Done():
		return false
	}
}

func (e *Executor) release(class Class) {
	if class == ClassLifecycle {
		e.lifecycle.Unlock()
		return
	}
	<-e.general
}

func (e *Executor) ensureTerminal(jobID string) {
	job, err := e.store.GetJob(context.Background(), jobID)
	if err != nil {
		log.Printf("job %s terminal-state read failed: %v", jobID, err)
		return
	}
	if job.Status == "completed" || job.Status == "failed" || job.Status == "cancelled" {
		return
	}
	if e.ctx.Err() != nil {
		e.updateFailure(jobID, ErrorInterruptedByShutdown, "job interrupted during shutdown", e.ctx.Err())
		return
	}
	e.updateFailure(jobID, ErrorMissingTerminalState, "job ended without a terminal state", nil)
}

func (e *Executor) updateFailure(jobID, code, message string, cause error) {
	detail := message
	if cause != nil {
		detail = cause.Error()
	}
	if err := e.UpdateWithCode(jobID, "failed", 0, message, detail, code); err != nil {
		log.Printf("job %s failure-state update failed: %v", jobID, err)
	}
}
