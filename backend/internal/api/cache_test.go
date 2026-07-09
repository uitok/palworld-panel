package api

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTTLCacheHitRefreshAndExpiry(t *testing.T) {
	cache := newTTLCache()
	var calls int32
	load := func(context.Context) (any, error) {
		return atomic.AddInt32(&calls, 1), nil
	}

	value, status, err := cache.GetOrLoad(context.Background(), "key", 20*time.Millisecond, false, load)
	if err != nil || status != cacheStatusMiss || value.(int32) != 1 {
		t.Fatalf("first load value=%#v status=%s err=%v", value, status, err)
	}
	value, status, err = cache.GetOrLoad(context.Background(), "key", 20*time.Millisecond, false, load)
	if err != nil || status != cacheStatusHit || value.(int32) != 1 || atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("cache hit value=%#v status=%s calls=%d err=%v", value, status, calls, err)
	}
	value, status, err = cache.GetOrLoad(context.Background(), "key", 20*time.Millisecond, true, load)
	if err != nil || status != cacheStatusMiss || value.(int32) != 2 {
		t.Fatalf("refresh value=%#v status=%s err=%v", value, status, err)
	}
	time.Sleep(25 * time.Millisecond)
	value, status, err = cache.GetOrLoad(context.Background(), "key", 20*time.Millisecond, false, load)
	if err != nil || status != cacheStatusMiss || value.(int32) != 3 {
		t.Fatalf("expired value=%#v status=%s err=%v", value, status, err)
	}
}

func TestTTLCacheSingleflight(t *testing.T) {
	cache := newTTLCache()
	var calls int32
	start := make(chan struct{})
	load := func(context.Context) (any, error) {
		atomic.AddInt32(&calls, 1)
		<-start
		return "loaded", nil
	}

	var wg sync.WaitGroup
	results := make(chan any, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, _, err := cache.GetOrLoad(context.Background(), "key", time.Minute, false, load)
			if err != nil {
				t.Errorf("GetOrLoad returned error: %v", err)
				return
			}
			results <- value
		}()
	}
	for atomic.LoadInt32(&calls) == 0 {
		time.Sleep(time.Millisecond)
	}
	close(start)
	wg.Wait()
	close(results)

	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("loader calls = %d, want 1", calls)
	}
	for value := range results {
		if value != "loaded" {
			t.Fatalf("value = %#v", value)
		}
	}
}
