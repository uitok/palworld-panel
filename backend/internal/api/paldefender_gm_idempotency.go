package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	palDefenderGMIdempotencyHeader = "Idempotency-Key"
	palDefenderGMReplayHeader      = "Idempotency-Replayed"
	palDefenderGMIdempotencyTTL    = 10 * time.Minute
	maxGMIdempotencyEntries        = 1024
)

var (
	errGMIdempotencyConflict = errors.New("idempotency key was already used with a different request")
	errGMIdempotencyBusy     = errors.New("too many GM requests are currently being tracked")
	gmIdempotencyKeyPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
)

type gmIdempotencyEntry struct {
	fingerprint string
	done        chan struct{}
	result      any
	err         error
	createdAt   time.Time
	expiresAt   time.Time
}

type gmIdempotencyStore struct {
	mu      sync.Mutex
	entries map[string]*gmIdempotencyEntry
	now     func() time.Time
	ttl     time.Duration
}

func newGMIdempotencyStore() *gmIdempotencyStore {
	return &gmIdempotencyStore{
		entries: make(map[string]*gmIdempotencyEntry),
		now:     time.Now,
		ttl:     palDefenderGMIdempotencyTTL,
	}
}

func (s *gmIdempotencyStore) do(
	ctx context.Context,
	scope string,
	key string,
	fingerprint string,
	action func(context.Context) (any, error),
) (result any, replayed bool, err error) {
	entryKey := scope + "\x00" + key
	now := s.now()

	s.mu.Lock()
	for candidate, entry := range s.entries {
		if !entry.expiresAt.IsZero() && !entry.expiresAt.After(now) {
			delete(s.entries, candidate)
		}
	}
	if existing, ok := s.entries[entryKey]; ok {
		if existing.fingerprint != fingerprint {
			s.mu.Unlock()
			return nil, false, errGMIdempotencyConflict
		}
		done := existing.done
		s.mu.Unlock()
		select {
		case <-done:
			return existing.result, true, existing.err
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
	}

	if len(s.entries) >= maxGMIdempotencyEntries {
		var oldestKey string
		var oldest time.Time
		for candidate, existing := range s.entries {
			if existing.expiresAt.IsZero() {
				continue
			}
			if oldestKey == "" || existing.createdAt.Before(oldest) {
				oldestKey = candidate
				oldest = existing.createdAt
			}
		}
		if oldestKey == "" {
			s.mu.Unlock()
			return nil, false, errGMIdempotencyBusy
		}
		delete(s.entries, oldestKey)
	}
	entry := &gmIdempotencyEntry{fingerprint: fingerprint, done: make(chan struct{}), createdAt: now}
	s.entries[entryKey] = entry
	s.mu.Unlock()

	var panicValue any
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				panicValue = recovered
				err = errors.New("GM action failed unexpectedly")
			}
		}()
		result, err = action(ctx)
	}()
	s.mu.Lock()
	entry.result = result
	entry.err = err
	entry.expiresAt = s.now().Add(s.ttl)
	close(entry.done)
	s.mu.Unlock()
	if panicValue != nil {
		panic(panicValue)
	}
	return result, false, err
}

func (s Server) runPalDefenderGMWrite(c *gin.Context, request any, action func(context.Context) (any, error)) {
	key := c.GetHeader(palDefenderGMIdempotencyHeader)
	if key == "" {
		fail(c, http.StatusBadRequest, "idempotency_key_required", palDefenderGMIdempotencyHeader+" is required for GM write operations")
		return
	}
	if !gmIdempotencyKeyPattern.MatchString(key) {
		fail(c, http.StatusBadRequest, "invalid_idempotency_key", palDefenderGMIdempotencyHeader+" must contain 8 to 128 safe ASCII characters")
		return
	}
	payload, err := json.Marshal(request)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", "request could not be encoded")
		return
	}
	fingerprintBytes := sha256.Sum256([]byte(c.Request.Method + "\n" + c.FullPath() + "\n" + string(payload)))
	fingerprint := hex.EncodeToString(fingerprintBytes[:])
	principal := CurrentPrincipal(c)
	scope := principal.UserID
	if scope == "" {
		scope = fmt.Sprintf("%s:%s", principal.Role, principal.Name)
	}
	store := s.gmIdempotency
	if store == nil {
		store = newGMIdempotencyStore()
	}
	result, replayed, err := store.do(c.Request.Context(), scope, key, fingerprint, action)
	if errors.Is(err, errGMIdempotencyConflict) {
		fail(c, http.StatusConflict, "idempotency_key_reused", err.Error())
		return
	}
	if errors.Is(err, errGMIdempotencyBusy) {
		fail(c, http.StatusServiceUnavailable, "gm_request_capacity_reached", err.Error())
		return
	}
	if replayed {
		c.Header(palDefenderGMReplayHeader, "true")
	}
	if err != nil {
		failPalDefenderGM(c, err)
		return
	}
	ok(c, result)
}
