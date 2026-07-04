package perf

import "context"

// ResourceClass names a class of operation whose concurrency this package
// bounds, mirroring OperationName's three benchmarked operations.
type ResourceClass string

const (
	// ClassIngestion bounds concurrent packages/ingestion pipeline runs.
	ClassIngestion ResourceClass = "ingestion"

	// ClassRetrieval bounds concurrent packages/hybridretrieval queries.
	ClassRetrieval ResourceClass = "retrieval"

	// ClassTraversal bounds concurrent packages/traversal walks.
	ClassTraversal ResourceClass = "traversal"
)

// ResourceLimits names the maximum concurrent operations permitted per
// ResourceClass. A deployment under memory or connection pressure can use
// this to cap how many expensive operations (ingestion's chained STT/OCR/
// segmentation calls, a hybrid retriever's vector-plus-graph fan-out, a
// traversal walk's breadth-first frontier expansion) run at once, without
// touching packages/ingestion, packages/hybridretrieval, or
// packages/traversal themselves -- a caller wraps its own call sites with a
// Limiter built from these limits.
type ResourceLimits struct {
	// Ingestion is the max concurrent ingestion pipeline runs.
	Ingestion int

	// Retrieval is the max concurrent hybrid-retrieval queries.
	Retrieval int

	// Traversal is the max concurrent graph-traversal walks.
	Traversal int
}

// DefaultResourceLimits returns conservative default concurrency ceilings:
// ingestion is the heaviest per-call operation (chained STT/OCR/
// normalization/segmentation/classification), so it gets the smallest
// ceiling; traversal is the lightest (bounded in-memory breadth-first
// walk), so it gets the largest.
func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		Ingestion: 4,
		Retrieval: 16,
		Traversal: 32,
	}
}

// asMap projects l into a ResourceClass-keyed map for Limiter's internal
// use.
func (l ResourceLimits) asMap() map[ResourceClass]int {
	return map[ResourceClass]int{
		ClassIngestion: l.Ingestion,
		ClassRetrieval: l.Retrieval,
		ClassTraversal: l.Traversal,
	}
}

// Limiter enforces per-ResourceClass concurrency ceilings using a
// stdlib buffered-channel semaphore per class -- no new dependency is added
// for this (golang.org/x/sync/semaphore is not imported; a channel of
// struct{} with capacity equal to the limit is the standard idiomatic Go
// counting semaphore).
type Limiter struct {
	sems map[ResourceClass]chan struct{}
}

// NewLimiter constructs a Limiter enforcing limits. A ResourceClass with a
// limit <= 0 is treated as unlimited (Acquire/Release for that class are
// no-ops).
func NewLimiter(limits ResourceLimits) *Limiter {
	sems := make(map[ResourceClass]chan struct{}, 3)
	for class, limit := range limits.asMap() {
		if limit > 0 {
			sems[class] = make(chan struct{}, limit)
		}
	}
	return &Limiter{sems: sems}
}

// Acquire blocks until a slot is available for class, or ctx is done
// (whichever comes first). Returns ctx.Err() if ctx is done before a slot
// becomes available. A class with no configured limit (see NewLimiter)
// always acquires immediately. Returns ErrLimiterClosed if class is not one
// of ClassIngestion/ClassRetrieval/ClassTraversal.
func (l *Limiter) Acquire(ctx context.Context, class ResourceClass) error {
	sem, limited := l.limitedSem(class)
	if !limited {
		if _, known := knownClasses[class]; !known {
			return wrapf("Acquire", ErrLimiterClosed)
		}
		return nil
	}

	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees one slot for class, allowing a blocked Acquire (or a
// future one) to proceed. Releasing a class with no configured limit is a
// no-op. Releasing more times than acquired for a limited class will block
// forever on the next Release call attempting to overfill the semaphore's
// buffer; callers must pair every successful Acquire with exactly one
// Release (typically via defer).
func (l *Limiter) Release(class ResourceClass) {
	sem, limited := l.limitedSem(class)
	if !limited {
		return
	}
	<-sem
}

// knownClasses is the exhaustive set of recognized ResourceClass values.
var knownClasses = map[ResourceClass]struct{}{
	ClassIngestion: {},
	ClassRetrieval: {},
	ClassTraversal: {},
}

// limitedSem returns class's semaphore channel and true if class has a
// configured (limit > 0) semaphore, or (nil, false) if class is unlimited
// or unrecognized.
func (l *Limiter) limitedSem(class ResourceClass) (chan struct{}, bool) {
	sem, ok := l.sems[class]
	return sem, ok
}
