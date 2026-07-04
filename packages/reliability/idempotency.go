package reliability

import (
	"context"
	"sync"
	"time"
)

// DefaultIdempotencyTTL is how long a cached idempotent result remains
// eligible for replay, when IdempotencyGuard is constructed with a
// non-positive ttl.
const DefaultIdempotencyTTL = 10 * time.Minute

// idempotencyEntry holds one cached outcome for a given key: either an
// in-progress call (via the done channel other concurrent callers wait
// on) or a completed one (value/err populated, done already closed).
type idempotencyEntry[T any] struct {
	done      chan struct{}
	value     T
	err       error
	expiresAt time.Time
}

// IdempotencyFunc[T] is the unit of work IdempotencyGuard.Execute
// deduplicates by key.
type IdempotencyFunc[T any] func(ctx context.Context) (T, error)

// IdempotencyGuard[T] is a generic, IdempotencyKey-based dedup guard:
// Execute(ctx, key, fn) invokes fn at most once per key within a TTL
// window, replaying the cached result (value and error) to every
// other caller using the same key while that window is open --
// including a caller racing a still-in-flight call for the same key,
// which blocks until it completes rather than invoking fn a second
// time.
//
// This generalizes the (key, stage) -> IdempotencyRecord pattern
// packages/ingestion.IdempotencyStore already applies per pipeline
// stage (Phase 029, packages/ingestion/retry.go): that store tracks
// completion/attempt-count bookkeeping for RunWithRetry to consult
// before deciding whether to re-run a stage, but does not itself cache
// or replay a stage's return *value* -- it is a bookkeeping ledger, not
// a result cache. IdempotencyGuard is a reusable, value-caching
// primitive suited to any pipeline (not just packages/ingestion's
// stage-keyed one) that needs "the second call with the same key
// returns the first call's result" semantics, e.g. a webhook handler
// replaying a payment-provider callback, or an API endpoint retried by
// a client after a network blip.
//
// The zero value is not usable; construct with NewIdempotencyGuard.
type IdempotencyGuard[T any] struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]*idempotencyEntry[T]
	now     func() time.Time
}

// NewIdempotencyGuard constructs an IdempotencyGuard whose cached
// results expire after ttl. A non-positive ttl falls back to
// DefaultIdempotencyTTL.
func NewIdempotencyGuard[T any](ttl time.Duration) *IdempotencyGuard[T] {
	if ttl <= 0 {
		ttl = DefaultIdempotencyTTL
	}
	return &IdempotencyGuard[T]{
		ttl:     ttl,
		entries: make(map[string]*idempotencyEntry[T]),
		now:     time.Now,
	}
}

// Execute invokes fn for the given idempotency key, unless a
// non-expired cached result (or an in-flight call) already exists for
// that key, in which case Execute returns the cached/in-flight result
// without invoking fn again. Returns ErrNilFunc if fn is nil and no
// cached result already exists for key (a cached replay never needs
// fn, so a nil fn is fine for an already-cached key).
func (g *IdempotencyGuard[T]) Execute(ctx context.Context, key string, fn IdempotencyFunc[T]) (T, error) {
	g.mu.Lock()
	now := g.now()

	if entry, ok := g.entries[key]; ok {
		if entry.expiresAt.After(now) || entry.expiresAt.IsZero() {
			// Either a completed, still-valid cache entry, or an
			// in-flight call (expiresAt is zero-valued until the call
			// finishes). Wait on it outside the lock.
			g.mu.Unlock()
			return waitForEntry(ctx, entry)
		}
		// Expired: fall through and start a fresh call, replacing the
		// stale entry below.
		delete(g.entries, key)
	}

	if fn == nil {
		g.mu.Unlock()
		var zero T
		return zero, wrapf("IdempotencyGuard.Execute", ErrNilFunc)
	}

	entry := &idempotencyEntry[T]{done: make(chan struct{})}
	g.entries[key] = entry
	g.mu.Unlock()

	value, err := fn(ctx)

	g.mu.Lock()
	entry.value = value
	entry.err = err
	entry.expiresAt = g.now().Add(g.ttl)
	g.mu.Unlock()
	close(entry.done)

	return value, err
}

// waitForEntry blocks until entry's in-flight call completes (or ctx
// is done, whichever comes first) and returns its cached outcome.
func waitForEntry[T any](ctx context.Context, entry *idempotencyEntry[T]) (T, error) {
	select {
	case <-entry.done:
		return entry.value, entry.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// Forget removes any cached or in-flight entry for key, so the next
// Execute call for that key always invokes fn fresh. Primarily useful
// for tests and for callers that need to explicitly invalidate a
// cached idempotent result (e.g. after confirming a downstream side
// effect did not actually take place).
func (g *IdempotencyGuard[T]) Forget(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, key)
}
