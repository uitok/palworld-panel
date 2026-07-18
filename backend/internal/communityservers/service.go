package communityservers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrRateLimited = errors.New("community server source refresh rate limit reached")

type Options struct {
	BaseURL   string
	ProxyURL  string
	CachePath string
	FreshTTL  time.Duration
	StaleTTL  time.Duration
	RateLimit int
	Fetcher   Fetcher
	Now       func() time.Time
}

type Service struct {
	mu            sync.Mutex
	fetcher       Fetcher
	baseURL       string
	proxy         bool
	cachePath     string
	freshTTL      time.Duration
	staleTTL      time.Duration
	rateLimit     int
	now           func() time.Time
	cache         map[string]cacheEntry
	attempts      []time.Time
	inflight      map[string]*fetchCall
	status        sourceRuntimeStatus
	cacheWritable bool
	cacheError    string
}

type cacheEntry struct {
	Query       Query     `json:"query"`
	Servers     []Server  `json:"servers"`
	SourceTotal int       `json:"source_total"`
	FetchedAt   time.Time `json:"fetched_at"`
}

type cacheDocument struct {
	Version int                   `json:"version"`
	Entries map[string]cacheEntry `json:"entries"`
}

type fetchCall struct {
	done  chan struct{}
	entry cacheEntry
	err   error
}

type sourceRuntimeStatus struct {
	lastAttempt time.Time
	lastSuccess time.Time
	lastError   string
}

func New(options Options) (*Service, error) {
	baseURL := strings.TrimSpace(options.BaseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	fetcher := options.Fetcher
	if fetcher == nil {
		client, err := NewClient(baseURL, options.ProxyURL)
		if err != nil {
			return nil, err
		}
		fetcher = client
	}
	freshTTL := options.FreshTTL
	if freshTTL <= 0 {
		freshTTL = DefaultFreshTTL
	}
	staleTTL := options.StaleTTL
	if staleTTL <= freshTTL {
		staleTTL = DefaultStaleTTL
		if staleTTL <= freshTTL {
			staleTTL = freshTTL * 2
		}
	}
	rateLimit := options.RateLimit
	if rateLimit <= 0 {
		rateLimit = DefaultRateLimit
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	service := &Service{
		fetcher: fetcher, baseURL: publicURL(baseURL), proxy: strings.TrimSpace(options.ProxyURL) != "",
		cachePath: strings.TrimSpace(options.CachePath), freshTTL: freshTTL, staleTTL: staleTTL,
		rateLimit: rateLimit, now: now, cache: map[string]cacheEntry{}, inflight: map[string]*fetchCall{},
		cacheWritable: strings.TrimSpace(options.CachePath) != "",
	}
	service.loadCache()
	return service, nil
}

func (s *Service) List(ctx context.Context, query Query) (Result, error) {
	return s.list(ctx, query.Normalize(), false)
}

func (s *Service) Refresh(ctx context.Context, query Query) (Result, error) {
	return s.list(ctx, query.Normalize(), true)
}

func (s *Service) list(ctx context.Context, query Query, force bool) (Result, error) {
	sourceQuery := sourceQueryFor(query)
	key := queryKey(sourceQuery)
	now := s.now().UTC()
	s.mu.Lock()
	cached, hasCached := s.cache[key]
	if hasCached && now.Sub(cached.FetchedAt) <= s.freshTTL && !force {
		s.mu.Unlock()
		return s.result(query, cached, false, now), nil
	}
	if call := s.inflight[key]; call != nil {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-call.done:
			if call.err != nil {
				return s.cachedFallback(query, key, call.err, s.now().UTC())
			}
			return s.result(query, call.entry, false, s.now().UTC()), nil
		}
	}
	if !s.allowRefreshLocked(now) {
		s.status.lastAttempt = now
		s.status.lastError = ErrRateLimited.Error()
		s.mu.Unlock()
		return s.cachedFallback(query, key, ErrRateLimited, now)
	}
	call := &fetchCall{done: make(chan struct{})}
	s.inflight[key] = call
	s.status.lastAttempt = now
	s.mu.Unlock()

	servers, total, err := s.fetcher.Fetch(ctx, sourceQuery)
	fetchedAt := s.now().UTC()
	entry := cacheEntry{Query: sourceQuery, Servers: cloneServers(servers), SourceTotal: total, FetchedAt: fetchedAt}

	s.mu.Lock()
	delete(s.inflight, key)
	call.err = err
	if err == nil {
		s.cache[key] = entry
		s.status.lastSuccess = fetchedAt
		s.status.lastError = ""
		call.entry = entry
		s.pruneLocked(fetchedAt)
		if persistErr := s.persistLocked(); persistErr != nil {
			s.cacheWritable = false
			s.cacheError = sanitizeError(persistErr.Error())
		} else {
			s.cacheWritable = s.cachePath != ""
			s.cacheError = ""
		}
	} else {
		s.status.lastError = sanitizeError(err.Error())
	}
	close(call.done)
	s.mu.Unlock()
	if err != nil {
		return s.cachedFallback(query, key, err, fetchedAt)
	}
	return s.result(query, entry, false, fetchedAt), nil
}

func (s *Service) cachedFallback(query Query, key string, cause error, now time.Time) (Result, error) {
	s.mu.Lock()
	entry, ok := s.cache[key]
	s.mu.Unlock()
	if ok && now.Sub(entry.FetchedAt) <= s.staleTTL {
		return s.result(query, entry, true, now), nil
	}
	return Result{}, cause
}

func (s *Service) Status() SourceStatus {
	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	status := SourceStatus{
		Source: DefaultCacheSource, Enabled: true, BaseURL: s.baseURL, ProxyConfigured: s.proxy,
		Reachable:     !s.status.lastSuccess.IsZero() && (s.status.lastAttempt.IsZero() || !s.status.lastSuccess.Before(s.status.lastAttempt)),
		CacheWritable: s.cacheWritable, CachedQueries: len(s.cache), RateLimit: s.rateLimit,
		LastError: s.status.lastError, CacheError: s.cacheError,
	}
	var newest time.Time
	for _, entry := range s.cache {
		age := now.Sub(entry.FetchedAt)
		if age <= s.staleTTL {
			status.CacheAvailable = true
		}
		if age <= s.freshTTL {
			status.CacheFresh = true
		}
		if entry.FetchedAt.After(newest) {
			newest = entry.FetchedAt
		}
	}
	if !s.status.lastAttempt.IsZero() {
		value := s.status.lastAttempt
		status.LastAttemptAt = &value
	}
	lastSuccess := s.status.lastSuccess
	if lastSuccess.IsZero() {
		lastSuccess = newest
	}
	if !lastSuccess.IsZero() {
		status.LastSuccessAt = &lastSuccess
		next := lastSuccess.Add(s.freshTTL)
		status.NextRefreshAt = &next
	}
	return status
}

func (s *Service) result(query Query, entry cacheEntry, stale bool, now time.Time) Result {
	filtered := filterServers(entry.Servers, query)
	total := len(filtered)
	start := (query.Page - 1) * query.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + query.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	servers := cloneServers(filtered[start:end])
	age := now.Sub(entry.FetchedAt)
	if age < 0 {
		age = 0
	}
	return Result{
		Servers: servers, Total: total, SourceTotal: entry.SourceTotal, Page: query.Page,
		PageSize: query.PageSize, Source: DefaultCacheSource, FetchedAt: entry.FetchedAt,
		Stale: stale, CacheAgeSeconds: int64(age / time.Second),
	}
}

func filterServers(servers []Server, query Query) []Server {
	result := make([]Server, 0, len(servers))
	version := strings.ToLower(query.Version)
	for _, server := range servers {
		if server.Players < query.MinPlayers || (query.MaxPlayers > 0 && server.Players > query.MaxPlayers) {
			continue
		}
		if query.Password != nil && server.Password != *query.Password {
			continue
		}
		if version != "" && !strings.Contains(strings.ToLower(server.Version), version) {
			continue
		}
		result = append(result, server)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Players == result[j].Players {
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		}
		return result[i].Players > result[j].Players
	})
	return result
}

func (s *Service) allowRefreshLocked(now time.Time) bool {
	cutoff := now.Add(-time.Minute)
	kept := s.attempts[:0]
	for _, attempt := range s.attempts {
		if attempt.After(cutoff) {
			kept = append(kept, attempt)
		}
	}
	s.attempts = kept
	if len(s.attempts) >= s.rateLimit {
		return false
	}
	s.attempts = append(s.attempts, now)
	return true
}

func (s *Service) loadCache() {
	if s.cachePath == "" {
		return
	}
	body, err := os.ReadFile(s.cachePath)
	if err != nil {
		return
	}
	var document cacheDocument
	if json.Unmarshal(body, &document) != nil || document.Version != 1 {
		return
	}
	now := s.now().UTC()
	for key, entry := range document.Entries {
		if !entry.FetchedAt.IsZero() && now.Sub(entry.FetchedAt) <= s.staleTTL {
			s.cache[key] = entry
		}
	}
}

func (s *Service) pruneLocked(now time.Time) {
	for key, entry := range s.cache {
		if now.Sub(entry.FetchedAt) > s.staleTTL {
			delete(s.cache, key)
		}
	}
}

func (s *Service) persistLocked() error {
	if s.cachePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.cachePath), 0o750); err != nil {
		return err
	}
	body, err := json.Marshal(cacheDocument{Version: 1, Entries: s.cache})
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.cachePath), ".community-servers-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, s.cachePath); err != nil {
		if removeErr := os.Remove(s.cachePath); removeErr != nil && !os.IsNotExist(removeErr) {
			return err
		}
		return os.Rename(temporaryPath, s.cachePath)
	}
	return nil
}

func queryKey(query Query) string {
	body, _ := json.Marshal(query.Normalize())
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func sourceQueryFor(query Query) Query {
	query = query.Normalize()
	return Query{
		Region: query.Region, Search: query.Search, Status: query.Status,
		Page: 1, PageSize: MaximumPageSize,
	}
}

func trimQuery(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > limit {
		value = string(runes[:limit])
	}
	return value
}

func cloneServers(servers []Server) []Server {
	return append([]Server(nil), servers...)
}

func sanitizeError(message string) string {
	if len(message) > 300 {
		message = message[:300]
	}
	return message
}

func publicURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parsed.User = nil
	return parsed.String()
}
