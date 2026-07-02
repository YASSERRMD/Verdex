package adaptiveretrieval

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilGraphStore is returned when a function requiring a
	// graph.GraphStore is called with a nil store.
	ErrNilGraphStore = errors.New("adaptiveretrieval: graph store must not be nil")

	// ErrNilTreeIndex is returned when a function requiring a
	// *treeindex.Indexer fallback is called with a nil indexer.
	ErrNilTreeIndex = errors.New("adaptiveretrieval: tree index must not be nil")

	// ErrEmptyCaseID is returned when a query is built or executed with no
	// case id set.
	ErrEmptyCaseID = errors.New("adaptiveretrieval: case id is required")

	// ErrEmptyAnchorNodeID is returned when a query is executed with no
	// anchor node id set — there is nothing to build a subgraph from.
	ErrEmptyAnchorNodeID = errors.New("adaptiveretrieval: anchor node id is required")

	// ErrInvalidBuildBudget is returned when a BuildBudget's fields cannot
	// produce a usable build (e.g. a negative max node/hop count).
	ErrInvalidBuildBudget = errors.New("adaptiveretrieval: build budget is invalid")

	// ErrInvalidCacheCapacity is returned when NewCache is called with a
	// negative capacity.
	ErrInvalidCacheCapacity = errors.New("adaptiveretrieval: cache capacity must be positive")

	// ErrBudgetExceeded is returned internally when an adaptive build
	// cannot complete within its BuildBudget and no fallback is
	// configured. Callers that configure a treeindex fallback (see
	// Builder.WithFallback) will not observe this error directly — the
	// Builder falls back to treeindex.Indexer.LookupPaths instead — but a
	// Builder used without a fallback surfaces it so the caller can decide
	// how to degrade.
	ErrBudgetExceeded = errors.New("adaptiveretrieval: build exceeded budget")

	// ErrNoFallbackAvailable is returned when an adaptive build exceeds
	// its budget (or is skipped by policy) and no treeindex fallback is
	// configured on the Builder.
	ErrNoFallbackAvailable = errors.New("adaptiveretrieval: no fallback available")
)
