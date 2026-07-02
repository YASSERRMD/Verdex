package adaptiveretrieval

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/traversal"
	"github.com/YASSERRMD/verdex/packages/treeindex"
)

// BuilderOptions configures a new Builder.
type BuilderOptions struct {
	// Budget bounds every adaptive build's cost. Zero value falls back to
	// DefaultBuildBudget.
	Budget BuildBudget

	// CacheOptions configures the Builder's internal Cache. Zero value
	// falls back to DefaultCacheCapacity.
	CacheOptions CacheOptions
}

// Builder is the package's main entry point: given an AdaptiveQuery, it
// constructs only the subgraph that query needs by walking outward from
// its anchor via a traversal.Walker, under a BuildBudget, reusing a
// cached Subgraph when one is available and not stale, and falling back
// to a treeindex.Indexer when configured and when an adaptive build is
// not worth attempting.
//
// Build one with NewBuilder, optionally attach a treeindex fallback with
// WithFallback, then call Build for each AdaptiveQuery.
//
// Builder is safe for concurrent use.
type Builder struct {
	walker   *traversal.Walker
	cache    *Cache
	budget   BuildBudget
	fallback *treeindex.Indexer
	tel      *telemetryRecorder
}

// NewBuilder constructs a Builder over a traversal.Walker built from
// store. Returns ErrNilGraphStore if store is nil, or wraps the error
// from NewCache if opts specifies an invalid CacheOptions.Capacity.
func NewBuilder(store graph.GraphStore, opts BuilderOptions) (*Builder, error) {
	if store == nil {
		return nil, ErrNilGraphStore
	}

	walker, err := traversal.NewWalker(store)
	if err != nil {
		return nil, fmt.Errorf("adaptiveretrieval: new builder: %w", err)
	}

	cache, err := NewCache(opts.CacheOptions)
	if err != nil {
		return nil, fmt.Errorf("adaptiveretrieval: new builder: %w", err)
	}

	budget := opts.Budget
	if (budget == BuildBudget{}) {
		budget = DefaultBuildBudget()
	}

	return &Builder{
		walker: walker,
		cache:  cache,
		budget: budget,
		tel:    &telemetryRecorder{},
	}, nil
}

// WithFallback returns a copy of b configured to fall back to idx's
// LookupPaths when an adaptive build exceeds its budget or errors. Passing
// a nil idx clears any previously configured fallback (adaptive-only
// mode, where a budget-exceeded build surfaces ErrNoFallbackAvailable
// instead of degrading to treeindex).
func (b *Builder) WithFallback(idx *treeindex.Indexer) *Builder {
	out := *b
	out.fallback = idx
	return &out
}

// Build resolves query into a Subgraph: it first checks b's Cache for a
// non-stale entry matching query's shape, and returns that immediately
// (Subgraph.Source == SourceCached) if found. On a cache miss or a stale
// hit, it resolves AdaptiveDepth for query, walks the resulting hop
// sequence from query.AnchorNodeID under b's BuildBudget (bounded by a
// per-build wall-clock deadline that guarantees Build never blocks past
// BuildBudget.MaxWallClock — see the "Update-latency safeguard" section
// below), and caches the result on success.
//
// # Fallback
//
// If the adaptive build cannot complete within budget (its context
// deadline expires or its visited-node count would exceed
// BuildBudget.MaxNodes) or the underlying traversal.Walker errors, Build
// falls back to b's configured treeindex.Indexer.LookupPaths (see
// WithFallback) rather than surfacing a build error, converting the
// looked-up treeindex.Path values into a Subgraph with Source ==
// SourceFallback. If no fallback is configured, Build returns
// ErrNoFallbackAvailable wrapping the underlying cause.
//
// # Update-latency safeguard
//
// Build derives its traversal context from a deadline set at
// BuildBudget.MaxWallClock from the moment Build is called (see
// buildTracker.withDeadline), independent of ctx's own deadline (if any).
// This means one caller's slow or pathologically expensive query can
// never hold traversal.Walker.Execute open longer than the configured
// budget, regardless of how large the underlying case's tree is — a
// timed-out build degrades to the treeindex fallback (or
// ErrNoFallbackAvailable) rather than blocking the caller, and never
// blocks any other concurrent Build call (Builder holds no build-wide
// lock; only the Cache's own short critical sections are shared).
func (b *Builder) Build(ctx context.Context, query AdaptiveQuery) (Subgraph, error) {
	if err := query.validate(); err != nil {
		return Subgraph{}, err
	}

	hops, depth := AdaptiveDepth(query, b.budget)
	shape := query.shapeKey(depth)

	cached, result := b.cache.get(query.CaseID, shape)
	switch result {
	case lookupHit:
		b.tel.recordCacheHit()
		cached.Source = SourceCached
		return cached, nil
	case lookupStale:
		b.tel.recordStaleRefresh()
		b.tel.recordCacheMiss()
	case lookupMiss:
		b.tel.recordCacheMiss()
	}

	subgraph, err := b.build(ctx, query, hops, depth)
	if err != nil {
		fb, fbErr := b.buildFallback(ctx, query)
		if fbErr != nil {
			return Subgraph{}, fmt.Errorf("adaptiveretrieval: build: %w", err)
		}
		b.tel.recordFallback()
		return fb, nil
	}

	b.cache.put(query.CaseID, shape, subgraph)
	return subgraph, nil
}

// build performs the actual adaptive traversal.Walker walk for query,
// bounded by b.budget. Returns ErrBudgetExceeded if the walk visited more
// nodes than BuildBudget.MaxNodes allows, or if the walk did not finish
// before BuildBudget.MaxWallClock elapsed — checked both via the walk
// context's own deadline (for a graph.GraphStore that observes ctx
// cancellation) and via a direct wall-clock check after Execute returns
// (for one that does not, e.g. InMemoryGraphStore's synchronous lookups).
func (b *Builder) build(ctx context.Context, query AdaptiveQuery, hops []hybridretrieval.ExpansionHop, depth int) (Subgraph, error) {
	tracker := newBuildTracker(b.budget)
	buildCtx, cancel := tracker.withDeadline(ctx)
	defer cancel()

	tq := toTraversalQuery(query, hops, depth)

	start := time.Now()
	result, err := b.walker.Execute(buildCtx, tq)
	elapsed := time.Since(start)

	if err != nil {
		if errors.Is(buildCtx.Err(), context.DeadlineExceeded) {
			return Subgraph{}, fmt.Errorf("adaptiveretrieval: %w: %v", ErrBudgetExceeded, err)
		}
		return Subgraph{}, fmt.Errorf("adaptiveretrieval: build subgraph: %w", err)
	}

	b.tel.recordBuild(result.VisitedCount, elapsed)

	if result.VisitedCount > b.budget.withDefaults().MaxNodes {
		return Subgraph{}, fmt.Errorf("adaptiveretrieval: %w: visited %d nodes, max %d", ErrBudgetExceeded, result.VisitedCount, b.budget.withDefaults().MaxNodes)
	}
	// Even when the underlying graph.GraphStore never observes ctx
	// cancellation mid-walk (e.g. an in-memory store's synchronous,
	// non-blocking lookups), a build that only finished after its
	// wall-clock deadline had already elapsed is still a budget breach:
	// the caller waited longer than BuildBudget.MaxWallClock allows, and
	// should degrade to the fallback the same as an explicit
	// context.DeadlineExceeded would trigger.
	if tracker.exceeded() {
		return Subgraph{}, fmt.Errorf("adaptiveretrieval: %w: wall-clock budget %s elapsed", ErrBudgetExceeded, b.budget.withDefaults().MaxWallClock)
	}

	return Subgraph{
		CaseID:       query.CaseID,
		AnchorNodeID: query.AnchorNodeID,
		Paths:        result.Paths,
		Depth:        depth,
		NodesVisited: result.VisitedCount,
		Truncated:    result.Truncated,
		Source:       SourceBuilt,
	}, nil
}

// buildFallback converts a treeindex.Indexer.LookupPaths result for
// query into a Subgraph. Returns ErrNoFallbackAvailable if b has no
// fallback configured.
func (b *Builder) buildFallback(ctx context.Context, query AdaptiveQuery) (Subgraph, error) {
	if b.fallback == nil {
		return Subgraph{}, ErrNoFallbackAvailable
	}

	paths, err := b.fallback.LookupPathsWithDepth(ctx, query.CaseID, query.AnchorNodeID, query.EdgeType, b.budget.withDefaults().MaxHops)
	if err != nil {
		return Subgraph{}, fmt.Errorf("adaptiveretrieval: treeindex fallback: %w", err)
	}

	return Subgraph{
		CaseID:       query.CaseID,
		AnchorNodeID: query.AnchorNodeID,
		Paths:        treeindexPathsToTraversalPaths(paths),
		Depth:        b.budget.withDefaults().MaxHops,
		NodesVisited: countNodes(paths),
		Source:       SourceFallback,
	}, nil
}

// Telemetry returns a point-in-time snapshot of b's cumulative build-cost
// counters. See BuildTelemetry.
func (b *Builder) Telemetry() BuildTelemetry {
	return b.tel.snapshot()
}

// SetRevision informs b's Cache that caseID has moved to revision's
// RevisionNumber, marking any previously cached Subgraph for that case
// stale on its next Build call. Equivalent to
// ReindexOnRevision(b.Cache(), revision), provided as a Builder method
// for callers that only hold a *Builder.
func (b *Builder) SetRevision(revision irac.TreeRevision) error {
	return ReindexOnRevision(b.cache, revision)
}
