package adaptiveretrieval

import (
	"sync"
	"time"
)

// BuildTelemetry is a point-in-time snapshot of a Builder's cumulative
// operational counters: how many builds ran, how expensive they were, and
// how often the cache or the treeindex fallback absorbed the request
// instead of a fresh adaptive build.
type BuildTelemetry struct {
	// Builds is the total number of Build calls that attempted (or
	// completed) an adaptive subgraph construction, i.e. cache misses that
	// were not immediately satisfied by a fallback decision made before
	// any traversal ran.
	Builds int

	// CacheHits is the number of Build calls served entirely from a
	// non-stale cached Subgraph, with no traversal performed.
	CacheHits int

	// CacheMisses is the number of Build calls that found no usable
	// cached Subgraph (absent or stale) and had to build (or attempt to
	// build) one.
	CacheMisses int

	// NodesVisited is the cumulative number of distinct nodes visited
	// across every adaptive build (cache hits contribute zero).
	NodesVisited int

	// TotalBuildTime is the cumulative wall-clock time spent inside
	// traversal.Walker.Execute across every adaptive build (cache hits
	// contribute zero).
	TotalBuildTime time.Duration

	// FallbacksTriggered is the number of Build calls that resolved via
	// the treeindex fallback (see Builder.WithFallback) rather than a
	// completed adaptive build — because the budget was exceeded, a
	// build error occurred, or policy chose the fallback outright.
	FallbacksTriggered int

	// StaleRefreshes is the number of Build calls that found a cached
	// Subgraph but discarded it as stale (relative to the case's current
	// irac.TreeRevision) and rebuilt.
	StaleRefreshes int
}

// telemetryRecorder accumulates BuildTelemetry counters under a mutex. A
// Builder holds one internally; it is not exported directly so that
// Builder.Telemetry's returned snapshot can never be mutated by a caller
// out from under the Builder (mirroring treeindex's statsTracker/Stats
// split, cache.go and stats.go).
type telemetryRecorder struct {
	mu    sync.Mutex
	stats BuildTelemetry
}

func (r *telemetryRecorder) recordCacheHit() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.CacheHits++
}

func (r *telemetryRecorder) recordCacheMiss() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.CacheMisses++
}

func (r *telemetryRecorder) recordStaleRefresh() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.StaleRefreshes++
}

func (r *telemetryRecorder) recordBuild(nodesVisited int, elapsed time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.Builds++
	r.stats.NodesVisited += nodesVisited
	r.stats.TotalBuildTime += elapsed
}

func (r *telemetryRecorder) recordFallback() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.FallbacksTriggered++
}

func (r *telemetryRecorder) snapshot() BuildTelemetry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stats
}
