package gateway

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter is the interface that wraps the basic rate-limiting operation.
// Allow reports whether the request identified by key should be allowed.
type RateLimiter interface {
	Allow(key string) bool
}

// windowEntry tracks the sliding-window state for a single key.
type windowEntry struct {
	mu         sync.Mutex
	timestamps []time.Time
}

// InMemoryRateLimiter implements a per-key sliding-window rate limiter that
// allows at most Limit requests per Interval.
type InMemoryRateLimiter struct {
	Limit    int           // maximum number of requests allowed per Interval
	Interval time.Duration // length of the sliding window

	mu      sync.Mutex
	entries map[string]*windowEntry
}

// NewInMemoryRateLimiter creates an InMemoryRateLimiter with the given request
// limit per time interval. A common configuration is (60, time.Minute) for
// 60 requests per minute.
func NewInMemoryRateLimiter(limit int, interval time.Duration) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		Limit:    limit,
		Interval: interval,
		entries:  make(map[string]*windowEntry),
	}
}

// Allow reports whether a request for key is within the rate limit.
// Expired timestamps outside the sliding window are pruned on each call.
func (rl *InMemoryRateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	e, ok := rl.entries[key]
	if !ok {
		e = &windowEntry{}
		rl.entries[key] = e
	}
	rl.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.Interval)

	// Prune timestamps outside the window.
	valid := e.timestamps[:0]
	for _, t := range e.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	e.timestamps = valid

	if len(e.timestamps) >= rl.Limit {
		return false
	}

	e.timestamps = append(e.timestamps, now)
	return true
}

// RateLimitMiddleware returns a middleware that applies rl to every request.
// keyFn extracts the rate-limit key from the request (e.g. the client IP).
// Requests that exceed the limit receive a 429 Too Many Requests response.
func RateLimitMiddleware(limiter RateLimiter, keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if !limiter.Allow(key) {
				WriteError(w, &APIError{
					Code:    ErrCodeTooManyRequests,
					Message: "rate limit exceeded; please slow down and try again later",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RemoteAddrKey is a convenience keyFn that uses r.RemoteAddr as the rate-limit
// key. For production use behind a proxy, prefer extracting the X-Forwarded-For
// or X-Real-IP header.
func RemoteAddrKey(r *http.Request) string {
	return r.RemoteAddr
}
