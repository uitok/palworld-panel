package communityservers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

type fakeFetcher struct {
	calls atomic.Int32
	mu    sync.Mutex
	items []Server
	total int
	err   error
	wait  chan struct{}
}

func (f *fakeFetcher) Fetch(ctx context.Context, _ Query) ([]Server, int, error) {
	f.calls.Add(1)
	if f.wait != nil {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-f.wait:
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneServers(f.items), f.total, f.err
}

func TestServiceFreshCacheFilteringAndStaleFallback(t *testing.T) {
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	password := true
	fetcher := &fakeFetcher{total: 100, items: []Server{
		{ID: "low", Name: "Low", Players: 2, Password: false, Version: "0.6.3"},
		{ID: "match", Name: "中文热门服", Players: 20, Password: true, Version: "0.6.4"},
		{ID: "high", Name: "High", Players: 30, Password: true, Version: "0.5.0"},
	}}
	service, err := New(Options{Fetcher: fetcher, FreshTTL: time.Minute, StaleTTL: 24 * time.Hour, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	query := Query{Region: "cn", Status: "online", MinPlayers: 10, MaxPlayers: 25, Password: &password, Version: "0.6", Page: 1, PageSize: 30}
	result, err := service.List(t.Context(), query)
	if err != nil || len(result.Servers) != 1 || result.Servers[0].ID != "match" || result.Total != 1 || result.Stale {
		t.Fatalf("first result = %#v, %v", result, err)
	}
	if fetcher.calls.Load() != 1 {
		t.Fatalf("fetch calls = %d", fetcher.calls.Load())
	}
	if page, err := service.List(t.Context(), Query{Region: "cn", Status: "online", Page: 2, PageSize: 1}); err != nil || len(page.Servers) != 1 || fetcher.calls.Load() != 1 {
		t.Fatalf("shared paginated cache = %#v calls=%d err=%v", page, fetcher.calls.Load(), err)
	}

	now = now.Add(30 * time.Second)
	if _, err := service.List(t.Context(), query); err != nil || fetcher.calls.Load() != 1 {
		t.Fatalf("fresh cache did not hit: calls=%d err=%v", fetcher.calls.Load(), err)
	}

	now = now.Add(2 * time.Minute)
	fetcher.mu.Lock()
	fetcher.err = errors.New("upstream unavailable")
	fetcher.mu.Unlock()
	stale, err := service.List(t.Context(), query)
	if err != nil || !stale.Stale || stale.CacheAgeSeconds != 150 {
		t.Fatalf("stale result = %#v, %v", stale, err)
	}
	status := service.Status()
	if status.Reachable || !status.CacheAvailable || status.CacheFresh || status.LastError == "" {
		t.Fatalf("status = %#v", status)
	}

	now = now.Add(25 * time.Hour)
	if _, err := service.List(t.Context(), query); err == nil {
		t.Fatal("expired stale cache should not hide upstream failure")
	}
}

func TestServicePersistsCacheAcrossRestart(t *testing.T) {
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	cachePath := t.TempDir() + "/community-cache.json"
	firstFetcher := &fakeFetcher{total: 1, items: []Server{{ID: "persisted", Name: "Persisted", Address: "127.0.0.1", Port: 8211}}}
	first, err := New(Options{Fetcher: firstFetcher, CachePath: cachePath, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	query := Query{Region: "cn", Status: "online", Page: 1, PageSize: 30}
	if _, err := first.List(t.Context(), query); err != nil {
		t.Fatal(err)
	}

	now = now.Add(5 * time.Minute)
	offline := &fakeFetcher{err: errors.New("offline")}
	second, err := New(Options{Fetcher: offline, CachePath: cachePath, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	result, err := second.List(t.Context(), query)
	if err != nil || !result.Stale || len(result.Servers) != 1 || result.Servers[0].ID != "persisted" {
		t.Fatalf("persisted result = %#v, %v", result, err)
	}
}

func TestServiceReportsPersistentCacheWriteFailure(t *testing.T) {
	root := t.TempDir()
	cachePath := filepath.Join(root, "community-cache.json")
	if err := os.MkdirAll(filepath.Join(cachePath, "blocker"), 0o755); err != nil {
		t.Fatal(err)
	}
	service, err := New(Options{
		Fetcher:   &fakeFetcher{total: 1, items: []Server{{ID: "one"}}},
		CachePath: cachePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.List(t.Context(), Query{Region: "cn"}); err != nil {
		t.Fatal(err)
	}
	status := service.Status()
	if status.CacheWritable || status.CacheError == "" || !status.CacheAvailable {
		t.Fatalf("status = %#v", status)
	}
}

func TestServiceRateLimitsRefreshAndCoalescesRequests(t *testing.T) {
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	release := make(chan struct{})
	fetcher := &fakeFetcher{items: []Server{{ID: "one"}}, total: 1, wait: release}
	service, err := New(Options{Fetcher: fetcher, RateLimit: 1, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	query := Query{Region: "cn"}
	results := make(chan error, 2)
	go func() { _, err := service.List(context.Background(), query); results <- err }()
	for fetcher.calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	go func() { _, err := service.List(context.Background(), query); results <- err }()
	close(release)
	if err := <-results; err != nil {
		t.Fatal(err)
	}
	if err := <-results; err != nil {
		t.Fatal(err)
	}
	if fetcher.calls.Load() != 1 {
		t.Fatalf("coalesced calls = %d", fetcher.calls.Load())
	}
	if _, err := service.Refresh(t.Context(), Query{Region: "global"}); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("refresh error = %v", err)
	}
}

func TestQueryNormalizationPreservesUTF8AtLengthLimit(t *testing.T) {
	query := (Query{Search: strings.Repeat("帕", 200), Version: strings.Repeat("鲁", 100)}).Normalize()
	if !utf8.ValidString(query.Search) || !utf8.ValidString(query.Version) || utf8.RuneCountInString(query.Search) != 128 || utf8.RuneCountInString(query.Version) != 64 {
		t.Fatalf("normalized query is not valid bounded UTF-8: %#v", query)
	}
}
