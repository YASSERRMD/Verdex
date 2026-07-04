package perf

import (
	"sync"
	"time"
)

// Cache is a generic, mutex-guarded, TTL-based cache wrapper safe for
// concurrent use. K must be comparable so it can key an internal map.
//
// Cache is a standalone, general-purpose utility: it is not wired into any
// existing package's hot path by this phase. packages/hybridretrieval
// (Retriever.Retrieve, which runs a fresh vector-recall + graph-expansion
// pass per HybridQuery) and packages/traversal (Walker.Execute, which
// already has its own query-shaped Cache in packages/traversal/cache.go)
// are both documented here as good future adopters of a short-TTL result
// cache keyed by query shape, without this phase modifying either package
// -- see doc.go's "what this phase does NOT modify" section.
type Cache[K comparable, V any] struct {
	mu  sync.Mutex
	ttl time.Duration
	now func() time.Time

	entries map[K]cacheEntry[V]
}

// cacheEntry pairs a cached value with its absolute expiry time.
type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

// NewCache constructs a Cache with the given time-to-live. A ttl <= 0 means
// entries never expire on their own (only Invalidate removes them).
func NewCache[K comparable, V any](ttl time.Duration) *Cache[K, V] {
	return &Cache[K, V]{
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[K]cacheEntry[V]),
	}
}

// newCacheWithClock constructs a Cache using an injectable clock instead of
// time.Now, so expiry can be tested deterministically without real sleeps.
func newCacheWithClock[K comparable, V any](ttl time.Duration, now func() time.Time) *Cache[K, V] {
	c := NewCache[K, V](ttl)
	c.now = now
	return c
}

// Get returns the value stored for key and true, or the zero value and
// false if key is absent or its entry has expired. An expired entry is
// lazily evicted on the next Get/Set that observes it.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		var zero V
		return zero, false
	}
	if c.expired(entry) {
		delete(c.entries, key)
		var zero V
		return zero, false
	}
	return entry.value, true
}

// Set stores value under key, resetting its expiry to now+ttl.
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if c.ttl > 0 {
		expiresAt = c.now().Add(c.ttl)
	}
	c.entries[key] = cacheEntry[V]{value: value, expiresAt: expiresAt}
}

// Invalidate removes key's entry, if present. It is not an error to
// invalidate an absent key.
func (c *Cache[K, V]) Invalidate(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Len returns the number of entries currently stored, including any not
// yet lazily evicted despite having expired.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// expired reports whether entry's expiry time has passed, according to
// c.now. A zero expiresAt (ttl <= 0 at Set time) never expires.
func (c *Cache[K, V]) expired(entry cacheEntry[V]) bool {
	if entry.expiresAt.IsZero() {
		return false
	}
	return c.now().After(entry.expiresAt)
}
