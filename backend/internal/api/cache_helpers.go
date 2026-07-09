package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"palpanel/internal/palrest"
)

const (
	cacheKeyServerPrefix = "server:"
	cacheKeySavePrefix   = "save:"
)

func (s Server) cached(c *gin.Context, key string, ttl time.Duration, loader func(context.Context) (any, error)) (any, cacheStatus, error) {
	refresh := c.Query("refresh") == "1" || strings.EqualFold(c.Query("refresh"), "true")
	value, status, err := s.cache.GetOrLoad(c.Request.Context(), key, ttl, refresh, loader)
	c.Header("X-Palpanel-Cache", string(status))
	return value, status, err
}

func cachedAs[T any](s Server, c *gin.Context, key string, ttl time.Duration, loader func(context.Context) (T, error)) (T, cacheStatus, error) {
	var zero T
	value, status, err := s.cached(c, key, ttl, func(ctx context.Context) (any, error) {
		return loader(ctx)
	})
	if err != nil {
		return zero, status, err
	}
	typed, ok := value.(T)
	if !ok {
		return zero, status, fmt.Errorf("cache value for %s has unexpected type", key)
	}
	return typed, status, nil
}

func cacheKey(parts ...any) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		values = append(values, fmt.Sprint(part))
	}
	return strings.Join(values, ":")
}

func (s Server) invalidateServerCaches() {
	s.cache.DeletePrefix(cacheKeyServerPrefix)
}

func (s Server) invalidateSaveCaches() {
	s.cache.DeletePrefix(cacheKeySavePrefix)
}

func (s Server) palworldRESTRead() palrest.Client {
	client := s.palworldREST()
	timeout := time.Duration(s.cfg.PalworldRESTReadTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 1200 * time.Millisecond
	}
	client.Client = &http.Client{Timeout: timeout}
	return client
}
