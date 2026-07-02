package traversal

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilGraphStore is returned when a function requiring a
	// graph.GraphStore is called with a nil store.
	ErrNilGraphStore = errors.New("traversal: graph store must not be nil")

	// ErrEmptyStartNodeID is returned when a Query is executed with no
	// starting node id set.
	ErrEmptyStartNodeID = errors.New("traversal: query start node id is required")

	// ErrEmptyCaseID is returned when a Query is executed with no case id
	// set.
	ErrEmptyCaseID = errors.New("traversal: query case id is required")

	// ErrNoHops is returned when a Query with zero hop steps is executed.
	// A traversal with no hops has nothing to walk; callers wanting "just
	// this node" should use graph.GraphStore.GetNode directly instead.
	ErrNoHops = errors.New("traversal: query must specify at least one hop")

	// ErrInvalidMaxDepth is returned when a Query's MaxDepth is negative.
	// Zero means "unbounded" (mirroring graph.TraversalQuery's
	// convention); negative values are rejected as a caller error rather
	// than silently treated as zero.
	ErrInvalidMaxDepth = errors.New("traversal: max depth must not be negative")

	// ErrInvalidCacheCapacity is returned when NewCache is called with a
	// non-positive capacity.
	ErrInvalidCacheCapacity = errors.New("traversal: cache capacity must be positive")

	// ErrStartNodeNotFound is returned when a Query's start node cannot be
	// resolved in the underlying graph.GraphStore.
	ErrStartNodeNotFound = errors.New("traversal: start node not found")
)
