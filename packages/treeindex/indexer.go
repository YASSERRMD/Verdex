package treeindex

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// DefaultCacheCapacity is the LRU cache capacity a zero-valued
// IndexerOptions falls back to. It is deliberately small: LookupPaths
// results are typically re-derived cheaply from an already-materialized
// PathIndex (see LookupPaths's doc comment), so the cache exists to serve
// hot, repeated lookups rather than to hold a case's entire path set.
const DefaultCacheCapacity = 256

// IndexerOptions configures a new Indexer.
type IndexerOptions struct {
	// CacheCapacity is the maximum number of LookupPaths results the
	// Indexer's LRU cache holds at once. Zero or negative falls back to
	// DefaultCacheCapacity.
	CacheCapacity int
}

// withDefaults returns a copy of o with zero-valued fields replaced by
// their documented defaults.
func (o IndexerOptions) withDefaults() IndexerOptions {
	out := o
	if out.CacheCapacity <= 0 {
		out.CacheCapacity = DefaultCacheCapacity
	}
	return out
}

// Indexer wraps a graph.GraphStore and maintains, per case, a materialized
// PathIndex of rule-grouped-issue and reasoning-chain Paths (see doc.go).
// It is the package's main entry point: build a case's index with
// RebuildCase (or the tree-revision-driven ReindexOnRevision), then read
// it back with LookupPaths.
//
// Indexer is safe for concurrent use.
type Indexer struct {
	store graph.GraphStore
	cache *lruCache
	stats statsTracker

	mu      sync.RWMutex
	indexes map[string]*PathIndex // caseID -> materialized index
}

// NewIndexer constructs an Indexer over store. Returns ErrNilGraphStore if
// store is nil, or wraps the error from newLRUCache if opts specifies an
// invalid CacheCapacity.
func NewIndexer(store graph.GraphStore, opts IndexerOptions) (*Indexer, error) {
	if store == nil {
		return nil, ErrNilGraphStore
	}
	opts = opts.withDefaults()

	cache, err := newLRUCache(opts.CacheCapacity)
	if err != nil {
		return nil, fmt.Errorf("treeindex: new indexer: %w", err)
	}

	return &Indexer{
		store:   store,
		cache:   cache,
		indexes: make(map[string]*PathIndex),
	}, nil
}

// RebuildCase performs a full rebuild of caseID's PathIndex: it re-walks
// store for every rule-grouped-issue Path and every reasoning-chain Path
// currently derivable from caseID's nodes and edges, replacing whatever
// PathIndex the Indexer previously held for caseID, and purges any cached
// LookupPaths results for caseID so a stale cache entry can never survive
// a rebuild.
//
// # Full rebuild, not incremental, by design
//
// An incremental alternative — patching the PathIndex in place as
// individual OnNodeCreated/OnEdgeCreated events arrive — was considered
// and rejected for v1. A single new edge can change the shape of an
// arbitrarily large slice of a case's reasoning-chain paths (e.g. adding
// an EdgeGoverns edge to a RuleNode that already governs ten issues,
// each with their own multi-hop chains, requires re-deriving all ten);
// getting the incremental patch logic exactly right for every one of the
// four edge types, including the reverse-walked ones, is exactly the kind
// of fiddly, easy-to-get-subtly-wrong logic packages/vectorindex's
// ReindexOnRevision doc comment warns against taking on before it's
// needed. A full rebuild is trivially correct (it is the same code path
// used to build a case's index the first time) and, because
// irac.TreeRevision changes are infrequent relative to LookupPaths reads,
// the cost is paid rarely. See doc/tree-indexing.md for the full
// discussion and ReindexOnRevision (reindex.go) for the tree-revision-
// driven entry point.
//
// Returns ErrEmptyCaseID if caseID is empty.
func (idx *Indexer) RebuildCase(ctx context.Context, caseID string) error {
	if caseID == "" {
		return ErrEmptyCaseID
	}

	start := time.Now()

	groupPaths, err := buildRuleGroupedIssuePaths(ctx, idx.store, caseID)
	if err != nil {
		return fmt.Errorf("treeindex: rebuild case %q: %w", caseID, err)
	}
	chainPaths, err := buildReasoningChainPaths(ctx, idx.store, caseID)
	if err != nil {
		return fmt.Errorf("treeindex: rebuild case %q: %w", caseID, err)
	}

	built := newPathIndex(caseID)
	for _, p := range groupPaths {
		built.add(p)
	}
	for _, p := range chainPaths {
		built.add(p)
	}

	idx.mu.Lock()
	idx.indexes[caseID] = built
	idx.mu.Unlock()

	idx.cache.purgeCase(caseID)
	idx.stats.recordBuild(time.Since(start), time.Now())

	return nil
}

// LookupPaths returns every Path in caseID's materialized index rooted at
// fromNodeID whose first Hop (if any) matches edgeType, optionally
// truncated to maxDepth hops. edgeType may be empty to mean "any edge
// type" (i.e. return every Path rooted at fromNodeID regardless of which
// edge leaves the root).
//
// LookupPaths never calls back into the underlying graph.GraphStore: it
// reads (and, on a cache miss, truncates and caches) whatever PathIndex
// the most recent RebuildCase/ReindexOnRevision call for caseID produced.
// A case that has never been built returns ErrCaseNotIndexed rather than
// silently returning an empty result, so callers can distinguish "this
// case truly has no matching paths" from "nobody has indexed this case
// yet".
//
// # Index-level short-circuiting
//
// maxDepth bounds are applied against the already-materialized Path
// values via Path.Truncate rather than by re-invoking
// graph.GraphStore.Traverse with a smaller MaxDepth: the full-depth Path
// was already computed and cached once by RebuildCase, so a depth-bounded
// request just slices that in-memory structure down, the same
// "materialize once, serve many differently-shaped reads from it" idea
// graph.TraversalQuery.MaxDepth uses at the GraphStore layer, applied one
// layer up at the PathIndex layer instead.
func (idx *Indexer) LookupPaths(_ context.Context, caseID string, fromNodeID string, edgeType irac.EdgeType) ([]Path, error) {
	return idx.LookupPathsWithDepth(context.Background(), caseID, fromNodeID, edgeType, 0)
}

// LookupPathsWithDepth is LookupPaths with an explicit maxDepth bound.
// maxDepth <= 0 means unbounded, mirroring graph.TraversalQuery.MaxDepth's
// convention. See LookupPaths for the full contract.
func (idx *Indexer) LookupPathsWithDepth(_ context.Context, caseID string, fromNodeID string, edgeType irac.EdgeType, maxDepth int) ([]Path, error) {
	if caseID == "" {
		return nil, ErrEmptyCaseID
	}
	if fromNodeID == "" {
		return nil, ErrEmptyNodeID
	}
	if edgeType != "" && !edgeType.IsValid() {
		return nil, ErrInvalidEdgeType
	}

	key := lookupKey{caseID: caseID, fromNodeID: fromNodeID, edgeType: string(edgeType)}
	if cached, ok := idx.cache.get(key); ok {
		idx.stats.recordHit()
		return truncateAll(cached, maxDepth), nil
	}
	idx.stats.recordMiss()

	idx.mu.RLock()
	pathIndex, ok := idx.indexes[caseID]
	idx.mu.RUnlock()
	if !ok {
		return nil, ErrCaseNotIndexed
	}

	candidates := pathIndex.PathsFromRoot(fromNodeID)
	matched := make([]Path, 0, len(candidates))
	for _, p := range candidates {
		if edgeType != "" && (len(p.Hops) == 0 || p.Hops[0].EdgeType != edgeType) {
			continue
		}
		matched = append(matched, p)
	}

	idx.cache.put(key, matched)
	return truncateAll(matched, maxDepth), nil
}

// truncateAll applies Path.Truncate(maxDepth) to every path in paths,
// returning a new slice (the input is never mutated in place, since
// cached entries must remain valid for a future differently-bounded
// lookup).
func truncateAll(paths []Path, maxDepth int) []Path {
	if maxDepth <= 0 {
		out := make([]Path, len(paths))
		copy(out, paths)
		return out
	}
	out := make([]Path, len(paths))
	for i, p := range paths {
		out[i] = p.Truncate(maxDepth)
	}
	return out
}

// Stats returns a point-in-time snapshot of the Indexer's operational
// counters: total indexed paths/cases, cumulative cache hit/miss counts,
// and the duration/timestamp of the most recent build. See Stats.
func (idx *Indexer) Stats() Stats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	totalPaths := 0
	for _, pathIndex := range idx.indexes {
		totalPaths += pathIndex.Len()
	}

	hits, misses, duration, at := idx.stats.snapshot()

	return Stats{
		IndexedPaths:      totalPaths,
		IndexedCases:      len(idx.indexes),
		CacheHits:         hits,
		CacheMisses:       misses,
		LastBuildDuration: duration,
		LastBuildAt:       at,
	}
}
