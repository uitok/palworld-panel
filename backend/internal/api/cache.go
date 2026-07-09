package api

import (
	"context"
	"sync"
	"time"
)

type cacheStatus string

const (
	cacheStatusHit   cacheStatus = "hit"
	cacheStatusMiss  cacheStatus = "miss"
	cacheStatusStale cacheStatus = "stale"
)

type ttlCache struct {
	mu      sync.Mutex
	items   map[string]ttlCacheItem
	flights map[string]*ttlCacheCall
}

type ttlCacheItem struct {
	value     any
	expiresAt time.Time
}

type ttlCacheCall struct {
	wg    sync.WaitGroup
	value any
	err   error
}

func newTTLCache() *ttlCache {
	return &ttlCache{
		items:   map[string]ttlCacheItem{},
		flights: map[string]*ttlCacheCall{},
	}
}

func (c *ttlCache) Get(key string) (any, cacheStatus, bool) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if !ok {
		return nil, cacheStatusMiss, false
	}
	if now.Before(item.expiresAt) {
		return item.value, cacheStatusHit, true
	}
	return item.value, cacheStatusStale, true
}

func (c *ttlCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	c.items[key] = ttlCacheItem{value: value, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

func (c *ttlCache) DeletePrefix(prefix string) {
	c.mu.Lock()
	for key := range c.items {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.items, key)
		}
	}
	c.mu.Unlock()
}

func (c *ttlCache) GetOrLoad(ctx context.Context, key string, ttl time.Duration, refresh bool, load func(context.Context) (any, error)) (any, cacheStatus, error) {
	if !refresh {
		if value, status, ok := c.Get(key); ok && status == cacheStatusHit {
			return value, cacheStatusHit, nil
		}
	}

	c.mu.Lock()
	if !refresh {
		if item, ok := c.items[key]; ok && time.Now().Before(item.expiresAt) {
			c.mu.Unlock()
			return item.value, cacheStatusHit, nil
		}
	}
	if call, ok := c.flights[key]; ok {
		c.mu.Unlock()
		call.wg.Wait()
		if call.err != nil {
			if value, _, ok := c.Get(key); ok {
				return value, cacheStatusStale, nil
			}
			return nil, cacheStatusMiss, call.err
		}
		return call.value, cacheStatusHit, nil
	}
	call := &ttlCacheCall{}
	call.wg.Add(1)
	c.flights[key] = call
	c.mu.Unlock()

	call.value, call.err = load(ctx)
	call.wg.Done()

	c.mu.Lock()
	delete(c.flights, key)
	if call.err == nil {
		c.items[key] = ttlCacheItem{value: call.value, expiresAt: time.Now().Add(ttl)}
	}
	c.mu.Unlock()

	if call.err != nil {
		if value, _, ok := c.Get(key); ok {
			return value, cacheStatusStale, nil
		}
		return nil, cacheStatusMiss, call.err
	}
	return call.value, cacheStatusMiss, nil
}
