package embedding

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
)

// Cache is a key-value store for [EmbeddedText] values, keyed by content hash.
//
// Implementations MUST be safe for concurrent use.
type Cache interface {
	// Get retrieves the embedding for the given hash.  Returns
	// ([ErrCacheMiss], nil) when the hash is absent.
	Get(ctx context.Context, hash string) (*EmbeddedText, error)

	// Set stores e under hash, overwriting any existing entry.
	Set(ctx context.Context, hash string, e EmbeddedText) error

	// Delete removes the entry for hash.  It is a no-op when absent.
	Delete(ctx context.Context, hash string) error

	// Flush removes all entries from the cache.
	Flush(ctx context.Context) error
}

// CacheKey derives the canonical content-addressable hash for a (text, model)
// pair.  The hash is a lower-case hex-encoded SHA-256 digest of
// "text|modelID".
func CacheKey(text, modelID string) string {
	h := sha256.New()
	h.Write([]byte(text))
	h.Write([]byte("|"))
	h.Write([]byte(modelID))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// InMemoryCache is a thread-safe in-process implementation of [Cache]
// backed by a plain map.  It is suitable for testing and single-process
// deployments; it does not survive process restarts.
type InMemoryCache struct {
	mu    sync.RWMutex
	store map[string]EmbeddedText
}

// NewInMemoryCache returns an initialised, empty [InMemoryCache].
func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		store: make(map[string]EmbeddedText),
	}
}

// Get implements [Cache].
func (c *InMemoryCache) Get(_ context.Context, hash string) (*EmbeddedText, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.store[hash]
	if !ok {
		return nil, ErrCacheMiss
	}
	copy := e // value copy; safe to return pointer
	return &copy, nil
}

// Set implements [Cache].
func (c *InMemoryCache) Set(_ context.Context, hash string, e EmbeddedText) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[hash] = e
	return nil
}

// Delete implements [Cache].
func (c *InMemoryCache) Delete(_ context.Context, hash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, hash)
	return nil
}

// Flush implements [Cache].
func (c *InMemoryCache) Flush(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string]EmbeddedText)
	return nil
}

// Len returns the number of entries currently held by the cache.  Useful for
// assertions in tests.
func (c *InMemoryCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
}
