package traversal

import (
	"container/list"
	"context"
	"sync"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// DefaultCacheCapacity is the Cache capacity a zero-valued
// CacheOptions falls back to, matching packages/treeindex's
// DefaultCacheCapacity precedent for this kind of small, hot-path result
// cache.
const DefaultCacheCapacity = 256

// cacheKey is the fully-qualified key a Cache entry is stored under: a
// Query's own cacheKey() plus the case's current tree revision number, so
// a stale entry from before a tree revision bump can never be served as
// a hit (see SetRevision).
type cacheKey struct {
	query    string
	revision int
}

// cacheEntry is the value stored per cacheKey.
type cacheEntry struct {
	key    cacheKey
	result TraversalResult
}

// Cache is a small, dependency-free, fixed-capacity least-recently-used
// cache in front of Walker.Execute, keyed by (Query shape, case tree
// revision). It is built on the standard library's container/list,
// mirroring packages/treeindex's lruCache (cache.go) rather than pulling
// in a third-party LRU dependency.
//
// # Invalidation model
//
// Unlike packages/treeindex, which purges cached entries explicitly on
// every RebuildCase call, Cache tracks a per-case "current revision"
// counter (see SetRevision) and folds that revision into every entry's
// key. A caller bumping the revision via SetRevision does not need to
// walk and evict every previously cached entry for that case: old
// entries simply become permanently unreachable (their key's revision no
// longer matches the case's current revision) and are naturally evicted
// by the LRU policy over time rather than eagerly purged. This trades a
// small amount of transient memory (stale entries lingering until
// evicted) for O(1) invalidation instead of packages/treeindex's O(cached
// entries for that case) purge — a reasonable tradeoff here since a
// traversal.Query's cached result set is typically much smaller than a
// full treeindex PathIndex.
//
// Cache is safe for concurrent use.
type Cache struct {
	mu       sync.Mutex
	capacity int
	items    map[cacheKey]*list.Element
	order    *list.List // front = most recently used, back = least recently used

	revisionsMu sync.RWMutex
	revisions   map[string]int // caseID -> current revision number
}

// CacheOptions configures a new Cache.
type CacheOptions struct {
	// Capacity is the maximum number of TraversalResults the Cache holds
	// at once. Zero or negative falls back to DefaultCacheCapacity.
	Capacity int
}

// NewCache constructs a Cache. Returns ErrInvalidCacheCapacity only if
// opts.Capacity is explicitly negative; zero falls back to
// DefaultCacheCapacity (see CacheOptions).
func NewCache(opts CacheOptions) (*Cache, error) {
	capacity := opts.Capacity
	if capacity == 0 {
		capacity = DefaultCacheCapacity
	}
	if capacity < 0 {
		return nil, ErrInvalidCacheCapacity
	}
	return &Cache{
		capacity:  capacity,
		items:     make(map[cacheKey]*list.Element, capacity),
		order:     list.New(),
		revisions: make(map[string]int),
	}, nil
}

// revisionFor returns the current cached revision number for caseID
// (zero if never set).
func (c *Cache) revisionFor(caseID string) int {
	c.revisionsMu.RLock()
	defer c.revisionsMu.RUnlock()
	return c.revisions[caseID]
}

// SetRevision records revision as caseID's current tree revision. Every
// TraversalResult cached under an older revision for this case becomes
// unreachable (see the Cache doc comment's "Invalidation model" section).
// A caller reacting to a new irac.TreeRevision (e.g. via ReindexOnQuery,
// reindex.go) should call this after the underlying tree changes.
func (c *Cache) SetRevision(caseID string, revision int) {
	c.revisionsMu.Lock()
	defer c.revisionsMu.Unlock()
	c.revisions[caseID] = revision
}

// get returns the cached TraversalResult for query at caseID's current
// revision, if present, promoting it to most-recently-used on a hit.
func (c *Cache) get(caseID string, query Query) (TraversalResult, bool) {
	key := cacheKey{query: query.cacheKey(), revision: c.revisionFor(caseID)}

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return TraversalResult{}, false
	}
	c.order.MoveToFront(elem)
	return elem.Value.(cacheEntry).result, true
}

// put stores result under (caseID, query) at caseID's current revision,
// evicting the least-recently-used entry if the cache is at capacity.
func (c *Cache) put(caseID string, query Query, result TraversalResult) {
	key := cacheKey{query: query.cacheKey(), revision: c.revisionFor(caseID)}

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		elem.Value = cacheEntry{key: key, result: result}
		c.order.MoveToFront(elem)
		return
	}

	elem := c.order.PushFront(cacheEntry{key: key, result: result})
	c.items[key] = elem

	if c.order.Len() > c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(cacheEntry).key)
		}
	}
}

// Len reports the number of entries currently cached, across all cases
// and revisions (including any stale, not-yet-evicted entries).
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// WithCache returns a copy of w that consults cache before executing a
// Query and populates it after, via ExecuteCached. Passing a nil cache
// disables caching (equivalent to the Walker returned by NewWalker).
func (w *Walker) WithCache(cache *Cache) *Walker {
	out := *w
	out.cache = cache
	return &out
}

// ExecuteCached behaves like Execute, but first consults w's Cache (set
// via WithCache) for a result cached under (query.CaseID's current
// revision, query's shape), and populates the cache on a miss. If w has
// no cache configured, ExecuteCached behaves exactly like Execute (every
// call is a "miss" that is never stored).
func (w *Walker) ExecuteCached(ctx context.Context, query Query) (TraversalResult, error) {
	if w.cache == nil {
		return w.Execute(ctx, query)
	}

	if cached, ok := w.cache.get(query.CaseID, query); ok {
		return cached, nil
	}

	result, err := w.Execute(ctx, query)
	if err != nil {
		return TraversalResult{}, err
	}
	w.cache.put(query.CaseID, query, result)
	return result, nil
}

// ReindexOnRevision informs cache that caseID has moved to revision's
// RevisionNumber, invalidating every previously cached
// ExecuteCached result for that case (see Cache's "Invalidation model").
// This mirrors packages/treeindex.ReindexOnRevision and
// packages/vectorindex's identically-shaped hook, so a caller wiring a
// "tree changed" event into every downstream index/cache can treat all
// three uniformly.
//
// Returns ErrEmptyCaseID if revision.CaseID is empty.
func ReindexOnRevision(cache *Cache, revision irac.TreeRevision) error {
	if revision.CaseID == "" {
		return ErrEmptyCaseID
	}
	if cache == nil {
		return nil
	}
	cache.SetRevision(revision.CaseID, revision.RevisionNumber)
	return nil
}
