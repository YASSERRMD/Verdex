package treeindex

import (
	"sync"
	"sync/atomic"
	"time"
)

// Stats reports operational counters and timings for an Indexer, useful
// for observability dashboards and for tests asserting on cache behavior.
// A Stats value returned by Indexer.Stats is a point-in-time snapshot; it
// is not itself updated after being returned.
type Stats struct {
	// IndexedPaths is the total number of materialized Path values across
	// every case currently held by the Indexer.
	IndexedPaths int

	// IndexedCases is the number of distinct cases the Indexer currently
	// holds a PathIndex for.
	IndexedCases int

	// CacheHits is the cumulative number of LookupPaths calls served from
	// the LRU cache without recomputation.
	CacheHits int64

	// CacheMisses is the cumulative number of LookupPaths calls that were
	// not found in the LRU cache (and were therefore computed from the
	// PathIndex directly).
	CacheMisses int64

	// LastBuildDuration is how long the most recent RebuildCase (or
	// ReindexOnRevision) call took to complete, across all cases.
	LastBuildDuration time.Duration

	// LastBuildAt is the timestamp the most recent RebuildCase (or
	// ReindexOnRevision) call completed. Zero if no build has run yet.
	LastBuildAt time.Time
}

// statsTracker accumulates the mutable counters backing Indexer.Stats.
// CacheHits/CacheMisses use atomics since LookupPaths may be called
// concurrently; LastBuildDuration/LastBuildAt are guarded by mu since
// RebuildCase calls are expected to be far less frequent and benefit more
// from a simple mutex than from juggling two more atomics.
type statsTracker struct {
	cacheHits   int64
	cacheMisses int64

	mu                sync.Mutex
	lastBuildDuration time.Duration
	lastBuildAt       time.Time
}

// recordHit increments the cache-hit counter.
func (s *statsTracker) recordHit() {
	atomic.AddInt64(&s.cacheHits, 1)
}

// recordMiss increments the cache-miss counter.
func (s *statsTracker) recordMiss() {
	atomic.AddInt64(&s.cacheMisses, 1)
}

// recordBuild records how long a build took and when it completed.
func (s *statsTracker) recordBuild(duration time.Duration, completedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastBuildDuration = duration
	s.lastBuildAt = completedAt
}

// snapshot returns the current hit/miss counts and last-build timing.
func (s *statsTracker) snapshot() (hits, misses int64, duration time.Duration, at time.Time) {
	s.mu.Lock()
	duration, at = s.lastBuildDuration, s.lastBuildAt
	s.mu.Unlock()
	return atomic.LoadInt64(&s.cacheHits), atomic.LoadInt64(&s.cacheMisses), duration, at
}
