package api

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestGMIdempotencyCoalescesConcurrentRequests(t *testing.T) {
	store := newGMIdempotencyStore()
	started := make(chan struct{})
	release := make(chan struct{})
	type outcome struct {
		result   any
		replayed bool
		err      error
	}
	outcomes := make(chan outcome, 2)
	var calls atomic.Int32

	go func() {
		result, replayed, err := store.do(t.Context(), "user-1", "gm-concurrent-001", "fingerprint", func(context.Context) (any, error) {
			calls.Add(1)
			close(started)
			<-release
			return "ok", nil
		})
		outcomes <- outcome{result: result, replayed: replayed, err: err}
	}()
	<-started
	go func() {
		result, replayed, err := store.do(t.Context(), "user-1", "gm-concurrent-001", "fingerprint", func(context.Context) (any, error) {
			calls.Add(1)
			return "duplicate", nil
		})
		outcomes <- outcome{result: result, replayed: replayed, err: err}
	}()
	close(release)

	first := <-outcomes
	second := <-outcomes
	if calls.Load() != 1 || first.err != nil || second.err != nil || first.result != "ok" || second.result != "ok" || first.replayed == second.replayed {
		t.Fatalf("calls=%d outcomes=%#v %#v", calls.Load(), first, second)
	}
}

func TestGMIdempotencyReplaysUncertainFailure(t *testing.T) {
	store := newGMIdempotencyStore()
	wantErr := errors.New("upstream outcome is uncertain")
	var calls atomic.Int32
	action := func(context.Context) (any, error) {
		calls.Add(1)
		return nil, wantErr
	}

	_, replayed, err := store.do(t.Context(), "user-1", "gm-error-replay-001", "fingerprint", action)
	if replayed || !errors.Is(err, wantErr) {
		t.Fatalf("first call replayed=%v err=%v", replayed, err)
	}
	_, replayed, err = store.do(t.Context(), "user-1", "gm-error-replay-001", "fingerprint", action)
	if !replayed || !errors.Is(err, wantErr) || calls.Load() != 1 {
		t.Fatalf("second call replayed=%v err=%v calls=%d", replayed, err, calls.Load())
	}
}
