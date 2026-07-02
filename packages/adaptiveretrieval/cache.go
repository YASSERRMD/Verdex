package adaptiveretrieval

import (
	"container/list"
	"sync"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// DefaultCacheCapacity is the Cache capacity a zero-valued CacheOptions
// falls back to, matching packages/treeindex's and packages/traversal's
// own DefaultCacheCapacity precedent.
const DefaultCacheCapacity = 256

// cacheKey identifies one cached Subgraph: a case, an AdaptiveQuery's
// cacheable shape, and the tree revision it was built at. Folding the
// revision into the key (rather than purging on every revision bump, as
// treeindex does) mirrors packages/traversal's Cache invalidation model:
// an entry keyed under a superseded revision becomes permanently
// unreachable without an eager walk-and-evict pass.
type cacheKey struct {
	caseID string
	shape  string
}

// cacheEntry is the value stored per cacheKey.
type cacheEntry struct {
	key      cacheKey
	subgraph Subgraph
}

// Cache is a small, fixed-capacity least-recently-used cache of
// Subgraphs, keyed by (case ID, query shape), with revision-aware
// staleness detection layered on top of the LRU eviction policy.
//
// # Staleness model
//
// Unlike packages/traversal's Cache (which folds the revision number
// directly into the cache key so a stale entry is simply unreachable),
// Cache here keeps the entry reachable by (caseID, shape) but validates
// its stored Subgraph.Revision against the case's current revision on
// every Get. This is a deliberate difference: adaptiveretrieval's
// Refresh (staleness.go) needs to distinguish "no entry exists yet" from
// "an entry exists but is stale" so it can report StaleRefreshes
// telemetry separately from ordinary CacheMisses — folding the revision
// into the key would make a stale entry indistinguishable from a
// never-built one (both would simply be a key that has never been
// written under the current revision).
//
// Cache is safe for concurrent use.
type Cache struct {
	mu       sync.Mutex
	capacity int
	items    map[cacheKey]*list.Element
	order    *list.List // front = most recently used, back = least recently used

	revisionsMu sync.RWMutex
	revisions   map[string]int // caseID -> current known revision number
}

// CacheOptions configures a new Cache.
type CacheOptions struct {
	// Capacity is the maximum number of Subgraphs the Cache holds at
	// once. Zero or negative falls back to DefaultCacheCapacity.
	Capacity int
}

// NewCache constructs a Cache. Returns ErrInvalidCacheCapacity only if
// opts.Capacity is explicitly negative; zero falls back to
// DefaultCacheCapacity.
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

// revisionFor returns the current known revision number for caseID (zero
// if never set via SetRevision).
func (c *Cache) revisionFor(caseID string) int {
	c.revisionsMu.RLock()
	defer c.revisionsMu.RUnlock()
	return c.revisions[caseID]
}

// SetRevision records revision as caseID's current tree revision. A
// cached Subgraph built at an earlier revision is reported stale by Get
// on its next read (see lookupResult). A caller reacting to a new
// irac.TreeRevision (e.g. via ReindexOnRevision) should call this after
// the underlying tree changes.
func (c *Cache) SetRevision(caseID string, revision int) {
	c.revisionsMu.Lock()
	defer c.revisionsMu.Unlock()
	c.revisions[caseID] = revision
}

// lookupResult classifies the outcome of a Cache.get call.
type lookupResult int

const (
	// lookupMiss means no entry exists for the key at all.
	lookupMiss lookupResult = iota
	// lookupStale means an entry exists but was built at an older
	// revision than the case's current known revision.
	lookupStale
	// lookupHit means a fresh (non-stale) entry was found.
	lookupHit
)

// get returns the cached Subgraph for (caseID, shape), classifying the
// outcome as a hit, a stale hit, or a miss. A stale hit's Subgraph is
// still returned (a caller may choose to serve it under degraded-mode
// pressure) but Refresh always treats lookupStale as requiring a rebuild.
func (c *Cache) get(caseID, shape string) (Subgraph, lookupResult) {
	key := cacheKey{caseID: caseID, shape: shape}

	c.mu.Lock()
	elem, ok := c.items[key]
	if !ok {
		c.mu.Unlock()
		return Subgraph{}, lookupMiss
	}
	c.order.MoveToFront(elem)
	entry := elem.Value.(cacheEntry)
	c.mu.Unlock()

	if entry.subgraph.Revision < c.revisionFor(caseID) {
		return entry.subgraph, lookupStale
	}
	return entry.subgraph, lookupHit
}

// put stores subgraph under (caseID, shape), evicting the
// least-recently-used entry if the cache is at capacity.
func (c *Cache) put(caseID, shape string, subgraph Subgraph) {
	key := cacheKey{caseID: caseID, shape: shape}

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		elem.Value = cacheEntry{key: key, subgraph: subgraph}
		c.order.MoveToFront(elem)
		return
	}

	elem := c.order.PushFront(cacheEntry{key: key, subgraph: subgraph})
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

// ReindexOnRevision informs cache that caseID has moved to revision's
// RevisionNumber, marking every previously cached Subgraph for that case
// stale on its next Get (see Cache's "Staleness model"). This mirrors
// packages/treeindex.ReindexOnRevision and packages/traversal's
// identically-shaped hook, so a caller wiring a "tree changed" event into
// every downstream index/cache can treat all three uniformly.
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
