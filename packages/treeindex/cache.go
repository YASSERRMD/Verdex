package treeindex

import (
	"container/list"
	"sync"
)

// lookupKey identifies one LookupPaths call's cacheable inputs: the case,
// the root node to look up paths from, and the edge type the caller is
// filtering by (empty means "no edge type filter" — see LookupPaths).
type lookupKey struct {
	caseID     string
	fromNodeID string
	edgeType   string
}

// lruCache is a small, dependency-free, fixed-capacity least-recently-used
// cache mapping a lookupKey to a materialized []Path result. It is built
// on the standard library's container/list rather than a third-party LRU
// package, matching this repository's preference (see e.g. packages/
// embedding's own Cache) for keeping infrastructure packages free of
// dependencies that aren't already pulled in by a real integration (a
// database driver, an SDK) elsewhere in the workspace.
//
// lruCache is safe for concurrent use: all access is guarded by mu.
type lruCache struct {
	mu       sync.Mutex
	capacity int
	items    map[lookupKey]*list.Element
	order    *list.List // front = most recently used, back = least recently used
}

// cacheEntry is the value stored in lruCache.order's list.Element.Value.
type cacheEntry struct {
	key   lookupKey
	paths []Path
}

// newLRUCache constructs an lruCache holding at most capacity entries.
// Returns ErrInvalidCacheCapacity if capacity is not positive.
func newLRUCache(capacity int) (*lruCache, error) {
	if capacity <= 0 {
		return nil, ErrInvalidCacheCapacity
	}
	return &lruCache{
		capacity: capacity,
		items:    make(map[lookupKey]*list.Element, capacity),
		order:    list.New(),
	}, nil
}

// get returns the cached paths for key, if present, promoting key to
// most-recently-used on a hit.
func (c *lruCache) get(key lookupKey) ([]Path, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(elem)
	return elem.Value.(cacheEntry).paths, true
}

// put stores paths under key, evicting the least-recently-used entry if
// the cache is at capacity. An existing entry for key is overwritten and
// promoted to most-recently-used.
func (c *lruCache) put(key lookupKey, paths []Path) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		elem.Value = cacheEntry{key: key, paths: paths}
		c.order.MoveToFront(elem)
		return
	}

	elem := c.order.PushFront(cacheEntry{key: key, paths: paths})
	c.items[key] = elem

	if c.order.Len() > c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(cacheEntry).key)
		}
	}
}

// purgeCase removes every cached entry belonging to caseID. Called by
// RebuildCase so a rebuild can never leave a stale cached result behind
// for a case whose underlying tree just changed.
func (c *lruCache) purgeCase(caseID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, elem := range c.items {
		if key.caseID != caseID {
			continue
		}
		c.order.Remove(elem)
		delete(c.items, key)
	}
}

// len reports the number of entries currently cached.
func (c *lruCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
